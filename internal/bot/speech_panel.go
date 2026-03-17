package bot

import (
	"fmt"
	"strings"
	"unicode"

	"discordbot/internal/runtimecfg"

	"github.com/bwmarrin/discordgo"
)

const (
	speechComponentPrefix     = "speech:"
	speechActionModeAll       = "speech:mode-all"
	speechActionModeNone      = "speech:mode-none"
	speechActionModeAllowlist = "speech:mode-allowlist"
	speechActionEditGuilds    = "speech:edit-guilds"
	speechActionEditChannels  = "speech:edit-channels"
	speechActionEditThreads   = "speech:edit-threads"
	speechActionRefresh       = "speech:refresh"
	speechModalGuilds         = "speech:modal-guilds"
	speechModalChannels       = "speech:modal-channels"
	speechModalThreads        = "speech:modal-threads"
	speechModalFieldIDs       = "speech:field-ids"
	speechListPreviewLimit    = 10
	speechModalInputMaxLength = 4000
)

func isSpeechInteractionCustomID(customID string) bool {
	return strings.HasPrefix(strings.TrimSpace(customID), speechComponentPrefix)
}

func (h *Handler) SpeechPanelCommandResponse(authorID string) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return simpleEphemeralInteractionResponse(permissionDenied()), nil
	}

	data, err := h.speechPanelResponseData("")
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) SpeechComponentResponse(authorID string, data discordgo.MessageComponentInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return h.speechPanelUpdateResponse(permissionDenied())
	}

	switch data.CustomID {
	case speechActionModeAll:
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAll); err != nil {
			return nil, err
		}
		return h.speechPanelUpdateResponse("已切换为：全部可发言。")
	case speechActionModeNone:
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeNone); err != nil {
			return nil, err
		}
		return h.speechPanelUpdateResponse("已切换为：均不发言。")
	case speechActionModeAllowlist:
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAllowlist); err != nil {
			return nil, err
		}
		return h.speechPanelUpdateResponse("已切换为：按 ID 白名单发言。")
	case speechActionEditGuilds:
		return h.speechIDsModalResponse("编辑允许发言的服务器 ID", speechModalGuilds, "每行一个服务器 ID，也支持逗号或空格分隔。留空提交可清空。", h.currentSpeechGuildIDs()), nil
	case speechActionEditChannels:
		return h.speechIDsModalResponse("编辑允许发言的频道 ID", speechModalChannels, "每行一个频道 ID，也支持逗号或空格分隔。留空提交可清空。", h.currentSpeechChannelIDs()), nil
	case speechActionEditThreads:
		return h.speechIDsModalResponse("编辑允许发言的子区 ID", speechModalThreads, "每行一个子区/线程/帖子 ID，也支持逗号或空格分隔。留空提交可清空。", h.currentSpeechThreadIDs()), nil
	case speechActionRefresh:
		return h.speechPanelUpdateResponse("已刷新发言范围面板。")
	default:
		return h.speechPanelUpdateResponse("未知的发言范围操作。")
	}
}

func (h *Handler) SpeechModalResponse(authorID string, data discordgo.ModalSubmitInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return h.speechPanelMessageResponse(permissionDenied())
	}

	ids, err := parseDiscordIDs(modalTextInputValue(data.Components, speechModalFieldIDs))
	if err != nil {
		return h.speechPanelMessageResponse(err.Error())
	}

	switch data.CustomID {
	case speechModalGuilds:
		if err := h.runtimeStore.SetAllowedGuildIDs(ids); err != nil {
			return nil, err
		}
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAllowlist); err != nil {
			return nil, err
		}
		return h.speechPanelMessageResponse(fmt.Sprintf("已更新允许发言的服务器 ID，共 %d 个。当前模式已切到白名单。", len(ids)))
	case speechModalChannels:
		if err := h.runtimeStore.SetAllowedChannelIDs(ids); err != nil {
			return nil, err
		}
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAllowlist); err != nil {
			return nil, err
		}
		return h.speechPanelMessageResponse(fmt.Sprintf("已更新允许发言的频道 ID，共 %d 个。当前模式已切到白名单。", len(ids)))
	case speechModalThreads:
		if err := h.runtimeStore.SetAllowedThreadIDs(ids); err != nil {
			return nil, err
		}
		if err := h.runtimeStore.SetSpeechMode(runtimecfg.SpeechModeAllowlist); err != nil {
			return nil, err
		}
		return h.speechPanelMessageResponse(fmt.Sprintf("已更新允许发言的子区 ID，共 %d 个。当前模式已切到白名单。", len(ids)))
	default:
		return nil, fmt.Errorf("unknown speech modal: %s", data.CustomID)
	}
}

func (h *Handler) speechPanelUpdateResponse(notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.speechPanelResponseData(notice)
	if err != nil {
		return nil, err
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	}, nil
}

func (h *Handler) speechPanelMessageResponse(notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.speechPanelResponseData(notice)
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) speechPanelResponseData(notice string) (*discordgo.InteractionResponseData, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	mode, guilds, channels, threads := h.runtimeStore.SpeechScope()
	return &discordgo.InteractionResponseData{
		Content: strings.TrimSpace(notice),
		Embeds: []*discordgo.MessageEmbed{
			buildSpeechPanelEmbed(mode, guilds, channels, threads),
		},
		Components: buildSpeechPanelComponents(mode),
	}, nil
}

func (h *Handler) speechIDsModalResponse(title, customID, placeholder string, ids []string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    truncateRunes(title, 45),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    speechModalFieldIDs,
							Label:       "ID 列表",
							Style:       discordgo.TextInputParagraph,
							Placeholder: placeholder,
							Value:       strings.Join(ids, "\n"),
							Required:    false,
							MaxLength:   speechModalInputMaxLength,
						},
					},
				},
			},
		},
	}
}

func buildSpeechPanelEmbed(mode string, guilds, channels, threads []string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Speech Scope Control",
		Description: "控制机器人在哪些位置允许发言。编辑任意 ID 列表后，会自动切换到白名单模式。留空提交可清空该列表。",
		Color:       speechModeColor(mode),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "当前模式",
				Value:  speechModeLabel(mode),
				Inline: true,
			},
			{
				Name:   "说明",
				Value:  "服务器 ID 允许整个服务器发言；频道 ID 允许该频道及其下属子区发言；子区 ID 只允许该子区发言。",
				Inline: false,
			},
			{
				Name:   fmt.Sprintf("允许服务器 ID (%d)", len(guilds)),
				Value:  speechIDFieldValue(guilds, "当前没有设置允许发言的服务器 ID。"),
				Inline: false,
			},
			{
				Name:   fmt.Sprintf("允许频道 ID (%d)", len(channels)),
				Value:  speechIDFieldValue(channels, "当前没有设置允许发言的频道 ID。"),
				Inline: false,
			},
			{
				Name:   fmt.Sprintf("允许子区 ID (%d)", len(threads)),
				Value:  speechIDFieldValue(threads, "当前没有设置允许发言的子区/线程/帖子 ID。"),
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "全部可发言 / 均不发言 是一键模式；白名单模式下只看你填的服务器、频道、子区 ID。",
		},
	}
}

func buildSpeechPanelComponents(mode string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "全部可发言",
					Style:    discordgo.SuccessButton,
					CustomID: speechActionModeAll,
					Disabled: mode == runtimecfg.SpeechModeAll,
				},
				discordgo.Button{
					Label:    "均不发言",
					Style:    discordgo.DangerButton,
					CustomID: speechActionModeNone,
					Disabled: mode == runtimecfg.SpeechModeNone,
				},
				discordgo.Button{
					Label:    "白名单模式",
					Style:    discordgo.PrimaryButton,
					CustomID: speechActionModeAllowlist,
					Disabled: mode == runtimecfg.SpeechModeAllowlist,
				},
				discordgo.Button{
					Label:    "刷新",
					Style:    discordgo.SecondaryButton,
					CustomID: speechActionRefresh,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "编辑服务器 ID",
					Style:    discordgo.SecondaryButton,
					CustomID: speechActionEditGuilds,
				},
				discordgo.Button{
					Label:    "编辑频道 ID",
					Style:    discordgo.SecondaryButton,
					CustomID: speechActionEditChannels,
				},
				discordgo.Button{
					Label:    "编辑子区 ID",
					Style:    discordgo.SecondaryButton,
					CustomID: speechActionEditThreads,
				},
			},
		},
	}
}

func speechModeLabel(mode string) string {
	switch mode {
	case runtimecfg.SpeechModeNone:
		return "均不发言"
	case runtimecfg.SpeechModeAllowlist:
		return "按 ID 白名单发言"
	default:
		return "全部可发言"
	}
}

func speechModeColor(mode string) int {
	switch mode {
	case runtimecfg.SpeechModeNone:
		return 0xc0392b
	case runtimecfg.SpeechModeAllowlist:
		return 0xf39c12
	default:
		return 0x27ae60
	}
}

func speechIDFieldValue(ids []string, emptyText string) string {
	if len(ids) == 0 {
		return emptyText
	}
	lines := make([]string, 0, minInt(len(ids), speechListPreviewLimit)+1)
	for index, id := range ids {
		if index >= speechListPreviewLimit {
			break
		}
		lines = append(lines, "• "+id)
	}
	if len(ids) > speechListPreviewLimit {
		lines = append(lines, fmt.Sprintf("… 还有 %d 个未展开", len(ids)-speechListPreviewLimit))
	}
	return strings.Join(lines, "\n")
}

func parseDiscordIDs(input string) ([]string, error) {
	normalized := strings.NewReplacer(",", "\n", "，", "\n", ";", "\n", "；", "\n", "\t", "\n", " ", "\n").Replace(strings.TrimSpace(input))
	if strings.TrimSpace(normalized) == "" {
		return []string{}, nil
	}

	fields := strings.Fields(normalized)
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !isDigitsOnly(field) {
			return nil, fmt.Errorf("无效的 Discord ID: %s", field)
		}
		ids = append(ids, field)
	}
	return ids, nil
}

func isDigitsOnly(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (h *Handler) currentSpeechGuildIDs() []string {
	_, guilds, _, _ := h.runtimeStore.SpeechScope()
	return guilds
}

func (h *Handler) currentSpeechChannelIDs() []string {
	_, _, channels, _ := h.runtimeStore.SpeechScope()
	return channels
}

func (h *Handler) currentSpeechThreadIDs() []string {
	_, _, _, threads := h.runtimeStore.SpeechScope()
	return threads
}
