package pluginapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

const jsonRPCVersion = "2.0"

const (
	rpcCodeMethodNotFound = -32601
	rpcCodeInternalError  = -32603
)

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type rpcResult struct {
	result json.RawMessage
	err    *RPCError
}

type RPCHandler func(ctx context.Context, params json.RawMessage) (any, error)

type RPCSession struct {
	decoder *json.Decoder
	encoder *json.Encoder

	writeMu sync.Mutex

	handlerMu sync.RWMutex
	handlers  map[string]RPCHandler

	pendingMu sync.Mutex
	pending   map[string]chan rpcResult
	nextID    atomic.Int64

	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
}

func NewRPCSession(reader io.Reader, writer io.Writer) *RPCSession {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	session := &RPCSession{
		decoder:  json.NewDecoder(reader),
		encoder:  encoder,
		handlers: map[string]RPCHandler{},
		pending:  map[string]chan rpcResult{},
		closed:   make(chan struct{}),
	}
	go session.readLoop()
	return session
}

func (s *RPCSession) RegisterHandler(method string, handler RPCHandler) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	method = stringsTrim(method)
	if method == "" || handler == nil {
		return
	}
	s.handlers[method] = handler
}

func (s *RPCSession) Call(ctx context.Context, method string, params any, result any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	method = stringsTrim(method)
	if method == "" {
		return errors.New("rpc method is required")
	}

	idValue, err := json.Marshal(s.nextID.Add(1))
	if err != nil {
		return err
	}
	idKey := normalizeRPCID(idValue)
	if idKey == "" {
		return errors.New("failed to allocate rpc id")
	}

	paramsValue, err := marshalRPCValue(params)
	if err != nil {
		return err
	}

	responseCh := make(chan rpcResult, 1)
	s.pendingMu.Lock()
	s.pending[idKey] = responseCh
	s.pendingMu.Unlock()

	if err := s.writeEnvelope(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		ID:      idValue,
		Method:  method,
		Params:  paramsValue,
	}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, idKey)
		s.pendingMu.Unlock()
		return err
	}

	select {
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, idKey)
		s.pendingMu.Unlock()
		return ctx.Err()
	case <-s.closed:
		return s.closeErr
	case response := <-responseCh:
		if response.err != nil {
			return response.err
		}
		if result == nil || len(response.result) == 0 || bytes.Equal(bytes.TrimSpace(response.result), []byte("null")) {
			return nil
		}
		return json.Unmarshal(response.result, result)
	}
}

func (s *RPCSession) Notify(method string, params any) error {
	method = stringsTrim(method)
	if method == "" {
		return errors.New("rpc method is required")
	}
	paramsValue, err := marshalRPCValue(params)
	if err != nil {
		return err
	}
	return s.writeEnvelope(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  paramsValue,
	})
}

func (s *RPCSession) CloseWithError(err error) {
	s.closeOnce.Do(func() {
		s.closeErr = err
		close(s.closed)
		if err == nil {
			err = io.EOF
		}
		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()
		for id, ch := range s.pending {
			delete(s.pending, id)
			ch <- rpcResult{err: &RPCError{Code: rpcCodeInternalError, Message: err.Error()}}
			close(ch)
		}
	})
}

func (s *RPCSession) readLoop() {
	for {
		var envelope rpcEnvelope
		if err := s.decoder.Decode(&envelope); err != nil {
			s.CloseWithError(err)
			return
		}
		if stringsTrim(envelope.Method) != "" {
			go s.handleRequest(envelope)
			continue
		}
		s.handleResponse(envelope)
	}
}

func (s *RPCSession) handleRequest(envelope rpcEnvelope) {
	method := stringsTrim(envelope.Method)
	s.handlerMu.RLock()
	handler := s.handlers[method]
	s.handlerMu.RUnlock()

	if handler == nil {
		if len(envelope.ID) > 0 {
			_ = s.writeEnvelope(rpcEnvelope{
				JSONRPC: jsonRPCVersion,
				ID:      envelope.ID,
				Error: &RPCError{
					Code:    rpcCodeMethodNotFound,
					Message: "method not found",
				},
			})
		}
		return
	}

	result, err := handler(context.Background(), envelope.Params)
	if len(envelope.ID) == 0 {
		return
	}
	if err != nil {
		_ = s.writeEnvelope(rpcEnvelope{
			JSONRPC: jsonRPCVersion,
			ID:      envelope.ID,
			Error: &RPCError{
				Code:    rpcCodeInternalError,
				Message: err.Error(),
			},
		})
		return
	}
	resultValue, marshalErr := marshalRPCValue(result)
	if marshalErr != nil {
		_ = s.writeEnvelope(rpcEnvelope{
			JSONRPC: jsonRPCVersion,
			ID:      envelope.ID,
			Error: &RPCError{
				Code:    rpcCodeInternalError,
				Message: marshalErr.Error(),
			},
		})
		return
	}
	_ = s.writeEnvelope(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		ID:      envelope.ID,
		Result:  resultValue,
	})
}

func (s *RPCSession) handleResponse(envelope rpcEnvelope) {
	idKey := normalizeRPCID(envelope.ID)
	if idKey == "" {
		return
	}
	s.pendingMu.Lock()
	ch, ok := s.pending[idKey]
	if ok {
		delete(s.pending, idKey)
	}
	s.pendingMu.Unlock()
	if !ok {
		return
	}
	ch <- rpcResult{
		result: envelope.Result,
		err:    envelope.Error,
	}
	close(ch)
}

func (s *RPCSession) writeEnvelope(envelope rpcEnvelope) error {
	select {
	case <-s.closed:
		if s.closeErr != nil {
			return s.closeErr
		}
		return io.EOF
	default:
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.encoder.Encode(envelope)
}

func marshalRPCValue(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage("null"), nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		if len(raw) == 0 {
			return json.RawMessage("null"), nil
		}
		return raw, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return json.RawMessage("null"), nil
	}
	return payload, nil
}

func normalizeRPCID(value json.RawMessage) string {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return ""
	}
	return string(value)
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
