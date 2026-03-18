package bot

import (
	"context"
	"fmt"
	"strings"

	"discordbot/internal/pluginhost"
	"discordbot/pkg/pluginapi"

	"github.com/bwmarrin/discordgo"
)

func (h *Handler) handlePluginCommand(ctx context.Context, authorID string, location speechLocation, options []*discordgo.ApplicationCommandInteractionDataOption) (string, bool, error) {
	if err := h.ensureRuntimeStore(); err != nil {
		return "", true, err
	}
	if h.pluginManager == nil {
		return "当前没有启用插件宿主。", true, nil
	}
	if !h.runtimeStore.IsAdmin(authorID) {
		return permissionDenied(), true, nil
	}

	subcommand, optionMap, err := slashSubcommand(options)
	if err != nil {
		return pluginHelp(), true, nil
	}

	switch subcommand {
	case "list":
		return renderPluginList(h.pluginManager.List()), true, nil
	case "permissions":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		permissions, err := h.pluginManager.Permissions(pluginID)
		if err != nil {
			return "", true, err
		}
		if len(permissions) == 0 {
			return fmt.Sprintf("插件 `%s` 当前没有任何授权能力。", pluginID), true, nil
		}
		lines := make([]string, 0, len(permissions)+1)
		lines = append(lines, fmt.Sprintf("插件 `%s` 的授权能力:", pluginID))
		for _, permission := range permissions {
			lines = append(lines, "- "+string(permission))
		}
		return strings.Join(lines, "\n"), true, nil
	case "allow_here":
		if strings.TrimSpace(location.GuildID) == "" {
			return "这个命令只能在服务器频道中使用。", true, nil
		}
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		if err := h.pluginManager.AllowGuild(pluginID, location.GuildID); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已允许插件 `%s` 在当前服务器使用。", pluginID), true, nil
	case "deny_here":
		if strings.TrimSpace(location.GuildID) == "" {
			return "这个命令只能在服务器频道中使用。", true, nil
		}
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		if err := h.pluginManager.DenyGuild(pluginID, location.GuildID); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已禁止插件 `%s` 在当前服务器使用。", pluginID), true, nil
	}

	if !h.runtimeStore.IsSuperAdmin(authorID) {
		return superAdminDenied(), true, nil
	}

	switch subcommand {
	case "install":
		repo, ok := slashStringOption(optionMap, "repo")
		if !ok {
			return pluginHelp(), true, nil
		}
		ref, _ := slashStringOption(optionMap, "ref")
		path, _ := slashStringOption(optionMap, "path")
		plugin, err := h.pluginManager.InstallFromGit(ctx, repo, ref, path)
		if err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已安装插件 `%s` (%s)，授权能力 %d 项。", plugin.ID, plugin.Version, len(plugin.GrantedCaps)), true, nil
	case "upgrade":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		ref, _ := slashStringOption(optionMap, "ref")
		plugin, err := h.pluginManager.UpgradeFromGit(ctx, pluginID, ref)
		if err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已升级插件 `%s` 到 %s。", plugin.ID, plugin.Version), true, nil
	case "remove":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		if err := h.pluginManager.Remove(pluginID); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已卸载插件 `%s`。", pluginID), true, nil
	case "enable":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		if err := h.pluginManager.EnableGlobal(pluginID); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已全局启用插件 `%s`。", pluginID), true, nil
	case "disable":
		pluginID, ok := slashStringOption(optionMap, "plugin")
		if !ok {
			return pluginHelp(), true, nil
		}
		if err := h.pluginManager.DisableGlobal(pluginID); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已全局禁用插件 `%s`。", pluginID), true, nil
	default:
		return pluginHelp(), true, nil
	}
}

func renderPluginList(plugins []pluginhost.InstalledPlugin) string {
	if len(plugins) == 0 {
		return "当前没有已安装插件。"
	}

	lines := []string{"已安装插件:"}
	for _, plugin := range plugins {
		scope := plugin.GuildMode
		if len(plugin.GuildIDs) > 0 {
			scope += ":" + strings.Join(plugin.GuildIDs, ",")
		}
		if scope == "" {
			scope = pluginhost.GuildModeAll
		}
		status := "disabled"
		if plugin.Enabled {
			status = "enabled"
		}
		lines = append(lines, fmt.Sprintf("- %s %s [%s] scope=%s caps=%d", plugin.ID, plugin.Version, status, scope, len(plugin.GrantedCaps)))
		if strings.TrimSpace(plugin.LastError) != "" {
			lines = append(lines, "  last_error: "+plugin.LastError)
		}
	}
	return strings.Join(lines, "\n")
}

func pluginCapabilitiesSummary(caps []pluginapi.Capability) string {
	if len(caps) == 0 {
		return "0"
	}
	return fmt.Sprintf("%d", len(caps))
}
