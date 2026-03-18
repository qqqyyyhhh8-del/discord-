package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	proactiveComponentPrefix       = "proactive:"
	proactiveActionEnable          = "proactive:enable"
	proactiveActionDisable         = "proactive:disable"
	proactiveActionEditChance      = "proactive:edit-chance"
	proactiveActionRefresh         = "proactive:refresh"
	proactiveModalChance           = "proactive:modal-chance"
	proactiveModalFieldChance      = "proactive:field-chance"
	proactiveChanceInputMaxLength  = 16
	proactiveChancePreviewDecimals = 2
)

func isProactiveInteractionCustomID(customID string) bool {
	return strings.HasPrefix(strings.TrimSpace(customID), proactiveComponentPrefix)
}

func (h *Handler) ProactivePanelCommandResponse(authorID string, location speechLocation) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return simpleEphemeralInteractionResponse(permissionDenied()), nil
	}
	if strings.TrimSpace(location.GuildID) == "" {
		return simpleEphemeralInteractionResponse("主动回复管理只能在服务器频道中使用。"), nil
	}

	data, err := h.proactivePanelResponseData(location, "")
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) ProactiveComponentResponse(authorID string, location speechLocation, data discordgo.MessageComponentInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return h.proactivePanelUpdateResponse(location, permissionDenied())
	}
	if strings.TrimSpace(location.GuildID) == "" {
		return h.proactivePanelUpdateResponse(location, "主动回复管理只能在服务器频道中使用。")
	}

	switch data.CustomID {
	case proactiveActionEnable:
		if !h.allowsSpeechAtLocation(location) {
			return h.proactivePanelUpdateResponse(location, proactiveSetupDeniedNotice())
		}
		if err := h.runtimeStore.SetProactiveReplyEnabled(true); err != nil {
			return nil, err
		}
		return h.proactivePanelUpdateResponse(location, "已开启主动回复。")
	case proactiveActionDisable:
		if err := h.runtimeStore.SetProactiveReplyEnabled(false); err != nil {
			return nil, err
		}
		return h.proactivePanelUpdateResponse(location, "已关闭主动回复。")
	case proactiveActionEditChance:
		_, chance := h.runtimeStore.ProactiveReplyConfig()
		return h.proactiveChanceModalResponse(chance), nil
	case proactiveActionRefresh:
		return h.proactivePanelUpdateResponse(location, "已刷新主动回复面板。")
	default:
		return h.proactivePanelUpdateResponse(location, "未知的主动回复操作。")
	}
}

func (h *Handler) ProactiveModalResponse(authorID string, location speechLocation, data discordgo.ModalSubmitInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return h.proactivePanelMessageResponse(location, permissionDenied())
	}
	if strings.TrimSpace(location.GuildID) == "" {
		return h.proactivePanelMessageResponse(location, "主动回复管理只能在服务器频道中使用。")
	}

	switch data.CustomID {
	case proactiveModalChance:
		chance, err := parseProbabilityPercent(modalTextInputValue(data.Components, proactiveModalFieldChance))
		if err != nil {
			return h.proactivePanelMessageResponse(location, err.Error())
		}
		if err := h.runtimeStore.SetProactiveReplyChance(chance); err != nil {
			return nil, err
		}
		return h.proactivePanelMessageResponse(location, fmt.Sprintf("已更新主动回复概率为 %s。", formatProbabilityPercent(chance)))
	default:
		return nil, fmt.Errorf("unknown proactive modal: %s", data.CustomID)
	}
}

func (h *Handler) proactivePanelUpdateResponse(location speechLocation, notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.proactivePanelResponseData(location, notice)
	if err != nil {
		return nil, err
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	}, nil
}

func (h *Handler) proactivePanelMessageResponse(location speechLocation, notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.proactivePanelResponseData(location, notice)
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) proactivePanelResponseData(location speechLocation, notice string) (*discordgo.InteractionResponseData, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}

	enabled, chance := h.runtimeStore.ProactiveReplyConfig()
	speechAllowed := h.allowsSpeechAtLocation(location)
	return &discordgo.InteractionResponseData{
		Content: strings.TrimSpace(notice),
		Embeds: []*discordgo.MessageEmbed{
			buildProactivePanelEmbed(location, enabled, chance, speechAllowed),
		},
		Components: buildProactivePanelComponents(enabled),
	}, nil
}

func (h *Handler) proactiveChanceModalResponse(chance float64) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: proactiveModalChance,
			Title:    "编辑主动回复概率",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    proactiveModalFieldChance,
							Label:       "概率百分比",
							Style:       discordgo.TextInputShort,
							Placeholder: "输入 0 到 100，例如 5 或 12.5",
							Value:       formatProbabilityPercent(chance),
							Required:    true,
							MaxLength:   proactiveChanceInputMaxLength,
						},
					},
				},
			},
		},
	}
}

func buildProactivePanelEmbed(location speechLocation, enabled bool, chance float64, speechAllowed bool) *discordgo.MessageEmbed {
	locationLabel := "当前服务器/频道在允许发言范围内"
	if !speechAllowed {
		locationLabel = "当前服务器/频道不在允许发言范围内"
	}

	description := "控制机器人在没有被 @ 或没有直接回复机器人的情况下，是否按概率主动回复普通群聊消息。"
	if !speechAllowed {
		description += "\n\n当前交互位置还没有在 `/setup` 里放行。开启主动回复时会被拦截。"
	}

	return &discordgo.MessageEmbed{
		Title:       "Proactive Reply Control",
		Description: description,
		Color:       proactivePanelColor(enabled, speechAllowed),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "当前状态",
				Value:  proactiveEnabledLabel(enabled),
				Inline: true,
			},
			{
				Name:   "当前概率",
				Value:  formatProbabilityPercent(chance),
				Inline: true,
			},
			{
				Name:   "当前交互位置",
				Value:  locationLabel,
				Inline: false,
			},
			{
				Name:   "说明",
				Value:  "主动回复只在群聊里生效，且仍然遵守 `/setup` 配置的服务器、频道、子区白名单。",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "命中概率后才会回复；未命中时不会打断当前聊天。",
		},
	}
}

func buildProactivePanelComponents(enabled bool) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "开启",
					Style:    discordgo.SuccessButton,
					CustomID: proactiveActionEnable,
					Disabled: enabled,
				},
				discordgo.Button{
					Label:    "关闭",
					Style:    discordgo.DangerButton,
					CustomID: proactiveActionDisable,
					Disabled: !enabled,
				},
				discordgo.Button{
					Label:    "编辑概率",
					Style:    discordgo.PrimaryButton,
					CustomID: proactiveActionEditChance,
				},
				discordgo.Button{
					Label:    "刷新",
					Style:    discordgo.SecondaryButton,
					CustomID: proactiveActionRefresh,
				},
			},
		},
	}
}

func proactiveEnabledLabel(enabled bool) string {
	if enabled {
		return "已开启"
	}
	return "已关闭"
}

func proactivePanelColor(enabled, speechAllowed bool) int {
	switch {
	case enabled && speechAllowed:
		return 0x10B981
	case enabled && !speechAllowed:
		return 0xF59E0B
	case !speechAllowed:
		return 0xDC2626
	default:
		return 0x6B7280
	}
}

func proactiveSetupDeniedNotice() string {
	return "当前服务器/频道/子区还没有在 `/setup` 里放行，请先配置允许发言范围后再开启主动回复。"
}

func parseProbabilityPercent(input string) (float64, error) {
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(input), "%"))
	if value == "" {
		return 0, fmt.Errorf("主动回复概率不能为空")
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("主动回复概率格式无效，请输入 0 到 100 之间的数字")
	}
	if parsed < 0 || parsed > 100 {
		return 0, fmt.Errorf("主动回复概率必须在 0 到 100 之间")
	}
	return parsed, nil
}

func formatProbabilityPercent(chance float64) string {
	text := strconv.FormatFloat(chance, 'f', proactiveChancePreviewDecimals, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" {
		text = "0"
	}
	return text + "%"
}
