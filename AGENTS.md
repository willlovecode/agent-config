# Working agreement

Personal, cross-project instructions for any coding agent on this PC. Project
specifics (commands, architecture, conventions) belong in each project's own
AGENTS.md.

## Environment
- Windows 11 desktop: Ryzen 5 3600, 16 GB RAM, RTX 3050 8 GB. Shell is PowerShell.
- Projects live in `C:\Users\walbe\Code`, one folder per project.
- Python projects use uv; JavaScript projects use the lockfile's package manager.

## Communication
- Reply in English. Lead with the result, then anything I need to decide.
- If a request is ambiguous and a wrong guess would be costly, ask one question.
  Otherwise make a reasonable call and say which call you made.

## Scope and autonomy
- Do what was asked. Mention unrelated improvements instead of making them.
- Small, clear changes: just do them. For large, risky or hard-to-undo changes,
  outline the plan first and wait for my OK.
- Ask before pushing, publishing, deleting data or spending money.

## Code
- Match the surrounding code: naming, structure, comment density.
- Comment only what the code itself can't tell the reader.
- Add tests for new logic where the project has a test setup.
- Before calling work done, run the project's checks (lint, types, tests) and
  report the outcome honestly, including failures.

## Model choice
Pick the cheapest setting that scores near the top for the kind of task. The
table is a reading of FrontierCode 1.1 (all reasoning levels) from 2026-10-28 (unchanged since 2026-10-18);
Artificial Analysis was last read on 2026-10-11 and not again.

| Task | Claude Code | Codex |
|---|---|---|
| Hardest design, debugging, review | Opus at medium (`review` agent for reviews) | `--profile deep` (GPT-6 Astra, max) |
| Everyday work in the main session | Opus at medium | default (GPT-6.1 Sol, medium) |
| Implementation from a clear brief | `implement` agent (Sonnet, high) | default |
| Mechanical edits across files | `mechanic` agent (Haiku, medium) | `--profile quick` (GPT-6.1 Sol, low) |
| Finding where things live | `scout` agent (Haiku, medium) | `--profile quick` |

- Opus at medium is the top score of the board (54.6% at $0.80 a task), so
  nothing Claude above it is worth its cost.
- Never use Opus at low, high or xhigh, Sonnet at low, medium, xhigh or max,
  Haiku at xhigh or max, or GPT-6.1 Sol above medium: each is beaten on score
  and cost by a neighbouring setting. Max is worth it only for GPT-6 Astra.
- A second opinion is worth more from the other vendor than from the same one.
- Before a large round of subagents, or when a new model has shipped, re-read
  the benchmark and update this table and its date. Say which reading a choice
  is based on.

## Commits and PRs
- Short messages: what changed and why. No marketing tone.
