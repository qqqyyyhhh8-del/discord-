# Discord Go Bot

这是一个基于 Go + Discordgo 的聊天机器人示例，具备：
- 基础对话能力（调用 OpenAI 格式兼容接口）
- 自动对话摘要（防止上下文过长）
- 简单 RAG 检索（使用 embedding 进行相似度召回）

## 功能概览
- **聊天**：通过 `OPENAI_CHAT_MODEL` 调用聊天模型。
- **自动总结**：当对话条数超过阈值时生成摘要并保留关键信息。
- **RAG 检索**：对历史用户消息生成 embedding，召回相似内容辅助回答。

## 配置文件
请直接编辑项目根目录的 `config.json`：
```json
{
  "discord_token": "你的discord bot token",
  "system_prompt": "你是Discord聊天助手，回答清晰、友好，避免重复。",
  "openai": {
    "api_key": "你的openai兼容key",
    "base_url": "https://api.openai.com/v1",
    "chat_model": "gpt-4o-mini",
    "embed_model": "text-embedding-3-small"
  }
}
```

## 本地运行
1. 安装依赖：
   ```bash
   # 根据系统安装 Go 与 git
   ```
2. 拉取代码并进入目录：
   ```bash
   git clone <你的仓库地址>
   cd discord-
   ```
3. 修改 `config.json` 后启动：
   ```bash
   go run ./cmd/discordbot
   ```

## 注意事项
- 机器人需要在 Discord 开发者后台开启 **Message Content Intent**。
