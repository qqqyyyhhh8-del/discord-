package bot

import (
	"context"
	"fmt"
	"strings"

	"discordbot/internal/pluginhost"
	"discordbot/pkg/pluginapi"

	"github.com/bwmarrin/discordgo"
)

const (
	pluginEmbedColorInfo    = 0x2563EB
	pluginEmbedColorSuccess = 0x059669
	pluginEmbedColorWarning = 0xD97706
	pluginEmbedColorDanger  = 0xDC2626
	pluginFieldGroupSize    = 4
)

func (h *Handler) handlePluginCommand(ctx context.Context, authorID string, location speechLocation, options []*discordgo.ApplicationCommandInteractionDataOption) (*discordgo.WebhookEdit, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return pluginErrorEdit("插件宿主不可用", err.Error()), nil
	}
	if h.pluginManager == nil {
		return pluginErrorEdit("插件宿主不可用", "当前没有启用插件宿主。"), nil
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return pluginErrorEdit("权限不足", permissionDenied()), nil
	}

	subcommand, optionMap, err := slashSubcommand(options)
	if err != nil {
		return pluginHelpEdit(), nil
	}

	switch subcommand {
	case "list":
		return pluginListEdit(h.pluginManager.List()), nil
	case "permissions":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !ok {
			return pluginErrorEdit("未找到插件", "插件不存在: `"+strings.TrimSpace(pluginID)+"`"), nil
		}
		return pluginPermissionsEdit(plugin), nil
	case "allow_here":
		if strings.TrimSpace(location.GuildID) == "" {
			return pluginErrorEdit("位置无效", "这个命令只能在服务器频道中使用。"), nil
		}
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		if err := h.pluginManager.AllowGuild(pluginID, location.GuildID); err != nil {
			return pluginErrorEdit("更新插件作用域失败", err.Error()), nil
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !ok {
			return pluginErrorEdit("更新插件作用域失败", "插件状态已更新，但刷新后的插件记录不可用。"), nil
		}
		return pluginActionEdit(
			"插件作用域已更新",
			"已允许插件在当前服务器使用。",
			pluginEmbedColorSuccess,
			plugin,
			[]*discordgo.MessageEmbedField{
				{Name: "当前服务器 ID", Value: codeValue(location.GuildID), Inline: false},
				{Name: "操作", Value: "allow_here", Inline: true},
			},
		), nil
	case "deny_here":
		if strings.TrimSpace(location.GuildID) == "" {
			return pluginErrorEdit("位置无效", "这个命令只能在服务器频道中使用。"), nil
		}
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		if err := h.pluginManager.DenyGuild(pluginID, location.GuildID); err != nil {
			return pluginErrorEdit("更新插件作用域失败", err.Error()), nil
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !ok {
			return pluginErrorEdit("更新插件作用域失败", "插件状态已更新，但刷新后的插件记录不可用。"), nil
		}
		return pluginActionEdit(
			"插件作用域已更新",
			"已禁止插件在当前服务器使用。",
			pluginEmbedColorWarning,
			plugin,
			[]*discordgo.MessageEmbedField{
				{Name: "当前服务器 ID", Value: codeValue(location.GuildID), Inline: false},
				{Name: "操作", Value: "deny_here", Inline: true},
			},
		), nil
	}

	if !h.runtimeStore.IsSuperAdmin(authorID) {
		return pluginErrorEdit("权限不足", superAdminDenied()), nil
	}

	switch subcommand {
	case "install":
		repo, ok := slashStringOption(optionMap, "repo")
		if !ok {
			return pluginHelpEdit(), nil
		}
		ref, _ := slashStringOption(optionMap, "ref")
		path, _ := slashStringOption(optionMap, "path")
		plugin, err := h.pluginManager.InstallFromGit(ctx, repo, ref, path)
		if err != nil {
			return pluginErrorEdit("安装插件失败", err.Error()), nil
		}
		plugin = refreshedPlugin(h.pluginManager, plugin)
		fields := []*discordgo.MessageEmbedField{
			{Name: "仓库", Value: codeValue(plugin.Repo), Inline: false},
		}
		if strings.TrimSpace(plugin.Ref) != "" {
			fields = append(fields, &discordgo.MessageEmbedField{Name: "Ref", Value: codeValue(plugin.Ref), Inline: true})
		}
		if strings.TrimSpace(plugin.SourcePath) != "" {
			fields = append(fields, &discordgo.MessageEmbedField{Name: "路径", Value: codeValue(plugin.SourcePath), Inline: true})
		}
		return pluginActionEdit("插件已安装", "插件安装完成，并已刷新 Slash 命令注册。", pluginEmbedColorSuccess, plugin, fields), nil
	case "upgrade":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		ref, _ := slashStringOption(optionMap, "ref")
		plugin, err := h.pluginManager.UpgradeFromGit(ctx, pluginID, ref)
		if err != nil {
			return pluginErrorEdit("升级插件失败", err.Error()), nil
		}
		plugin = refreshedPlugin(h.pluginManager, plugin)
		fields := []*discordgo.MessageEmbedField{
			{Name: "操作", Value: "upgrade", Inline: true},
		}
		if strings.TrimSpace(ref) != "" {
			fields = append(fields, &discordgo.MessageEmbedField{Name: "目标 Ref", Value: codeValue(ref), Inline: true})
		}
		return pluginActionEdit("插件已升级", "插件源码与命令注册已刷新。", pluginEmbedColorSuccess, plugin, fields), nil
	case "remove":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		plugin, found := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !found {
			return pluginErrorEdit("卸载插件失败", "插件不存在: `"+strings.TrimSpace(pluginID)+"`"), nil
		}
		if err := h.pluginManager.Remove(pluginID); err != nil {
			return pluginErrorEdit("卸载插件失败", err.Error()), nil
		}
		return pluginActionEdit(
			"插件已卸载",
			"插件源码、进程与命令注册都已移除。",
			pluginEmbedColorWarning,
			plugin,
			[]*discordgo.MessageEmbedField{
				{Name: "操作", Value: "remove", Inline: true},
			},
		), nil
	case "enable":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		if err := h.pluginManager.EnableGlobal(pluginID); err != nil {
			return pluginErrorEdit("启用插件失败", err.Error()), nil
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !ok {
			return pluginErrorEdit("启用插件失败", "插件已启用，但刷新后的插件记录不可用。"), nil
		}
		return pluginActionEdit(
			"插件已启用",
			"插件已经全局启用，并已刷新 Slash 命令注册。",
			pluginEmbedColorSuccess,
			plugin,
			[]*discordgo.MessageEmbedField{
				{Name: "操作", Value: "enable", Inline: true},
			},
		), nil
	case "disable":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelpEdit(), nil
		}
		if err := h.pluginManager.DisableGlobal(pluginID); err != nil {
			return pluginErrorEdit("禁用插件失败", err.Error()), nil
		}
		plugin, ok := findInstalledPlugin(h.pluginManager.List(), pluginID)
		if !ok {
			return pluginActionEdit(
				"插件已禁用",
				"插件已经全局禁用，并已刷新 Slash 命令注册。",
				pluginEmbedColorWarning,
				pluginhost.InstalledPlugin{ID: strings.TrimSpace(pluginID), Enabled: false},
				[]*discordgo.MessageEmbedField{
					{Name: "操作", Value: "disable", Inline: true},
				},
			), nil
		}
		return pluginActionEdit(
			"插件已禁用",
			"插件已经全局禁用，并已刷新 Slash 命令注册。",
			pluginEmbedColorWarning,
			plugin,
			[]*discordgo.MessageEmbedField{
				{Name: "操作", Value: "disable", Inline: true},
			},
		), nil
	default:
		return pluginHelpEdit(), nil
	}
}

func pluginListEdit(plugins []pluginhost.InstalledPlugin) *discordgo.WebhookEdit {
	embeds := []*discordgo.MessageEmbed{
		{
			Title:       "Plugin Control Center",
			Description: "管理外部插件的安装、启用状态、授权能力和服务器作用域。",
			Color:       pluginEmbedColorInfo,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "已安装插件", Value: fmt.Sprintf("%d", len(plugins)), Inline: true},
				{Name: "已启用", Value: fmt.Sprintf("%d", countEnabledPlugins(plugins)), Inline: true},
				{Name: "最近有错误", Value: fmt.Sprintf("%d", countErroredPlugins(plugins)), Inline: true},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "使用 /plugin permissions 查看单个插件的详细授权能力。",
			},
		},
	}

	if len(plugins) == 0 {
		embeds[0].Fields = append(embeds[0].Fields, &discordgo.MessageEmbedField{
			Name:   "当前状态",
			Value:  "当前还没有已安装插件。可以使用 `/plugin install` 从 Git 仓库安装官方插件或第三方插件。",
			Inline: false,
		})
		return pluginEmbedsEdit(embeds...)
	}

	for start := 0; start < len(plugins); start += pluginFieldGroupSize {
		end := start + pluginFieldGroupSize
		if end > len(plugins) {
			end = len(plugins)
		}
		fields := make([]*discordgo.MessageEmbedField, 0, end-start)
		for _, plugin := range plugins[start:end] {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   truncateRunes(pluginPanelTitle(plugin), 256),
				Value:  truncateRunes(pluginPanelValue(plugin), 1024),
				Inline: false,
			})
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:  fmt.Sprintf("插件明细 %d-%d", start+1, end),
			Color:  pluginEmbedColorInfo,
			Fields: fields,
		})
	}

	return pluginEmbedsEdit(embeds...)
}

func pluginPermissionsEdit(plugin pluginhost.InstalledPlugin) *discordgo.WebhookEdit {
	description := "查看插件当前声明并授予的能力。"
	if strings.TrimSpace(plugin.Description) != "" {
		description += "\n\n" + strings.TrimSpace(plugin.Description)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "插件", Value: pluginIdentityValue(plugin), Inline: false},
		{Name: "状态", Value: pluginStatusLabel(plugin), Inline: true},
		{Name: "作用域", Value: pluginScopeLabel(plugin), Inline: true},
		{Name: "命令", Value: pluginCommandList(plugin), Inline: false},
		{Name: "授权能力", Value: pluginCapabilityList(plugin.GrantedCaps), Inline: false},
	}
	if strings.TrimSpace(plugin.LastError) != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "最近错误",
			Value:  "```text\n" + truncateRunes(plugin.LastError, 980) + "\n```",
			Inline: false,
		})
	}

	return pluginEmbedsEdit(&discordgo.MessageEmbed{
		Title:       "Plugin Permissions",
		Description: description,
		Color:       pluginColor(plugin),
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "能力授权来源于插件 manifest。",
		},
	})
}

func pluginActionEdit(title, description string, color int, plugin pluginhost.InstalledPlugin, extraFields []*discordgo.MessageEmbedField) *discordgo.WebhookEdit {
	fields := []*discordgo.MessageEmbedField{
		{Name: "插件", Value: pluginIdentityValue(plugin), Inline: false},
		{Name: "状态", Value: pluginStatusLabel(plugin), Inline: true},
		{Name: "作用域", Value: pluginScopeLabel(plugin), Inline: true},
	}
	if commands := pluginCommandList(plugin); commands != "无" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "命令", Value: commands, Inline: false})
	}
	fields = append(fields, extraFields...)
	if strings.TrimSpace(plugin.LastError) != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "最近错误",
			Value:  "```text\n" + truncateRunes(plugin.LastError, 980) + "\n```",
			Inline: false,
		})
	}
	return pluginEmbedsEdit(&discordgo.MessageEmbed{
		Title:       title,
		Description: strings.TrimSpace(description),
		Color:       color,
		Fields:      fields,
	})
}

func pluginHelpEdit() *discordgo.WebhookEdit {
	return pluginEmbedsEdit(&discordgo.MessageEmbed{
		Title:       "Plugin Manager Help",
		Description: "通过 `/plugin` 管理插件安装、启用状态、授权能力和服务器作用域。",
		Color:       pluginEmbedColorInfo,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "查看", Value: "`/plugin list`\n`/plugin permissions plugin:<id>`", Inline: false},
			{Name: "当前服务器作用域", Value: "`/plugin allow_here plugin:<id>`\n`/plugin deny_here plugin:<id>`", Inline: false},
			{Name: "超级管理员操作", Value: "`/plugin install repo:<repo> [ref] [path]`\n`/plugin upgrade plugin:<id> [ref]`\n`/plugin enable plugin:<id>`\n`/plugin disable plugin:<id>`\n`/plugin remove plugin:<id>`", Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "allow_here / deny_here 需要在目标服务器内执行。",
		},
	})
}

func pluginErrorEdit(title, message string) *discordgo.WebhookEdit {
	return pluginEmbedsEdit(&discordgo.MessageEmbed{
		Title:       title,
		Description: strings.TrimSpace(message),
		Color:       pluginEmbedColorDanger,
	})
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

func pluginPanelTitle(plugin pluginhost.InstalledPlugin) string {
	name := strings.TrimSpace(plugin.Name)
	if name == "" {
		name = "未命名插件"
	}
	if strings.TrimSpace(plugin.ID) == "" {
		return name
	}
	return fmt.Sprintf("%s (`%s`)", name, plugin.ID)
}

func pluginPanelValue(plugin pluginhost.InstalledPlugin) string {
	lines := []string{
		"状态: " + pluginStatusLabel(plugin),
		"版本: " + firstNonEmpty(codeValue(plugin.Version), "`unknown`"),
		"作用域: " + pluginScopeLabel(plugin),
		"命令: " + pluginCommandList(plugin),
		fmt.Sprintf("能力数: `%d`", len(plugin.GrantedCaps)),
	}
	if strings.TrimSpace(plugin.Description) != "" {
		lines = append(lines, "说明: "+truncateRunes(singleLine(plugin.Description), 180))
	}
	if strings.TrimSpace(plugin.LastError) != "" {
		lines = append(lines, "最近错误: "+truncateRunes(singleLine(plugin.LastError), 180))
	}
	return strings.Join(lines, "\n")
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
	return truncateRunes(strings.Join(lines, "\n"), 1000)
}

func pluginIdentityValue(plugin pluginhost.InstalledPlugin) string {
	lines := []string{
		"ID: " + firstNonEmpty(codeValue(plugin.ID), "`unknown`"),
		"名称: " + firstNonEmpty(codeValue(plugin.Name), "`unknown`"),
		"版本: " + firstNonEmpty(codeValue(plugin.Version), "`unknown`"),
	}
	if strings.TrimSpace(plugin.Repo) != "" {
		lines = append(lines, "仓库: "+codeValue(plugin.Repo))
	}
	if strings.TrimSpace(plugin.SourcePath) != "" {
		lines = append(lines, "路径: "+codeValue(plugin.SourcePath))
	}
	return strings.Join(lines, "\n")
}

func pluginColor(plugin pluginhost.InstalledPlugin) int {
	switch {
	case strings.TrimSpace(plugin.LastError) != "":
		return pluginEmbedColorWarning
	case plugin.Enabled:
		return pluginEmbedColorSuccess
	default:
		return pluginEmbedColorInfo
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

func refreshedPlugin(manager *pluginhost.Manager, plugin pluginhost.InstalledPlugin) pluginhost.InstalledPlugin {
	if manager == nil {
		return plugin
	}
	if refreshed, ok := findInstalledPlugin(manager.List(), plugin.ID); ok {
		return refreshed
	}
	return plugin
}

func codeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "`" + value + "`"
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
