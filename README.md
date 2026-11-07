# agent-config

A portable toolkit for Claude Code, Codex, and Gemini CLI / Antigravity.
See `docs/portable-toolkit-design.md` for the toolkit design.

## Install agtk

agtk is a single static binary (no Go, Node or PowerShell needed to run it). Releases
carry builds for Linux, macOS and Windows on x86-64 and arm64, plus `SHA256SUMS`. Nothing
below needs admin rights or sudo; the binary goes to `~/.agtk/bin/agtk` (`%USERPROFILE%\.agtk\bin\agtk.exe`
on Windows), the same place `agtk apply` copies itself to for its hooks.

**Linux and macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/willlovecode/agent-config/master/get-agtk.sh | sh
```

`get-agtk.sh` picks the archive for your OS and CPU, checks it against `SHA256SUMS`, installs it
and prints the one line that puts `~/.agtk/bin` on your PATH for your shell (bash, zsh or fish).
It never edits your shell files. At a terminal it then asks one question, "Run the setup wizard
now? [Y/n]", answered with a single key (it reads `/dev/tty`, so it works under `curl | sh`);
yes starts `~/.agtk/bin/agtk wizard` by full path, so PATH does not have to be set yet. Without
a terminal it only prints the next step. `AGTK_VERSION=1.2.3` picks a release other than the latest.

By hand: download `agtk_<version>_<os>_<arch>.tar.gz` and `SHA256SUMS` from the
[releases page](https://github.com/willlovecode/agent-config/releases), then

```sh
sha256sum -c --ignore-missing SHA256SUMS      # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
mkdir -p ~/.agtk/bin && tar -xzf agtk_*_linux_amd64.tar.gz -C ~/.agtk/bin agtk
```

**Windows**

```powershell
irm https://raw.githubusercontent.com/willlovecode/agent-config/master/get-agtk.ps1 | iex
```

`get-agtk.ps1` (Windows PowerShell 5.1 or 7) does the same for x64 and arm64: it prints the
command that adds `%USERPROFILE%\.agtk\bin` to your user PATH without running it, and at a console
asks "Run the setup wizard now? [Y/n]" (one key), then starts the installed `agtk.exe wizard`. By hand, unzip
`agtk_<version>_windows_<arch>.zip` from the releases page to that folder after checking its SHA-256
(`Get-FileHash`) against `SHA256SUMS`.

**Next**: if you did not answer yes to the wizard question, open a new terminal and run `agtk wizard`. Until PATH is set, use the full path
(`~/.agtk/bin/agtk wizard`). After `agtk apply` or the wizard, agtk repeats the PATH line when
`~/.agtk/bin` is still missing from PATH.

**Offline, or a private repository.** Download the archive and `SHA256SUMS` on a machine that can
reach the release (a private repo needs you logged in, or `gh release download`), copy them over
(`scp agtk_*_linux_amd64.tar.gz SHA256SUMS laptop:`), and install without the network:

```sh
AGTK_FILE=./agtk_1.2.3_linux_amd64.tar.gz AGTK_SUMS=./SHA256SUMS sh get-agtk.sh
```

Without `AGTK_SUMS` the archive is installed unverified, with a warning. `AGTK_BASE_URL` points the
script at another folder holding `SHA256SUMS` and the archives (an internal mirror). The `curl | sh`
one-liner cannot read a private repository: download `get-agtk.sh` and the archive by hand and use
`AGTK_FILE`.

**From source** needs Go matching `go.mod` (1.27). With network access an older Go downloads the
right toolchain by itself (`GOTOOLCHAIN=auto`, the default):

```sh
go build -o dist/agtk ./cmd/agtk              # Windows: go build -o dist\agtk.exe .\cmd\agtk
bash scripts/release.sh [version]                # all six platforms into dist/release/ (Git Bash works on Windows)
```

Pushing a tag `v*` runs `.github/workflows/release.yml`, which tests, runs `scripts/release.sh` and
publishes the archives and `SHA256SUMS` as a GitHub Release.

**Windows only:** the scheduled auto-update (`auto-update/`, a Windows Task Scheduler job), the legacy
`install.ps1` fallback, and the project-level PowerShell helpers in `bootstrap/`. agtk itself, the
wizard, hooks, skills and `agtk check` work the same on all three systems.

## agtk (engine and wizard)

`agtk` is one Go binary that installs the same setup without Node or PowerShell
(design: `docs/wizard-design.md`). The catalog (`catalog/*.yaml`) and vendored packs
are embedded; personal profiles are read from disk.

Install it first (above). `apply --profile profiles/home` and `check` read files from a checkout of this repo, so run them there; the other commands work anywhere.

```sh
agtk wizard                          # full-screen wizard, one keypress per answer
agtk wizard --plain                  # scrolling prompts, Enter after each answer
agtk --answers answers.yaml --save   # unattended; remember the profile for apply
agtk apply --profile profiles/home   # later runs: agtk apply
agtk status                          # what is installed, what is left to do
agtk undo                            # restore the files the last apply changed
agtk recommend                       # catalog suggestions for this machine and profile
agtk project add-checks ~/Code/repo "uv run pytest -q"
agtk check                           # catalog, baseline, profile, budget checks
```

The wizard works like `p10k configure`: one screen at a time, the terminal cleared
before each page, nothing scrolling back. It asks before checking your machine.
Choices are listed as `(1)  Personal.` and a single keypress picks one, with no
Enter needed. On a list, a number toggles an item and redraws the page, **y** or
Enter continues, **n** and **p** turn pages, **?** describes a choice, **b** goes
back, **r** restarts and **q** quits and writes nothing. The last screens show
exactly what will be written; press **y** to apply. It runs the same on Windows,
Linux and macOS, needs a window of at least 60x16, and honors `NO_COLOR`.

With `--plain`, a terminal that is too small, `TERM=dumb`, or redirected input or
output (pipes, CI), the scrolling wizard runs instead: press Enter after each answer.
For a multiple-choice question, enter one or more numbers separated by spaces
or commas to toggle them and continue. In the Items list, Enter opens Review.
Use `--answers` to automate installation from a YAML profile.

It copies itself to `~/.agtk/bin`, and the hooks run as `agtk hook git-guard` and
`agtk hook stop-checks`. Every changed file is backed up to `~/.agtk/backups` first.
`profiles/<p>/profile.yaml` is the agtk profile; `profile.json` stays for the
PowerShell installer below, and `agtk check` fails when the two disagree.
`--skip-tools` leaves out the `claude`/`npm` commands (MCP, plugins, npm packages).

**Instruction files.** Each tool gets its own, generated from the base text plus the
profile's `instructions` file, with a short "generated by agtk" comment on top. Neither
imports the other:

| File | Written when | Holds |
| --- | --- | --- |
| `~/.claude/CLAUDE.md` | Claude Code is chosen | base + profile instructions, plus routing guidance if enabled |
| `~/.codex/AGENTS.md` | Codex is chosen | base + profile instructions, plus routing guidance for confirmed models |
| `~/.gemini/GEMINI.md` | Gemini CLI / Antigravity is chosen | the same text as AGENTS.md without routing, plus the handoff procedure |

A Claude-only setup creates nothing under `~/.codex`, and a Codex-only one nothing under
`~/.claude`. An older one-line `@import` CLAUDE.md is replaced (backed up first). Edit the
profile, not these files, and re-run `agtk apply`.

**Gemini and handoff.** Choosing "Gemini CLI / Antigravity" writes only `~/.gemini/GEMINI.md`, which both read; nothing else under `~/.gemini` is touched. It is off by default unless `gemini` is on PATH. Agents cannot share live context, so the `handoff` skill (on in the home profile; Gemini gets the same steps inline in GEMINI.md) writes `.agent-handoff/latest.md` in the repo root and keeps it out of git through `.git/info/exclude`. To hand over, run the handoff skill in agent A, then paste the one line it prints into agent B: "Read .agent-handoff/latest.md, then run git status and git diff, then continue with Next step." The routing table tells an agent to do this when a limit is hit, and the shared instructions tell every agent to read a handoff newer than the last commit at session start.

**Profile.** Pick Personal or Company and choose from grouped tasks, languages,
frameworks and platforms. Use **n** and **p** to page through long lists. Press
**/** and enter text to filter, or press **?** then a number to see details.
In line mode, use `/text` and `?N`.

**Items.** Press **v** to choose a view, or type its name in line mode.
**General** includes optional skills
such as `i-have-adhd` for action-first replies and `caveman` for terse technical
replies. **Popular** is a dated shortlist from GitHub repository stars and
skills.sh All Time installs. Each entry shows its purpose and source.
`find-skills` can look up current options after you activate it. Pass
`wizard --project DIR` to include project skills.

**Models.** Select clients, then choose accounts and confirm accessible models
before comparing routing presets. Detected clients start checked. You can also
prepare configuration for a client you plan to install later. Preset details
show the task routes and dated figures for token limits, price, usage and speed.
Continue without selecting accounts or models, or choose **Keep current models**,
to leave your model settings in place.
agtk sets supported Claude Code or Codex defaults and writes guidance for
switching tools or accounts yourself. Older Personal plan-only profiles still
work; new ones record `answers.models` alongside `plans` and `routing`.

**Review and Company.** Review lists the named components for each client,
file changes and possible commands. Company keeps current models by
default and requires IT approval for external integrations, command-based
skills, plugins and model changes. The same checks apply to unattended
installs and reapplying a profile. See the [wizard design and Company policy](docs/wizard-design.md#company-policy) for the allowlist and German deployment guidance.

## PowerShell installer (fallback)

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1 -Profile home   # or -Profile work
```

The profile is remembered; later runs need no argument. After an install with
hooks, open Codex once and run `/hooks` to trust the hook (Codex skips untrusted
hooks silently; the execpolicy rules work without trust).

The PowerShell path is a fallback for machines without agtk. It does not support the
Models step (plans, routing presets, per-tool instruction files). When
`%USERPROFILE%\.agtk\bin\agtk.exe` exists and the profile has a `profile.yaml`,
`install.ps1` hands over to `agtk apply --profile profiles\<p>` (passing `-Tier` as
`--tier`) and exits with its exit code, so `auto-update/update-config.ps1` and old habits
keep working and give the same result as agtk. With a Go toolchain and this checkout it
builds a fresh agtk first, because agtk embeds the packs and base text. Without agtk, or
for a profile that has only `profile.json`, it runs the legacy path below: it writes the
same text to `~/.codex/AGENTS.md` and to a standalone `~/.claude/CLAUDE.md` (neither imports
the other, as with agtk), without agtk's generated-file comment, and applies no routing, so
that CLAUDE.md has no routing table. Run agtk once on that machine to get the table and the
Models step.

| Path | Installs to |
| --- | --- |
| `base/AGENTS.md` + `profiles/<p>/AGENTS.md` | legacy path: concatenated into `%USERPROFILE%\.codex\AGENTS.md` and, as the same text, `%USERPROFILE%\.claude\CLAUDE.md` (agtk adds a generated-file comment and the routing table to the Claude one, see above) |
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

Skills written in this repo (`handoff`) are marked `authored: true` in their catalog item: no pin, not in `baseline.json`, so only agtk installs them.

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
