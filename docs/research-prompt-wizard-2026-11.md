# Research prompt: best setup for the AI toolkit wizard

Paste this into a fresh Claude Code session started in `C:\Users\walbe\Code\agent-config`.

---

Research the best way to build an AI-toolkit installer wizard, from a clean slate, and
give me a report to approve item by item. This is research and small prototypes only:
nothing gets installed into my real setup, and you don't change this repo except for
writing the report.

## What I want to build

A program people download from my GitHub and run on **Windows or any Linux**. It scans
the machine (hardware, installed tools, company policies), asks a short questionnaire
(personal or corporate use, how models are accessed, stacks, kind of work, autonomy,
strictness), then recommends and installs a setup for coding agents: Claude Code and
Codex first. It covers instructions (AGENTS.md), skills, hooks, MCP servers, plugins and
model roles. Each recommended item has a reason and can be changed before anything is
written.

- **Look:** old-school DOS-style setup first: blue screen, box frames, keyboard only.
  A Windows GUI wizard comes later and should reuse the same engine.
- **Privacy:** the scan is never stored or sent. At most the user's choices are saved,
  so the setup can be re-applied.
- **Corporate machines** must be considered: company LLM gateways, blocked plugins and
  hooks, possibly blocked unsigned binaries.

## Start clean

Form your own recommendation first. Only then read `docs/wizard-design.md`, an earlier
draft, and compare: where you agree, where you differ, and why. Don't read it before you
have your own answer, so it doesn't anchor you. Context you may read any time:

- `README.md` and `docs/portable-toolkit-design.md`: the current PowerShell-based toolkit
  this would replace.
- `reports/addons-research-2026-11-04.html`: the vetted add-ons that become the first
  catalog entries.

Another agent may be working in this repo in parallel. Don't edit its files.

## Questions to answer

1. **Prior art.** Does something already do this, or most of it, well enough to build
   on or fork? Look at:
   - agent config sync and setup tools: vercel-labs `skills` CLI, openskills, rulesync,
     ruler, claude-code-templates and others;
   - Anthropic's and OpenAI's own setup helpers;
   - dotfile managers (chezmoi, yadm, home-manager);
   - Ninite- or winget-configure-style installers.
2. **Language and UI stack** for one codebase on Windows and Linux, a DOS-style console
   UI now and a Windows GUI later. Compare at least:
   - Go (tcell or tview, Bubble Tea);
   - Rust (ratatui, cursive);
   - .NET (Terminal.Gui, single-file publish);
   - C++ (magiblot/tvision, a modern Turbo Vision port);
   - Free Pascal (Free Vision);
   - Python (Textual, packaged).

   Judge each on: authentic old-school look, terminal compatibility (Windows Terminal,
   old conhost, Linux terminals, SSH), binary size, startup time, dependencies on the
   target machine, the GUI path later, and how maintainable it is for one person.
3. **Machine scan without admin rights** on both OSes:
   - CPU, RAM, GPU;
   - installed tools and versions;
   - Claude Code managed settings and Codex `requirements.toml` locations per OS;
   - proxies and gateways.

   Which libraries, and what must not be collected?
4. **Catalog and recommendation rules:** the rule format (simple expressions, CEL,
   JSONLogic, Rego, plain code), schema validation, how to show "why" for each item, and
   how to measure the always-loaded context cost of a setup.
5. **Writing config safely:** merging JSON and TOML while keeping formatting and
   comments, idempotent re-runs, backups and undo, and how to test it (golden files, WSL,
   containers, Windows Sandbox).
6. **Distribution and trust:**
   - GitHub Releases with checksums;
   - signing (Authenticode cost and SmartScreen, Sigstore/cosign);
   - winget, scoop, apt and other channels;
   - the risks of `curl | sh`;
   - corporate AppLocker and unsigned-binary policies, offline installs;
   - self-update risks, SBOMs and reproducible builds.
7. **Scope beyond Claude Code and Codex:** Copilot CLI, Gemini CLI, Cursor, OpenCode.
   Which share AGENTS.md and the skills format, and is supporting them worth it now?
8. **Old-school UX done right:**
   - references: MS-DOS and Windows 3.x setup, InstallShield, the Debian installer,
     whiptail/dialog, NetBSD sysinst;
   - keyboard conventions;
   - box-drawing fallbacks and colour limits;
   - small terminals, and screen-reader accessibility.
9. **Licensing:** may the vetted third-party skills be redistributed (vendored) in a
   public repo, and what attribution is required?

## How to work

- **Evidence:** official docs and repos first, then practitioner reports from the last
  60 days (HN via the Algolia API, Reddit via the Arctic Shift archive, GitHub). Give
  each claim a link and a date, and say plainly what you could not read.
- **Prototypes:** for the top 2-3 UI stacks, build a tiny "setup screen" in
  `C:\Users\walbe\Code\_scratch\wizard-proto-<date>`: blue background, double frame, a
  radio list, a checkbox list, bottom key bar. Run each in Windows Terminal, the old
  console host and WSL Ubuntu. Record binary size, startup time, how it renders,
  whether resizing works and whether the box characters fall back cleanly, with
  screenshots.
- **Models and cost:** follow the model table in AGENTS.md. Run the main session on
  Opus at medium. Wide searching and reading go to `scout` (Haiku at medium), and there
  are no Opus fan-outs. Say which reading of the table each choice is based on.
- **Second opinion:** after the shortlist, get Codex's view, read-only:
  `codex --profile deep exec "<shortlist and reasoning>"`. Report where it agrees and
  where it disagrees.

## Deliverable

One HTML report at `reports/wizard-research-<date>.html`, in the same style as
`reports/addons-research-2026-11-04.html` (light and dark). Contents:

1. Short summary: the recommended stack and architecture in a few sentences.
2. Numbered recommendations per question, each with: the choice, the alternatives and
   why not, the evidence and links, the prototype results where relevant, risks, and
   recommend (yes / consider).
3. Comparison with `docs/wizard-design.md`: agree, differ, and what to change.
4. Rejected options, one line each.
5. Codex's second opinion.
6. What you could not check.

Then stop and wait for my answer, which will look like "1-4 yes, 6 change X".
