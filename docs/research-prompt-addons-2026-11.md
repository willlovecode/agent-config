# Research prompt: add-on baseline for Claude Code and Codex

Paste this into a fresh Claude Code session started in `C:\Users\walbe\Code\agent-config`.

---

Research the best add-on setup for my coding agents and give me a report to
approve item by item. Nothing gets installed into my real setup until I approve.

## Where things stand

- `agent-config` is the single source for Claude Code (`~/.claude`) and Codex
  (`~/.codex`). `install.ps1` applies it; `skills/baseline.json` lists every
  skill and plugin, pinned to a commit. Read `README.md` first.
- On 2026-11-04 I removed every skill, hook, plugin and MCP server I had added,
  in both tools. `skills/baseline.json` is empty on purpose. What was there
  before, and why it went: `git log -p skills/baseline.json`,
  `reports/skills-baseline-2026-11-04.html`, and the archives in
  `C:\Users\walbe\Code\_archive\skills-2026-11-04` and `addons-reset-2026-11-04`
  (each has a `MANIFEST.txt`). Flags from that round: duplicates between plugins
  and `npx skills` copies, retired upstream skills, over-triggering
  descriptions, and last30days rated CRITICAL by the skills.sh audits.
- Still installed and out of scope: Anthropic's account skills and built-in
  commands, and OpenAI's own Codex plugins (browser, computer-use, documents,
  Gmail, GitHub and others). Don't propose duplicates of them.
- Earlier research to build on rather than repeat: `docs/research.md`
  (2026-10-07: few, sharp skills; the listing budget; hooks over prose rules;
  registry malware), `C:\Users\walbe\Code\vega\docs\research\community-skills.md`
  and `community-scams-malware.md`, and `C:\Users\walbe\Code\sol\docs\research\self-extension.md`.
  Project-level skills in `vbgym/.claude/skills` and `vbgym/skills-lock.json`
  show what I chose for UI work there.

## Scope

All four kinds of add-ons, for both tools:
1. **Skills** (SKILL.md; shared in `~/.agents/skills`, linked into Claude Code).
2. **Plugins** and their marketplaces (Claude Code and Codex).
3. **Hooks**: at least a global PreToolUse guard that blocks `git push`,
   `git reset --hard` and `git clean`, and a per-project Stop hook that runs that
   project's checks. `docs/research.md` recommended both; neither was built.
   Codex equivalents where they exist.
4. **MCP servers** that earn their place over the built-in tools.

Start with what I need now and keep room to grow. Current work:
- Java / Spring Boot (practice project for a Werkstudent interview)
- Python with uv, and JS/TS web apps (for example vbgym)
- UI design (Apple-style and glass interfaces)
- Research and agent projects (papers and web research; Sol, Vega, infoStacker)

Also judge cross-cutting add-ons (code review, planning, TDD, git workflow,
Windows/PowerShell) for whether they help across all of these.

## How to judge

- **Trust:** prefer first-party (Anthropic, OpenAI, Vercel) and well-known,
  maintained authors. Don't drop something just because the author is unknown:
  GitHub trending, the skills.sh trending tab and new repos are good places to
  find those. Every recommendation is pinned to a commit, and you read every
  file in it (scripts too) before recommending it.
- **Evidence:** adoption is enough: install counts, stars, maintained activity,
  and practitioner consensus from the last 60 days. Still give each item its
  numbers and links, and note any security audit result (skills.sh audits from
  Gen, Socket and Snyk; GitHub issues).
- **Size:** no cap. Each item must earn its place and not overlap another one or
  a built-in. Say how many skill descriptions Claude Code would list in total.
- **Fit:** Windows 11, PowerShell 5.1, Claude Code plus Codex. Skills that work
  in both tools score higher. Say where an item only works in one.
- **Risk to flag:** network calls, install or "prerequisite" steps, credential or
  cookie access, `!command` shell injection, `allowed-tools` pre-approvals,
  descriptions that over-trigger ("You MUST use this…"), and unpinned
  auto-updates.

## Sources

skills.sh (leaderboard, trending, audits), `npx skills find` (read the
find-skills method, don't install it), anthropics/skills and
anthropics/claude-plugins-official, OpenAI's plugin repos, vercel-labs,
obra/superpowers, mattpocock/skills, GitHub trending and search, the Claude Code
and Codex docs for skills, plugins, hooks and MCP, and community discussion
(Reddit, Hacker News, X, blogs). Reddit and X may need a browser; use the
built-in browser if a plain fetch shows nothing, and say when you could not read
a source.

## Hands-on trials

Shortlist first, then read every file of each shortlisted item. Trial the top
candidates on a small throwaway task in your scratchpad (or in a scratch folder
under `C:\Users\walbe\Code\_scratch`), installed at project scope there. Never
touch `~/.claude`, `~/.codex`, `~/.agents` or my real projects. Record per trial
whether the skill triggered when it should, stayed quiet when it should, and
whether the result was better than without it.

## Second opinion

After the shortlist, have Codex review it from the other vendor's side:
`codex --profile deep exec "<the shortlist and your reasoning>"`, read-only, no
installs. Put where Codex agrees and disagrees in the report.

## Models and cost

Follow the model table in `AGENTS.md`. Run the main session on Opus at medium.
Wide searching and reading go to cheap subagents (`scout`, Haiku at medium),
with no Opus fan-outs; earlier fan-outs hit my session limit. Say which reading
of the table each choice is based on.

## Deliverable

One HTML report at `agent-config/reports/addons-research-<date>.html`, in the
same style as `reports/skills-baseline-2026-11-04.html` (light and dark, opens
in my browser). Contents:
1. Short summary: what you recommend and the total listing count.
2. Numbered recommendations, grouped by skills, plugins, hooks and MCP. Each one
   gives:
   - action (add / skip / later) and recommend (yes / consider);
   - the source repo and pinned commit;
   - which tools, and which of my work areas, it serves;
   - adoption numbers and the audit result;
   - what you read and what the trial showed;
   - risks;
   - overlap with other items;
   - links.
3. Candidates you rejected, one line each with the reason.
4. Codex's second opinion.
5. What you could not check.

Then stop and wait for my answer. I'll reply like "1-4 yes, 6 later, 9 no".
After that, and only for the approved items, add them to
`skills/baseline.json` (pinned), add the hooks to agent-config, run
`install.ps1`, verify both tools load them, and commit locally. Never push.
