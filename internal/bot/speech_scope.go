package bot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

type speechLocation struct {
	GuildID   string
	ChannelID string
	ThreadID  string
}

func (h *Handler) AllowsSpeechForMessage(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if h == nil || h.runtimeStore == nil {
		return true
	}

	mode, _, _, _ := h.runtimeStore.SpeechScope()
	if mode == "all" {
		return true
	}
	if mode == "none" {
		return false
	}

	location := speechLocationForDiscordMessage(s, m)
	return h.runtimeStore.AllowsSpeech(location.GuildID, location.ChannelID, location.ThreadID)
}

func speechLocationForDiscordMessage(s *discordgo.Session, m *discordgo.MessageCreate) speechLocation {
	location := speechLocation{}
	if m == nil || m.Message == nil {
		return location
	}

	location.GuildID = strings.TrimSpace(m.GuildID)
	location.ChannelID = strings.TrimSpace(m.ChannelID)

	channel := resolveDiscordChannel(s, location.ChannelID)
	if channel == nil {
		return location
	}
	if location.GuildID == "" {
		location.GuildID = strings.TrimSpace(channel.GuildID)
	}
	if channel.IsThread() {
		location.ThreadID = strings.TrimSpace(channel.ID)
		if parentID := strings.TrimSpace(channel.ParentID); parentID != "" {
			location.ChannelID = parentID
		}
		return location
	}
	location.ChannelID = strings.TrimSpace(channel.ID)
	return location
}

func resolveDiscordChannel(s *discordgo.Session, channelID string) *discordgo.Channel {
	channelID = strings.TrimSpace(channelID)
	if s == nil || channelID == "" {
		return nil
	}
	if s.State != nil {
		if channel, err := s.State.Channel(channelID); err == nil && channel != nil {
			return channel
		}
	}
	channel, err := s.Channel(channelID)
	if err != nil {
		return nil
	}
	return channel
}
