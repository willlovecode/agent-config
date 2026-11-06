# Shrink the base file, enforce the rest

> Note 2026-11-05: this report's advice to have CLAUDE.md import `~/.codex/AGENTS.md` is no longer what the toolkit does. Both `agtk apply` and the `install.ps1` fallback now write a standalone `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` from the same text; neither imports the other, and only `agtk` adds the routing table to CLAUDE.md.

As of October 2026, official docs, independent studies and practitioners agree on the setup. Keep one short, hand-written base file. It should hold only personal, cross-project rules the agent cannot work out for itself: language, how to communicate, how much autonomy to take, and how to report work. Store it once as AGENTS.md and import it into Claude Code. Anything that must happen every time belongs in hooks, permissions or plan/approval modes, not prose. Skills are for task-specific procedures you can name, and you should install few of them, from sources you have audited and pinned. The evidence behind this is fairly recent. A February 2026 ETH Zurich study found that LLM-generated context files slightly *lowered* task success and raised cost by 20–23%. Human-written files gained only about 4% and still raised cost ([arXiv 2602.11988](https://arxiv.org/html/2602.11988v1)). Anthropic and OpenAI now both warn that their models follow instructions so literally that emphatic or blanket rules cause over-triggering. Skill registries have also turned out to carry real malware: Snyk found 76 confirmed malicious payloads among 3,984 public skills ([Snyk](https://snyk.io/blog/toxicskills-malicious-ai-agent-skills-clawhub)). Your current AGENTS.md is already small, at about 1 KB. Its problems are elsewhere. Several rules are always-on gates or project-specific checks that belong in hooks or project files. Your 15 custom skills overlap heavily, so fewer of them fire reliably. They sit in a legacy folder. And at least one, last30days, carries a security and context cost that calls for pinning and explicit-only use. A reset on Windows comes down to four things: a canonical `%USERPROFILE%\.codex\AGENTS.md`, a `%USERPROFILE%\.claude\CLAUDE.md` that imports it, a canonical `%USERPROFILE%\.agents\skills` folder linked into Claude Code with directory junctions, and permission/hook configuration kept separate for each tool.

A caveat on sources: Reddit and Hacker News could not be fetched directly. Community sentiment below therefore comes from blogs, press write-ups and issue mirrors, not from the original threads. No published study looks at *global* (user-level) files specifically; all the empirical work measures repository-level files.

## Every line in a global file runs on every task

The two tools load user-level instructions differently, and that shapes what belongs in the file.

**Claude Code** loads `%USERPROFILE%\.claude\CLAUDE.md` plus `~/.claude/rules/*.md` at the start of every session, alongside the project files. **Neither layer overrides the other.** When a user rule and a project rule conflict, "Claude may follow either one" ([Claude Code memory docs](https://code.claude.com/docs/en/memory)). **Codex** loads `~/.codex/AGENTS.md` (or `AGENTS.override.md`) first, then concatenates the project AGENTS.md files from the git root down to the working directory. Closer files override earlier ones. All of them together are capped by `project_doc_max_bytes`, **32 KiB by default** ([Codex AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)). Two consequences follow. A bloated global file in Codex eats into the budget for project files. A global "always do X" in Claude Code cannot be cleanly switched off per project.

The size guidance converges. Anthropic says to "target under 200 lines per CLAUDE.md file. Longer files consume more context and reduce adherence." Its test for each line is: "Would removing this cause Claude to make mistakes? If not, cut it" ([memory docs](https://code.claude.com/docs/en/memory); [best practices](https://code.claude.com/docs/en/best-practices)). HumanLayer puts the ceiling at under 300 lines and keeps its own file under 60. Its reasoning is that frontier models reliably follow roughly 150–200 instructions, and the harness system prompt already uses about 50 of them ([HumanLayer](https://www.humanlayer.dev/blog/writing-a-good-claude-md)). That argument extrapolates from the IFScale benchmark, where even the best models reached only **68% accuracy at 500 simultaneous instructions** and earlier instructions were favoured ([arXiv 2507.11538](https://arxiv.org/abs/2507.11538)). IFScale used synthetic keyword tasks, so applying it to CLAUDE.md is a practitioner inference, not a measurement. For a *global* file the right size is far below these ceilings, because every line applies to every repository. Something like 15–40 lines is enough.

The empirical record explains why less is more. In the ETH study, **agents used tools mentioned in a context file 1.6–2.5× more often**, tested more, explored more, and spent 14–22% more reasoning tokens ([arXiv 2602.11988](https://arxiv.org/html/2602.11988v1)). Instructions work, which means a blanket instruction is also paid for on tasks where it does not help. A separate paired study ran Codex on 124 PRs and found that AGENTS.md cut median wall-clock time by **28.6%** and output tokens by about 17–20%. Quality was checked only by manual sampling, not measured as correctness ([arXiv 2601.20404](https://arxiv.org/html/2601.20404v2)). Read together, the two studies say that short, human-written operational facts help, while generated overviews and blanket procedures mostly add cost. This is why HumanLayer and the ETH authors advise against `/init`-style generated files, even though Anthropic's docs still suggest `/init` as a starting point ([HumanLayer](https://www.humanlayer.dev/blog/writing-a-good-claude-md); [best practices](https://code.claude.com/docs/en/best-practices)). For a personal global file, write it by hand.

The content that consistently earns its place is about *you*, not the project ([Ray Thanni](https://raythanni.substack.com/p/claude-code-memory-files-global-vs)):

- your language and how you want answers delivered;
- how much autonomy to take;
- non-default style preferences;
- personal tooling;
- cross-project working agreements, such as OpenAI's own example "Ask for confirmation before adding new production dependencies" ([Codex AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)).

The content to avoid:

- anything the agent can infer from the code;
- standard language conventions;
- tutorials and documentation that changes often;
- file-by-file descriptions;
- platitudes like "write clean code" ([best practices](https://code.claude.com/docs/en/best-practices)).

**Tone now matters as much as content.** Anthropic warns that recent Claude models "may now overtrigger. The fix is to dial back any aggressive language," so "CRITICAL: You MUST" becomes "Use X when Y" ([Claude prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices)). OpenAI says the same about GPT-5. Shouting "Be THOROUGH" makes the model overcompensate with extra tool calls, and conflicting instructions degrade it ([OpenAI cheatsheet](https://cdn.openai.com/API/docs/gpt-5-for-coding-cheatsheet.pdf); [GPT-5.1 guide](https://developers.openai.com/cookbook/examples/gpt-5/gpt-5-1_prompting_guide)). Use emphasis on one line at most: "If you emphasize many lines, none of them stands out" ([best practices](https://code.claude.com/docs/en/best-practices)).

Files also age badly. Claude Code's `/doctor prompt-audit` looks specifically for "instructions written for older models" ([memory docs](https://code.claude.com/docs/en/memory)). Secondary reports quote Boris Cherny, creator of Claude Code, advising users to delete and rebuild CLAUDE.md every six months. That quote could not be verified against the primary interview ([ainauten](https://news.ainauten.com/en/story/claude-code-erfinder-boris-cherny-raet-loesch-deine-claudemd)). The 2026 community trend runs in the same direction, under titles like "Delete your CLAUDE.md" and "CLAUDE.md context diet" ([Charlie Hills](https://charliehills.substack.com/p/delete-your-claudemd); [wotai.co](https://wotai.co/blog/claude-md-context-diet)). It is backed by the MSR '26 finding that AGENTS.md edits are mostly additions and half of the files are never touched again after creation ([arXiv 2510.21413](https://arxiv.org/html/2510.21413v4)).

## Hooks and modes guarantee what prose only suggests

Anthropic is unambiguous here. CLAUDE.md is "context, not enforced configuration. To block an action regardless of what Claude decides, use a PreToolUse hook instead," and hooks are for "actions that must happen every time with zero exceptions" ([memory docs](https://code.claude.com/docs/en/memory); [best practices](https://code.claude.com/docs/en/best-practices)). For a "verify before done" rule, Anthropic ranks the options from weakest to strongest: ask in the prompt, set a `/goal` condition, add a **Stop hook as a deterministic gate**, or use a verification subagent ([best practices](https://code.claude.com/docs/en/best-practices)). The "3-layer harness" pattern popular in the community follows the same logic: instruction file, then skills/docs, then hooks ([dev.to](https://dev.to/akaranjkar08/stop-dumping-instructions-into-claudemd-the-3-layer-agent-harness-pattern-50o5)). So does a small cottage industry of Stop-hook packs that keep agents from "faking done" ([Product Hunt](https://www.producthunt.com/p/no-excuses/no-excuses)). HumanLayer's version is "Don't use Claude as a linter" ([HumanLayer](https://www.humanlayer.dev/blog/writing-a-good-claude-md)).

Approval gates are where prose rules cause the most friction. Anthropic says plan mode "adds overhead… If you could describe the diff in one sentence, skip the plan." It recommends planning for uncertain, multi-file or unfamiliar work ([best practices](https://code.claude.com/docs/en/best-practices)). Its own conservative-autonomy snippet applies only "when the user's intent is ambiguous", not to every task ([Claude prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices)). One documented case shows the clash. A user's CLAUDE.md rule, "Present plan/proposal first, then wait for 'OK'", collided with Claude Code's built-in EnterPlanMode tool. The issue was closed as "not planned" ([issue #25720 mirror](https://claudeissues.com/issue/25720-claude-code-uses-enterplanmode-tool-despite-explicit-instructions-to-present-pla)). For Claude Code the cleaner route is the harness's own plan and permission modes. If you approve every action manually, Anthropic notes that "after the tenth approval you're clicking through rather than reviewing," and recommends allowlists, sandboxing or auto mode instead ([best practices](https://code.claude.com/docs/en/best-practices)).

The vendors are not fully aligned. OpenAI's Codex docs list linting standards and approval workflows for dependency changes as reasonable *global* AGENTS.md content ([Codex AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)). Anthropic pushes the same concerns into hooks. The practical way to reconcile them: keep a one-line *intent* in the shared base ("verify with the project's linter and type checker before reporting done"). Put the exact commands in each project's AGENTS.md. Add a Stop hook where you need a guarantee in Claude Code. Rely on Codex's approval and sandbox settings for hard limits, since no Codex equivalent of hooks turned up in the research.

Several Claude Code settings replace things people write into CLAUDE.md:

| Need | Mechanism |
|---|---|
| Block or allow actions | `permissions.deny`, sandbox settings |
| Commit and PR conventions | `includeGitInstructions: false` plus `attribution`, so your own rules don't compete with built-in git instructions |
| Tone and format | `outputStyle`, instead of tone prose |

Sources: [memory docs](https://code.claude.com/docs/en/memory); [settings reference](https://code.claude.com/docs/en/settings-reference).

Permission files are a separate track. One Windows/macOS power user found `~/.claude/settings.json` and `~/.codex/rules/default.rules` "completely incompatible" and generates both from a single YAML source ([zenn.dev](https://zenn.dev/waki285/articles/chezmoi-dotfiles?locale=en)).

## Fewer, sharper skills route better than a large library

Skills load in stages. Only the name and description, about 100 tokens each, sit in context all the time. The body loads when the skill is triggered, and reference files load only when needed ([Agent Skills overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)). The always-loaded listing has a hard budget in both tools:

- **Claude Code:** about 1% of the context window, with each description plus `when_to_use` cut off at 1,536 characters. When the budget overflows, the least-invoked skills drop out first ([rulestack](https://dev.to/rulestack/too-many-claude-code-skills-how-the-listing-budget-decides-which-descriptions-claude-sees-4a6m); [skills docs](https://code.claude.com/docs/en/skills)).
- **Codex:** at most 2% of the context window, or 8,000 characters when the window size is unknown. Codex "shortens descriptions before omitting skills entirely" ([Codex Build skills](https://learn.chatgpt.com/docs/build-skills)).

Truncation is silent and breaks routing, as one issue report documents ([issue #64606 mirror](https://claudeissues.com/issue/64606-skill-description-budget-silently-truncates-routing-information-causing-skill-ro)). So the practical limit depends on total description length and how much skills overlap, not on a fixed count. A rough estimate: about 15–25 typical descriptions fit Claude Code's default budget on a 200K-token window before truncation begins. That is my own arithmetic, not a vendor figure. Vendors do not publish an ideal number.

A second problem matters as much as the budget: **behaviour-shaping skills often never fire on their own.** In JetBrains' controlled test of ponytail, 80 paired tasks on Sonnet 5, the skill cut code by 15.4% and cost by 10.3% with no quality loss. But its **self-activation rate was zero** unless it was injected deliberately ([JetBrains](https://blog.jetbrains.com/ai/2026/07/ponytail-skill-claude-tested/)). Caveman claims a 65–75% cut in output tokens. Independent tests measured 12–23%, which works out to only 1–3% of the bill, and most of the effect comes back with a plain "be concise" instruction ([implicator.ai](https://www.implicator.ai/caveman-claude-code-skill-cuts-output-20-your-bill-barely-notices-2/)). A widely shared May 2026 audit of Claude Code session logs found that about half of the installed skills never activated ([AI Weekly](https://aiweekly.co/alerts/claude-code-skills-waste-23k-tokens-per-session)). Its "23K tokens per session" figure assumes full skill bodies load at startup, which contradicts the documented design, so treat that number as anecdotal.

The lesson: always-on style and minimalism rules belong in the base file. Skills should be *procedures* you can name, such as a migration, a refactor protocol or a document workflow.

Choosing skills follows the same logic. Anthropic's document skills and skill-creator are the low-controversy baseline, although no independent evaluation of them was found. Methodology packs like Superpowers get the verdict "the methodology survives; the monolithic installation doesn't." Experienced solo developers are advised to cherry-pick, because native plan mode and subagents now duplicate much of the pack ([mcp.directory](https://mcp.directory/blog/superpowers-skill-worth-it-2026)).

When you write or keep a skill, follow the open spec and Anthropic's authoring guide:

- `name` and `description` only, for portability;
- a description that states *what* the skill does and *when* to use it, key use case first, in third person, under 1,024 characters;
- a body under 500 lines (about 5k tokens);
- reference files one level deep;
- scripts for fragile, deterministic steps;
- at least three evaluations ([agentskills.io spec](https://agentskills.io/specification); [Anthropic best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)).

OpenAI adds "keep each skill focused on one job" and "test prompts against the skill description" ([Codex Build skills](https://learn.chatgpt.com/docs/build-skills)). Adding negative boundaries ("not for X") and merging overlapping skills helps routing more than raising the budget does ([rulestack](https://dev.to/rulestack/too-many-claude-code-skills-how-the-listing-budget-decides-which-descriptions-claude-sees-4a6m)).

Both tools let you keep a skill installed while removing it from automatic routing:

| Tool | Mechanism | Effect |
|---|---|---|
| Claude Code | `disable-model-invocation: true` | Removes the description from the listing; manual invocation only |
| Claude Code | `skillOverrides` with `"name-only"` / `"off"` | Shrinks or hides the listing entry |
| Codex | `allow_implicit_invocation: false` in `agents/openai.yaml` | Skill runs only when invoked explicitly |
| Codex | `[[skills.config]] path=… enabled=false` in `config.toml` | Disables the skill without deleting it |

Sources: [skills docs](https://code.claude.com/docs/en/skills); [Codex Build skills](https://learn.chatgpt.com/docs/build-skills). The settings reference renders the `skillOverrides` values differently from the skills page; the skills page is treated as authoritative here.

## Treat every third-party skill as installed software

Anthropic's guidance is the most explicit. Use skills "only from trusted sources: those you created yourself or obtained from Anthropic," audit every bundled file, and "treat like installing software." A malicious skill can direct tool use or code execution that has nothing to do with its stated purpose ([Agent Skills overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)). Claude Code adds that `allowed-tools` lets a skill pre-approve tools for itself, and that `disableSkillShellExecution` turns off the `!command` injection that runs before content reaches the model ([skills docs](https://code.claude.com/docs/en/skills)). OpenAI's skills page offers configuration controls but no comparable trust warning.

The threat is measured, not hypothetical:

- **Snyk ToxicSkills:** of 3,984 skills from ClawHub and skills.sh, 13.4% had critical issues, 36.8% had at least one flaw, and 76 carried confirmed malicious payloads ([Snyk](https://snyk.io/blog/toxicskills-malicious-ai-agent-skills-clawhub)).
- **ClawHavoc:** the campaign grew from 341 to 824 malicious skills by June 2026. It used fake "Prerequisites" sections that led users to password-protected malware archives, plus typosquats ([Daniel Vaughan](https://codex.danielvaughan.com/2026/06/07/agent-skill-supply-chain-crisis-clawhavoc-toxicskills-skillsieve-codex-cli-defence/)).
- **OWASP's incubating Agentic Skills Top 10** names the classes of risk: malicious skills, supply-chain compromise, over-privileged skills, weak isolation, update drift from unpinned installs, and loss of security metadata when skills are ported between platforms ([OWASP](https://owasp.github.io/www-project-agentic-skills-top-10/top10.html)).

Vaughan's Codex hardening checklist is the most concrete one available ([Vaughan](https://codex.danielvaughan.com/2026/06/07/agent-skill-supply-chain-crisis-clawhavoc-toxicskills-skillsieve-codex-cli-defence/)):

1. Read SKILL.md and its scripts before installing.
2. Scan with `mcp-scan`.
3. Pin to commit hashes.
4. Enable skill and sandbox approval.
5. Deny-read `.env`, `.ssh` and `.aws`.
6. Restrict network access.
7. Prefer curated marketplaces over open registries.

Install popular skills from their original repository, not from look-alike forks. last30days, for example, exists in several copies on skills.sh ([skills.sh](https://www.skills.sh/openclaw/skills/last30days/security/socket)).

## One source of truth on Windows: AGENTS.md, imports and junctions

> Note 2026-11-05: the import (and the "Claude overlay" row in the table below) describes the October 2026 recommendation; the toolkit now writes standalone files, see the note at the top.

Neither tool reads the other's user-level files. Claude Code has read *project* AGENTS.md natively since v2.1.277, but it does not read `~/.codex/AGENTS.md`, `AGENTS.override.md`, or anything under `.agents/`. Codex can read CLAUDE.md only as a project fallback filename, and it does not expand `@` imports ([Claude Code memory docs](https://code.claude.com/docs/en/memory); [Codex AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)). So the robust direction is **AGENTS.md as master**, with Claude importing it. Anthropic documents this pattern: put `@AGENTS.md` in CLAUDE.md, then add Claude-only lines below. It also tells Windows users explicitly to prefer the import over a symlink, because symlinks need Administrator rights or Developer Mode and Git checks them out as plain text unless `core.symlinks` is enabled ([memory docs](https://code.claude.com/docs/en/memory)). User-scope files load their imports without the approval prompt, and the docs show the `@~/...` home-directory form. Two caveats apply. Cowork desktop sessions skip user-scope imports that resolve outside the working directory. And how `~` expands on native Windows is not spelled out, so an absolute path with forward slashes is the safer choice. Import paths with spaces need backslash-escaped spaces.

Skills follow the same idea. Codex reads user skills from `$HOME/.agents/skills` and follows symlinked folders ([Codex Build skills](https://learn.chatgpt.com/docs/build-skills)). Tooling has already moved off the old `~/.codex/skills` location ([oo-cli PR #254](https://github.com/oomol-lab/oo-cli/pull/254)). Claude Code reads `~/.claude/skills/<name>`, where each entry "can be a symlink to a directory elsewhere on disk," and it loads a skill only once even if several entries point at the same target ([skills docs](https://code.claude.com/docs/en/skills)). The standard pattern, used by the vercel-labs `skills` CLI and by community guides, is one canonical folder linked into each agent's folder ([skills CLI docs](https://mintlify.wiki/vercel-labs/skills/guides/installation-methods); [Kanaries](https://docs.kanaries.net/zh/articles/codex-vs-claude-code-skills)). On Windows, **directory junctions** (`mklink /J`) avoid the symlink privilege problem; Superpowers' Codex install uses them because "junctions normally work without special permissions" ([superpowers README.codex](https://gitea.com/TristanSun/superpowers/src/branch/main/docs/README.codex.md)). Claude Code's docs say "symlink", not junction, so check that skills load after setup. If you put your own source repo under version control, a chezmoi-style template keeps the shared base, the per-tool overlays, and a PowerShell copy step in one place ([zenn.dev](https://zenn.dev/waki285/articles/chezmoi-dotfiles?locale=en)).

| Item | Windows location | Role |
|---|---|---|
| Shared base instructions | `%USERPROFILE%\.codex\AGENTS.md` | Canonical, tool-neutral; Codex reads it directly (path inferred from `~/.codex`) |
| Claude overlay | `%USERPROFILE%\.claude\CLAUDE.md` | First line imports the base by absolute path, then Claude-only lines |
| Codex-only extras | Same AGENTS.md (or generate it from a template) | `AGENTS.override.md` *replaces* the global file rather than appending to it |
| Canonical skills | `%USERPROFILE%\.agents\skills\<name>` | Codex user scope; `name` + `description` frontmatter only; Codex extras in `agents/openai.yaml` |
| Claude skills | `%USERPROFILE%\.claude\skills\<name>` | Junction per skill pointing to the canonical folder |
| Hard rules | `~/.claude/settings.json` hooks/permissions; Codex `config.toml` approval/sandbox | Per tool; formats are incompatible |
| Verification | `/memory`, `/context`, `/doctor prompt-audit`; `codex "Summarize the current instructions."` | Confirms what actually loaded |

Write the shared base without tool-specific terms (no `/compact`, `$skill` or "Task tool"), so both models read it the same way ([cross-tool guidance](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)).

Two Windows notes have no source behind them and are general precautions. First, keep the canonical folders out of OneDrive's Files On-Demand, because junction targets there may be cloud-only placeholders. Second, if the folders live in Git, add `.gitattributes` with `*.md text eol=lf`. There is also an open Windows regression report: in April 2026, skills in `~/.agents/skills` were not discovered after a Codex update ([OpenAI community](https://community.openai.com/t/local-skills-in-agents-skills-are-no-longer-discovered-in-new-codex-sessions/1379522)). Check that skills appear after each Codex upgrade.

## Your setup: lean file, overloaded skill shelf

### The AGENTS.md

At about 1 KB, the file is already well inside every budget, so the reset is not about cutting length. It is about moving lines to where they work.

| Current rule | Verdict | Reason and action |
|---|---|---|
| Numbered plan, wait for "go" on non-trivial tasks | Keep, but narrow | Define "non-trivial" (multi-file, ambiguous, destructive). In Claude Code, use plan/permission modes instead, since prose approval rules collide with EnterPlanMode ([#25720](https://claudeissues.com/issue/25720-claude-code-uses-enterplanmode-tool-despite-explicit-instructions-to-present-pla)) and Anthropic says to skip planning for one-sentence diffs |
| Do only what's needed, no research tangents | Keep | Matches Anthropic's anti-overengineering guidance and replaces lean-build/ponytail-style skills ([prompting guide](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices)) |
| Ask instead of guessing | Keep, scoped | Phrase it as "when intent is ambiguous", as in Anthropic's own snippet, to avoid over-asking |
| Answer in German | Keep | Universal and not enforceable by a tool; add which language to use for code identifiers, comments and commits so the rules don't conflict (my inference) |
| Tests first for new logic | Move or qualify | Blanket testing rules add steps and cost on every task ([ETH](https://arxiv.org/html/2602.11988v1)); limit it to repos with a test setup, or move it to project files |
| Linter and type checker before "done" | Split | Keep the one-line intent globally (OpenAI endorses it); put commands in project AGENTS.md; add a Claude Stop hook for a guarantee |
| Verify frontend with browser/screenshot | Move | Only relevant to frontend work; put it in a frontend skill or the frontend projects' files |
| No ALL-CAPS in UI | Move | A project style rule, not a personal global one |
| New code matches surrounding code | Probably cut | Fails the "would removing this cause mistakes?" test for current models; keep only if you see violations |
| Comments only for the non-obvious | Keep | A short, non-default style preference |
| PR descriptions in German, short | Keep | In Claude Code, also set `includeGitInstructions: false` and `attribution` so built-in git instructions don't compete |

### The skills

**Location.** The skills sit in the legacy `~/.codex/skills`. Move them to `%USERPROFILE%\.agents\skills`, the current documented user scope, and junction the keepers into `.claude\skills`.

**Overlap.** Most of the custom skills overlap with each other or with the base file:

- **caveman and i-have-adhd** both enforce terse, no-preamble output. Measured savings are 1–3% of spend. Keep one, or replace both with two lines in the base file ([implicator.ai](https://www.implicator.ai/caveman-claude-code-skill-cuts-output-20-your-bill-barely-notices-2/)). i-have-adhd may come from ayghri/i-have-adhd or from the caveman repo, so check where your copy came from ([AI Weekly](https://aiweekly.co/alerts/i-have-adhd-skill-trends-on-hn-cuts-coding-agent-preamble)).
- **The four ponytail skills** compete for routing, and the core skill never self-activated in JetBrains' test. Fold its "does this need to exist?" ladder into your existing "do only what's needed" line, or keep ponytail as explicit-only. No evidence was found for ponytail-audit, ponytail-debt or ponytail-review; drop them unless you actually invoke them.
- **lean-build, verify-and-stop and investigate-first** duplicate your base-file rules.
- **surgical-patch, safe-refactor and migration** are genuine named procedures. Keep them explicit-only. surgical-patch has a registry trust score of 15/25 ([claudemarket.ai](https://www.claudemarket.ai/skills/juliusbrussee/caveman/surgical-patch)).
- **archify and hindsight-docs** have unverified origins. Audit them before deciding.
- **System skills** (imagegen, openai-docs, review-agent, skill-creator, skill-installer) are first-party. Keep them.

**last30days.** This skill needs the most attention. It runs a Python engine with broad network egress, and it **extracts browser cookies and macOS Keychain secrets** for X authentication ([GitHub](https://github.com/mvanhorn/last30days-skill)). That is OWASP's over-privileged / weak-isolation risk. Its 240 KB SKILL.md also means roughly 60K tokens enter context on every activation, by a rough four-characters-per-token estimate, which is about a hundred times the 500-line guideline. Either remove it, or reinstall it from mvanhorn's repo pinned to a commit, set `allow_implicit_invocation: false`, and run it only when you mean to.

**Plugins.** The many bundled plugins and the third-party ProjectAtlas plugin from a git marketplace fall under the same supply-chain logic. Disable the bundled plugins you don't use. Pin and review ProjectAtlas the way you would any third-party skill (OWASP AST02/AST07).

**Claude Code.** `~/.claude` could not be inspected, so nothing can be said about its current state. Starting it fresh from the overlay pattern above avoids carrying over undocumented rules.

## Conclusion

The practical lesson of 2026 is that instruction files and skills differ in *reliability*, not just in where they live. A global file is followed almost too well, on every task, so it should contain only rules you want applied everywhere. A skill is followed only if routing picks it, and for style-type skills routing often doesn't. Hooks and permissions are the only layer that guarantees anything. So the reset is less a rewrite than a sort: always-true personal preferences go into a short, calm AGENTS.md; must-happen checks go into hooks and approval settings; named procedures go into a small, pinned, audited skill set; and everything else goes. Your existing file is already close to the target size. Your skill shelf is where most of the cleanup is.

Two things are worth planning for. First, the guidance is tuned to Claude 4.5/4.6 and GPT-5.x. No vendor has yet published instruction-file guidance specific to Claude 5.x or GPT-6, and the push to prune comes from the observation that old rules turn into "shackles" on newer models. Re-auditing with `/doctor prompt-audit` and Codex's instruction summary after each major model change is worth more than any one-time setup. Second, the cross-tool bridge works today only through imports and junctions, while Claude Code is steadily adopting the AGENTS.md / `.agents/` conventions. Keeping AGENTS.md and `~/.agents/skills` as the canonical sources means a future native cross-read will simply remove a link rather than force a migration.
