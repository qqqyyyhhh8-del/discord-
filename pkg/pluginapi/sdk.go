package pluginapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const (
	MethodPluginInitialize            = "plugin.initialize"
	MethodPluginShutdown              = "plugin.shutdown"
	MethodPluginOnSlashCommand        = "plugin.on_slash_command"
	MethodPluginOnComponent           = "plugin.on_component"
	MethodPluginOnModal               = "plugin.on_modal"
	MethodPluginOnMessage             = "plugin.on_message"
	MethodPluginOnPromptBuild         = "plugin.on_prompt_build"
	MethodPluginOnResponsePostprocess = "plugin.on_response_postprocess"
	MethodPluginOnInterval            = "plugin.on_interval"

	MethodHostStorageGet      = "host.storage.get"
	MethodHostStorageSet      = "host.storage.set"
	MethodHostListGuildEmojis = "host.discord.list_guild_emojis"
	MethodHostChat            = "host.chat"
	MethodHostEmbed           = "host.embed"
	MethodHostRerank          = "host.rerank"
	MethodHostSendMessage     = "host.send_message"
	MethodHostReplyToMessage  = "host.reply_to_message"
	MethodHostSpeechAllowed   = "host.speech.allowed"
	MethodHostGetWorldBook    = "host.worldbook.get"
	MethodHostUpsertWorldBook = "host.worldbook.upsert"
	MethodHostDeleteWorldBook = "host.worldbook.delete"
	MethodHostLog             = "host.log"
)

type Plugin interface {
	Initialize(ctx context.Context, host *HostClient, req InitializeRequest) error
	Shutdown(ctx context.Context, host *HostClient, req ShutdownRequest) error
	OnSlashCommand(ctx context.Context, host *HostClient, req SlashCommandRequest) (*InteractionResponse, error)
	OnComponent(ctx context.Context, host *HostClient, req ComponentRequest) (*InteractionResponse, error)
	OnModal(ctx context.Context, host *HostClient, req ModalRequest) (*InteractionResponse, error)
	OnMessage(ctx context.Context, host *HostClient, req MessageEvent) error
	OnPromptBuild(ctx context.Context, host *HostClient, req PromptBuildRequest) (*PromptBuildResponse, error)
	OnResponsePostprocess(ctx context.Context, host *HostClient, req ResponsePostprocessRequest) (*ResponsePostprocessResponse, error)
	OnInterval(ctx context.Context, host *HostClient, req IntervalRequest) error
}

type BasePlugin struct{}

func (BasePlugin) Initialize(context.Context, *HostClient, InitializeRequest) error {
	return nil
}

func (BasePlugin) Shutdown(context.Context, *HostClient, ShutdownRequest) error {
	return nil
}

func (BasePlugin) OnSlashCommand(context.Context, *HostClient, SlashCommandRequest) (*InteractionResponse, error) {
	return nil, nil
}

func (BasePlugin) OnComponent(context.Context, *HostClient, ComponentRequest) (*InteractionResponse, error) {
	return nil, nil
}

func (BasePlugin) OnModal(context.Context, *HostClient, ModalRequest) (*InteractionResponse, error) {
	return nil, nil
}

func (BasePlugin) OnMessage(context.Context, *HostClient, MessageEvent) error {
	return nil
}

func (BasePlugin) OnPromptBuild(context.Context, *HostClient, PromptBuildRequest) (*PromptBuildResponse, error) {
	return nil, nil
}

func (BasePlugin) OnResponsePostprocess(context.Context, *HostClient, ResponsePostprocessRequest) (*ResponsePostprocessResponse, error) {
	return nil, nil
}

func (BasePlugin) OnInterval(context.Context, *HostClient, IntervalRequest) error {
	return nil
}

type HostClient struct {
	session *RPCSession
}

func (c *HostClient) StorageGet(ctx context.Context, key string, target any) (bool, error) {
	var response StorageGetResponse
	if err := c.session.Call(ctx, MethodHostStorageGet, StorageGetRequest{Key: key}, &response); err != nil {
		return false, err
	}
	if !response.Found || len(response.Value) == 0 || target == nil {
		return response.Found, nil
	}
	return true, json.Unmarshal(response.Value, target)
}

func (c *HostClient) StorageSet(ctx context.Context, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.session.Call(ctx, MethodHostStorageSet, StorageSetRequest{Key: key, Value: payload}, nil)
}

func (c *HostClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	var response ChatResponse
	if err := c.session.Call(ctx, MethodHostChat, ChatRequest{Messages: messages}, &response); err != nil {
		return "", err
	}
	return response.Content, nil
}

func (c *HostClient) ListGuildEmojis(ctx context.Context, guildID string) ([]GuildEmoji, error) {
	var response ListGuildEmojisResponse
	if err := c.session.Call(ctx, MethodHostListGuildEmojis, ListGuildEmojisRequest{GuildID: guildID}, &response); err != nil {
		return nil, err
	}
	return response.Emojis, nil
}

func (c *HostClient) Embed(ctx context.Context, input string) ([]float64, error) {
	var response EmbedResponse
	if err := c.session.Call(ctx, MethodHostEmbed, EmbedRequest{Input: input}, &response); err != nil {
		return nil, err
	}
	return response.Vector, nil
}

func (c *HostClient) Rerank(ctx context.Context, query string, documents []string, topN int) ([]string, error) {
	var response RerankResponse
	if err := c.session.Call(ctx, MethodHostRerank, RerankRequest{
		Query:     query,
		Documents: documents,
		TopN:      topN,
	}, &response); err != nil {
		return nil, err
	}
	return response.Documents, nil
}

func (c *HostClient) SendMessage(ctx context.Context, request SendMessageRequest) error {
	return c.session.Call(ctx, MethodHostSendMessage, request, nil)
}

func (c *HostClient) ReplyToMessage(ctx context.Context, message MessageContext) error {
	return c.session.Call(ctx, MethodHostReplyToMessage, ReplyToMessageRequest{Message: message}, nil)
}

func (c *HostClient) SpeechAllowed(ctx context.Context, guildID, channelID, threadID string) (bool, error) {
	var response SpeechAllowedResponse
	if err := c.session.Call(ctx, MethodHostSpeechAllowed, SpeechAllowedRequest{
		GuildID:   guildID,
		ChannelID: channelID,
		ThreadID:  threadID,
	}, &response); err != nil {
		return false, err
	}
	return response.Allowed, nil
}

func (c *HostClient) GetWorldBook(ctx context.Context, key string) (*GetWorldBookResponse, error) {
	var response GetWorldBookResponse
	if err := c.session.Call(ctx, MethodHostGetWorldBook, GetWorldBookRequest{Key: key}, &response); err != nil {
		return nil, err
	}
	if !response.Found {
		return nil, nil
	}
	return &response, nil
}

func (c *HostClient) UpsertWorldBook(ctx context.Context, request UpsertWorldBookRequest) error {
	return c.session.Call(ctx, MethodHostUpsertWorldBook, request, nil)
}

func (c *HostClient) DeleteWorldBook(ctx context.Context, key string) error {
	return c.session.Call(ctx, MethodHostDeleteWorldBook, DeleteWorldBookRequest{Key: key}, nil)
}

func (c *HostClient) Log(ctx context.Context, level, message string) error {
	return c.session.Call(ctx, MethodHostLog, LogRequest{Level: level, Message: message}, nil)
}

func Serve(manifest Manifest, plugin Plugin) error {
	manifest = manifest.Normalize()
	if err := manifest.Validate(); err != nil {
		return err
	}
	if plugin == nil {
		plugin = BasePlugin{}
	}

	session := NewRPCSession(os.Stdin, os.Stdout)
	host := &HostClient{session: session}

	session.RegisterHandler(MethodPluginInitialize, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request InitializeRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return struct{}{}, plugin.Initialize(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginShutdown, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request ShutdownRequest
		if len(params) > 0 {
			if err := json.Unmarshal(params, &request); err != nil {
				return nil, err
			}
		}
		return struct{}{}, plugin.Shutdown(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnSlashCommand, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request SlashCommandRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return plugin.OnSlashCommand(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnComponent, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request ComponentRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return plugin.OnComponent(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnModal, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request ModalRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return plugin.OnModal(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnMessage, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request MessageEvent
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return struct{}{}, plugin.OnMessage(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnPromptBuild, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request PromptBuildRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return plugin.OnPromptBuild(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnResponsePostprocess, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request ResponsePostprocessRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return plugin.OnResponsePostprocess(ctx, host, request)
	})
	session.RegisterHandler(MethodPluginOnInterval, func(ctx context.Context, params json.RawMessage) (any, error) {
		var request IntervalRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return struct{}{}, plugin.OnInterval(ctx, host, request)
	})

	<-session.closed
	if errors.Is(session.closeErr, io.EOF) {
		return nil
	}
	return session.closeErr
}
