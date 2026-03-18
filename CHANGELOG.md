# Changelog

All notable changes to this project will be documented in this file.

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
