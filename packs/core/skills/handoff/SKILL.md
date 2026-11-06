---
name: handoff
description: Write .agent-handoff/latest.md so another agent can continue this work. Use when a usage limit is near or hit, or the user wants to switch to Claude Code, Codex or Gemini.
---

Agents cannot share live context, so pass the state through a file in the repo root.

1. Run `git status` and `git diff --stat`. Keep the output summary for the file.
2. Make sure `.agent-handoff/` is ignored without touching the tracked `.gitignore`: find the exclude file with `git rev-parse --git-path info/exclude` and append the line `.agent-handoff/` if it is missing. Never commit the folder. Outside a git repo, skip this step and the git commands.
3. Write `.agent-handoff/latest.md` (overwrite the old one) with these sections, short and concrete:
   - **Goal**: what the user wants, in the user's terms.
   - **Done**: what is finished and verified.
   - **Left**: what remains, in order.
   - **Files touched**: paths with a few words each, plus the `git status` and `git diff --stat` summary.
   - **Decisions and gotchas**: choices made and why, dead ends, traps the next agent would step in.
   - **How to run checks**: the exact commands and their last result.
   - **Next step**: the one thing to do first.
4. Never put secrets in it: no tokens, passwords or connection strings. Name the env var or vault key instead.
5. Tell the user to paste this line into the next agent:

   > Read .agent-handoff/latest.md, then run git status and git diff, then continue with Next step.
