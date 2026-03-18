package runtimecfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorePersistsAdminsPersonasAndPrompts(t *testing.T) {
	store, path := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {
    "default": "你是默认人设。"
  },
  "active_persona": "default",
  "system_prompt": "初始 system prompt"
}`)

	if !store.IsSuperAdmin("owner-1") {
		t.Fatal("expected owner-1 to be super admin")
	}
	if !store.IsAdmin("admin-1") {
		t.Fatal("expected admin-1 to be admin")
	}

	if err := store.GrantAdmin("admin-2"); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
	if err := store.UpsertPersona("maid", "你是女仆人设。"); err != nil {
		t.Fatalf("upsert persona: %v", err)
	}
	if err := store.SetActivePersona("maid"); err != nil {
		t.Fatalf("set active persona: %v", err)
	}
	if err := store.SetSystemPrompt("新的 system prompt"); err != nil {
		t.Fatalf("set system prompt: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	systemPrompt, personaPrompt := reopened.ComposePrompts("基础 prompt")
	if systemPrompt != "基础 prompt\n\n新的 system prompt" {
		t.Fatalf("unexpected system prompt: %q", systemPrompt)
	}
	if personaPrompt != "你是女仆人设。" {
		t.Fatalf("unexpected persona prompt: %q", personaPrompt)
	}
	if !reopened.IsAdmin("admin-2") {
		t.Fatal("expected granted admin to persist")
	}
}

func TestRevokeAdminDoesNotAllowSuperAdminRemoval(t *testing.T) {
	store, _ := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": ["admin-1"],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	if err := store.RevokeAdmin("owner-1"); err == nil {
		t.Fatal("expected error when revoking super admin")
	}
}

func TestOpenAcceptsNumericDiscordIDs(t *testing.T) {
	store, path := newTestStore(t, `{
  "super_admin_ids": [1403633737668886558],
  "admin_ids": [2403633737668886558, "3403633737668886558"],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	if !store.IsSuperAdmin("1403633737668886558") {
		t.Fatal("expected numeric super admin id to be loaded as string")
	}
	if !store.IsAdmin("2403633737668886558") {
		t.Fatal("expected numeric admin id to be loaded as string")
	}
	if !store.IsAdmin("3403633737668886558") {
		t.Fatal("expected string admin id to be loaded")
	}

	if err := store.GrantAdmin("4403633737668886558"); err != nil {
		t.Fatalf("grant admin: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if !reopened.IsAdmin("4403633737668886558") {
		t.Fatal("expected granted admin to persist")
	}
}

func TestDeletePersonaClearsActivePersona(t *testing.T) {
	store, path := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {
    "maid": "你是女仆。",
    "butler": "你是管家。"
  },
  "active_persona": "maid",
  "system_prompt": ""
}`)

	if err := store.DeletePersona("maid"); err != nil {
		t.Fatalf("delete persona: %v", err)
	}
	if active := store.ActivePersonaName(); active != "" {
		t.Fatalf("expected active persona to be cleared, got %q", active)
	}
	if _, ok := store.PersonaPrompt("maid"); ok {
		t.Fatal("expected deleted persona to be removed")
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if active := reopened.ActivePersonaName(); active != "" {
		t.Fatalf("expected reopened active persona to be cleared, got %q", active)
	}
	if _, ok := reopened.PersonaPrompt("maid"); ok {
		t.Fatal("expected deleted persona to stay removed after reopen")
	}
}

func TestSpeechScopeModesAndAllowlist(t *testing.T) {
	store, path := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "allowlist",
  "allowed_guild_ids": ["guild-1"],
  "allowed_channel_ids": ["channel-1"],
  "allowed_thread_ids": ["thread-1"]
}`)

	mode, guilds, channels, threads := store.SpeechScope()
	if mode != SpeechModeAllowlist {
		t.Fatalf("expected allowlist mode, got %q", mode)
	}
	if len(guilds) != 1 || guilds[0] != "guild-1" {
		t.Fatalf("unexpected guild scope: %#v", guilds)
	}
	if len(channels) != 1 || channels[0] != "channel-1" {
		t.Fatalf("unexpected channel scope: %#v", channels)
	}
	if len(threads) != 1 || threads[0] != "thread-1" {
		t.Fatalf("unexpected thread scope: %#v", threads)
	}

	if !store.AllowsSpeech("guild-1", "x", "y") {
		t.Fatal("expected allowed guild to permit speech")
	}
	if !store.AllowsSpeech("guild-2", "channel-1", "") {
		t.Fatal("expected allowed channel to permit speech")
	}
	if !store.AllowsSpeech("guild-2", "channel-2", "thread-1") {
		t.Fatal("expected allowed thread to permit speech")
	}
	if store.AllowsSpeech("guild-2", "channel-2", "thread-2") {
		t.Fatal("expected non-allowlisted location to be blocked")
	}

	if err := store.SetSpeechMode(SpeechModeNone); err != nil {
		t.Fatalf("set speech mode none: %v", err)
	}
	if store.AllowsSpeech("guild-1", "channel-1", "thread-1") {
		t.Fatal("expected none mode to block speech")
	}

	if err := store.SetAllowedGuildIDs([]string{"guild-9"}); err != nil {
		t.Fatalf("set allowed guilds: %v", err)
	}
	if err := store.SetAllowedChannelIDs([]string{"channel-9"}); err != nil {
		t.Fatalf("set allowed channels: %v", err)
	}
	if err := store.SetAllowedThreadIDs([]string{"thread-9"}); err != nil {
		t.Fatalf("set allowed threads: %v", err)
	}
	if err := store.SetSpeechMode(SpeechModeAllowlist); err != nil {
		t.Fatalf("set speech mode allowlist: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if !reopened.AllowsSpeech("guild-9", "", "") {
		t.Fatal("expected reopened allowlist guild to permit speech")
	}
	if !reopened.AllowsSpeech("", "channel-9", "") {
		t.Fatal("expected reopened allowlist channel to permit speech")
	}
	if !reopened.AllowsSpeech("", "", "thread-9") {
		t.Fatal("expected reopened allowlist thread to permit speech")
	}
}

func TestAllowsSpeechSupportsMultipleGuildChannelAndThreadIDs(t *testing.T) {
	store, _ := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "allowlist",
  "allowed_guild_ids": ["guild-1", "guild-2", "guild-3"],
  "allowed_channel_ids": ["channel-1", "channel-2", "channel-3"],
  "allowed_thread_ids": ["thread-1", "thread-2", "thread-3"]
}`)

	if !store.AllowsSpeech("guild-2", "", "") {
		t.Fatal("expected second guild id to permit speech")
	}
	if !store.AllowsSpeech("", "channel-2", "") {
		t.Fatal("expected second channel id to permit speech")
	}
	if !store.AllowsSpeech("", "", "thread-2") {
		t.Fatal("expected second thread id to permit speech")
	}
	if !store.AllowsSpeech("guild-3", "", "") {
		t.Fatal("expected third guild id to permit speech")
	}
	if !store.AllowsSpeech("", "channel-3", "") {
		t.Fatal("expected third channel id to permit speech")
	}
	if !store.AllowsSpeech("", "", "thread-3") {
		t.Fatal("expected third thread id to permit speech")
	}
}

func TestLegacySpeechModeAllNormalizesToAllowlist(t *testing.T) {
	store, _ := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "all",
  "allowed_guild_ids": [],
  "allowed_channel_ids": [],
  "allowed_thread_ids": []
}`)

	mode, _, _, _ := store.SpeechScope()
	if mode != SpeechModeAllowlist {
		t.Fatalf("expected legacy all mode to normalize to allowlist, got %q", mode)
	}
	if store.AllowsSpeech("guild-1", "channel-1", "thread-1") {
		t.Fatal("expected empty allowlist to block speech")
	}
}

func TestProactiveReplyConfigPersists(t *testing.T) {
	store, path := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "proactive_reply": true,
  "proactive_chance": 17.5
}`)

	enabled, chance := store.ProactiveReplyConfig()
	if !enabled {
		t.Fatal("expected proactive reply to be enabled")
	}
	if chance != 17.5 {
		t.Fatalf("unexpected proactive chance: %v", chance)
	}

	if err := store.SetProactiveReplyEnabled(false); err != nil {
		t.Fatalf("disable proactive reply: %v", err)
	}
	if err := store.SetProactiveReplyChance(32.25); err != nil {
		t.Fatalf("set proactive chance: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	enabled, chance = reopened.ProactiveReplyConfig()
	if enabled {
		t.Fatal("expected proactive reply to be disabled after reopen")
	}
	if chance != 32.25 {
		t.Fatalf("unexpected reopened proactive chance: %v", chance)
	}
}

func TestSetProactiveReplyChanceRejectsOutOfRange(t *testing.T) {
	store, _ := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": ""
}`)

	if err := store.SetProactiveReplyChance(-1); err == nil {
		t.Fatal("expected negative proactive chance to fail")
	}
	if err := store.SetProactiveReplyChance(101); err == nil {
		t.Fatal("expected proactive chance above 100 to fail")
	}
}

func TestComposePromptsForGuildIncludesMatchingWorldBookOnly(t *testing.T) {
	store, _ := newTestStore(t, `{
  "super_admin_ids": ["owner-1"],
  "admin_ids": [],
  "personas": {
    "maid": "你是女仆。"
  },
  "active_persona": "maid",
  "system_prompt": "额外 system",
  "worldbook_entries": {
    "emoji:guild:guild-1": {
      "title": "Guild 1 Emoji",
      "content": "guild-1 的表情规则",
      "guild_id": "guild-1",
      "source": "emoji_analysis",
      "updated_at": "2026-03-18 20:00:00 UTC+8"
    },
    "emoji:guild:guild-2": {
      "title": "Guild 2 Emoji",
      "content": "guild-2 的表情规则",
      "guild_id": "guild-2",
      "source": "emoji_analysis",
      "updated_at": "2026-03-18 20:00:00 UTC+8"
    }
  }
}`)

	systemPrompt, personaPrompt := store.ComposePromptsForGuild("基础 prompt", "guild-1")
	if personaPrompt != "你是女仆。" {
		t.Fatalf("unexpected persona prompt: %q", personaPrompt)
	}
	if !strings.Contains(systemPrompt, "基础 prompt") || !strings.Contains(systemPrompt, "额外 system") {
		t.Fatalf("expected base and extra system prompt, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "guild-1 的表情规则") {
		t.Fatalf("expected guild-1 worldbook, got %q", systemPrompt)
	}
	if strings.Contains(systemPrompt, "guild-2 的表情规则") {
		t.Fatalf("expected guild-2 worldbook to be excluded, got %q", systemPrompt)
	}
}

func newTestStore(t *testing.T, content string) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bot_config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store, path
}
