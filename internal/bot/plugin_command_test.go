package bot

import (
	"strings"
	"testing"

	"discordbot/internal/pluginhost"
	"discordbot/pkg/pluginapi"
)

func TestPluginListEditUsesEmbeds(t *testing.T) {
	edit := pluginListEdit([]pluginhost.InstalledPlugin{
		{
			ID:          "official_persona",
			Name:        "Official Persona Plugin",
			Version:     "v0.1.0",
			Description: "Persona management",
			Enabled:     true,
			GuildMode:   pluginhost.GuildModeAll,
			GrantedCaps: []pluginapi.Capability{pluginapi.CapabilityPluginStorage, pluginapi.CapabilityDiscordInteractionRespond},
			Manifest: pluginapi.Manifest{
				Commands: []pluginapi.CommandSpec{{Name: "persona", Description: "persona"}},
			},
		},
	})

	if edit == nil || edit.Embeds == nil {
		t.Fatal("expected embeds edit")
	}
	embeds := *edit.Embeds
	if len(embeds) < 2 {
		t.Fatalf("expected summary and detail embeds, got %d", len(embeds))
	}
	if embeds[0].Title != "Plugin Control Center" {
		t.Fatalf("unexpected summary title: %q", embeds[0].Title)
	}
	if len(embeds[1].Fields) == 0 {
		t.Fatal("expected detail fields")
	}
	if !strings.Contains(embeds[1].Fields[0].Value, "`/persona`") {
		t.Fatalf("expected command list in detail field, got %q", embeds[1].Fields[0].Value)
	}
}

func TestPluginPermissionsEditIncludesCapabilities(t *testing.T) {
	edit := pluginPermissionsEdit(pluginhost.InstalledPlugin{
		ID:          "official_emoji",
		Name:        "Official Emoji Plugin",
		Version:     "v0.1.0",
		Description: "Emoji analysis",
		Enabled:     true,
		GuildMode:   pluginhost.GuildModeAllowlist,
		GuildIDs:    []string{"123", "456"},
		GrantedCaps: []pluginapi.Capability{pluginapi.CapabilityDiscordReadGuildEmojis, pluginapi.CapabilityWorldBookWrite},
		Manifest: pluginapi.Manifest{
			Commands: []pluginapi.CommandSpec{{Name: "emoji", Description: "emoji"}},
		},
	})

	if edit == nil || edit.Embeds == nil || len(*edit.Embeds) != 1 {
		t.Fatal("expected single embed response")
	}
	embed := (*edit.Embeds)[0]
	if embed.Title != "Plugin Permissions" {
		t.Fatalf("unexpected title: %q", embed.Title)
	}
	foundCaps := false
	for _, field := range embed.Fields {
		if field == nil || field.Name != "授权能力" {
			continue
		}
		foundCaps = true
		if !strings.Contains(field.Value, "discord.read_guild_emojis") || !strings.Contains(field.Value, "worldbook.write") {
			t.Fatalf("expected capability list in field, got %q", field.Value)
		}
	}
	if !foundCaps {
		t.Fatal("expected 授权能力 field")
	}
}
