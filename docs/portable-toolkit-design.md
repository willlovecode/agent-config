# Portable agent toolkit: design

Status: approved and built 2026-11-04 (first iteration). Where the build deviates
from the design below:

- Packs are installed as plain files by `install.ps1`, not as Claude Code / Codex
  plugins. One mechanism works in both tiers, and Codex plugins can't carry hooks.
  Marketplace manifests can be added later if needed.
- Windows only: no `install.sh` yet (home and work are both Windows).
- The `java` pack is not built: its candidates still need a read and a trial, and
  the work stack's versions are unknown. Work installs `core` only for now.
- Approved from the add-on report: core hooks, Codex rules, Stop runner, tdd,
  diagnosing-bugs, grilling/grill-me, and (home only) Context7, pyright-lsp,
  paper-lookup in a `research` pack, and the Spring Boot skill installed into
  energy-billing only (`projectSkills` in the home profile). Edits to vendored
  skills are listed in `packs/MODIFICATIONS.md`.

## Goal

One private GitHub repo that makes Claude Code and Codex behave the same way on any
machine: home (Windows, personal accounts) and work (Windows, a corporate LLM gateway,
a partly legacy Java energy-trading project migrating to the cloud). It should cover a
lot of ground without making sessions slow or filling context before the task starts.

## Principle: pay only for what the task uses

| Layer | Contents | When it costs context | Budget (checked in CI) |
|---|---|---|---|
| Base rules | `AGENTS.md`: communication, autonomy, verification, git and secrets rules | Every session | ≤ 1,200 tokens |
| Enforcement | Hooks (git guard, Stop checks), Codex execpolicy rules, permission denies | Never: runs as code | 0 |
| Procedures | Skills, in packs | Description only; body on use | ≤ 15 listed skills, ≤ 1,500 tokens of descriptions; everything else explicit-only (0) |
| Roles | Subagents (scout, implement, review, mechanic); Codex profiles | Description only | ≤ 400 tokens |
| Tools | MCP servers (Context7 at home) | Deferred until used | ~0 idle |
| Reference | Longer guides (model table, Windows notes, migration playbooks) | Only when a skill or the agent opens them | 0 |

Today's always-loaded cost is about 700 tokens (AGENTS.md) plus about 250 (subagent
descriptions), so the budget allows growth. The model-choice table (half of today's
AGENTS.md) moves out of the base into the home profile, because a corporate gateway has
different models.

## Repo layout

```
agent-config/
  base/AGENTS.md              shared core rules, tool-neutral, no personal data
  profiles/
    home/                     model table, machine notes, personal packs on, Context7 on
    work/                     gateway model mapping, stricter defaults, personal packs off
  packs/                      each pack is one plugin for both tools
    core/                     workflow skills (tdd, diagnosing-bugs, grilling), subagents
    java/                     jdtls LSP config, legacy-Java and Spring skills, migration playbook
    python/  web/  ui/        language and UI skills (home use mostly)
    research/                 paper-lookup etc. (home only)
  enforcement/
    hooks/                    git-guard.mjs, stop-checks.mjs, tests
    codex-rules/              execpolicy .rules
  bootstrap/
    install.ps1  install.sh   profile + tier aware installer (see below)
  checks/
    budget.mjs                measures always-loaded tokens; fails CI over budget
  .claude-plugin/marketplace.json        Claude Code marketplace (local path or GitHub)
  .agents/plugins/marketplace.json       Codex marketplace
  .github/workflows/ci.yml               budget check, hook tests, skill validation
```

Third-party skills are vendored (copied into the repo at a pinned commit, with a
`SOURCE` file), so installing never fetches live upstream code. That matters on a
company machine and removes the npx/marketplace network dependency.

## Profiles

`install.ps1 -Profile home|work` composes base + profile + chosen packs.

| | home | work |
|---|---|---|
| Base rules | yes | yes |
| Personal context (memory, Sol/Vega/infoStacker notes, machine specs) | yes | **never**, not even in the repo's work paths |
| Model table | FrontierCode-based, current one | maps roles to whatever the gateway serves (aliases `opus`/`sonnet`/`haiku` in Claude Code, provider + model in Codex `config.toml`); filled in on day one |
| Packs | core, java, python, web, ui, research | core, java |
| MCP | Context7 | none by default (data leaves the company); enable only if allowed |
| Auto-update task | yes | no (update by `git pull` + rerun, deliberately) |
| Extra rules | | no customer or trading data in prompts to external services; never print secrets or connection strings; read-only by default on infra/cloud tooling |

## Tiers: what to do when the work machine is locked down

You don't know yet what the company allows, so the installer detects and degrades:

| Tier | Situation | What gets installed | Enforcement left |
|---|---|---|---|
| A, open | Can clone the repo, install plugins, run hooks | Everything, via local-path marketplaces (no network fetch needed after clone) | Hooks + execpolicy + permissions |
| B, no plugins/hooks | Managed settings block plugins or hooks | Plain files only: user-level `AGENTS.md`, skill folders in `~/.agents/skills` (+ junctions into `~/.claude/skills`), `permissions.deny` for `git push`/`reset --hard`/`clean` in `~/.claude/settings.json` if allowed, Codex execpolicy rules | Permissions and rules only; the git guard becomes a sentence in AGENTS.md |
| C, no personal repo | Can't clone personal GitHub on the work machine | A release zip built by CI (`agent-kit-work.zip`, work profile only, no personal data), copied over a permitted channel; or, minimally, paste `AGENTS.md` | As B |

The installer reports which tier it reached and what it skipped, so you know what
protection you actually have. Important: use only what your employer permits. If
personal tooling on work machines needs approval, the work zip is small and readable
enough to show IT.

## Corporate gateway notes

- Claude Code and Codex both run against a gateway (`ANTHROPIC_BASE_URL` /
  apps-gateway sign-in; Codex `model_providers` in `config.toml`). The toolkit never
  hardcodes model IDs in the base or packs; subagents use aliases, and the work profile
  maps them.
- Plugins that call the model API themselves (e.g. security-guidance) would send code to
  the gateway; none are planned for work.
- Codex user hooks need `/hooks` trust once per machine and after each update; the
  installer prints that as a final step, because untrusted hooks are skipped silently
  in `codex exec`. The execpolicy rules work without trust.

## The work pack: legacy Java to cloud, energy trading

Candidates, each to be read and trialled before it goes in (same rule as the add-on
research):

- Characterization-tests-first workflow: before changing legacy code, pin current
  behaviour with tests (tdd skill + a short playbook).
- jdtls LSP for navigation in a large codebase (needs a JDK and a jdtls wrapper; fits
  Tier A only).
- Anthropic's `code-modernization` plugin (official marketplace, legacy assessment and
  migration map); not yet read.
- A migration playbook as reference (strangler-fig steps, config externalisation,
  12-factor checks, containerisation), loaded on demand.
- Domain glossary support (Matt Pocock's domain-modeling) so trading terms stay
  consistent.
- Not the SivaLabs Spring Boot skill as is: it targets Boot 4 greenfield, while a legacy
  project likely runs Java 8/11/17 and Boot 2/3. Revisit once you know the stack.

## Checks that keep it honest

- `checks/budget.mjs`: counts the always-loaded parts per profile (each tool's instruction
  file built from base and profile overlay, CLAUDE.md with the routing table when the profile
  routes models; listed skill descriptions; subagent descriptions) and fails CI above budget.
- Hook tests (the 45 guard cases, the Stop-runner states) run in CI on Windows.
- Skill validation: frontmatter, `name` = folder, no `allowed-tools` pre-approvals in
  vendored skills, pinned `SOURCE` present.
- A trigger-test script (the one used today) to rerun after adding a skill.

## Migration from today's agent-config

1. Restructure the repo into the layout above; keep `install.ps1` working for home
   throughout.
2. Apply the approved items from the add-on report as the first contents of `core`
   and packs.
3. Write the work profile and Tier B/C fallbacks; build the work zip in CI.
4. Push to a private GitHub repo (only on your OK).
5. Day one at work: run the installer with `-Profile work`, note the tier, fill in the
   gateway model mapping, trust Codex hooks.

## Open questions (defaults if unanswered)

- Repo visibility: private (default).
- Should the work profile live in the same repo? Default yes, as long as it contains no
  personal data; the alternative is a second, work-only repo.
- Work Java/Spring versions: unknown until day one; the java pack stays generic until then.
