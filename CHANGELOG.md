# Changelog

All notable changes to this project will be documented in this file.

## v0.5.0 - 2026-03-18

### Added

- Added the official plugin monorepo `discord-bot-plugins`, containing the first-party `persona`, `proactive`, and `emoji` plugins.
- Added new host/plugin capabilities for core-powered replies, guild emoji listing, worldbook read/write, speech allowlist checks, richer interaction components, and admin role visibility in plugin context.

### Changed

- Migrated `/persona`, `/proactive`, and `/emoji` from built-in bot features into installable official plugins.
- Updated the main README files to document the new plugin-based installation flow and link to the official plugin repository.
- Bumped the host version to `v0.5.0`.

### Fixed

- Fixed a migration conflict where the core bot could still inject a legacy persona prompt after the official persona plugin was installed.
- Fixed a migration conflict where the core bot could still run legacy proactive-reply logic after the official proactive plugin was installed.

## v0.4.0 - 2026-03-18

### Added

- Added an external plugin host based on `JSON-RPC 2.0 over stdio`, with a shared `pkg/pluginapi` protocol package and Go SDK helpers for plugin authors.
- Added plugin registry persistence under `plugins/registry.json`, per-plugin private storage, capability manifests, dynamic slash command registration, and runtime plugin process management.
- Added `/plugin list|install|upgrade|remove|enable|disable|allow_here|deny_here|permissions` for plugin lifecycle management.
- Added prompt-build and response-postprocess plugin hooks, plus message event dispatch for installed plugins.
- Added an official example plugin at `examples/plugins/style-note` that demonstrates Git-installable prompt injection backed by plugin private storage.

### Changed

- Version bumped to `v0.4.0`.
- Discord command registration now merges core commands with enabled plugin commands and refreshes after plugin install, upgrade, remove, or global enable/disable.

### Fixed

- Fixed plugin command and component prefix conflict detection so installed plugins cannot shadow core management routes.

## v0.3.0 - 2026-03-18

### Added

- Added `/setup show|server|channel|thread|clear` to manage allowed speaking scopes directly from the current guild, channel, or thread, with persistent storage in `bot_config.json`.
- Added a dedicated `/proactive` management panel for enabling proactive replies and editing reply probability.
- Added tests covering the new setup flow, proactive reply behavior, allowlist persistence, and prompt rendering.

### Changed

- Default speaking behavior is now deny-by-default. The bot only speaks in locations explicitly allowed through `/setup`.
- Removed the old `/speech` panel flow and replaced it with the new `/setup` slash command flow.
- Startup logs now include the tracked application version `v0.3.0`.
- Updated both Chinese and English README files to document the current setup and release note entry point.

### Fixed

- Fixed proactive reply permission checks so they still obey the current allowed speaking scope.
- Fixed reply sending fallback logic when Discord rejects message references, retrying without the reference and logging the failure.
- Fixed context formatting leakage where assistant history could be sent back to the model with `时间(UTC+8)` / `发送者` / `内容` metadata, which sometimes caused the model to imitate that header format in replies.
