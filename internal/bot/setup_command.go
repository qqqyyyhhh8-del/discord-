package bot

import (
	"fmt"
	"strings"

	"discordbot/internal/runtimecfg"

	"github.com/bwmarrin/discordgo"
)

func buildSetupSlashCommand(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	subcommand, _, err := slashSubcommand(options)
	if err != nil {
		return "", err
	}

	switch subcommand {
	case "show", "server", "channel", "thread", "clear":
		return subcommand, nil
	default:
		return "", fmt.Errorf("unknown setup subcommand")
	}
}

func (h *Handler) handleSetupCommand(authorID string, location speechLocation, options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return "", err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return permissionDenied(), nil
	}

	subcommand, err := buildSetupSlashCommand(options)
	if err != nil {
		return setupHelp(), nil
	}

	switch subcommand {
	case "show":
		return h.setupSummary(), nil
	case "server":
		guildID := strings.TrimSpace(location.GuildID)
		if guildID == "" {
			return "请在目标服务器中执行 `/setup server`。", nil
		}
		if err := h.runtimeStore.AddAllowedGuildID(guildID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已放行当前服务器: `%s`\n\n%s", guildID, h.setupSummary()), nil
	case "channel":
		guildID := strings.TrimSpace(location.GuildID)
		channelID := strings.TrimSpace(location.ChannelID)
		if guildID == "" || channelID == "" {
			return "请在目标服务器频道中执行 `/setup channel`。", nil
		}
		if err := h.runtimeStore.AddAllowedChannelID(channelID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已放行当前频道: `%s`\n\n%s", channelID, h.setupSummary()), nil
	case "thread":
		guildID := strings.TrimSpace(location.GuildID)
		threadID := strings.TrimSpace(location.ThreadID)
		if guildID == "" {
			return "请在目标服务器子区中执行 `/setup thread`。", nil
		}
		if threadID == "" {
			return "当前不在子区/线程/帖子内，请进入目标子区后再执行 `/setup thread`。", nil
		}
		if err := h.runtimeStore.AddAllowedThreadID(threadID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已放行当前子区: `%s`\n\n%s", threadID, h.setupSummary()), nil
	case "clear":
		if err := h.runtimeStore.SetAllowedGuildIDs(nil); err != nil {
			return "", err
		}
		if err := h.runtimeStore.SetAllowedChannelIDs(nil); err != nil {
			return "", err
		}
		if err := h.runtimeStore.SetAllowedThreadIDs(nil); err != nil {
			return "", err
		}
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAllowlist); err != nil {
			return "", err
		}
		return "已清空所有允许发言范围。机器人现在默认不会在任何服务器、频道或子区发言。", nil
	default:
		return setupHelp(), nil
	}
}

func (h *Handler) setupSummary() string {
	mode, guilds, channels, threads := h.runtimeStore.SpeechScope()

	status := "当前状态：默认不发言（还没有放行任何位置）"
	if mode == runtimecfg.SpeechModeNone {
		status = "当前状态：已完全禁言"
	} else if len(guilds)+len(channels)+len(threads) > 0 {
		status = "当前状态：仅在以下白名单位置发言"
	}

	return strings.Join([]string{
		status,
		"服务器 ID:",
		renderIDList(guilds),
		"频道 ID:",
		renderIDList(channels),
		"子区 ID:",
		renderIDList(threads),
	}, "\n")
}
