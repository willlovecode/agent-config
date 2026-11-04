# AI toolkit wizard: design

Status: approved 2026-11-04 (name changed to agtk). Phase 1 built. Revised 2026-11-05 with the
approved research (`reports/wizard-research-2026-11-05.html`, items 1-9): Go confirmed after
Go/Rust/.NET prototypes; the changes are listed under "Changes from the 2026-11-05 research".

## Goal

Anyone downloads one file from the GitHub releases, runs it, answers a short
questionnaire in an old-school DOS-style setup screen, and gets Claude Code and Codex
configured: instructions, skills, hooks, MCP servers, plugins and model roles, each
recommended for their machine and use (personal or corporate), each changeable before
anything is written. Windows and Linux. A Windows GUI comes later on the same engine.

## One program, no runtime: `agtk`

A single Go binary per platform (`agtk.exe` for Windows x64/arm64, `agtk` for Linux
x64/arm64), built and attached to GitHub releases by CI. No Node, Python or
PowerShell 7 needed on the target machine. The same binary does everything:

| Command | Does |
|---|---|
| `agtk` | the wizard (scan, questionnaire, recommendations, review, install) |
| `agtk apply [--profile file]` | non-interactive install of a saved profile (re-run, CI, a second machine) |
| `agtk hook git-guard` / `agtk hook stop-checks ...` | the hooks themselves, so they need no Node; today's .mjs logic ported 1:1 with the same 46 test cases |
| `agtk project add-checks <dir> "<cmd>"` | wire the Stop check runner into a project |
| `agtk status` | what's installed, which tier, what Codex still needs to trust |
| `agtk undo` | restore the backup taken before the last apply |
| `agtk check` | the budget, hook and catalog checks (CI uses the same code) |

Why Go: static cross-compiled binaries from one codebase; the console UI library
`tcell` gives full control of colours and box drawing on Windows Terminal, conhost and
Linux terminals; the GUI later can reuse the engine (Fyne or a native Windows shell).
Go 1.27 is installed here; WSL Ubuntu serves as the Linux test machine.

## The look

80x25, blue background, grey/white text, double-line frames, a highlighted bar, key
hints on the bottom line, like MS-DOS and early Windows setup programs. Keyboard only.
Falls back to plain ASCII frames when the terminal can't draw box characters.

```
 AI Toolkit Setup                                                    v0.1
 ════════════════════════════════════════════════════════════════════════

   ╔═══════════════════ Step 3 of 6: How will you use it? ════════════╗
   ║                                                                  ║
   ║   Who is this machine for?                                       ║
   ║                                                                  ║
   ║     ( ) Personal                                                 ║
   ║     (•) Corporate, through a company LLM gateway                 ║
   ║     ( ) Corporate, with my own API key                           ║
   ║                                                                  ║
   ║   Which agent tools will you use?                                ║
   ║                                                                  ║
   ║     [X] Claude Code        found: 2.1.318                        ║
   ║     [X] Codex              found: 0.166.0                        ║
   ║                                                                  ║
   ╚══════════════════════════════════════════════════════════════════╝

 ENTER=Continue  SPACE=Select  ESC=Back  F1=Help  F3=Exit
```

```
   ╔══════════════════ Step 5 of 6: Recommended setup ════════════════╗
   ║                                                                  ║
   ║  [X] Git guard hook          blocks push / reset --hard / clean  ║
   ║  [X] Project check runner    checks run before the agent stops   ║
   ║  [X] tdd                     test-first loop (both tools)        ║
   ║  [X] diagnosing-bugs         feedback-loop debugging             ║
   ║  [ ] Context7                off: corporate, data leaves company ║
   ║  [X] jdtls (Java LSP)        Java 21 found; legacy codebase      ║
   ║                                                                  ║
   ║  Always loaded: ~980 tokens of 3,100 budget   ████████░░░░░░░░   ║
   ╚══════════════════════════════════════════════════════════════════╝

 ENTER=Install  SPACE=Toggle  F1=Why this?  ESC=Back
```

## Flow

1. **Welcome**: what it will and won't do; nothing is written before step 6.
2. **Scan** (shown, never stored): OS and version, shell, CPU, RAM, GPU; installed
   tools and versions (claude, codex, git, node, npm, java/jdk, mvn/gradle, python/uv,
   go, docker); company policies that block hooks or plugins (Claude Code managed
   settings, Codex `requirements.toml`, on both OSes); whether agtk can run from its
   install folder. No network checks. Result: tier A (hooks allowed) or B (rules only).
3. **Use**: personal or corporate (gateway / own key); which tools; how models are
   accessed (subscription, API key, gateway; for a gateway, which model names it serves).
4. **Work**: languages and stacks (Java/Spring, Python, JS/TS, Go, ...), kinds of work
   (legacy migration, greenfield, research, UI, data, ops), autonomy (ask often /
   balanced / autonomous), guardrail strictness, context budget (lean / normal).
5. **Recommendations**: the catalog's rules turn scan + answers into a pre-ticked
   list; F1 on a line shows why it's recommended or not, its source, pin and audit
   result. A live meter shows the always-loaded token cost against the budget.
6. **Review and install**: a summary of every file that will change, backup, apply,
   then a result screen with what's left for the user (e.g. "open Codex, run /hooks").

## The catalog

`catalog/` in the repo: one YAML file per item (skill, hook, MCP server, plugin,
instruction module, model-role preset). Each item carries:

- what it is, source repo, pinned commit, vendored path, audit results, licence;
- which tools and OSes it supports;
- `recommend_when` rules over scan facts and answers, e.g.
  `stack contains java and tool claude_code and jdk >= 17`;
- `never_when` rules, e.g. Context7 never for corporate unless the user opts in;
- a one-line reason for each outcome, shown in the wizard;
- its always-loaded token cost (measured by `agtk check`).

Today's vetted items (git guard, Stop checks, Codex rules, tdd, diagnosing-bugs,
grilling, paper-lookup, Spring Boot skill, Context7, pyright-lsp, subagents) become
the first catalog entries. Instruction text is split into modules (communication,
autonomy levels, verification, secrets, corporate data, legacy code) that the wizard
assembles into AGENTS.md according to the answers.

## Privacy

- The scan lives in memory for the run and is never written, logged or sent anywhere.
- The only thing saved is the profile: the user's choices (no hardware, no hostnames,
  no usernames), at `~/.config/agtk/profile.yaml` (Linux) / `%APPDATA%\agtk\profile.yaml`
  (Windows), so `agtk apply` and re-runs work. The wizard says so and offers not to save.
- No telemetry, no network calls except fetching the release itself and the user's
  chosen MCP servers.
- Backups of changed files go to a local folder; `agtk undo` restores them; the user
  can delete them from the result screen.

## Public toolkit, private you

If the repo is public, it must stay generic. Your personal setup becomes just a saved
profile (your answers + your model table) in a separate private repo or local file,
applied with `agtk apply --profile`. The current `profiles/home` and `profiles/work`
turn into two such saved profiles. The work zip becomes unnecessary: the release binary
plus a profile file is the whole kit.

## Platform notes

| | Windows | Linux |
|---|---|---|
| Config locations | `%USERPROFILE%\.claude`, `.codex`, `.agents` | `~/.claude`, `~/.codex`, `~/.agents` |
| Skill links into Claude Code | directory junctions | symlinks |
| Managed policies scanned | `C:\Program Files\ClaudeCode\managed-settings.json` + `.d\`, registry `Policies\ClaudeCode`, `%ProgramData%\OpenAI\Codex\requirements.toml` | `/etc/claude-code/managed-settings.json` + `.d/`, `/etc/codex/requirements.toml` |
| Hook commands | `agtk.exe hook ...` (no shell needed) | `agtk hook ...` |
| Terminal | Windows Terminal, conhost (VT mode) | any VT100 terminal |

## Build plan (each phase ends usable)

1. **Engine without UI**: Go module, catalog loader, scan, recommendation rules,
   `agtk apply` reproducing today's install for both saved profiles on Windows, with
   backups and `undo`. The hooks ported to `agtk hook`, same test cases.
2. **Linux**: paths, symlinks, policy files; tested in WSL Ubuntu; CI builds both OSes.
3. **DOS-style wizard**: the six screens on top of the engine.
4. **Release**: GitHub Actions builds and attaches binaries with checksums; README
   with the one-line download per OS.
5. **Later**: Windows GUI wizard on the same engine; more catalog items (java pack,
   more stacks) as each is read and trialled.

The current PowerShell installer keeps working until phase 1 replaces it.

### Phase 1 as built (2026-11-04)

- Layout: `cmd/agtk` (CLI), `internal/{hooks,catalog,profile,scan,install,check,ojson}`,
  `assets.go` embeds `catalog/`, `base/`, `packs/`, the Codex rules and
  `skills/baseline.json`. Personal profiles are not embedded.
- agtk keeps its binary, backups and `state.json` in `~/.agtk`. The state remembers the
  profile path, so `agtk apply` without `--profile` repeats the last one. Saving the
  wizard's answers to `%APPDATA%\agtk\profile.yaml` comes with the wizard (phase 3).
- The PowerShell installer stays as the fallback for machines that block unsigned
  binaries, so `profile.json` and `skills/baseline.json` stay too. `agtk check` fails
  when they disagree with `profile.yaml` and the catalog.
- Instructions are still base + the profile's own `AGENTS.md`. Splitting them into
  modules belongs with the questionnaire (phase 3).
- The scan does not look at the GPU or the network yet (both are only shown in the
  wizard), and it reads Codex `requirements.toml` only at the Linux path, because the
  Windows location is not confirmed.
- Settings files are edited with their key order kept, and written as UTF-8. That also
  fixes the PowerShell 5.1 installer's habit of reading `settings.json` as ANSI.
- The skill pin hash now sorts files ordinally in both installers. The old culture sort
  in PowerShell gave a different hash for `spring-boot-skill`.

## Changes from the 2026-11-05 research

Approved items 1-9 of `reports/wizard-research-2026-11-05.html`. Each lands in the phase named.

- **Hooks fail closed** (done 2026-11-05): a Claude Code hook that can't start, times out or
  errors lets the tool call through by default (verified live). The git-guard group now carries
  `"onFailure": "block"` (Claude Code 2.1.295+) in both `agtk apply` and `install.ps1`. Before
  registering hooks, agtk must run itself from `~/.agtk/bin` once; if that fails (App Control /
  AppLocker), it falls back to tier B and says so. Phase 1 follow-up.
- **Scan, policies** (phase 2): Codex `%ProgramData%\OpenAI\Codex\requirements.toml` (confirmed)
  besides `/etc/codex/requirements.toml`; keys: top-level `allow_managed_hooks_only`,
  `[features] plugins`, `[marketplaces]`, `mcp_servers`. Claude Code: also
  `managed-settings.d\`, and the registry value `Settings` under
  `HKLM`/`HKCU\SOFTWARE\Policies\ClaudeCode`; `%ProgramData%\ClaudeCode` is legacy and not
  read by Claude Code. Each capability is reported as allowed / blocked / unknown with its
  source; unknown never counts as allowed (cloud and MDM policy can't be seen locally).
- **Scan, hardware and network**: GPU VRAM via DXGI or the registry `qwMemorySize`, never WMI
  `AdapterRAM` (32-bit). Drop the network reachability check (it contradicted "no network
  calls"); gateways and proxies are recorded as "set / not set" only, never values.
- **Rules** (before the catalog grows): structured YAML conditions (`all` / `any` / `not` over
  `{fact, op, value}`) instead of the text language, with "unknown" when a fact is missing.
  The evaluator returns a per-clause trace that F1 shows; `why:` stays as the summary line.
  Catalog files get a JSON Schema checked in CI; unknown fact names fail CI. Items also carry
  supported client versions, dependencies, conflicts and install scope.
- **Writing** (phase 2): `state.json` keeps the previous written value per owned key/block, so
  re-runs do a three-way comparison and show user edits as conflicts. Undo keeps later user
  edits. Windows-safe atomic replace (`os.Rename` doesn't promise it), a journal for multi-file
  runs, protected backups. v0.1 asks the user to close Claude Code and Codex before writing
  shared files; prefer `claude mcp add` over editing `~/.claude.json`.
- **UI** (phase 3): tview on tcell, widgets behind a small own interface. ASCII fallback is
  explicit (tcell prints `?` for double lines without a UTF-8 locale or codepage 65001), also
  for selection marks; single lines on `TERM=linux`; `NO_COLOR`; an 80x25 "too small" screen
  that offers plain mode. `--plain` (numbered prompts) and `--answers file` for screen
  readers and CI.
- **GUI** (later): drives the engine through a versioned `agtk plan --json` / `apply --plan`
  protocol (progress, cancel, structured errors), so Wails, Tauri or WinForms all stay possible.
- **Release** (phase 4): GoReleaser, `SHA256SUMS`, SBOM, build attestations, immutable releases,
  `govulncheck`; no `curl | sh`; self-update only notifies. Signing via SignPath Foundation once
  eligible (Azure Artifact Signing doesn't take individuals outside US/CA). winget and Scoop
  after signing. An "export for IT" output (managed-settings snippet, requirements.toml
  fragment, file list) for machines that block the binary; the PowerShell fallback is subject
  to Constrained Language Mode there.
- **Scope**: Claude Code and Codex only; OpenCode is the cheapest third target later. Linux
  support means a tested matrix (Ubuntu LTS, Debian, Fedora, Alpine) in CI containers.
- **Licences**: `THIRD_PARTY_NOTICES` generated from `packs/*/skills/_licenses` in every release
  archive, plus an About -> Licences screen.

## Open questions (defaults if unanswered)

- Public or private repo: default **public toolkit**, personal profiles private.
- Name: `agtk` (agent toolkit), decided 2026-11-04.
- Code signing for `agtk.exe`: none at first; SignPath Foundation once the project qualifies.
  Some company machines block unsigned binaries; the PowerShell installer and the IT export
  are the fallbacks for those.
- GUI framework: decided in the GUI phase (Wails, Tauri or WinForms over the JSON plan).
