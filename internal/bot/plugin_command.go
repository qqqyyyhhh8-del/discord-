package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kizuna/internal/pluginhost"
	"kizuna/pkg/pluginapi"

	"github.com/bwmarrin/discordgo"
)

const (
	pluginComponentPrefix = "plugin:"

	pluginActionRefresh     = "refresh"
	pluginActionOpenInstall = "open-install"
	pluginActionSelect      = "select"
	pluginActionEnable      = "enable"
	pluginActionDisable     = "disable"
	pluginActionAllowHere   = "allow-here"
	pluginActionDenyHere    = "deny-here"
	pluginActionOpenUpgrade = "open-upgrade"
	pluginActionOpenConfig  = "open-config"
	pluginActionRemove      = "remove"

	pluginModalInstall = "modal-install"
	pluginModalUpgrade = "modal-upgrade"
	pluginModalConfig  = "modal-config"

	pluginModalFieldRepo = "plugin:field-repo"
	pluginModalFieldRef  = "plugin:field-ref"
	pluginModalFieldPath = "plugin:field-path"

	pluginSelectOptionLimit   = 25
	pluginListPreviewLimit    = 10
	pluginCapabilityPreview   = 1200
	pluginDetailPreview       = 1024
	pluginDescriptionPreview  = 220
	pluginErrorPreview        = 900
	pluginEmbedColorInfo      = 0x2563EB
	pluginEmbedColorSuccess   = 0x059669
	pluginEmbedColorWarning   = 0xD97706
	pluginEmbedColorDanger    = 0xDC2626
	pluginPanelEmptySelection = "__none__"
)

func isPluginInteractionCustomID(customID string) bool {
	return strings.HasPrefix(strings.TrimSpace(customID), pluginComponentPrefix)
}

func (h *Handler) PluginPanelCommandResponse(authorID string, location speechLocation) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return pluginErrorInteractionResponse("插件宿主不可用", err.Error()), nil
	}
	if h.pluginManager == nil {
		return pluginErrorInteractionResponse("插件宿主不可用", "当前没有启用插件宿主。"), nil
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return pluginErrorInteractionResponse("权限不足", permissionDenied()), nil
	}

	data, err := h.pluginPanelResponseData(authorID, location, "", "")
	if err != nil {
		return nil, err
	}
	data.Flags = discordgo.MessageFlagsEphemeral
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}, nil
}

func (h *Handler) PluginComponentResponse(authorID string, location speechLocation, data discordgo.MessageComponentInteractionData) (*discordgo.InteractionResponse, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return pluginErrorInteractionResponse("插件宿主不可用", err.Error()), nil
	}
	if h.pluginManager == nil {
		return pluginErrorInteractionResponse("插件宿主不可用", "当前没有启用插件宿主。"), nil
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return pluginErrorInteractionResponse("权限不足", permissionDenied()), nil
	}

	action, selectedPluginID := pluginActionParts(data.CustomID)
	switch action {
	case pluginActionRefresh:
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "已刷新插件管理面板。")
	case pluginActionOpenInstall:
		return h.pluginInstallModalResponse(), nil
	case pluginActionOpenUpgrade:
		if !h.runtimeStore.IsSuperAdmin(authorID) {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, superAdminDenied())
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), selectedPluginID)
		if !ok {
			return h.pluginPanelUpdateResponse(authorID, location, "", "请先选择一个要升级的插件。")
		}
		return h.pluginUpgradeModalResponse(plugin), nil
	case pluginActionOpenConfig:
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), selectedPluginID)
		if !ok {
			return h.pluginPanelUpdateResponse(authorID, location, "", "请先选择一个要配置的插件。")
		}
		response, err := h.pluginConfigModalResponse(plugin)
		if err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "打开插件配置失败: "+err.Error())
		}
		return response, nil
	case pluginActionSelect:
		if len(data.Values) == 0 {
			return h.pluginPanelUpdateResponse(authorID, location, "", "请选择一个插件。")
		}
		selected := strings.TrimSpace(data.Values[0])
		if selected == "" || selected == pluginPanelEmptySelection {
			return h.pluginPanelUpdateResponse(authorID, location, "", "当前没有可选插件。")
		}
		return h.pluginPanelUpdateResponse(authorID, location, selected, "已切换当前选中的插件。")
	case pluginActionEnable:
		if !h.runtimeStore.IsSuperAdmin(authorID) {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, superAdminDenied())
		}
		if err := h.pluginManager.EnableGlobal(selectedPluginID); err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "启用插件失败: "+err.Error())
		}
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "已全局启用插件。")
	case pluginActionDisable:
		if !h.runtimeStore.IsSuperAdmin(authorID) {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, superAdminDenied())
		}
		if err := h.pluginManager.DisableGlobal(selectedPluginID); err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "禁用插件失败: "+err.Error())
		}
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "已全局禁用插件。")
	case pluginActionAllowHere:
		if strings.TrimSpace(location.GuildID) == "" {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "这个操作只能在服务器频道中使用。")
		}
		if err := h.pluginManager.AllowGuild(selectedPluginID, location.GuildID); err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "更新服务器作用域失败: "+err.Error())
		}
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "已允许该插件在当前服务器使用。")
	case pluginActionDenyHere:
		if strings.TrimSpace(location.GuildID) == "" {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "这个操作只能在服务器频道中使用。")
		}
		if err := h.pluginManager.DenyGuild(selectedPluginID, location.GuildID); err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "更新服务器作用域失败: "+err.Error())
		}
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "已禁止该插件在当前服务器使用。")
	case pluginActionRemove:
		if !h.runtimeStore.IsSuperAdmin(authorID) {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, superAdminDenied())
		}
		if err := h.pluginManager.Remove(selectedPluginID); err != nil {
			return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "卸载插件失败: "+err.Error())
		}
		return h.pluginPanelUpdateResponse(authorID, location, "", "已卸载插件。")
	default:
		return h.pluginPanelUpdateResponse(authorID, location, selectedPluginID, "未知的插件面板操作。")
	}
}

func (h *Handler) PluginModalEdit(ctx context.Context, authorID string, location speechLocation, data discordgo.ModalSubmitInteractionData) (*discordgo.WebhookEdit, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return pluginErrorEdit("插件宿主不可用", err.Error()), nil
	}
	if h.pluginManager == nil {
		return pluginErrorEdit("插件宿主不可用", "当前没有启用插件宿主。"), nil
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return pluginErrorEdit("权限不足", permissionDenied()), nil
	}

	action, selectedPluginID := pluginActionParts(data.CustomID)
	switch action {
	case pluginModalInstall:
		repo := pluginhost.NormalizeLocatorField(modalTextInputValue(data.Components, pluginModalFieldRepo), "repo")
		ref := pluginhost.NormalizeLocatorField(modalTextInputValue(data.Components, pluginModalFieldRef), "ref")
		path := pluginhost.NormalizeLocatorField(modalTextInputValue(data.Components, pluginModalFieldPath), "path")
		if strings.TrimSpace(repo) == "" {
			return h.pluginPanelEdit(authorID, location, "", "安装插件失败: 仓库地址不能为空。")
		}
		plugin, err := h.pluginManager.InstallFromGit(ctx, repo, ref, path)
		if err != nil {
			return h.pluginPanelEdit(authorID, location, "", "安装插件失败: "+err.Error())
		}
		return h.pluginPanelEdit(authorID, location, plugin.ID, "插件安装完成，并已刷新命令注册。")
	case pluginModalUpgrade:
		if !h.runtimeStore.IsSuperAdmin(authorID) {
			return pluginErrorEdit("权限不足", superAdminDenied()), nil
		}
		if strings.TrimSpace(selectedPluginID) == "" {
			return h.pluginPanelEdit(authorID, location, "", "升级插件失败: 没有目标插件。")
		}
		ref := pluginhost.NormalizeLocatorField(modalTextInputValue(data.Components, pluginModalFieldRef), "ref")
		plugin, err := h.pluginManager.UpgradeFromGit(ctx, selectedPluginID, ref)
		if err != nil {
			return h.pluginPanelEdit(authorID, location, selectedPluginID, "升级插件失败: "+err.Error())
		}
		return h.pluginPanelEdit(authorID, location, plugin.ID, "插件升级完成，并已刷新命令注册。")
	case pluginModalConfig:
		if strings.TrimSpace(selectedPluginID) == "" {
			return h.pluginPanelEdit(authorID, location, "", "保存插件配置失败: 没有目标插件。")
		}
		schema, err := h.pluginManager.ConfigSchema(selectedPluginID)
		if err != nil {
			return h.pluginPanelEdit(authorID, location, selectedPluginID, "保存插件配置失败: "+err.Error())
		}
		payload, err := pluginConfigJSONFromModal(schema, data.Components)
		if err != nil {
			return h.pluginPanelEdit(authorID, location, selectedPluginID, "保存插件配置失败: "+err.Error())
		}
		if err := h.pluginManager.SetConfig(selectedPluginID, payload); err != nil {
			return h.pluginPanelEdit(authorID, location, selectedPluginID, "保存插件配置失败: "+err.Error())
		}
		return h.pluginPanelEdit(authorID, location, selectedPluginID, "已保存插件配置。")
	default:
		return pluginErrorEdit("未知表单", "未知的插件管理表单。"), nil
	}
}

func (h *Handler) pluginPanelUpdateResponse(authorID string, location speechLocation, selectedPluginID, notice string) (*discordgo.InteractionResponse, error) {
	data, err := h.pluginPanelResponseData(authorID, location, selectedPluginID, notice)
	if err != nil {
		return nil, err
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	}, nil
}

func (h *Handler) pluginPanelEdit(authorID string, location speechLocation, selectedPluginID, notice string) (*discordgo.WebhookEdit, error) {
	data, err := h.pluginPanelResponseData(authorID, location, selectedPluginID, notice)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(data.Content)
	components := data.Components
	embeds := data.Embeds
	return &discordgo.WebhookEdit{
		Content:    &content,
		Components: &components,
		Embeds:     &embeds,
	}, nil
}

func (h *Handler) pluginInstallModalResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: pluginActionCustomID(pluginModalInstall, ""),
			Title:    "安装插件",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    pluginModalFieldRepo,
							Label:       "Git 仓库（只填 URL）",
							Style:       discordgo.TextInputShort,
							Placeholder: "https://github.com/owner/repo.git",
							Required:    true,
							MinLength:   1,
							MaxLength:   400,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    pluginModalFieldRef,
							Label:       "Ref（可选）",
							Style:       discordgo.TextInputShort,
							Placeholder: "例如 main / v1.0.0 / commit",
							Required:    false,
							MaxLength:   120,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    pluginModalFieldPath,
							Label:       "仓库子目录（只填路径）",
							Style:       discordgo.TextInputShort,
							Placeholder: "例如 plugins/persona",
							Required:    false,
							MaxLength:   200,
						},
					},
				},
			},
		},
	}
}

func (h *Handler) pluginUpgradeModalResponse(plugin pluginhost.InstalledPlugin) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: pluginActionCustomID(pluginModalUpgrade, plugin.ID),
			Title:    truncateRunes("升级插件: "+plugin.ID, 45),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    pluginModalFieldRef,
							Label:       "目标 Ref（可选）",
							Style:       discordgo.TextInputShort,
							Placeholder: "留空则沿用当前记录的 Ref",
							Value:       strings.TrimSpace(plugin.Ref),
							Required:    false,
							MaxLength:   120,
						},
					},
				},
			},
		},
	}
}

func (h *Handler) pluginConfigModalResponse(plugin pluginhost.InstalledPlugin) (*discordgo.InteractionResponse, error) {
	schema, err := h.pluginManager.ConfigSchema(plugin.ID)
	if err != nil {
		return nil, err
	}
	if len(schema.Properties) == 0 {
		return nil, fmt.Errorf("插件没有声明 config_schema")
	}
	if len(schema.Properties) > 5 {
		return nil, fmt.Errorf("当前仅支持最多 5 个配置字段，插件声明了 %d 个", len(schema.Properties))
	}

	current, _, err := h.pluginManager.ConfigValue(plugin.ID)
	if err != nil {
		return nil, err
	}
	rows, err := pluginConfigModalComponents(schema, current)
	if err != nil {
		return nil, err
	}

	title := "配置插件: " + plugin.ID
	if schema.Title != "" {
		title = schema.Title
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   pluginActionCustomID(pluginModalConfig, plugin.ID),
			Title:      truncateRunes(title, 45),
			Components: rows,
		},
	}, nil
}

func (h *Handler) pluginPanelResponseData(authorID string, location speechLocation, selectedPluginID, notice string) (*discordgo.InteractionResponseData, error) {
	plugins := h.pluginManager.List()
	selected, hasSelected := resolveSelectedPlugin(plugins, selectedPluginID)

	return &discordgo.InteractionResponseData{
		Content:    strings.TrimSpace(notice),
		Embeds:     []*discordgo.MessageEmbed{buildPluginPanelEmbed(plugins, selected, hasSelected, location, h.runtimeStore.IsSuperAdmin(authorID), notice)},
		Components: buildPluginPanelComponents(plugins, selected, hasSelected, location, h.runtimeStore.IsAdmin(authorID), h.runtimeStore.IsSuperAdmin(authorID)),
	}, nil
}

func buildPluginPanelEmbed(plugins []pluginhost.InstalledPlugin, selected pluginhost.InstalledPlugin, hasSelected bool, location speechLocation, isSuperAdmin bool, notice string) *discordgo.MessageEmbed {
	description := "统一插件管理面板。使用下方选择菜单切换当前插件，再通过按钮完成安装、升级、配置、启用、禁用和当前服务器授权。"
	if !isSuperAdmin {
		description += "\n\n管理员可以安装插件、查看详情和设置当前服务器授权；升级、启用、禁用、卸载仍然只允许超级管理员执行。"
	}
	if strings.TrimSpace(notice) != "" {
		description += "\n\n提示: " + strings.TrimSpace(notice)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "已安装插件", Value: fmt.Sprintf("%d", len(plugins)), Inline: true},
		{Name: "已启用", Value: fmt.Sprintf("%d", countEnabledPlugins(plugins)), Inline: true},
		{Name: "最近有错误", Value: fmt.Sprintf("%d", countErroredPlugins(plugins)), Inline: true},
		{Name: "当前位置", Value: pluginLocationLabel(location), Inline: false},
		{Name: fmt.Sprintf("插件列表预览 (%d)", len(plugins)), Value: pluginListPreviewValue(plugins), Inline: false},
	}

	if hasSelected {
		fields = append(fields,
			&discordgo.MessageEmbedField{Name: "当前选中", Value: pluginSelectedSummary(selected, location.GuildID), Inline: false},
			&discordgo.MessageEmbedField{Name: "命令", Value: pluginCommandList(selected), Inline: false},
			&discordgo.MessageEmbedField{Name: "授权能力", Value: pluginCapabilityList(selected.GrantedCaps), Inline: false},
		)
		if strings.TrimSpace(selected.LastError) != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "最近错误",
				Value:  "```text\n" + truncateRunes(selected.LastError, pluginErrorPreview) + "\n```",
				Inline: false,
			})
		}
	} else {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "当前选中",
			Value:  "当前没有已安装插件。管理员可以直接点击下方 `安装` 打开表单。",
			Inline: false,
		})
	}

	if len(plugins) > pluginSelectOptionLimit {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "提示",
			Value:  fmt.Sprintf("选择菜单最多显示前 %d 个插件，当前共有 %d 个。", pluginSelectOptionLimit, len(plugins)),
			Inline: false,
		})
	}

	return &discordgo.MessageEmbed{
		Title:       "Plugin Control Center",
		Description: description,
		Color:       pluginPanelColor(selected, hasSelected),
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "安装与升级通过 Modal 输入 Git 仓库信息；启用/禁用会影响全局命令注册。",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func buildPluginPanelComponents(plugins []pluginhost.InstalledPlugin, selected pluginhost.InstalledPlugin, hasSelected bool, location speechLocation, isAdmin, isSuperAdmin bool) []discordgo.MessageComponent {
	selectedID := ""
	configEnabled := false
	if hasSelected {
		selectedID = selected.ID
		configEnabled = pluginSupportsConfig(selected)
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "安装",
					Style:    discordgo.SuccessButton,
					CustomID: pluginActionCustomID(pluginActionOpenInstall, ""),
					Disabled: !isAdmin,
				},
				discordgo.Button{
					Label:    "升级",
					Style:    discordgo.PrimaryButton,
					CustomID: pluginActionCustomID(pluginActionOpenUpgrade, selectedID),
					Disabled: !isSuperAdmin || !hasSelected,
				},
				discordgo.Button{
					Label:    "配置",
					Style:    discordgo.SecondaryButton,
					CustomID: pluginActionCustomID(pluginActionOpenConfig, selectedID),
					Disabled: !isAdmin || !hasSelected || !configEnabled,
				},
				discordgo.Button{
					Label:    "卸载",
					Style:    discordgo.DangerButton,
					CustomID: pluginActionCustomID(pluginActionRemove, selectedID),
					Disabled: !isSuperAdmin || !hasSelected,
				},
				discordgo.Button{
					Label:    "刷新",
					Style:    discordgo.SecondaryButton,
					CustomID: pluginActionCustomID(pluginActionRefresh, selectedID),
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "启用",
					Style:    discordgo.SuccessButton,
					CustomID: pluginActionCustomID(pluginActionEnable, selectedID),
					Disabled: !isSuperAdmin || !hasSelected || selected.Enabled,
				},
				discordgo.Button{
					Label:    "禁用",
					Style:    discordgo.SecondaryButton,
					CustomID: pluginActionCustomID(pluginActionDisable, selectedID),
					Disabled: !isSuperAdmin || !hasSelected || !selected.Enabled,
				},
				discordgo.Button{
					Label:    "当前服务器允许",
					Style:    discordgo.PrimaryButton,
					CustomID: pluginActionCustomID(pluginActionAllowHere, selectedID),
					Disabled: !isAdmin || !hasSelected || strings.TrimSpace(location.GuildID) == "",
				},
				discordgo.Button{
					Label:    "当前服务器禁止",
					Style:    discordgo.DangerButton,
					CustomID: pluginActionCustomID(pluginActionDenyHere, selectedID),
					Disabled: !isAdmin || !hasSelected || strings.TrimSpace(location.GuildID) == "",
				},
			},
		},
	}

	if selectRow := buildPluginSelectRow(plugins, selectedID, isAdmin); selectRow != nil {
		components = append(components, selectRow)
	}
	return components
}

func buildPluginSelectRow(plugins []pluginhost.InstalledPlugin, selectedPluginID string, isAdmin bool) discordgo.MessageComponent {
	if len(plugins) == 0 {
		return nil
	}

	options := make([]discordgo.SelectMenuOption, 0, minInt(len(plugins), pluginSelectOptionLimit))
	for index, plugin := range plugins {
		if index >= pluginSelectOptionLimit {
			break
		}
		label := strings.TrimSpace(plugin.Name)
		if label == "" {
			label = plugin.ID
		}
		description := truncateRunes(strings.Join([]string{
			firstNonEmpty(strings.TrimSpace(plugin.Version), "unknown"),
			pluginStatusShort(plugin),
			pluginScopeShort(plugin),
		}, " | "), 100)
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateRunes(label, 100),
			Value:       plugin.ID,
			Description: description,
			Default:     plugin.ID == selectedPluginID,
		})
	}
	if len(options) == 0 {
		return nil
	}

	minValues := 1
	placeholder := "选择插件并查看详情"
	if strings.TrimSpace(selectedPluginID) != "" {
		placeholder = "当前选中: " + truncateRunes(selectedPluginID, 80)
	}

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:    discordgo.StringSelectMenu,
				CustomID:    pluginActionCustomID(pluginActionSelect, ""),
				Placeholder: placeholder,
				MinValues:   &minValues,
				MaxValues:   1,
				Options:     options,
				Disabled:    !isAdmin,
			},
		},
	}
}

func pluginActionCustomID(action, pluginID string) string {
	action = strings.TrimSpace(action)
	pluginID = strings.TrimSpace(pluginID)
	if action == "" {
		return pluginComponentPrefix
	}
	if pluginID == "" {
		return pluginComponentPrefix + action
	}
	return pluginComponentPrefix + action + ":" + pluginID
}

func pluginActionParts(customID string) (string, string) {
	customID = strings.TrimSpace(customID)
	if !strings.HasPrefix(customID, pluginComponentPrefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(customID, pluginComponentPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) == 0 {
		return "", ""
	}
	action := strings.TrimSpace(parts[0])
	pluginID := ""
	if len(parts) > 1 {
		pluginID = strings.TrimSpace(parts[1])
	}
	return action, pluginID
}

func resolveSelectedPlugin(plugins []pluginhost.InstalledPlugin, selectedPluginID string) (pluginhost.InstalledPlugin, bool) {
	if plugin, ok := findInstalledPlugin(plugins, selectedPluginID); ok {
		return plugin, true
	}
	if len(plugins) == 0 {
		return pluginhost.InstalledPlugin{}, false
	}
	return plugins[0], true
}

func pluginListPreviewValue(plugins []pluginhost.InstalledPlugin) string {
	if len(plugins) == 0 {
		return "暂无已安装插件。"
	}
	lines := make([]string, 0, minInt(len(plugins), pluginListPreviewLimit)+1)
	for index, plugin := range plugins {
		if index >= pluginListPreviewLimit {
			break
		}
		lines = append(lines, fmt.Sprintf("• %s (%s)", pluginPanelTitle(plugin), pluginStatusShort(plugin)))
	}
	if len(plugins) > pluginListPreviewLimit {
		lines = append(lines, fmt.Sprintf("… 还有 %d 个未展开", len(plugins)-pluginListPreviewLimit))
	}
	return strings.Join(lines, "\n")
}

func pluginSelectedSummary(plugin pluginhost.InstalledPlugin, guildID string) string {
	lines := []string{
		"ID: " + firstNonEmpty(codeValue(plugin.ID), "`unknown`"),
		"名称: " + firstNonEmpty(codeValue(plugin.Name), "`unknown`"),
		"版本: " + firstNonEmpty(codeValue(plugin.Version), "`unknown`"),
		"状态: " + pluginStatusLabel(plugin),
		"作用域: " + pluginScopeLabel(plugin),
	}
	if strings.TrimSpace(guildID) != "" {
		lines = append(lines, "当前服务器可用: "+boolLabel(pluginAllowsGuild(plugin, guildID)))
	}
	if strings.TrimSpace(plugin.Repo) != "" {
		lines = append(lines, "仓库: "+codeValue(plugin.Repo))
	}
	if strings.TrimSpace(plugin.Ref) != "" {
		lines = append(lines, "Ref: "+codeValue(plugin.Ref))
	}
	if strings.TrimSpace(plugin.SourcePath) != "" {
		lines = append(lines, "路径: "+codeValue(plugin.SourcePath))
	}
	if configSummary := pluginConfigSummary(plugin); configSummary != "" {
		lines = append(lines, "配置: "+configSummary)
	}
	if strings.TrimSpace(plugin.Description) != "" {
		lines = append(lines, "说明: "+truncateRunes(singleLine(plugin.Description), pluginDescriptionPreview))
	}
	return truncateRunes(strings.Join(lines, "\n"), pluginDetailPreview)
}

func pluginLocationLabel(location speechLocation) string {
	if strings.TrimSpace(location.GuildID) == "" {
		return "当前在私聊或非服务器上下文。`当前服务器允许/禁止` 按钮将不可用。"
	}
	lines := []string{
		"服务器: " + codeValue(location.GuildID),
		"频道: " + firstNonEmpty(codeValue(location.ChannelID), "无"),
	}
	if strings.TrimSpace(location.ThreadID) != "" {
		lines = append(lines, "子区: "+codeValue(location.ThreadID))
	}
	return strings.Join(lines, "\n")
}

func pluginStatusShort(plugin pluginhost.InstalledPlugin) string {
	if plugin.Enabled {
		return "启用"
	}
	return "禁用"
}

func pluginScopeShort(plugin pluginhost.InstalledPlugin) string {
	switch strings.TrimSpace(plugin.GuildMode) {
	case pluginhost.GuildModeAllowlist:
		return "白名单"
	case pluginhost.GuildModeDenylist:
		return "黑名单"
	default:
		return "全局"
	}
}

func pluginStatusLabel(plugin pluginhost.InstalledPlugin) string {
	switch {
	case plugin.Enabled && strings.TrimSpace(plugin.LastError) != "":
		return "已启用，但最近有错误"
	case plugin.Enabled:
		return "已启用"
	default:
		return "已禁用"
	}
}

func pluginScopeLabel(plugin pluginhost.InstalledPlugin) string {
	switch strings.TrimSpace(plugin.GuildMode) {
	case pluginhost.GuildModeAllowlist:
		if len(plugin.GuildIDs) == 0 {
			return "白名单模式（当前为空）"
		}
		return "白名单: " + pluginGuildIDSummary(plugin.GuildIDs)
	case pluginhost.GuildModeDenylist:
		if len(plugin.GuildIDs) == 0 {
			return "黑名单模式（当前为空）"
		}
		return "黑名单: " + pluginGuildIDSummary(plugin.GuildIDs)
	default:
		return "全部服务器"
	}
}

func pluginGuildIDSummary(ids []string) string {
	if len(ids) == 0 {
		return "无"
	}
	items := make([]string, 0, len(ids))
	for index, id := range ids {
		if index >= 5 {
			items = append(items, fmt.Sprintf("... 共 %d 个", len(ids)))
			break
		}
		items = append(items, codeValue(id))
	}
	return strings.Join(items, ", ")
}

func pluginCommandList(plugin pluginhost.InstalledPlugin) string {
	if len(plugin.Manifest.Commands) == 0 {
		return "无"
	}
	items := make([]string, 0, len(plugin.Manifest.Commands))
	for _, command := range plugin.Manifest.Commands {
		name := strings.TrimSpace(command.Name)
		if name == "" {
			continue
		}
		items = append(items, "`/"+name+"`")
	}
	if len(items) == 0 {
		return "无"
	}
	return strings.Join(items, ", ")
}

func pluginCapabilityList(caps []pluginapi.Capability) string {
	if len(caps) == 0 {
		return "无"
	}
	lines := make([]string, 0, len(caps))
	for _, cap := range caps {
		lines = append(lines, "- `"+strings.TrimSpace(string(cap))+"`")
	}
	return truncateRunes(strings.Join(lines, "\n"), pluginCapabilityPreview)
}

func pluginSupportsConfig(plugin pluginhost.InstalledPlugin) bool {
	schema, err := pluginhost.ParseConfigSchema(plugin.Manifest.ConfigSchema)
	return err == nil && len(schema.Properties) > 0 && len(schema.Properties) <= 5
}

func pluginConfigSummary(plugin pluginhost.InstalledPlugin) string {
	schema, err := pluginhost.ParseConfigSchema(plugin.Manifest.ConfigSchema)
	if err != nil {
		return "schema 无效"
	}
	if len(schema.Properties) == 0 {
		return ""
	}
	if len(schema.Properties) > 5 {
		return fmt.Sprintf("%d 个字段（超出当前面板支持上限 5）", len(schema.Properties))
	}
	return fmt.Sprintf("%d 个字段", len(schema.Properties))
}

func pluginConfigModalComponents(schema pluginhost.ConfigSchema, current json.RawMessage) ([]discordgo.MessageComponent, error) {
	values := map[string]json.RawMessage{}
	current = json.RawMessage(strings.TrimSpace(string(current)))
	if len(current) > 0 && string(current) != "null" {
		if err := json.Unmarshal(current, &values); err != nil {
			return nil, err
		}
	}

	rows := make([]discordgo.MessageComponent, 0, len(schema.Properties))
	for _, property := range schema.Properties {
		value := property.Default
		if currentValue, ok := values[property.Name]; ok {
			value = currentValue
		}
		valueText, err := pluginConfigDisplayValue(property.Type, value)
		if err != nil {
			return nil, fmt.Errorf("配置字段 %s 当前值无效: %w", property.Name, err)
		}
		style := discordgo.TextInputShort
		if property.Type == "string" && (len(valueText) > 80 || len(property.Description) > 70) {
			style = discordgo.TextInputParagraph
		}
		label := firstNonEmpty(strings.TrimSpace(property.Title), strings.TrimSpace(property.Name))
		rows = append(rows, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    pluginConfigFieldCustomID(property.Name),
					Label:       truncateRunes(label, 45),
					Style:       style,
					Placeholder: truncateRunes(pluginConfigPlaceholder(property), 100),
					Value:       truncateRunes(valueText, 4000),
					Required:    property.Required,
					MaxLength:   4000,
				},
			},
		})
	}
	return rows, nil
}

func pluginConfigJSONFromModal(schema pluginhost.ConfigSchema, components []discordgo.MessageComponent) (json.RawMessage, error) {
	values := make(map[string]json.RawMessage, len(schema.Properties))
	for _, property := range schema.Properties {
		input := modalTextInputValue(components, pluginConfigFieldCustomID(property.Name))
		raw, hasValue, err := pluginConfigRawValue(property.Type, input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", firstNonEmpty(strings.TrimSpace(property.Title), property.Name), err)
		}
		if hasValue {
			values[property.Name] = raw
		}
	}
	if len(values) == 0 {
		return json.RawMessage(`{}`), nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func pluginConfigFieldCustomID(name string) string {
	return "plugin:config:" + strings.TrimSpace(name)
}

func pluginConfigPlaceholder(property pluginhost.ConfigProperty) string {
	typeHint := map[string]string{
		"string":  "文本",
		"integer": "整数",
		"number":  "数字",
		"boolean": "true / false",
	}[property.Type]
	description := strings.TrimSpace(property.Description)
	if description == "" {
		return typeHint
	}
	if typeHint == "" {
		return description
	}
	return description + " | " + typeHint
}

func pluginConfigDisplayValue(propertyType string, value json.RawMessage) (string, error) {
	value = json.RawMessage(strings.TrimSpace(string(value)))
	if len(value) == 0 {
		return "", nil
	}
	switch propertyType {
	case "string":
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return "", err
		}
		return text, nil
	case "integer", "number":
		return strings.TrimSpace(string(value)), nil
	case "boolean":
		var flag bool
		if err := json.Unmarshal(value, &flag); err != nil {
			return "", err
		}
		if flag {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("unsupported config type %s", propertyType)
	}
}

func pluginConfigRawValue(propertyType, input string) (json.RawMessage, bool, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, false, nil
	}
	switch propertyType {
	case "string":
		payload, err := json.Marshal(input)
		return payload, true, err
	case "integer":
		number, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return nil, false, fmt.Errorf("必须是整数")
		}
		return json.RawMessage(strconv.FormatInt(number, 10)), true, nil
	case "number":
		number, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return nil, false, fmt.Errorf("必须是数字")
		}
		return json.RawMessage(strconv.FormatFloat(number, 'f', -1, 64)), true, nil
	case "boolean":
		switch strings.ToLower(input) {
		case "true", "1", "yes", "y", "on":
			return json.RawMessage("true"), true, nil
		case "false", "0", "no", "n", "off":
			return json.RawMessage("false"), true, nil
		default:
			return nil, false, fmt.Errorf("必须是 true 或 false")
		}
	default:
		return nil, false, fmt.Errorf("不支持的配置类型 %s", propertyType)
	}
}

func pluginPanelColor(selected pluginhost.InstalledPlugin, hasSelected bool) int {
	if !hasSelected {
		return pluginEmbedColorInfo
	}
	switch {
	case strings.TrimSpace(selected.LastError) != "":
		return pluginEmbedColorWarning
	case selected.Enabled:
		return pluginEmbedColorSuccess
	default:
		return pluginEmbedColorInfo
	}
}

func pluginAllowsGuild(plugin pluginhost.InstalledPlugin, guildID string) bool {
	if !plugin.Enabled {
		return false
	}
	guildID = strings.TrimSpace(guildID)
	switch strings.TrimSpace(plugin.GuildMode) {
	case pluginhost.GuildModeAllowlist:
		return guildID != "" && containsTrimmedString(plugin.GuildIDs, guildID)
	case pluginhost.GuildModeDenylist:
		return guildID == "" || !containsTrimmedString(plugin.GuildIDs, guildID)
	default:
		return true
	}
}

func pluginPanelTitle(plugin pluginhost.InstalledPlugin) string {
	name := strings.TrimSpace(plugin.Name)
	if name == "" {
		name = "未命名插件"
	}
	if strings.TrimSpace(plugin.ID) == "" {
		return name
	}
	return name + " / " + plugin.ID
}

func pluginEmbedsEdit(embeds ...*discordgo.MessageEmbed) *discordgo.WebhookEdit {
	cleaned := make([]*discordgo.MessageEmbed, 0, len(embeds))
	for _, embed := range embeds {
		if embed == nil {
			continue
		}
		cleaned = append(cleaned, embed)
	}
	empty := ""
	return &discordgo.WebhookEdit{
		Content: &empty,
		Embeds:  &cleaned,
	}
}

func pluginErrorEdit(title, message string) *discordgo.WebhookEdit {
	return pluginEmbedsEdit(&discordgo.MessageEmbed{
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(message),
		Color:       pluginEmbedColorDanger,
	})
}

func pluginErrorInteractionResponse(title, message string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       strings.TrimSpace(title),
					Description: strings.TrimSpace(message),
					Color:       pluginEmbedColorDanger,
				},
			},
		},
	}
}

func countEnabledPlugins(plugins []pluginhost.InstalledPlugin) int {
	count := 0
	for _, plugin := range plugins {
		if plugin.Enabled {
			count++
		}
	}
	return count
}

func countErroredPlugins(plugins []pluginhost.InstalledPlugin) int {
	count := 0
	for _, plugin := range plugins {
		if strings.TrimSpace(plugin.LastError) != "" {
			count++
		}
	}
	return count
}

func findInstalledPlugin(plugins []pluginhost.InstalledPlugin, pluginID string) (pluginhost.InstalledPlugin, bool) {
	pluginID = strings.TrimSpace(pluginID)
	for _, plugin := range plugins {
		if strings.TrimSpace(plugin.ID) == pluginID {
			return plugin, true
		}
	}
	return pluginhost.InstalledPlugin{}, false
}

func codeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "`" + value + "`"
}

func boolLabel(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func containsTrimmedString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
