package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func slashCommands(pluginCommands []*discordgo.ApplicationCommand) []*discordgo.ApplicationCommand {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "查看机器人管理命令帮助",
		},
		{
			Name:        "system",
			Description: "管理额外 system prompt",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "show",
					Description: "查看当前 system prompt",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "set",
					Description: "设置 system prompt",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "prompt",
							Description: "system prompt 内容",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "clear",
					Description: "清空 system prompt",
				},
			},
		},
		{
			Name:        "admin",
			Description: "管理管理员列表",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看超级管理员和管理员列表",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "添加管理员",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "目标用户",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "移除管理员",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "目标用户",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "setup",
			Description: "设置允许机器人发言的服务器/频道/子区",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "show",
					Description: "查看当前允许发言范围",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "server",
					Description: "放行当前所在服务器",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "channel",
					Description: "放行当前所在频道",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "thread",
					Description: "放行当前所在子区",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "clear",
					Description: "清空所有允许发言范围",
				},
			},
		},
		{
			Name:        "plugin",
			Description: "管理外部插件生态",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看已安装插件",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "install",
					Description: "通过 Git 仓库安装插件",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "repo",
							Description: "Git 仓库地址或本地 Git 仓库路径",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "ref",
							Description: "可选，分支、标签或提交",
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "path",
							Description: "可选，插件在仓库内的子目录",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "upgrade",
					Description: "升级已安装插件",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "ref",
							Description: "可选，新的分支、标签或提交",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "卸载插件",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "enable",
					Description: "全局启用插件",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "disable",
					Description: "全局禁用插件",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "allow_here",
					Description: "允许插件在当前服务器启用",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "deny_here",
					Description: "禁止插件在当前服务器启用",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "permissions",
					Description: "查看插件能力授权",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "plugin",
							Description: "插件 ID",
							Required:    true,
						},
					},
				},
			},
		},
	}
	commands = append(commands, pluginCommands...)
	return commands
}

func (h *Handler) HandleSlashCommand(ctx context.Context, authorID string, data discordgo.ApplicationCommandInteractionData) (string, bool, error) {
	_ = ctx
	if err := h.ensureRuntimeStore(); err != nil {
		return "", true, err
	}

	switch data.Name {
	case "help":
		return commandHelp(), true, nil
	case "persona":
		command, err := buildPersonaSlashCommand(data.Options)
		if err != nil {
			return personaHelp(), true, nil
		}
		response, err := h.handlePersonaCommand(command, authorID)
		return response, true, err
	case "system":
		command, err := buildSystemSlashCommand(data.Options)
		if err != nil {
			return systemHelp(), true, nil
		}
		response, err := h.handleSystemCommand(command, authorID)
		return response, true, err
	case "admin":
		command, err := buildAdminSlashCommand(data.Options)
		if err != nil {
			return adminHelp(), true, nil
		}
		response, err := h.handleAdminCommand(command, authorID)
		return response, true, err
	case "setup":
		return "请直接在目标服务器、频道或子区中使用 `/setup`。", true, nil
	default:
		return "未知命令。", true, nil
	}
}

func buildPersonaSlashCommand(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	subcommand, optionMap, err := slashSubcommand(options)
	if err != nil {
		return "", err
	}

	switch subcommand {
	case "list":
		return "!persona list", nil
	case "current":
		return "!persona current", nil
	case "show":
		name, ok := slashStringOption(optionMap, "name")
		if !ok {
			return "", fmt.Errorf("missing persona name")
		}
		return "!persona show " + name, nil
	case "add":
		name, ok := slashStringOption(optionMap, "name")
		if !ok {
			return "", fmt.Errorf("missing persona name")
		}
		prompt, ok := slashStringOption(optionMap, "prompt")
		if !ok {
			return "", fmt.Errorf("missing persona prompt")
		}
		return "!persona add " + name + " " + prompt, nil
	case "use":
		name, ok := slashStringOption(optionMap, "name")
		if !ok {
			return "", fmt.Errorf("missing persona name")
		}
		return "!persona use " + name, nil
	case "delete":
		name, ok := slashStringOption(optionMap, "name")
		if !ok {
			return "", fmt.Errorf("missing persona name")
		}
		return "!persona delete " + name, nil
	case "clear":
		return "!persona clear", nil
	default:
		return "", fmt.Errorf("unknown persona subcommand")
	}
}

func buildSystemSlashCommand(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	subcommand, optionMap, err := slashSubcommand(options)
	if err != nil {
		return "", err
	}

	switch subcommand {
	case "show":
		return "!system show", nil
	case "set":
		prompt, ok := slashStringOption(optionMap, "prompt")
		if !ok {
			return "", fmt.Errorf("missing system prompt")
		}
		return "!system set " + prompt, nil
	case "clear":
		return "!system clear", nil
	default:
		return "", fmt.Errorf("unknown system subcommand")
	}
}

func buildAdminSlashCommand(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	subcommand, optionMap, err := slashSubcommand(options)
	if err != nil {
		return "", err
	}

	switch subcommand {
	case "list":
		return "!admin list", nil
	case "add":
		userID, ok := slashUserIDOption(optionMap, "user")
		if !ok {
			return "", fmt.Errorf("missing admin target user")
		}
		return "!admin add " + userID, nil
	case "remove":
		userID, ok := slashUserIDOption(optionMap, "user")
		if !ok {
			return "", fmt.Errorf("missing admin target user")
		}
		return "!admin remove " + userID, nil
	default:
		return "", fmt.Errorf("unknown admin subcommand")
	}
}

func slashSubcommand(options []*discordgo.ApplicationCommandInteractionDataOption) (string, map[string]*discordgo.ApplicationCommandInteractionDataOption, error) {
	if len(options) == 0 {
		return "", nil, fmt.Errorf("missing subcommand")
	}
	subcommand := options[0]
	if subcommand.Type != discordgo.ApplicationCommandOptionSubCommand {
		return "", nil, fmt.Errorf("invalid subcommand")
	}

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(subcommand.Options))
	for _, option := range subcommand.Options {
		optionMap[option.Name] = option
	}
	return subcommand.Name, optionMap, nil
}

func slashStringOption(options map[string]*discordgo.ApplicationCommandInteractionDataOption, name string) (string, bool) {
	option, ok := options[name]
	if !ok || option == nil || option.Type != discordgo.ApplicationCommandOptionString {
		return "", false
	}
	return strings.TrimSpace(option.StringValue()), true
}

func slashUserIDOption(options map[string]*discordgo.ApplicationCommandInteractionDataOption, name string) (string, bool) {
	option, ok := options[name]
	if !ok || option == nil || option.Type != discordgo.ApplicationCommandOptionUser {
		return "", false
	}
	return strings.TrimSpace(option.UserValue(nil).ID), true
}
