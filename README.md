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
| `skills/baseline.json` | skills pinned to a commit in `%USERPROFILE%\.agents\skills\` (Codex), junction-linked into `.claude\skills`; plus the Claude Code and Codex skill plugins |

`install.ps1` only adds and updates; it never removes hooks, plugins, skills or agents.
`skills/install-skills.ps1` (called by it) replaces a skill folder only if it
carries its own `.agent-config-pin` file. `explicit_only` skills get
`disable-model-invocation: true` (Claude Code) and
`allow_implicit_invocation: false` (Codex). The pre-baseline skills are in
`Code\_archive\skills-2026-11-04` (`MANIFEST.txt` lists every move).

## Skills

Few, audited, pinned. To add one, read every file in it first, then add an
entry to `skills/baseline.json` with the commit you read and run `install.ps1`.
Project-specific skills (vbgym's UI skills, Sol's Apple-design skills) stay in
their projects.

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

3. Skills, at most weekly (`auto-update/skills-check.ps1`, the task's second
   action): the script lists upstream commits since each pin and local skills
   the baseline does not manage. A headless run on the `implement` agent's
   model with only WebFetch/WebSearch judges updates, audit risks and better
   alternatives. It applies nothing. Findings open as a report in the browser
   with a toast; approve in a Claude Code session in this repo ("Apply skill
   findings 1, 3 from <json>"), which edits `skills/baseline.json`, runs
   `install.ps1` and commits locally.

Every report opens in the default browser by itself (toast clicks cannot open
links from a script). Logs and reports: `%LOCALAPPDATA%\agent-config\logs` (last
30 runs). `-Force` runs the update phase even when the check reports no change,
or the skills check before its week is up.

Edit here, then run:

```powershell
powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\Code\agent-config\install.ps1"
```

Rules of thumb (from the 2026-10-07 research, see `docs/research.md`):
- `AGENTS.md` holds only cross-project, personal rules. Keep it short and hand-written.
- Project commands and conventions go in each project's own `AGENTS.md`.
- Anything that must always happen becomes a hook or setting, not a sentence.
- Add a skill only after reading every file in it; pin third-party skills to a commit.
