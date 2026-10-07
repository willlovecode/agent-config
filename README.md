# agent-config

Single source of truth for how coding agents behave on this PC.

| File | Goes to |
| --- | --- |
| `AGENTS.md` | `%USERPROFILE%\.codex\AGENTS.md` (Codex); Claude Code imports it |
| `claude/CLAUDE.md` | `%USERPROFILE%\.claude\CLAUDE.md`, one line: `@C:/Users/walbe/.codex/AGENTS.md` |
| `claude/agents/` | `%USERPROFILE%\.claude\agents\` (Claude Code subagents: scout on Haiku, review on Opus) |
| `skills/` | `%USERPROFILE%\.agents\skills\` (Codex), junction-linked into `.claude\skills` |

Edit here, then run:

```powershell
powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\Code\agent-config\install.ps1"
```

Rules of thumb (from the 2026-10-07 research, see `docs/research.md`):
- `AGENTS.md` holds only cross-project, personal rules. Keep it short and hand-written.
- Project commands and conventions go in each project's own `AGENTS.md`.
- Anything that must always happen becomes a hook or setting, not a sentence.
- Add a skill only after reading every file in it; pin third-party skills to a commit.
