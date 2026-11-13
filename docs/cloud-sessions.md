# Claude Code cloud sessions

A cloud session (claude.ai/code, phone included) runs in a fresh Linux container. It sees
only the repo it was started on, not `~/.claude` of the home PC. The `cloud` profile
(`profiles/cloud`) gives it the same instructions, subagents and skills, minus what does not
fit: no git-guard (pushes must work from the phone), no Codex or Gemini files, no MCP servers,
npm packages or plugins.

## Setup script

Paste this into the setup script of the cloud environment (claude.ai/code, environment settings):

```bash
#!/bin/bash
set -e
curl -fsSL https://raw.githubusercontent.com/willlovecode/agent-config/master/get-agtk.sh | sh </dev/null
git clone --depth 1 https://github.com/willlovecode/agent-config.git "$HOME/.agtk/agent-config"
"$HOME/.agtk/bin/agtk" apply --profile "$HOME/.agtk/agent-config/profiles/cloud" --skip-tools
```

`agent-config` is public, so no login is needed. The script needs network access to
`github.com` and `raw.githubusercontent.com` in the environment's network settings.

## Tested

2026-11-13 on Ubuntu (WSL) with an empty `HOME` and the v0.1.1 Linux binary: unattended,
no wizard, writes `CLAUDE.md`, `agents/*`, six skills and a `settings.json` without hooks.
Not yet tried inside a real cloud container.

## Limits

- The instructions come from the agtk release the script installs. A change to `base/` or the
  profile reaches the cloud only after a new release is tagged (the profile files are read from
  the clone, but the base text is embedded in the binary).
- The delete guard (recursive deletes through variables or home/root paths) is part of
  git-guard and is not active in cloud sessions, which have no hooks. The rule in the
  instructions still applies.
- Keep the `Model choice` table in `profiles/cloud/AGENTS.md` equal to `profiles/home/AGENTS.md`;
  only the Environment section differs.
