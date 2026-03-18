package pluginhost

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"kizuna/internal/memory"
	"kizuna/pkg/pluginapi"
)

func TestRegisterHostHandlersSupportsPluginConfig(t *testing.T) {
	manager, err := NewManager(Config{PluginsDir: filepath.Join(t.TempDir(), "plugins")})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })

	plugin := InstalledPlugin{
		ID:          "demo",
		Name:        "Demo Plugin",
		Version:     "v0.1.0",
		Manifest:    pluginapi.Manifest{ID: "demo", Name: "Demo Plugin", Version: "v0.1.0"},
		Enabled:     true,
		GrantedCaps: []pluginapi.Capability{pluginapi.CapabilityPluginConfigRead, pluginapi.CapabilityPluginConfigWrite},
	}
	if err := manager.registry.Upsert(plugin); err != nil {
		t.Fatalf("upsert plugin: %v", err)
	}

	hostSession, pluginSession, cleanup := newRPCSessionPair(t)
	defer cleanup()

	manager.registerHostHandlers(&managedPlugin{install: plugin, session: hostSession})

	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostConfigSet, pluginapi.ConfigSetRequest{
		Value: json.RawMessage(`{"enabled":true,"threshold":3}`),
	}, nil); err != nil {
		t.Fatalf("config set: %v", err)
	}

	var response pluginapi.ConfigGetResponse
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostConfigGet, struct{}{}, &response); err != nil {
		t.Fatalf("config get: %v", err)
	}
	if !response.Found {
		t.Fatal("expected stored config")
	}
	if string(response.Value) != `{"enabled":true,"threshold":3}` {
		t.Fatalf("unexpected config payload: %s", response.Value)
	}
}

func TestRegisterHostHandlersSupportsMemoryReadWrite(t *testing.T) {
	store := memory.NewStore(func(ctx context.Context, input string) ([]float64, error) {
		return []float64{1, 0, 0}, nil
	})
	t.Cleanup(func() { _ = store.Close() })

	manager, err := NewManager(Config{
		PluginsDir:  filepath.Join(t.TempDir(), "plugins"),
		MemoryStore: store,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })

	plugin := InstalledPlugin{
		ID:       "memory_demo",
		Name:     "Memory Demo",
		Version:  "v0.1.0",
		Manifest: pluginapi.Manifest{ID: "memory_demo", Name: "Memory Demo", Version: "v0.1.0"},
		Enabled:  true,
		GrantedCaps: []pluginapi.Capability{
			pluginapi.CapabilityMemoryRead,
			pluginapi.CapabilityMemoryWrite,
		},
	}
	if err := manager.registry.Upsert(plugin); err != nil {
		t.Fatalf("upsert plugin: %v", err)
	}

	hostSession, pluginSession, cleanup := newRPCSessionPair(t)
	defer cleanup()

	manager.registerHostHandlers(&managedPlugin{install: plugin, session: hostSession})

	appendRequest := pluginapi.MemoryAppendRequest{
		ChannelID: "channel-1",
		Message: pluginapi.MemoryMessage{
			Role:    "user",
			Content: "alpha memory",
			Time:    time.Now().UTC().Format(time.RFC3339),
			Author: pluginapi.UserInfo{
				ID:          "user-1",
				Username:    "user-1",
				DisplayName: "user-1",
			},
		},
	}
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemoryAppend, appendRequest, nil); err != nil {
		t.Fatalf("memory append: %v", err)
	}
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemoryAppend, pluginapi.MemoryAppendRequest{
		ChannelID: "channel-1",
		Message: pluginapi.MemoryMessage{
			Role:    "assistant",
			Content: "assistant memory",
			Time:    time.Now().UTC().Format(time.RFC3339),
			Author: pluginapi.UserInfo{
				ID:          "bot-1",
				Username:    "bot-1",
				DisplayName: "bot-1",
			},
		},
	}, nil); err != nil {
		t.Fatalf("second memory append: %v", err)
	}
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemorySetSummary, pluginapi.MemorySetSummaryRequest{
		ChannelID: "channel-1",
		Summary:   "summary text",
	}, nil); err != nil {
		t.Fatalf("memory set summary: %v", err)
	}

	var snapshot pluginapi.MemoryGetResponse
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemoryGet, pluginapi.MemoryGetRequest{
		ChannelID: "channel-1",
	}, &snapshot); err != nil {
		t.Fatalf("memory get: %v", err)
	}
	if snapshot.Summary != "summary text" {
		t.Fatalf("unexpected summary: %q", snapshot.Summary)
	}
	if len(snapshot.Messages) != 2 || snapshot.Messages[0].Content != "alpha memory" || snapshot.Messages[1].Content != "assistant memory" {
		t.Fatalf("unexpected memory snapshot: %#v", snapshot.Messages)
	}

	search := waitForMemorySearchResults(t, pluginSession, pluginapi.MemorySearchRequest{
		ChannelID: "channel-1",
		Query:     "alpha memory",
		TopN:      1,
	})
	if len(search.Results) != 1 || search.Results[0].Content != "alpha memory" {
		t.Fatalf("unexpected memory search results: %#v", search.Results)
	}

	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemoryTrim, pluginapi.MemoryTrimRequest{
		ChannelID: "channel-1",
		Keep:      1,
	}, nil); err != nil {
		t.Fatalf("memory trim: %v", err)
	}
	if err := pluginSession.Call(context.Background(), pluginapi.MethodHostMemoryGet, pluginapi.MemoryGetRequest{
		ChannelID: "channel-1",
	}, &snapshot); err != nil {
		t.Fatalf("memory get after trim: %v", err)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "assistant memory" {
		t.Fatalf("expected trimmed memory messages, got %#v", snapshot.Messages)
	}
}

func waitForMemorySearchResults(t *testing.T, session *pluginapi.RPCSession, request pluginapi.MemorySearchRequest) pluginapi.MemorySearchResponse {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		var response pluginapi.MemorySearchResponse
		if err := session.Call(context.Background(), pluginapi.MethodHostMemorySearch, request, &response); err != nil {
			t.Fatalf("memory search: %v", err)
		}
		if len(response.Results) > 0 {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for indexed memory search results")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func newRPCSessionPair(t *testing.T) (*pluginapi.RPCSession, *pluginapi.RPCSession, func()) {
	t.Helper()

	hostReader, pluginWriter := io.Pipe()
	pluginReader, hostWriter := io.Pipe()

	hostSession := pluginapi.NewRPCSession(hostReader, hostWriter)
	pluginSession := pluginapi.NewRPCSession(pluginReader, pluginWriter)

	cleanup := func() {
		hostSession.CloseWithError(io.EOF)
		pluginSession.CloseWithError(io.EOF)
		_ = hostReader.Close()
		_ = pluginReader.Close()
		_ = hostWriter.Close()
		_ = pluginWriter.Close()
	}
	return hostSession, pluginSession, cleanup
}
