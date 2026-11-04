# agent-config

A portable toolkit for how coding agents (Claude Code and Codex) behave, on any
machine. Design: `docs/portable-toolkit-design.md`.

## agtk (phase 1: engine, no wizard yet)

`agtk` is one Go binary that installs the same setup without Node or PowerShell
(design: `docs/wizard-design.md`). The catalog (`catalog/*.yaml`) and vendored packs
are embedded; personal profiles are read from disk.

```powershell
go build -o dist\agtk.exe .\cmd\agtk
dist\agtk.exe apply --profile profiles\home   # later runs: dist\agtk.exe apply
dist\agtk.exe status                          # what is installed, what is left to do
dist\agtk.exe undo                            # restore the files the last apply changed
dist\agtk.exe recommend                       # catalog suggestions for this machine and profile
dist\agtk.exe project add-checks C:\path\to\repo "uv run pytest -q"
dist\agtk.exe check                           # catalog, baseline, profile, budget checks
```

It copies itself to `~/.agtk/bin`, and the hooks run as `agtk hook git-guard` and
`agtk hook stop-checks`. Every changed file is backed up to `~/.agtk/backups` first.
`profiles/<p>/profile.yaml` is the agtk profile; `profile.json` stays for the
PowerShell installer below, and `agtk check` fails when the two disagree.
`--skip-tools` leaves out the `claude`/`npm` commands (MCP, plugins, npm packages).

## PowerShell installer (fallback)

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1 -Profile home   # or -Profile work
```

The profile is remembered; later runs need no argument. After an install with
hooks, open Codex once and run `/hooks` to trust the hook (Codex skips untrusted
hooks silently; the execpolicy rules work without trust).

| Path | Installs to |
| --- | --- |
| `base/AGENTS.md` + `profiles/<p>/AGENTS.md` | concatenated into `%USERPROFILE%\.codex\AGENTS.md`; `~/.claude/CLAUDE.md` is generated to import it |
| `profiles/<p>/profile.json` | what the profile installs: packs, model settings, Codex profiles |
| `profiles/home/claude/model-settings.json` | merged into `modelSettings` of `~/.claude/settings.json` |
| `profiles/home/codex/*.toml` | `model`/effort patched into `~/.codex/config.toml`; `deep`, `quick` profiles |
| `packs/<pack>/agents/` | `~/.claude/agents/` (scout, mechanic, implement, review) |
| `packs/<pack>/skills/` | vendored skills, listed in `skills/baseline.json` with their pinned source commit; copied to `~/.agents/skills` and junctioned into `~/.claude/skills` |
| `enforcement/hooks/` | `~/.agent-config/hooks/`; git-guard registered as PreToolUse in both tools |
| `enforcement/codex-rules/` | `~/.codex/rules/agent-config.rules` (git push, reset --hard forbidden) |
| `profile.json` `mcp` | Claude Code user-scope MCP server; Codex `[mcp_servers.<name>]` with `enabled_tools` |
| `profile.json` `npmGlobal`, `claudePlugins` | pinned global npm packages (pyright) and Claude Code plugins (pyright-lsp) |
| `profile.json` `projectSkills` | skills with `"scope": "project"` installed into one project's `.agents/skills` / `.claude/skills` (git-excluded) |

Profiles: `home` (personal accounts, FrontierCode model table, auto-update) and
`work` (corporate gateway, no personal data, core pack only). Tiers: `A` installs
hooks; `B` (chosen automatically when `node` is missing or managed settings block
user hooks, or forced with `-Tier B`) writes `permissions.deny` rules instead.

`install.ps1` only adds and updates; it never removes hooks, plugins, skills or
agents. `skills/install-skills.ps1` replaces a skill folder only if it carries its
own `.agent-config-pin`; it strips `allowed-tools` from every skill and makes
`explicit_only` skills hidden from the model (`disable-model-invocation`,
`allow_implicit_invocation: false`). Backups of the pre-toolkit files are in
`Code\_archive\agent-config-before-toolkit-2026-11-04`.

## Other tools

- `bootstrap/add-project-checks.ps1 -Project <dir> -Command "<checks>"` wires the
  Stop-hook check runner into one project (`.claude/settings.local.json`,
  `.codex/hooks.json`, both git-excluded).
- `bootstrap/build-work-kit.ps1` builds `dist/agent-kit-work.zip` for a work machine
  that can't clone this repo; it fails if a personal marker is in the kit. CI builds
  it as an artifact.
- `checks/budget.mjs` (always-loaded tokens per profile against the budgets),
  `checks/test-hooks.mjs`, `checks/validate-skills.mjs`; all run in CI.

## Skills

Few, audited, pinned, vendored. To add one: read every file in it, copy it into
`packs/<pack>/skills/<name>` at the commit you read, add an entry to
`skills/baseline.json`, run the checks and `install.ps1`. Project-specific skills
(vbgym's UI skills, Sol's Apple-design skills) stay in their projects.

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
   findings 1, 3 from <json>"), which updates the vendored copy in `packs/` and
   its pin in `skills/baseline.json`, runs the checks and `install.ps1`, and
   commits locally. The auto-update applies to the home profile only.

Every report opens in the default browser by itself (toast clicks cannot open
links from a script). Logs and reports: `%LOCALAPPDATA%\agent-config\logs` (last
30 runs). `-Force` runs the update phase even when the check reports no change,
or the skills check before its week is up.

Rules of thumb (from the 2026-10-07 research, see `docs/research.md`):
- `AGENTS.md` holds only cross-project, personal rules. Keep it short and hand-written.
- Project commands and conventions go in each project's own `AGENTS.md`.
- Anything that must always happen becomes a hook or setting, not a sentence.
- Add a skill only after reading every file in it; pin third-party skills to a commit.
