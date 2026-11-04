# Working agreement

Personal, cross-project instructions for any coding agent, on any machine. The
sections after "Commits and PRs" are specific to this machine. Project specifics
(commands, architecture, conventions) belong in each project's own AGENTS.md.

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

## Secrets
- Never print, log, commit or paste into prompts any secret, token, password or
  connection string. Refer to it by name (env var, vault key) instead.

## Commits and PRs
- Short messages: what changed and why. No marketing tone.
