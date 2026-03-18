# Plugin Authoring Guide

[Back to README](README.en.md) | [简体中文](PLUGIN_GUIDE.md)

This project uses an external-process plugin model over JSON-RPC via stdio. The host starts the plugin process, routes slash/component/modal/message events, and only exposes host APIs declared in `plugin.json`.

## Minimal Layout

```text
your-plugin/
├── plugin.json
└── cmd/
    └── your-plugin/
        └── main.go
```

- `plugin.json` must live at the plugin root.
- `runtime.command` and `runtime.args` run relative to that root.
- If the plugin is installed through `/plugin`, the selected Git path must contain this layout.

## 1. Write `plugin.json`

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

Important fields:

- `id`: unique plugin identifier
- `runtime`: how the host starts the plugin
- `capabilities`: host permissions the plugin needs
- `commands`: slash commands to register
- `component_prefixes`: required when handling buttons or modals
- `interval_seconds`: enables periodic `OnInterval` callbacks

See [../pkg/pluginapi/types.go](../pkg/pluginapi/types.go) for the exact schema.

## 2. Write `main.go`

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

Use `pluginapi.BasePlugin` and only override the hooks you need.

## 3. Choose the Right Hooks

- `OnSlashCommand`: slash commands
- `OnComponent`: buttons and select menus
- `OnModal`: modal submissions
- `OnMessage`: passive message listeners
- `OnPromptBuild`: inject prompt blocks before the core model call
- `OnResponsePostprocess`: rewrite the final core reply
- `OnInterval`: scheduled work

If you handle components, declare `component_prefixes` in `plugin.json` and keep every `custom_id` under those prefixes.

## 4. Use Host Capabilities

`HostClient` lives in [../pkg/pluginapi/sdk.go](../pkg/pluginapi/sdk.go).

Common mappings:

- `plugin.storage`: `StorageGet`, `StorageSet`, `StorageDelete`, `StorageListKeys`
- `plugin.config.read` / `plugin.config.write`: `ConfigGet`, `ConfigSet`
- `memory.read`: `MemoryGet`, `MemorySearch`
- `memory.write`: `MemoryAppend`, `MemorySetSummary`, `MemoryTrimHistory`
- `discord.read_guild_emojis`: `ListGuildEmojis`
- `llm.chat`: `Chat`
- `llm.embed`: `Embed`
- `llm.rerank`: `Rerank`
- `discord.send_message`: `SendMessage`
- `discord.reply_with_core`: `ReplyToMessage`
- `worldbook.read` / `worldbook.write`: worldbook APIs

Only request the capabilities you actually use.

## Data Storage Boundaries

The important boundary is this:

- Plugins do **not** get raw SQL access to the host SQLite database.
- The host persists runtime config, chat memory, worldbook data, plugin registry state, and plugin data in its own SQLite store.
- Plugins must go through `HostClient` APIs for any persisted data they touch.

Use each data surface for a different job:

- `plugin.storage`: plugin-private KV state for cursors, cache, job state, processed IDs, and other internal state.
- `plugin.config.read/write`: plugin configuration JSON for stable admin-controlled settings such as thresholds, modes, allowlists, and templates.
- `memory.read/write`: core conversation memory for reading current channel context or explicitly appending / trimming memory.

Recommended rule of thumb:

- Use `plugin.config` for user-facing configuration.
- Use `plugin.storage` for plugin-internal runtime state.
- Use `memory.write` only when the plugin truly needs to influence the bot's core conversation memory.

About `plugin.config`:

- `ConfigSet` stores one full JSON value, not a nested KV map.
- The host now validates the basic structure of `config_schema` during install.
- The `/plugin` panel can now open a generic config form from `config_schema`, but the current host only supports:
  top-level `type=object`
  property types `string`, `integer`, `number`, and `boolean`
  up to 5 fields
- The host rejects unknown fields and performs basic type validation on save.
- If your plugin needs nested objects, arrays, enums, or a more complex multi-step UX, the current generic panel is not enough yet; provide a plugin-specific panel instead.

About `memory`:

- `MemoryGet(channelID)` returns the current summary plus recent messages for that channel.
- `MemorySearch(channelID, query, topN)` uses the host's existing embedding retrieval and only searches indexed memory from that channel.
- `MemoryAppend` writes into the host's core memory, not a plugin-private log.
- At the moment, only appended `user` text messages are indexed for vector search, and indexing is asynchronous, so an immediate search right after append may have a short delay.
- `MemoryTrimHistory` is for trimming the recent-message window, not for low-level database management.

## 5. Install and Debug

1. Make sure `go run ./cmd/your-plugin` starts successfully.
2. Push the plugin to Git.
3. Open the bot `/plugin` panel and click `Install`.
4. Fill in `repo`, optional `ref`, and optional `path`.

The host refreshes slash commands after installation.

## Example Navigation

- Example manifest: [../examples/plugins/style-note/plugin.json](../examples/plugins/style-note/plugin.json)
- Example main: [../examples/plugins/style-note/cmd/style-note-plugin/main.go](../examples/plugins/style-note/cmd/style-note-plugin/main.go)
- Manifest and types: [../pkg/pluginapi/types.go](../pkg/pluginapi/types.go)
- SDK and `HostClient`: [../pkg/pluginapi/sdk.go](../pkg/pluginapi/sdk.go)
- Manifest loader: [../pkg/pluginapi/manifest.go](../pkg/pluginapi/manifest.go)

The `style-note` sample shows slash commands, plugin-private storage, and `OnPromptBuild` injection.
