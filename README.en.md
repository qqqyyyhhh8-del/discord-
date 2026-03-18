# Discord Go Bot

[简体中文](README.md) | [English](README.en.md) | [Plugin Guide](PLUGIN_GUIDE.en.md)

Current version: `v0.6.0`  
See [CHANGELOG.md](CHANGELOG.md) for release notes.

This is a Discord bot built with Go + Discordgo. It includes:
- Basic chat via OpenAI-compatible APIs
- Automatic conversation summarization to reduce context growth
- Simple RAG retrieval with embeddings and optional rerank
- Plugin-based persona, guild emoji, and proactive-reply extensions
- Core slash management for speaking scopes, admins, and extra system prompt
- External plugin hosting with Git-installed extensions
- Worldbook injection with persistent guild emoji summaries

## Features
- **Chat**: Uses `OPENAI_CHAT_MODEL` for the main conversation model.
- **Trigger rules**: Replies in DMs directly. In guilds, it only replies when mentioned or when a user replies to the bot.
- **Context model**: Context is stored per channel, not isolated per user. The model sees sender ID, username, nickname, display name, UTC+8 timestamp, and explicit reply metadata.
- **Multimodal input**: Custom guild emojis in user messages are converted into images and sent to the chat model. Image attachments are also included as image input.
- **Auto summary**: Generates summaries after the message count crosses a threshold.
- **RAG retrieval**: Embeds historical user messages, retrieves relevant items, and optionally reranks them.
- **Official plugins**: `/persona`, `/emoji`, and `/proactive` now live in the official `discord-bot-plugins` repository and are installed on demand.
- **Allowed speaking scope**: By default the bot is not allowed to speak in any guild, channel, or thread. Admins must run `/setup` in the target location and use the panel buttons to allow the current guild, channel, or thread.
- **Plugin ecosystem**: External process plugins can be installed from Git repositories. Plugins connect over JSON-RPC over stdio and can register slash commands, receive message events, inject prompt blocks, and postprocess model replies with capability checks.

## Official Plugins
- Official plugin repo: [`qqqyyyhhh8-del/discord-bot-plugins`](https://github.com/qqqyyyhhh8-del/discord-bot-plugins)
- Open the `/plugin` panel, click `Install`, then fill in:
  Persona: `repo=https://github.com/qqqyyyhhh8-del/discord-bot-plugins.git`, `path=plugins/persona`
  Proactive: `repo=https://github.com/qqqyyyhhh8-del/discord-bot-plugins.git`, `path=plugins/proactive`
  Emoji: `repo=https://github.com/qqqyyyhhh8-del/discord-bot-plugins.git`, `path=plugins/emoji`

After installation, the host will register `/persona`, `/proactive`, and `/emoji` automatically.

## Environment Variables
| Variable | Description |
| --- | --- |
| `DISCORD_TOKEN` | Discord bot token (required). Also accepts `DISCORDTOKEN`, `DISCORD_BOT_TOKEN`, and `discordtoken` |
| `OPENAI_API_KEY` | OpenAI-compatible API key (required) |
| `OPENAI_BASE_URL` | OpenAI-compatible base URL for chat (default: `https://api.openai.com/v1`) |
| `OPENAI_CHAT_MODEL` | Chat model (default: `gpt-4o-mini`) |
| `OPENAI_EMBED_BASE_URL` | Base URL for embeddings. Falls back to `OPENAI_BASE_URL` if empty |
| `OPENAI_EMBED_API_KEY` | API key for embeddings. Falls back to `OPENAI_API_KEY` if empty |
| `OPENAI_EMBED_MODEL` | Embedding model (default: `text-embedding-3-small`) |
| `OPENAI_RERANK_BASE_URL` | Base URL for rerank. Falls back to `OPENAI_BASE_URL` if empty |
| `OPENAI_RERANK_API_KEY` | API key for rerank. Falls back to `OPENAI_API_KEY` if empty |
| `OPENAI_RERANK_MODEL` | Rerank model. Leave empty to disable rerank |
| `OPENAI_HTTP_TIMEOUT_SECONDS` | Optional HTTP client timeout for OpenAI-compatible endpoints. If empty, the outer context timeout is used |
| `SYSTEM_PROMPT` | Optional base system prompt |
| `BOT_CONFIG_FILE` | Runtime config file path (default: `bot_config.json`) |
| `BOT_COMMAND_GUILD_ID` | Optional guild ID for slash command registration. If empty, commands are global |
| `PLUGINS_DIR` | Plugin host working directory (default: `plugins`) containing the plugin registry and installed source trees |

## Quick Start
1. Clone the repo and enter the directory:
   ```bash
   git clone <your-repo-url>
   cd discord-
   ```
2. Create `.env`:
   ```bash
   cp .env.example .env
   ```
3. Edit `.env` as needed.
4. Start the bot:
   ```bash
   go run ./cmd/discordbot
   ```

The bot loads `.env` from the current directory automatically. Existing shell environment variables take precedence over `.env`.

## Config File
If `BOT_CONFIG_FILE` does not exist, it will be created automatically on startup:

```json
{
  "super_admin_ids": ["your_discord_user_id"],
  "admin_ids": [],
  "system_prompt": "",
  "speech_mode": "allowlist",
  "allowed_guild_ids": [],
  "allowed_channel_ids": [],
  "allowed_thread_ids": [],
  "worldbook_entries": {}
}
```

- `super_admin_ids` can only be edited in the config file. Both string IDs and numeric Discord IDs are accepted.
- `admin_ids` can be edited in the config file or granted/revoked by a super admin through slash commands.
- `system_prompt` stores extra system prompt content, such as jailbreak-style policy overrides.
- `speech_mode` currently defaults to `allowlist`; the bot only speaks when a location matches the configured allowlist.
- `allowed_guild_ids` is the allowlist of guild IDs.
- `allowed_channel_ids` is the allowlist of channel IDs.
- `allowed_thread_ids` is the allowlist of thread/forum post IDs.
- `worldbook_entries` stores worldbook entries. Guild emoji analysis currently writes here automatically.
- Older configs may still contain `personas`, `active_persona`, `proactive_reply`, `proactive_chance`, and `guild_emoji_profiles`; those are legacy compatibility fields from before the official-plugin migration. The official plugins now use plugin-private storage.

## Slash Commands
- `/help`: show command help
- `/setup`: open the speaking-scope management panel
- `/plugin`: open the all-in-one plugin management panel
- `/system show`: show the extra system prompt
- `/system set prompt:<prompt>`: set the extra system prompt
- `/system clear`: clear the extra system prompt
- `/admin list`: show super admins and admins
- `/admin add user:<@user>`: grant admin to a user (super admin only)
- `/admin remove user:<@user>`: revoke admin from a user (super admin only)

## Notes
- Enable **Message Content Intent** in the Discord developer portal.
- In guilds, use `@bot your message` or reply directly to the bot to trigger a response.
- The bot shows `typing` while it is processing a reply.
- On first start, the bot will not speak in any guild location until `/setup` is configured from the target location.
- Management has been moved to slash commands; old message-prefix commands such as `!persona`, `!system`, and `!admin` are not used anymore.
- `/persona`, `/emoji`, and `/proactive` are now provided by official plugins. If a plugin is not installed, its slash command will not exist.
- If the official `/emoji` plugin times out during analysis, check the response speed of your OpenAI-compatible endpoint. If needed, set `OPENAI_HTTP_TIMEOUT_SECONDS=600` in `.env`.
- On startup, the bot clears old slash commands in the current scope before re-registering them in bulk.

## Plugin Development

- Plugin guide: [PLUGIN_GUIDE.en.md](PLUGIN_GUIDE.en.md)
- Example navigation:
  [examples/plugins/style-note/plugin.json](examples/plugins/style-note/plugin.json) ·
  [examples/plugins/style-note/cmd/style-note-plugin/main.go](examples/plugins/style-note/cmd/style-note-plugin/main.go) ·
  [pkg/pluginapi/types.go](pkg/pluginapi/types.go) ·
  [pkg/pluginapi/sdk.go](pkg/pluginapi/sdk.go)
- The shared protocol and Go SDK live in `pkg/pluginapi`.
- Every plugin must provide a `plugin.json` manifest.
- The current host supports slash commands, button/modal prefix routing, message hooks, prompt injection, response postprocessing, interval hooks, plugin-private storage, and capability-checked host calls.
- An official example plugin is available in `examples/plugins/style-note`.
- To install the sample plugin from this repository, open the `/plugin` panel and fill:
  `repo=https://github.com/qqqyyyhhh8-del/discord-.git`, `path=examples/plugins/style-note`

## License

This project is licensed under the [MIT License](LICENSE).
