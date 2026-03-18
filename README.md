# Discord Go Bot

[简体中文](README.md) | [English](README.en.md)

当前版本：`v0.3.0`  
更新记录见 [CHANGELOG.md](CHANGELOG.md)

这是一个基于 Go + Discordgo 的聊天机器人示例，具备：
- 基础对话能力（调用 OpenAI 格式兼容接口）
- 自动对话摘要（防止上下文过长）
- 简单 RAG 检索（使用 embedding 召回，可选 rerank 重排）
- 人设 / 允许发言范围 / 服务器表情 / 主动回复的 Slash 管理能力
- 世界书注入与服务器表情世界书持久化

## 功能概览
- **聊天**：通过 `OPENAI_CHAT_MODEL` 调用聊天模型。
- **触发规则**：私聊会直接回复，群聊中在 `@机器人` 或直接回复机器人消息时回复，避免刷屏。
- **上下文**：按频道维度保留上下文，不按用户隔离；模型能看到每条消息的用户 ID、用户名、昵称、显示名、UTC+8 时间，以及回复目标消息的显式元数据。
- **多模态输入**：用户消息中的服务器自定义表情会转成图片一起发给聊天模型；图片附件也会作为图片输入发送。
- **自动总结**：当对话条数超过阈值时生成摘要并保留关键信息。
- **RAG 检索**：对历史用户消息生成 embedding，召回后可选再走 rerank 重排。
- **表情管理**：管理员可在 `/emoji` 面板里做增量分析或完整重建；机器人会把服务器表情总结写入世界书，用于后续聊天时适当使用表情。
- **允许发言范围**：机器人默认不会在任何服务器、频道、子区发言；管理员需要在目标位置直接执行 `/setup server`、`/setup channel`、`/setup thread` 来放行当前服务器、当前频道或当前子区。
- **主动回复**：管理员可在 `/proactive` 面板里设置开关和概率；启用后，机器人会在普通群聊消息中按概率主动回复，但仍然受 `/setup` 白名单限制。

## 环境变量
| 变量 | 说明 |
| --- | --- |
| `DISCORD_TOKEN` | Discord 机器人 Token（必填）；也兼容读取 `DISCORDTOKEN`、`DISCORD_BOT_TOKEN`、`discordtoken` |
| `OPENAI_API_KEY` | OpenAI 兼容 API Key（必填） |
| `OPENAI_BASE_URL` | OpenAI 兼容 API Base URL（默认 `https://api.openai.com/v1`） |
| `OPENAI_CHAT_MODEL` | 聊天模型（默认 `gpt-4o-mini`） |
| `OPENAI_EMBED_BASE_URL` | embedding 专用 OpenAI 兼容 Base URL；不填则沿用 `OPENAI_BASE_URL` |
| `OPENAI_EMBED_API_KEY` | embedding 专用 API Key；不填则沿用 `OPENAI_API_KEY` |
| `OPENAI_EMBED_MODEL` | embedding 模型（默认 `text-embedding-3-small`） |
| `OPENAI_RERANK_BASE_URL` | rerank 专用 OpenAI 兼容 Base URL；不填则沿用 `OPENAI_BASE_URL` |
| `OPENAI_RERANK_API_KEY` | rerank 专用 API Key；不填则沿用 `OPENAI_API_KEY` |
| `OPENAI_RERANK_MODEL` | rerank 模型；为空则关闭 rerank |
| `OPENAI_HTTP_TIMEOUT_SECONDS` | 可选，给 OpenAI 兼容接口设置 HTTP 客户端超时秒数；不填则主要依赖外层 context 超时 |
| `SYSTEM_PROMPT` | 系统提示词（可选） |
| `BOT_CONFIG_FILE` | 运行时配置文件路径（默认 `bot_config.json`） |
| `BOT_COMMAND_GUILD_ID` | 可选，slash 命令注册到指定 guild；不填则注册为全局命令 |

## 快速开始
1. 拉取仓库并进入目录：
   ```bash
   git clone <你的仓库地址>
   cd discord-
   ```
2. 创建 `.env`：
   ```bash
   cp .env.example .env
   ```
3. 按需编辑 `.env`。
4. 启动：
   ```bash
   go run ./cmd/discordbot
   ```

程序启动时会自动读取当前目录下的 `.env`。如果你已经在系统环境里设置了同名变量，系统环境优先。

## 配置文件
启动时如果 `BOT_CONFIG_FILE` 不存在，会自动生成一个配置文件：

```json
{
  "super_admin_ids": ["你的Discord用户ID"],
  "admin_ids": [],
  "personas": {},
  "active_persona": "",
  "system_prompt": "",
  "speech_mode": "allowlist",
  "allowed_guild_ids": [],
  "allowed_channel_ids": [],
  "allowed_thread_ids": [],
  "proactive_reply": false,
  "proactive_chance": 0,
  "worldbook_entries": {},
  "guild_emoji_profiles": {}
}
```

- `super_admin_ids` 只能手动编辑配置文件；支持直接写字符串 ID，也支持写数字形式的 Discord ID。
- `admin_ids` 可以手动编辑，也可以由超级管理员通过 slash 命令增删。
- `personas` 用来保存人设 Prompt。
- `system_prompt` 用来保存额外的 system prompt，例如你说的“破限配置”。
- `speech_mode` 当前默认固定为 `allowlist`；机器人只有命中允许范围时才会发言。
- `allowed_guild_ids` 是允许发言的服务器 ID 列表。
- `allowed_channel_ids` 是允许发言的频道 ID 列表。
- `allowed_thread_ids` 是允许发言的子区/线程/帖子 ID 列表。
- `proactive_reply` 用来控制是否启用主动回复。
- `proactive_chance` 是主动回复概率，范围为 `0` 到 `100`，按百分比表示。
- `worldbook_entries` 用来保存世界书条目；目前服务器表情分析结果会自动写到这里。
- `guild_emoji_profiles` 用来保存各服务器已分析的表情 ID、摘要、最近分析人和分析时间。

## Slash 命令
- `/help`：查看命令帮助
- `/persona`：打开一站式人设管理面板
  面板内支持查看、切换、新增/覆盖、编辑当前、删除当前人设、清空当前启用，并带交互按钮和选择菜单
- `/setup show`：查看当前允许发言范围
- `/setup server`：放行当前所在服务器
- `/setup channel`：放行当前所在频道
- `/setup thread`：放行当前所在子区
- `/setup clear`：清空所有允许发言范围
- `/emoji`：打开服务器表情管理面板
  面板内支持增量分析、完整重建、刷新和查看当前世界书；分析时会把表情按 16 个一组拼成 4x4 图组送去模型理解
- `/proactive`：打开主动回复管理面板
  面板内支持开启、关闭、编辑回复概率；只有当前服务器/频道/子区已经在 `/setup` 中放行时才可开启
- `/system show`：查看额外 system prompt
- `/system set prompt:<prompt>`：设置额外 system prompt
- `/system clear`：清空额外 system prompt
- `/admin list`：查看超级管理员和管理员列表
- `/admin add user:<@user>`：超级管理员添加管理员
- `/admin remove user:<@user>`：超级管理员移除管理员

## 注意事项
- 机器人需要在 Discord 开发者后台开启 **Message Content Intent**。
- 群聊里请使用 `@机器人 你的问题`，或直接回复机器人上一条消息来触发回复。
- 机器人在收到触发消息后，会在生成回复的过程中持续显示 `typing`。
- 首次启动后机器人默认不会在任何群聊位置发言；请先使用 `/setup` 配置允许范围。
- 管理配置改为 slash commands，不再使用 `!persona`、`!system`、`!admin` 这类消息前缀命令。
- `/persona` 面板默认以 ephemeral 形式打开；普通用户可查看，管理员和超级管理员可操作。
- `/emoji` 面板默认以 ephemeral 形式打开；只有管理员和超级管理员可以触发表情分析。
- `/proactive` 面板默认以 ephemeral 形式打开；只有管理员和超级管理员可以操作。
- 如果 `/emoji` 分析时遇到超时，优先检查你的 OpenAI 兼容站点响应速度；必要时可在 `.env` 里设置 `OPENAI_HTTP_TIMEOUT_SECONDS=600`。
- 机器人启动时会先清空当前作用域下旧的 slash commands，再批量重新注册，避免逐个删命令带来的额外请求。

## 许可证

本项目使用 [MIT License](LICENSE)。
