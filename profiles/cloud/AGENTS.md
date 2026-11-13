## Environment (cloud session)
- Claude Code cloud container: Linux, bash, no GPU, no access to the home PC.
  Only pushed work is visible; say so when something I mention may be unpushed.
- The repo `willlovecode/workspace` indexes all projects in `projects.md`; read it
  first when I name a project. Use `gh` or `git clone` for other repos if allowed.
- Pushing to GitHub is expected here; there is no git guard in this profile.
- Python projects use uv; JavaScript projects use the lockfile's package manager.

## Model choice
Pick the cheapest setting that scores near the top for the kind of task. The
table is a reading of FrontierCode 1.1 (all reasoning levels) from 2026-11-07 (unchanged since 2026-10-18);
Artificial Analysis was last read on 2026-10-11 and not again.

| Task | Claude Code | Codex |
|---|---|---|
| Hardest design, debugging, review | Opus at medium (`review` agent for reviews) | `--profile deep` (GPT-6 Astra, max) |
| Everyday work in the main session | Sonnet at high (the default model; Opus at medium for hard tasks and review) | default (GPT-6.1 Sol, medium) |
| Implementation from a clear brief | `implement` agent (Sonnet, high) | default |
| Mechanical edits across files | `mechanic` agent (Haiku, medium) | `--profile quick` (GPT-6.1 Sol, low) |
| Finding where things live | `scout` agent (Haiku, medium) | `--profile quick` |

- Opus at medium is the strongest Claude setting per task (the top score of the
  board, 54.6% at $0.80 a task); nothing Claude above it is worth its cost.
  Sonnet at high is chosen for daily work to save subscription limit.
- Never use Opus at low, high, xhigh or max, Sonnet at low, medium, xhigh or max,
  Haiku at xhigh or max, GPT-6 Astra at low, medium or xhigh, or GPT-6.1 Sol at
  high, xhigh or max: each is beaten on score and cost by a neighbouring
  setting. Max is worth it only for GPT-6 Astra.
- A second opinion is worth more from the other vendor than from the same one.
- Before a large round of subagents, or when a new model has shipped, re-read
  the benchmark and update this table and its date. Say which reading a choice
  is based on.

