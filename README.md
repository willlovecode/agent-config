# agent-config

Single source of truth for how coding agents behave on this PC.

| File | Goes to |
| --- | --- |
| `AGENTS.md` | `%USERPROFILE%\.codex\AGENTS.md` (Codex); Claude Code imports it |
| `claude/CLAUDE.md` | `%USERPROFILE%\.claude\CLAUDE.md`, one line: `@C:/Users/walbe/.codex/AGENTS.md` |
| `claude/agents/` | `%USERPROFILE%\.claude\agents\` (Claude Code subagents: scout, mechanic, implement, review) |
| `claude/model-settings.json` | merged into `modelSettings` of `%USERPROFILE%\.claude\settings.json` |
| `codex/default.toml` | `model` and `model_reasoning_effort` patched into `%USERPROFILE%\.codex\config.toml` |
| `codex/*.config.toml` | `%USERPROFILE%\.codex\` (profiles `deep`, `quick`) |
| `skills/` | `%USERPROFILE%\.agents\skills\` (Codex), junction-linked into `.claude\skills` |

`install.ps1` only adds and updates; it never removes hooks, plugins, skills or agents.

## Auto-update

`auto-update/update-config.ps1` runs at logon (task "agent-config auto-update",
registered by `auto-update/register-task.ps1`: 2 min delay, network only).

1. Check: the script fetches FrontierCode `data.json`, then a headless `claude -p`
   with only WebFetch/WebSearch compares it (plus cross-checks) with the model
   table and returns JSON (`check.schema.json`). Its model is in
   `check-model.json` and is re-dialled by every update.
2. Unchanged: exits silently. Changed: toast, then a headless run with only
   Read/Edit/Write/Glob/Grep, on the `implement` agent's model, applies the
   validated reading to the allowlisted files. The script rolls back anything
   else, runs `install.ps1`, commits locally (never pushes) and toasts.

Logs: `%LOCALAPPDATA%\agent-config\logs` (last 30 runs). `-Force` runs the
update phase even when the check reports no change.

Edit here, then run:

```powershell
powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\Code\agent-config\install.ps1"
```

Rules of thumb (from the 2026-10-07 research, see `docs/research.md`):
- `AGENTS.md` holds only cross-project, personal rules. Keep it short and hand-written.
- Project commands and conventions go in each project's own `AGENTS.md`.
- Anything that must always happen becomes a hook or setting, not a sentence.
- Add a skill only after reading every file in it; pin third-party skills to a commit.
