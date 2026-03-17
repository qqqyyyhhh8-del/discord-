package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	personaComponentPrefix       = "persona:"
	personaActionRefresh         = "persona:refresh"
	personaActionOpenUpsert      = "persona:open-upsert"
	personaActionOpenEditActive  = "persona:open-edit-active"
	personaActionDeleteActive    = "persona:delete-active"
	personaActionClearActive     = "persona:clear-active"
	personaActionUseSelect       = "persona:use-select"
	personaModalUpsert           = "persona:modal-upsert"
	personaModalEditActive       = "persona:modal-edit-active"
	personaModalFieldName        = "persona:field-name"
	personaModalFieldPrompt      = "persona:field-prompt"
	personaModalFieldEditPrompt  = "persona:field-edit-prompt"
	personaListPreviewLimit      = 10
	personaSelectOptionLimit     = 25
	personaPromptPreviewMaxRunes = 700
)

func isPersonaInteractionCustomID(customID string) bool {
	return strings.HasPrefix(strings.TrimSpace(customID), personaComponentPrefix)
}

func (h *Handler) PersonaPanelCommandResponse(authorID string) (*discordgo.InteractionResponse, error) {
	data, err := h.personaPanelResponseData(authorID, "")
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) PersonaComponentResponse(authorID string, data discordgo.MessageComponentInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}

	switch data.CustomID {
	case personaActionRefresh:
		return h.personaPanelUpdateResponse(authorID, "已刷新人设面板。")
	case personaActionOpenUpsert:
		if !h.runtimeStore.IsAdmin(authorID) {
			return h.personaPanelUpdateResponse(authorID, permissionDenied())
		}
		return h.personaUpsertModalResponse(), nil
	case personaActionOpenEditActive:
		if !h.runtimeStore.IsAdmin(authorID) {
			return h.personaPanelUpdateResponse(authorID, permissionDenied())
		}
		if h.runtimeStore.ActivePersonaName() == "" {
			return h.personaPanelUpdateResponse(authorID, "当前没有启用中的人设，无法编辑。")
		}
		return h.personaEditActiveModalResponse(), nil
	case personaActionDeleteActive:
		if !h.runtimeStore.IsAdmin(authorID) {
			return h.personaPanelUpdateResponse(authorID, permissionDenied())
		}
		active := h.runtimeStore.ActivePersonaName()
		if active == "" {
			return h.personaPanelUpdateResponse(authorID, "当前没有启用中的人设，无法删除。")
		}
		if err := h.runtimeStore.DeletePersona(active); err != nil {
			return h.personaPanelUpdateResponse(authorID, err.Error())
		}
		return h.personaPanelUpdateResponse(authorID, fmt.Sprintf("已删除人设: %s", active))
	case personaActionClearActive:
		if !h.runtimeStore.IsAdmin(authorID) {
			return h.personaPanelUpdateResponse(authorID, permissionDenied())
		}
		if h.runtimeStore.ActivePersonaName() == "" {
			return h.personaPanelUpdateResponse(authorID, "当前没有启用中的人设。")
		}
		if err := h.runtimeStore.ClearActivePersona(); err != nil {
			return nil, err
		}
		return h.personaPanelUpdateResponse(authorID, "已清空当前启用人设。")
	case personaActionUseSelect:
		if !h.runtimeStore.IsAdmin(authorID) {
			return h.personaPanelUpdateResponse(authorID, permissionDenied())
		}
		if len(data.Values) == 0 {
			return h.personaPanelUpdateResponse(authorID, "请选择一个人设。")
		}
		name := strings.TrimSpace(data.Values[0])
		if err := h.runtimeStore.SetActivePersona(name); err != nil {
			return h.personaPanelUpdateResponse(authorID, err.Error())
		}
		return h.personaPanelUpdateResponse(authorID, fmt.Sprintf("已切换到人设: %s", name))
	default:
		return h.personaPanelUpdateResponse(authorID, "未知的人设面板操作。")
	}
}

func (h *Handler) PersonaModalResponse(authorID string, data discordgo.ModalSubmitInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}
	if data.CustomID != personaModalUpsert && data.CustomID != personaModalEditActive {
		return nil, fmt.Errorf("unknown persona modal: %s", data.CustomID)
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return h.personaPanelMessageResponse(authorID, permissionDenied())
	}

	if data.CustomID == personaModalEditActive {
		active := h.runtimeStore.ActivePersonaName()
		if active == "" {
			return h.personaPanelMessageResponse(authorID, "当前没有启用中的人设，无法编辑。")
		}
		prompt := modalTextInputValue(data.Components, personaModalFieldEditPrompt)
		if err := h.runtimeStore.UpsertPersona(active, prompt); err != nil {
			return h.personaPanelMessageResponse(authorID, err.Error())
		}
		if err := h.runtimeStore.SetActivePersona(active); err != nil {
			return h.personaPanelMessageResponse(authorID, err.Error())
		}
		return h.personaPanelMessageResponse(authorID, fmt.Sprintf("已更新当前人设: %s", active))
	}

	name := modalTextInputValue(data.Components, personaModalFieldName)
	prompt := modalTextInputValue(data.Components, personaModalFieldPrompt)
	if err := h.runtimeStore.UpsertPersona(name, prompt); err != nil {
		return h.personaPanelMessageResponse(authorID, err.Error())
	}
	if err := h.runtimeStore.SetActivePersona(name); err != nil {
		return h.personaPanelMessageResponse(authorID, err.Error())
	}
	return h.personaPanelMessageResponse(authorID, fmt.Sprintf("已保存并切换到人设: %s", strings.TrimSpace(name)))
}

func (h *Handler) personaPanelUpdateResponse(authorID, notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.personaPanelResponseData(authorID, notice)
	if err != nil {
		return nil, err
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	}, nil
}

func (h *Handler) personaPanelMessageResponse(authorID, notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.personaPanelResponseData(authorID, notice)
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) personaUpsertModalResponse() *discordgo.InteractionResponse {
	activeName := h.runtimeStore.ActivePersonaName()
	activePrompt, _ := h.runtimeStore.PersonaPrompt(activeName)

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: personaModalUpsert,
			Title:    "新增或覆盖人设",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    personaModalFieldName,
							Label:       "人设名称",
							Style:       discordgo.TextInputShort,
							Placeholder: "例如：maid / assistant / character",
							Value:       activeName,
							Required:    true,
							MinLength:   1,
							MaxLength:   64,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    personaModalFieldPrompt,
							Label:       "人设 Prompt",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "输入完整人设提示词，保存后会自动切换到这个人设。",
							Value:       activePrompt,
							Required:    true,
							MinLength:   1,
							MaxLength:   4000,
						},
					},
				},
			},
		},
	}
}

func (h *Handler) personaEditActiveModalResponse() *discordgo.InteractionResponse {
	activeName := h.runtimeStore.ActivePersonaName()
	activePrompt, _ := h.runtimeStore.PersonaPrompt(activeName)
	title := "编辑当前人设"
	if strings.TrimSpace(activeName) != "" {
		title = truncateRunes("编辑当前人设: "+activeName, 45)
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: personaModalEditActive,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    personaModalFieldEditPrompt,
							Label:       "当前人设 Prompt",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "修改当前启用人设的 Prompt 内容",
							Value:       activePrompt,
							Required:    true,
							MinLength:   1,
							MaxLength:   4000,
						},
					},
				},
			},
		},
	}
}

func (h *Handler) personaPanelResponseData(authorID, notice string) (*discordgo.InteractionResponseData, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return nil, err
	}

	isAdmin := h.runtimeStore.IsAdmin(authorID)
	isSuperAdmin := h.runtimeStore.IsSuperAdmin(authorID)
	names := h.runtimeStore.PersonaNames()
	active := h.runtimeStore.ActivePersonaName()
	activePrompt, _ := h.runtimeStore.PersonaPrompt(active)

	data := &discordgo.InteractionResponseData{
		Content:    strings.TrimSpace(notice),
		Embeds:     []*discordgo.MessageEmbed{buildPersonaPanelEmbed(names, active, activePrompt, isAdmin, isSuperAdmin)},
		Components: h.personaPanelComponents(names, active, isAdmin),
	}
	return data, nil
}

func (h *Handler) personaPanelComponents(names []string, active string, isAdmin bool) []discordgo.MessageComponent {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "新增/覆盖",
					Style:    discordgo.SuccessButton,
					CustomID: personaActionOpenUpsert,
					Disabled: !isAdmin,
				},
				discordgo.Button{
					Label:    "编辑当前",
					Style:    discordgo.PrimaryButton,
					CustomID: personaActionOpenEditActive,
					Disabled: !isAdmin || strings.TrimSpace(active) == "",
				},
				discordgo.Button{
					Label:    "删除当前",
					Style:    discordgo.DangerButton,
					CustomID: personaActionDeleteActive,
					Disabled: !isAdmin || strings.TrimSpace(active) == "",
				},
				discordgo.Button{
					Label:    "清空启用",
					Style:    discordgo.SecondaryButton,
					CustomID: personaActionClearActive,
					Disabled: !isAdmin || strings.TrimSpace(active) == "",
				},
				discordgo.Button{
					Label:    "刷新",
					Style:    discordgo.PrimaryButton,
					CustomID: personaActionRefresh,
				},
			},
		},
	}

	selectRow := buildPersonaSelectRow(h, names, active, isAdmin)
	if selectRow != nil {
		components = append(components, selectRow)
	}

	return components
}

func buildPersonaSelectRow(h *Handler, names []string, active string, isAdmin bool) discordgo.MessageComponent {
	if len(names) == 0 {
		return nil
	}

	options := make([]discordgo.SelectMenuOption, 0, len(names))
	for index, name := range names {
		if index >= personaSelectOptionLimit {
			break
		}
		prompt, _ := h.runtimeStore.PersonaPrompt(name)
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateRunes(name, 100),
			Value:       name,
			Description: truncateRunes(singleLine(prompt), 90),
			Default:     name == active,
		})
	}

	if len(options) == 0 {
		return nil
	}

	minValues := 1
	placeholder := "选择人设并立即切换"
	if strings.TrimSpace(active) != "" {
		placeholder = "当前人设: " + truncateRunes(active, 80)
	}

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:    discordgo.StringSelectMenu,
				CustomID:    personaActionUseSelect,
				Placeholder: placeholder,
				MinValues:   &minValues,
				MaxValues:   1,
				Options:     options,
				Disabled:    !isAdmin,
			},
		},
	}
}

func buildPersonaPanelEmbed(names []string, active, activePrompt string, isAdmin, isSuperAdmin bool) *discordgo.MessageEmbed {
	roleLabel := "查看模式"
	if isSuperAdmin {
		roleLabel = "超级管理员"
	} else if isAdmin {
		roleLabel = "管理员"
	}

	activeLabel := "未启用"
	if strings.TrimSpace(active) != "" {
		activeLabel = active
	}

	description := "一站式人设管理面板。使用下方选择菜单切换当前人设，使用按钮新增、覆盖、删除或清空。"
	if !isAdmin {
		description = "当前是只读视图。你可以查看当前状态，但只有管理员和超级管理员可以修改人设。"
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Persona Studio",
		Description: description,
		Color:       personaPanelColor(isAdmin, active),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "当前启用",
				Value:  truncateRunes(activeLabel, 1024),
				Inline: true,
			},
			{
				Name:   "你的权限",
				Value:  roleLabel,
				Inline: true,
			},
			{
				Name:   fmt.Sprintf("已保存人设 (%d)", len(names)),
				Value:  personaListFieldValue(names, active),
				Inline: false,
			},
			{
				Name:   "当前 Prompt 预览",
				Value:  personaPromptPreview(activePrompt),
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "保存新的人设后会自动切换。编辑当前只改当前 Prompt。删除当前会连同启用状态一起清掉。",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(names) > personaSelectOptionLimit {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "提示",
			Value:  fmt.Sprintf("切换菜单最多显示前 %d 个名称；当前总数为 %d。", personaSelectOptionLimit, len(names)),
			Inline: false,
		})
	}

	return embed
}

func personaPanelColor(isAdmin bool, active string) int {
	if strings.TrimSpace(active) != "" {
		return 0x2ecc71
	}
	if isAdmin {
		return 0xf39c12
	}
	return 0x3498db
}

func personaListFieldValue(names []string, active string) string {
	if len(names) == 0 {
		return "暂无人设。点击下方 `新增/覆盖` 按钮创建第一个。"
	}

	items := make([]string, 0, minInt(len(names), personaListPreviewLimit)+1)
	for index, name := range names {
		if index >= personaListPreviewLimit {
			break
		}
		label := "• " + name
		if name == active {
			label += "  <- 当前"
		}
		items = append(items, label)
	}
	if len(names) > personaListPreviewLimit {
		items = append(items, fmt.Sprintf("… 还有 %d 个未展开", len(names)-personaListPreviewLimit))
	}
	return strings.Join(items, "\n")
}

func personaPromptPreview(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "当前未启用人设，保存或切换后会在这里显示预览。"
	}
	return "```text\n" + truncateRunes(prompt, personaPromptPreviewMaxRunes) + "\n```"
}

func modalTextInputValue(components []discordgo.MessageComponent, customID string) string {
	for _, component := range components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, child := range row.Components {
			input, ok := child.(*discordgo.TextInput)
			if !ok || input.CustomID != customID {
				continue
			}
			return strings.TrimSpace(input.Value)
		}
	}
	return ""
}

func singleLine(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	return strings.Join(fields, " ")
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
