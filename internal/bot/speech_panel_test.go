package bot

import (
	"context"
	"strings"
	"testing"

	"discordbot/internal/config"
	"discordbot/internal/memory"
	"discordbot/internal/openai"
	"discordbot/internal/runtimecfg"

	"github.com/bwmarrin/discordgo"
)

func TestSpeechPanelCommandResponseForAdmin(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "allowlist",
  "allowed_guild_ids": ["111"],
  "allowed_channel_ids": ["222"],
  "allowed_thread_ids": ["333"]
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.SpeechPanelCommandResponse("admin-1")
	if err != nil {
		t.Fatalf("speech panel response: %v", err)
	}
	if response.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Fatalf("unexpected response type: %v", response.Type)
	}
	if response.Data == nil || response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("expected ephemeral response, got %#v", response.Data)
	}
	if len(response.Data.Embeds) != 1 || response.Data.Embeds[0].Title != "Speech Scope Control" {
		t.Fatalf("unexpected embeds: %#v", response.Data.Embeds)
	}
	if len(response.Data.Components) != 2 {
		t.Fatalf("expected two component rows, got %#v", response.Data.Components)
	}
}

func TestSpeechPanelCommandResponseRejectsNonAdmin(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.SpeechPanelCommandResponse("user-1")
	if err != nil {
		t.Fatalf("speech panel deny response: %v", err)
	}
	if response.Data == nil || response.Data.Content != permissionDenied() {
		t.Fatalf("unexpected deny response: %#v", response.Data)
	}
}

func TestSpeechComponentResponseModeSwitch(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "all"
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.SpeechComponentResponse("admin-1", discordgo.MessageComponentInteractionData{
		CustomID: speechActionModeNone,
	})
	if err != nil {
		t.Fatalf("speech mode switch: %v", err)
	}
	if response.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("unexpected response type: %v", response.Type)
	}
	mode, _, _, _ := runtimeStore.SpeechScope()
	if mode != runtimecfg.SpeechModeNone {
		t.Fatalf("expected speech mode none, got %q", mode)
	}
}

func TestSpeechModalResponseUpdatesGuildIDsAndForcesAllowlist(t *testing.T) {
	runtimeStore := newTestRuntimeStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "all"
}`)

	handler := newPanelTestHandler(runtimeStore)
	response, err := handler.SpeechModalResponse("admin-1", discordgo.ModalSubmitInteractionData{
		CustomID: speechModalGuilds,
		Components: []discordgo.MessageComponent{
			&discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: speechModalFieldIDs, Value: "123\n456"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("speech modal response: %v", err)
	}
	if response.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Fatalf("unexpected response type: %v", response.Type)
	}
	mode, guilds, _, _ := runtimeStore.SpeechScope()
	if mode != runtimecfg.SpeechModeAllowlist {
		t.Fatalf("expected allowlist mode, got %q", mode)
	}
	if len(guilds) != 2 || guilds[0] != "123" || guilds[1] != "456" {
		t.Fatalf("unexpected guild list: %#v", guilds)
	}
	if !strings.Contains(response.Data.Content, "当前模式已切到白名单") {
		t.Fatalf("unexpected response content: %q", response.Data.Content)
	}
}

func TestParseDiscordIDsRejectsInvalidInput(t *testing.T) {
	if _, err := parseDiscordIDs("123 abc"); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func newPanelTestHandler(runtimeStore *runtimecfg.Store) *Handler {
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
