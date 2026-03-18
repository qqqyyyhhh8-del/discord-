# 插件编写指南

[返回 README](../README.md) | [English](PLUGIN_GUIDE.en.md)

本项目的插件是“独立进程 + JSON-RPC over stdio”模式。宿主负责拉起插件进程、路由 Slash/按钮/Modal/消息事件，并按 `plugin.json` 中声明的能力开放宿主接口。

## 最小目录结构

```text
your-plugin/
├── plugin.json
└── cmd/
    └── your-plugin/
        └── main.go
```

- `plugin.json` 必须放在插件根目录。
- `runtime.command` / `runtime.args` 相对插件根目录执行。
- 如果你要被 `/plugin` 从 Git 安装，仓库里最终传给宿主的目录必须包含这两个入口。

## 第一步：写 `plugin.json`

最小示例：

```json
{
  "id": "hello_plugin",
  "name": "Hello Plugin",
  "version": "v0.1.0",
  "description": "A minimal slash-command plugin.",
  "min_host_version": "v0.4.0",
  "runtime": {
    "command": "go",
    "args": ["run", "./cmd/hello-plugin"]
  },
  "capabilities": [
    "discord.interaction.respond"
  ],
  "commands": [
    {
      "name": "hello",
      "description": "Reply with hello"
    }
  ]
}
```

关键字段：

- `id`：插件唯一 ID，安装后作为目录名和注册主键。
- `runtime`：宿主启动插件进程的命令。
- `capabilities`：声明插件需要的宿主权限；不声明就拿不到对应能力。
- `commands`：要注册的 Slash 命令。
- `component_prefixes`：如果你要处理按钮或 Modal，必须声明此前缀，且你的 `custom_id` 要以该前缀开头。
- `interval_seconds`：如果要定时任务，设置触发间隔秒数。

字段定义和校验逻辑见 [../pkg/pluginapi/types.go](../pkg/pluginapi/types.go)。

## 第二步：写 `main.go`

最小 Go 插件：

```go
package main

import (
	"context"
	"log"

	"kizuna/pkg/pluginapi"
)

type helloPlugin struct {
	pluginapi.BasePlugin
}

func (p *helloPlugin) OnSlashCommand(ctx context.Context, host *pluginapi.HostClient, req pluginapi.SlashCommandRequest) (*pluginapi.InteractionResponse, error) {
	return &pluginapi.InteractionResponse{
		Type: pluginapi.InteractionResponseTypeMessage,
		Message: &pluginapi.InteractionMessage{
			Content:   "hello",
			Ephemeral: true,
		},
	}, nil
}

func main() {
	manifest, err := pluginapi.ReadManifest("plugin.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := pluginapi.Serve(manifest, &helloPlugin{}); err != nil {
		log.Fatal(err)
	}
}
```

常见写法：

- 嵌入 `pluginapi.BasePlugin`，只重写你需要的 hook。
- 启动时先 `ReadManifest("plugin.json")`，再 `Serve(...)`。
- `OnSlashCommand`、`OnComponent`、`OnModal` 返回交互响应；`OnMessage`、`OnInterval` 则是副作用型 hook。

## 第三步：按场景选择 Hook

- `OnSlashCommand`：处理 Slash 命令。
- `OnComponent`：处理按钮、下拉框交互。
- `OnModal`：处理表单提交。
- `OnMessage`：被动接收消息事件，适合监听、统计、缓存。
- `OnPromptBuild`：在主模型调用前注入 prompt block。
- `OnResponsePostprocess`：改写主模型最终回复。
- `OnInterval`：周期任务。

如果你要处理组件交互，`plugin.json` 里一定要声明 `component_prefixes`；否则宿主不会把交互路由给你的插件。

## 第四步：使用宿主能力

`HostClient` 已经封装了常用宿主调用，定义见 [../pkg/pluginapi/sdk.go](../pkg/pluginapi/sdk.go)。

常用能力对照：

- `plugin.storage`：`StorageGet` / `StorageSet`
- `plugin.config.read` / `plugin.config.write`：`ConfigGet` / `ConfigSet`
- `memory.read`：`MemoryGet` / `MemorySearch`
- `memory.write`：`MemoryAppend` / `MemorySetSummary` / `MemoryTrimHistory`
- `discord.interaction.respond`：返回 Slash/按钮/Modal 响应
- `discord.read_guild_emojis`：`ListGuildEmojis`
- `llm.chat`：`Chat`
- `llm.embed`：`Embed`
- `llm.rerank`：`Rerank`
- `discord.send_message`：`SendMessage`
- `discord.reply_with_core`：`ReplyToMessage`
- `worldbook.read` / `worldbook.write`：`GetWorldBook` / `UpsertWorldBook` / `DeleteWorldBook`

建议：只声明真正需要的 capability。宿主会按声明做权限检查。

## 数据存储与数据库边界

先把边界说清楚：

- 插件**没有**原生 SQL 连接，也不应该直接读写宿主 SQLite。
- 宿主会把运行时配置、聊天记忆、世界书、插件注册表和插件数据统一持久化到主 SQLite。
- 插件能接触的数据，必须通过 `HostClient` 提供的能力接口完成。

三类数据能力的定位分别是：

- `plugin.storage`：插件私有 KV 状态。适合缓存、游标、任务状态、已处理 ID 等“插件内部状态”。
- `plugin.config.read/write`：插件配置 JSON。适合管理员可调整的稳定配置，例如阈值、模式、白名单、模板文本。
- `memory.read/write`：核心聊天记忆。适合需要读当前频道上下文，或显式追加/修剪记忆的插件。

当前建议：

- 想做“用户可配置项”，优先用 `plugin.config`。
- 想做“插件运行状态”，优先用 `plugin.storage`。
- 想影响主对话上下文，再用 `memory.write`；这个能力应该少给。

关于 `plugin.config`：

- `ConfigSet` 保存的是**整块 JSON 值**，不是 KV 子项。
- `plugin.json` 里的 `config_schema` 字段现在可以作为插件自己的 schema 说明，但宿主**暂未自动校验或自动渲染配置表单**。
- 也就是说，schema 目前是给插件作者、面板和未来扩展预留的元数据，不是强校验器。

关于 `memory`：

- `MemoryGet(channelID)` 返回该频道当前的摘要和最近消息。
- `MemorySearch(channelID, query, topN)` 会走宿主已有的 embedding 检索，只查该频道已索引的记忆。
- `MemoryAppend` 追加的是宿主核心记忆，不是插件私有日志。
- 目前只有追加的 `user` 文本消息会进入向量索引，而且索引是异步完成的；刚 append 后立刻 search，可能会有极短延迟。
- `MemoryTrimHistory` 主要用于修剪最近消息窗口；它不是数据库管理接口。

## 第五步：本地调试与安装

1. 在插件目录确认 `go run ./cmd/your-plugin` 能独立启动。
2. 把插件推到 Git 仓库，保证 `plugin.json` 在安装根目录。
3. 在 bot 的 `/plugin` 面板点击“安装”。
4. 填写：
   - `repo`：Git 仓库地址
   - `ref`：可选，分支 / tag / commit
   - `path`：插件在仓库中的子目录

安装成功后，宿主会自动刷新 Slash 命令。

## 示例插件导航

- 示例 manifest：[../examples/plugins/style-note/plugin.json](../examples/plugins/style-note/plugin.json)
- 示例主程序：[../examples/plugins/style-note/cmd/style-note-plugin/main.go](../examples/plugins/style-note/cmd/style-note-plugin/main.go)
- Manifest / 类型定义：[../pkg/pluginapi/types.go](../pkg/pluginapi/types.go)
- SDK / HostClient：[../pkg/pluginapi/sdk.go](../pkg/pluginapi/sdk.go)
- Manifest 读取入口：[../pkg/pluginapi/manifest.go](../pkg/pluginapi/manifest.go)

`style-note` 这个示例演示了三件事：

- 通过 `commands` 注册 Slash 命令
- 用 `StorageGet` / `StorageSet` 做插件私有存储
- 用 `OnPromptBuild` 把内容注入主 prompt

## 常见坑

- `plugin.json` 缺字段会在安装阶段直接失败。
- 组件交互忘记配 `component_prefixes`，按钮点了不会路由到插件。
- 调用了宿主能力但没声明 capability，会被宿主拒绝。
- Slash 命令名和现有核心命令或其他插件冲突，会导致注册失败。
