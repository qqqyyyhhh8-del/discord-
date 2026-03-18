package bot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"discordbot/internal/config"
	"discordbot/internal/memory"
	"discordbot/internal/openai"
	"discordbot/internal/runtimecfg"

	"github.com/bwmarrin/discordgo"
)

func TestHandleSetupCommandAddsCurrentServerAndPersistsAllowlist(t *testing.T) {
	runtimeStore, configPath := newTestStoreForBot(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "allowed_guild_ids": [],
  "allowed_channel_ids": [],
  "allowed_thread_ids": []
}`)

	handler := newSetupTestHandler(runtimeStore)

	response, err := handler.handleSetupCommand("admin-1", speechLocation{
		GuildID: "100",
	}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "server",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup server: %v", err)
	}
	if !strings.Contains(response, "已放行当前服务器: `100`") {
		t.Fatalf("unexpected setup response: %q", response)
	}
	if !runtimeStore.AllowsSpeech("100", "", "") {
		t.Fatal("expected configured guild id to permit speech")
	}

	reopened, err := openRuntimeStore(configPath)
	if err != nil {
		t.Fatalf("reopen runtime store: %v", err)
	}
	if !reopened.AllowsSpeech("100", "", "") {
		t.Fatal("expected configured guild id to persist")
	}
}

func TestHandleSetupCommandAddsCurrentChannelAndThread(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "allowed_guild_ids": [],
  "allowed_channel_ids": [],
  "allowed_thread_ids": []
}`)

	handler := newPanelTestHandler(runtimeStore)

	channelResponse, err := handler.handleSetupCommand("admin-1", speechLocation{
		GuildID:   "guild-1",
		ChannelID: "channel-2",
		ThreadID:  "thread-3",
	}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "channel",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup channel: %v", err)
	}
	if !strings.Contains(channelResponse, "已放行当前频道: `channel-2`") {
		t.Fatalf("unexpected setup channel response: %q", channelResponse)
	}
	if !runtimeStore.AllowsSpeech("", "channel-2", "") {
		t.Fatal("expected configured channel id to permit speech")
	}

	threadResponse, err := handler.handleSetupCommand("admin-1", speechLocation{
		GuildID:   "guild-1",
		ChannelID: "channel-2",
		ThreadID:  "thread-3",
	}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "thread",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup thread: %v", err)
	}
	if !strings.Contains(threadResponse, "已放行当前子区: `thread-3`") {
		t.Fatalf("unexpected setup thread response: %q", threadResponse)
	}
	if !runtimeStore.AllowsSpeech("", "", "thread-3") {
		t.Fatal("expected configured thread id to permit speech")
	}
}

func TestHandleSetupCommandThreadRejectsNonThreadLocation(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.handleSetupCommand("admin-1", speechLocation{
		GuildID:   "guild-1",
		ChannelID: "channel-1",
	}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "thread",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup non-thread: %v", err)
	}
	if !strings.Contains(response, "当前不在子区/线程/帖子内") {
		t.Fatalf("unexpected non-thread response: %q", response)
	}
}

func TestHandleSetupCommandShowSummarizesCurrentAllowlist(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "allowed_guild_ids": ["100"],
  "allowed_channel_ids": ["200"],
  "allowed_thread_ids": ["300"]
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.handleSetupCommand("admin-1", speechLocation{}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "show",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup show: %v", err)
	}
	if !strings.Contains(response, "当前状态：仅在以下白名单位置发言") {
		t.Fatalf("unexpected setup summary: %q", response)
	}
	if !strings.Contains(response, "100") || !strings.Contains(response, "200") || !strings.Contains(response, "300") {
		t.Fatalf("expected setup summary to contain ids, got %q", response)
	}
}

func TestHandleSetupCommandRejectsNonAdmin(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.handleSetupCommand("user-1", speechLocation{}, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "show",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		},
	})
	if err != nil {
		t.Fatalf("handle setup non-admin: %v", err)
	}
	if response != permissionDenied() {
		t.Fatalf("unexpected non-admin response: %q", response)
	}
}

func newSetupTestHandler(runtimeStore *runtimecfg.Store) *Handler {
	return NewHandler(
		config.BotConfig{SystemPrompt: "基础 system prompt"},
		func(ctx context.Context, messages []openai.ChatMessage) (string, error) {
			return "ok", nil
		},
		func(ctx context.Context, input string) ([]float64, error) {
			return []float64{1, 2, 3}, nil
		},
		nil,
		memory.NewStore(func(ctx context.Context, input string) ([]float64, error) {
			return []float64{1, 2, 3}, nil
		}),
		runtimeStore,
	)
}

func newTestStoreForBot(t *testing.T, content string) (*runtimecfg.Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bot_config.json")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	store, err := runtimecfg.Open(path)
	if err != nil {
		t.Fatalf("open runtime config: %v", err)
	}
	return store, path
}

func openRuntimeStore(path string) (*runtimecfg.Store, error) {
	return runtimecfg.Open(path)
}
