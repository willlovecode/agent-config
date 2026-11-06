# AI harnesses, agents, local LLMs, Jarvis interfaces and trading bots

Working dossier, 2026-10-07. Built in two passes. First, the infoStacker vault (reel-derived notes, 342 notes, six concept pages read in full). Second, the web, driven by the questions the vault itself left open. It is meant to be read by a coding agent before it designs or advises on any of these topics.

**How to read the tags.** `[vault]` is something a saved reel claimed. Reels are mostly creator marketing, so treat these as leads, not facts. `[web]` is checked against a linked source on 2026-10-07. `[bg]` is my own background knowledge or arithmetic, not verified today. Where I could not check something I say so.

---

## 1. The ten things that matter most

1. **A harness is the loop around the model: context, tools, memory, permissions, verification.** Most agent quality differences between products come from it, not from the model. Anthropic's own guidance is to prefer a fixed workflow when the steps are known, and to use a free-running agent only for open-ended work [web: [Building effective agents](https://www.anthropic.com/research/building-effective-agents)].
2. **Long tasks fail from lost state, not weak models.** The fix that works is boring: a progress file, a feature list, git commits, and one small task per session [web: [Anthropic harness post](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)].
3. **Context is the scarce resource.** Techniques that hold up: compaction, notes kept outside the context, subagents with narrow contexts, and loading data just in time instead of up front [web, secondary: [summary of Anthropic's context-engineering guidance](https://agentic-ai.readthedocs.io/en/latest/ContextEngineering/anthropic/)].
4. **Multi-agent is expensive and only sometimes better.** Anthropic measured +90.2% on a research benchmark but about 15× the tokens of a chat; it says coding is usually a poor fit because it needs shared context [web: [multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system)].
5. **Flat agent swarms fail; hierarchies work.** Cursor's week-long browser build only worked after replacing self-coordinating agents with planners, workers and a judge [web: [Cursor](https://cursor.com/blog/scaling-agents)].
6. **Route by verification, not by the model's self-reported confidence.** Models are overconfident, and escalation triggered by a verifier beats escalation triggered by the model's own number [web: [calibration note](https://tianpan.co/blog/2026/04/20/llm-calibration-production-overconfidence), [arXiv 2604.19781](https://arxiv.org/pdf/2604.19781)].
7. **On an 8 GB GPU, run 4–9B models locally and send everything hard to the cloud.** Big mixture-of-experts models need roughly 100 GB of RAM to be usable at all (section 5).
8. **Never combine private data, untrusted content and a way to send data out in one agent.** Remove one of the three [web: [Willison's "lethal trifecta"](https://aiweekly.co/alerts/willison-names-lethal-trifecta-behind-ai-agent-data-leaks)].
9. **LLM trading agents have no demonstrated edge.** Three independent studies find they rarely beat buy-and-hold or retail baselines. The valuable parts of a trading system are risk guards, validation and execution plumbing (section 7).
10. **Most reel claims about income, speed multipliers and "137-agent companies" are unverifiable.** The vault's own notes already say so; the scorecard in section 8 grades each.

---

## 2. Harnesses and single agents

### 2.1 Workflows versus agents
Anthropic's distinction: a **workflow** runs LLMs and tools along code paths you wrote; an **agent** lets the model direct its own process. Workflow patterns worth knowing are prompt chaining, routing, parallelization, **orchestrator-workers** (a lead model splits the task at run time) and **evaluator-optimizer** (one call generates, another critiques, in a loop). Use workflows for repeatable low-variance jobs, agents for open-ended ones where the number of steps is unknown [web: [Anthropic](https://www.anthropic.com/research/building-effective-agents), [summary](https://mer.vin/2026/05/when-not-to-build-ai-agents-anthropics-workflow-vs-agent-playbook/)].

infoStacker is a workflow, not an agent, and should stay one: its steps are fixed and each can be tested.

### 2.2 Long-running work
Agents work in separate sessions with no memory of earlier ones. Anthropic's harness: an **initializer** run writes `init.sh`, a `claude-progress.txt` log, a `feature_list.json` and a first commit; every later session reads the progress file and git log, checks the app still works, implements **one** failing feature, tests it and commits. This targets four failures: doing everything at once, re-investigating from scratch, declaring victory early, and building on a broken base [web: [Anthropic](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)].

### 2.3 Context engineering
- **Compaction** summarizes a nearly full conversation and restarts from the summary.
- **Structured notes** move state outside the window and bring back only what is needed.
- **Subagents** do deep work in a clean context and return a short summary.
- **Just-in-time retrieval** keeps paths and queries, and loads data when needed. Claude Code loads `CLAUDE.md` up front and uses glob and grep for the rest [web, secondary: [context engineering summary](https://agentic-ai.readthedocs.io/en/latest/ContextEngineering/anthropic/)].

The vault's "40–50K tokens, then reset" rule `[vault]` is a heuristic; I found no measurement behind it. The vault also asks whether classifier-based compaction beats `/compact`; its only evidence was a handful of demos. Treat both as unproven.

### 2.4 Instruction files and skills
- **AGENTS.md** is the shared project-instruction file, supported by Codex, Copilot, Cursor, Windsurf and 20+ tools. **SKILL.md** (Agent Skills, started by Anthropic in December 2025) is the shared packaging format for reusable procedures, with 30+ tools by April 2026. Skills load progressively: about 100 tokens of metadata at start, the full text only when a task matches [web: [adoption note](https://noqta.tn/en/news/skill-md-open-standard-30-ai-coding-tools-adoption-2026)].
- Keep the base file short and hand-written. The ETH Zurich result, already cited in this repo's `docs/research.md`, found that generated context files slightly lowered success and raised cost.
- Skill registries have carried real malware (same file). Install few skills, from sources you have read.

### 2.5 Tools: MCP, CLIs and agent-friendly output
- **MCP** is a standard way to connect tools. Its criticized weakness is context cost: loading one server's full tool schema can take tens of thousands of tokens before any work starts. By March 2026 several teams (reportedly including Perplexity) moved workflows to plain APIs and CLIs. Claude Code answered with **deferred tool loading**, reported to cut that overhead by 85%+ [web: [CLI vs MCP](https://buildwithfern.com/post/cli-vs-mcp-which-api-interface-first), [MCP vs CLI](https://noqta.tn/en/blog/mcp-vs-cli-ai-agents-tool-protocol-comparison-2026)].
- CLIs cost nothing until used and are discoverable through `--help`, but raw shell access needs sandboxing, credential isolation and approval gates. MCP offers OAuth, per-user permissions and audit trails (same sources).
- **AXI** (Agent eXperience Interface) is a set of 10 design rules for agent-native CLIs: compact output, minimal schemas, truncation with escape hatches, precomputed aggregates, explicit empty states, structured errors, non-interactive behaviour. Its TOON output format is claimed to save about 40% of tokens versus JSON [web: [github.com/kunchenguid/axi](https://github.com/kunchenguid/axi)]. The vault says its author is at Meta; I could not confirm the affiliation.
- Practical rule `[bg]`: write tools that return small, pre-aggregated answers with clear errors, whatever the transport.

### 2.6 Scheduled and "loop" agents
Claude Code's `/loop` is a session-scoped scheduler built on cron tools. Tasks die when the session closes and recurring ones expire after 3 days, with at most 50 per session. Cloud routines and desktop scheduled tasks are the persistent alternatives [web: [MindStudio](https://www.mindstudio.ai/blog/what-is-claude-code-loop-scheduled-recurring-tasks), [Claude Code loops guide](https://mer.vin/2026/07/claude-code-loops-guide-turn-based-goal-time-and-proactive-agent-patterns/)].

The vault's "five-part loop" (automation, worksheets, skills, connectors, subagents) `[vault]` is one creator's framework. I found no product or spec that defines a "worksheet", so the vault's question about its format is still open. A reasonable reading: a shared task file or board that agents claim work from. Cursor's experience (section 3.1) says a shared file with locking becomes a bottleneck.

### 2.7 Security
- **Lethal trifecta:** an agent with (a) private data, (b) exposure to untrusted content and (c) a way to send data out can be turned against its owner by one crafted web page or email. The fix is architectural: break one leg. Documented exfiltration exploits hit Microsoft 365 Copilot, GitHub's MCP server, GitLab Duo, ChatGPT, Amazon Q and Slack between 2023 and 2025 [web: [CSA note](https://labs.cloudsecurityalliance.org/research/csa-research-note-ai-agent-lethal-trifecta-capability-securi), [summary](https://aiweekly.co/alerts/willison-names-lethal-trifecta-behind-ai-agent-data-leaks)].
- Practical rules `[bg]`: least-privilege credentials per agent; separate "reads untrusted text" agents from "can act" agents; require human approval for sending, spending and publishing; log every tool call.

---

## 3. Agent networks (multi-agent systems)

### 3.1 What the evidence says
| Source | Finding |
|---|---|
| [Cursor, Jan 2026](https://cursor.com/blog/scaling-agents) | Hundreds of concurrent agents, "close to a week", 1M+ lines in 1,000 files for a from-scratch browser. **Flat self-coordination through a shared file failed** (agents held locks too long; 20 agents performed like 2–3). **Optimistic concurrency failed** (agents turned risk-averse and made small safe edits). **Planner → worker → judge worked.** Models were mixed by role. Periodic fresh starts were needed against drift. The browser stayed experimental, not production-ready. |
| [Anthropic research system](https://www.anthropic.com/engineering/multi-agent-research-system) | Lead agent (Opus) plus parallel subagents (Sonnet): **+90.2%** over a single Opus agent on its internal research eval. Agents use about 4× the tokens of chat, multi-agent about **15×**. Token use explained 80% of the variance on BrowseComp. Poor fit for tightly coupled work such as most coding. |
| [MAST, Why do multi-agent systems fail?](https://arxiv.org/abs/2503.13657) | 14 failure modes in 3 groups: specification issues, inter-agent misalignment, task verification. Traces from 7 frameworks over 200+ tasks. Failures stem mainly from system design and misalignment, not just model weakness, and gains over single agents are often minimal. |

### 3.2 Design rules that follow
1. Start with one agent. Add agents only for work that is **parallel** (independent subtasks), **bigger than one context window**, or **worth the token bill**.
2. Use a hierarchy with an explicit **judge** or verifier. Do not rely on peer agents negotiating.
3. Give each subagent an objective, an output format, tool guidance and boundaries. Vague delegation causes duplicated work [web: Anthropic post above].
4. Budget for state: checkpoints, resumption, tracing, and rainbow deployments so a running agent is not broken by an update (same source).
5. Evaluate from day one on ~20 realistic queries with an LLM judge plus human spot checks (same source).
6. A shared knowledge store at the center, which the vault repeats `[vault]`, matches Anthropic's artifact-based handoffs. It is a good default, but it is a design choice, not a proven law.

### 3.3 Protocols
- **MCP** connects an agent to tools and data (vertical). **A2A** (Agent2Agent, Linux Foundation, 150+ organizations at its one-year mark in April 2026) lets agents from different vendors talk to each other (horizontal), using AgentCards, HTTP, JSON-RPC and server-sent events [web: [Linux Foundation](https://linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year)]. For a single-owner personal system you need neither A2A nor a framework: files, a task queue and a CLI are enough `[bg]`.

### 3.4 Simulation swarms (MiroFish)
MiroFish is real: open source, about 49k GitHub stars by April 2026, incubated by Shanda Group. Pipeline: GraphRAG knowledge graph from your documents → thousands of generated personas → a Twitter-like and a Reddit-like world on the OASIS engine (CAMEL-AI) → a ReportAgent that analyses the outcome; agent memory runs on Zep Cloud [web: [overview](https://moclaw.ai/blog/what-is-mirofish), [repo mirror](https://github.com/Grandlulu/MiroFish)]. I found **no accuracy validation** against real outcomes, so treat predictions as brainstorming. The "offline edition: 0 cloud APIs" is a README claim and its hardware needs are unstated `[vault]`. Note that Zep Cloud is a cloud service, which sits awkwardly with "100% local".

---

## 4. Models, routing and cost

### 4.1 The October 2026 landscape `[web: [BenchLM](https://benchlm.ai/frontier-ai-models), [MarkTechPost comparison](https://www.marktechpost.com/2026/10/04/gpt-6-astra-vs-gpt-6-1-sol-vs-gemini-4-argon-vs-claude-fable-5-1-which-frontier-model-fits-which-job/)]`
| Model | Role in practice | Price per million tokens (fresh in / cached in / out) |
|---|---|---|
| Claude Opus 5.5 | Everyday frontier pick; #1 on one leaderboard (86.39), Artificial Analysis index 58 | $4 / $0.20 / $20 |
| Claude Fable 5.1 | Hardest coding and agentic work; 80.3% SWE-Bench Pro; index 53 | $10 / $0.25 / $50 |
| GPT-6.1 Sol | Strong value; index 52; 1.05M context | $2 / $0.10 / $10 |
| Grok 4.7 | Real-time X and web data | $2 / $0.50 / $6 |
| GPT-6 Astra, Gemini 4 Argon | Top-tier alternatives | not checked |

Prices and rankings move monthly. Re-check before any cost decision. The vault's "Luna / Sol / Astra" tiers `[vault]` match OpenAI's real naming; its "illustrative credits" are not dollars, so its 5–15× savings cannot be recomputed from the reel alone.

### 4.2 Routing and cascades
- **RouteLLM** (ICLR 2025): a trained router sending only 14% of queries to the strong model kept 95% of GPT-4's quality and cut cost by over 85% on MT Bench. **FrugalGPT:** a tuned cascade matched GPT-4 on several benchmarks at up to 98% lower cost [web: [RouteLLM](https://arxiv.org/html/2406.18665v2), [survey](https://arxiv.org/pdf/2603.04445)]. The vault's cascade figures (62/26/12% of queries, 5× cheaper, 82% correct) `[vault]` are the same order of magnitude, and it is plausible.
- **Cascade (try cheap first, escalate) versus router (pick one model up front):** a cascade pays for every failed cheap attempt, so it fits workloads of mixed difficulty and is wasteful when most requests are hard.
- **Escalation signal:** self-reported confidence is the weak option. A cascade's savings depend on how well its confidence signal discriminates; a verifier (tests, a checker, a schema, a second model) is stronger, and proposer confidence plus verifier support is best [web: [arXiv 2604.19781](https://arxiv.org/pdf/2604.19781), [calibration note](https://tianpan.co/blog/2026/04/20/llm-calibration-production-overconfidence)]. This matches the vault's "what changed" entry that moved from confidence-triggered to verification-triggered escalation.
- **Reasoning effort is a second routing axis** (Low to Max) `[vault]`; vendors expose it as a setting. Verify the saving on your own tasks.

### 4.3 What infoStacker's own bake-off showed `[my measurements, 2026-10-07, 20 videos]`
Codex Sol (high) drafted a note in about 30 s with ratings spread 1–5; local 4–9B models took 19–75 s and rated about 55% of videos 4/5. Claude Sonnet and Haiku agreed closely with Codex on ratings (8 videos) at about $0.10 and $0.04 per video in API-equivalent terms. For writing whole concept pages, Opus merged across videos best (about $0.60 per page), Sonnet about 5× cheaper but cited fewer sources, Codex xhigh slower and less merged. Small samples, and Codex was only the reference. Routing lesson: **cheap local models for volume and drafts, a strong cloud model for rating and synthesis, a verifier for escalation.**

---

## 5. Local LLMs

### 5.1 The physics `[bg]`
Generating a token reads the model's **active** weights from memory once. So decode speed is roughly `memory bandwidth ÷ bytes of active weights`, scaled by an efficiency of maybe 20–60%. Prompt processing is compute-bound and much faster per token. This explains almost every local-inference puzzle in the vault.

### 5.2 Your hardware (RTX 3050 8 GB, 16 GB RAM, Windows)
- The 3050 8 GB has about 224 GB/s of bandwidth `[bg]`; dual-channel DDR4 is far lower (roughly 40–50 GB/s) `[bg]`. Anything that spills out of VRAM slows sharply.
- **What fits:** dense models up to ~9B at 4-bit (Qwen3.5-9B is 6.6–7.6 GB in Ollama [web: [Ollama](https://ollama.com/library/qwen3.5)]) and everything smaller. My own runs: Qwen3.5-4B about 19 s per video note, Qwen3.5-9B about 39 s, and `qwen3-vl:8b` 200–580 s because it spilled.
- **What does not:** an estimator site puts GPT-OSS-20B at ~4.3 tok/s and Qwen3.5-35B-A3B at ~3.1 tok/s on this GPU with CPU offload, with 45–63 s to the first token [web, rough: [willitrunai](https://willitrunai.com/de/can-run/gpt-oss-20b-on-rtx-3050-8gb)]. Not worth it.
- **Qwen3.8 changed the ladder:** the family is a 27B dense model (Apache 2.0, GPQA Diamond 89.2, SWE-bench Pro 61.7), then 180B-A6B, then 2.4T-A95B. There is **no 4B/8B/14B** and the mid-size MoE band disappeared. The 27B wants about 24 GB of VRAM; 127 tok/s single-user on a 24 GB card with tuned vLLM, 26–61 tok/s depending on precision elsewhere [web: [Northflank](https://northflank.com/blog/qwen3-8-27b-performance-benchmarks-gpu-requirements-and-how-to-run-it), [HyperQwen](https://github.com/syv-ai/qwen38-27b-rtx3090)]. For your card the small models remain Qwen3.5 (0.8–9B) and similar (a Gemma 4 E2B build appears in a Jetson voice project [web]).

### 5.3 Big mixture-of-experts on a budget
- **DeepSeek V4 Flash:** 284B total, **13B active**, 1M context. Quantized files: 1-bit 92 GB, 2-bit 102 GB, 3-bit 110–135 GB, 4-bit 155 GB, lossless 162 GB; you need RAM about equal to the file plus context. Unsloth says to have at least 110 GB for the 3-bit build. **V4 Pro** is 1.6T total, 49B active [web: [Unsloth guide](https://unsloth.ai/docs/models/deepseek-v4)].
- This corrects the vault: the "80 GB community quant on a 256 GB Xeon at ~3 tok/s" `[vault]` is a believable order of magnitude but does not match a published size (nearest is the 92 GB 1-bit file). My arithmetic `[bg]`: 13B active at ~2 bits is ~3.3 GB per token; with ~80 GB/s of Xeon memory bandwidth the ceiling is ~24 tok/s, so 3 tok/s is plausible at poor efficiency. The vault's "100 tok/s under vLLM" for DeepSeek V4 is on datacenter GPUs (the vendor's figure is 60 tok/s baseline, 120 with speculative decoding on a B200) [web: Unsloth].
- **Technique:** `llama.cpp` keeps attention and dense layers on the GPU and moves routed experts to RAM with `--n-cpu-moe N` or `-ot "exps=CPU"`. Start with all experts on the CPU and walk the number down until VRAM runs out; speed is dominated by memory bandwidth [web: [HF guide](https://huggingface.co/blog/Doctor-Shotgun/llamacpp-moe-offload-guide)]. One report doubled Qwen3-35B-A3B decode speed (17 to 34 tok/s) on 12 GB of VRAM by tuning this flag [web: [AI Weekly](https://aiweekly.co/alerts/qwen3-moe-cpu-expert-offload-doubles-decode-speed-on-12gb-vram)].

### 5.4 Runtimes `[bg]`
Ollama: easiest, fine for one user. llama.cpp: the most control (MoE offload, quant choices). vLLM/SGLang: many concurrent users, needs real VRAM. LM Studio: a GUI over llama.cpp.

### 5.5 Policy and availability risk
The vault claims Hugging Face banned GLM-5.3 and OpenAI banned a user twice for distillation `[vault]`. **I did not verify either.** The general lesson stands: keep a copy of any open-weights model you depend on, and read provider terms before distilling a hosted model.

---

## 6. Jarvis-style interfaces

### 6.1 Reference stack
`mic → wake word → VAD/endpointing → speech-to-text → router → (rules | small model | agent harness) → text-to-speech → speaker`, with a memory store and tool permissions around the router.
- Wake word: **openWakeWord** (runs on the edge). VAD: **Silero**. STT: **faster-whisper** or whisper.cpp. TTS: **Kokoro-82M** (Piper was archived on 2025-10-06, so no more fixes) [web: [stack overview](https://www.kunalganglani.com/blog/local-ai-voice-assistant-whisper-piper-ollama), [JarvisAi](https://github.com/PanPenek/JarvisAi)].
- **Wyoming protocol** (JSON over TCP) is the glue Home Assistant uses to swap STT, TTS and wake-word services [web: same stack overview].
- **Latency:** Whisper under 1 s on an x86 mini-PC (about 8 s on a Pi 4); a 3B model 2–3 s on a 16 GB x86 box; end to end 3–4 s on an Intel NUC and under 2 s on Apple Silicon. A Rust framework (Skadoosh) targets under 150 ms from end of speech to first audio and speaks per clause [web: same sources]. Streaming and speaking the first sentence early matter more than a faster model.
- **Hard parts:** barge-in (interrupting while it speaks) and endpointing are not solved by the basic stack; the context window fills during long voice sessions [web].

### 6.2 Routing and memory
- Three tiers `[vault]`: rules for known commands, a small fast model for simple interpretation, a full agent for real work. This is the cascade idea applied to an assistant, and it is sound. The "Jev" classifier is a **promotional** product; its 193.6× faster and 444.6× cheaper numbers are unbenchmarked and the reels admit it `[vault]`. A well-prompted small model with a rules layer does the same job `[bg]`.
- **Memory:** a markdown vault plus `grep`/glob is what Claude Code and Cursor do. Boris Cherny said agentic search "outperformed everything by a lot" [web, secondary: [Latent Space via Zilliz critique](https://zilliz.com/blog/why-im-against-claude-codes-grep-only-retrieval-it-just-burns-too-many-tokens)]. Use embeddings when the corpus is too large to explore, or matching is by meaning; Zilliz argues grep burns tokens at scale. The vault's own caution applies: an organized graph by itself gives no retrieval benefit.
- **Actions:** device control needs the approval and guard patterns from section 7.4.

### 6.3 Ready-made pieces
- **Hermes Agent** (Nous Research, open source) is a self-improving personal agent: skills it writes and refines, cross-session memory (full-text search plus summaries), a messaging gateway (Telegram, Discord, Slack, WhatsApp, Signal), a cron scheduler, and subagents; runs from a $5 VPS up, with 200+ models through OpenRouter [web: [Hermes docs](https://hermes-agent.nousresearch.com/docs)]. The vault's "Ultron" stack describes "Hermes" as a Python layer connecting LLMs to devices `[vault]`; that is a partial match, so the reel may be describing a different component or loosely using the name.
- **OpenWorker, Odysseus** `[vault]` are open-source local agents I did not independently check.

---

## 7. Trading bots

This is engineering and risk knowledge, not investment advice, and nothing here says any strategy makes money.

### 7.1 Evidence on LLM trading agents
- **StockBench** (contamination-free, multi-month daily decisions): most models fail to beat buy-and-hold; some show better risk management [web: [arXiv 2510.02209](https://arxiv.org/abs/2510.02209)].
- **Agent Market Arena** (live, multi-market): agent *framework* design changed behavior more than the underlying model [web: [arXiv 2510.11695](https://arxiv.org/abs/2510.11695)].
- **Six-month production record** (7.5M model calls, ~300K onchain actions; 3,505 funded vaults and 500–599 user-made agents): "neither fleet shows a directional edge", the Hyperliquid fleet's win rate was 41% versus 50% for a retail benchmark, frontier models were statistically indistinguishable, a risk slider explained leverage, and median leverage stayed 5× at every volatility level [web: [arXiv 2609.05663](https://arxiv.org/abs/2609.05663)].

Conclusion: an LLM is useful for **research, code generation and monitoring**, not as the thing deciding trades.

### 7.2 Validation: why most backtests lie
- The vault's validation ladder (hypothesis first, rules before results, chronological 70/30 split, full costs, robustness across periods and assets, count every version you tried, fewer than 100 trades is a hypothesis) `[vault]` is standard good practice.
- **Multiple testing is the core problem.** The **Deflated Sharpe Ratio** corrects a selected strategy's Sharpe for the number of trials and for non-normal returns [web: [Bailey and López de Prado, SSRN](https://papers.ssrn.com/abstract=2460551)]. Any parameter sweep or strategy generator needs it, or the equivalent probability-of-backtest-overfitting test. This answers the vault's open question about "which correction" and its tension with "59,470 strategies a day": at that volume, the best of them is expected to look great by chance.
- **Timestamp leakage is real and measurable** `[vault]`: assigning news to the first actionable session cut one strategy's Sharpe from 3.18 to 0.98 in a 5,908-article sample.
- **Costs decide small edges:** the vault's XAUUSD sweep statistics (92.1% of sweeps reclaim, then a 49–51% coin flip, negative after costs) are a clean example `[vault]`.

### 7.3 Engines and venues
- **Nautilus Trader:** Rust-native core, deterministic event-driven design, same execution semantics in research and live runs; Python is the control plane. Three paths exist: v1 (Cython, broadest coverage), v2 Rust, and v2 PyO3 (Python strategies on the Rust engine). As of the docs I saw, **the Interactive Brokers adapter exists in v1 only**, so check adapter coverage before choosing v2 [web: [PyPI](https://pypi.org/project/nautilus_trader/2.0.0rc2/), [docs](https://nautilustrader.io/docs/latest/concepts/rust/)].
- Alternatives `[bg]`: vectorbt for fast vectorized research, Freqtrade for crypto bots with a UI, backtrader for older event-driven backtests. I did not re-verify their current maintenance.
- **OpenAlice** (TraderAlice, TypeScript, ~5.4k stars): each trading account owns its broker connection, a git-like history and a guard pipeline; the AI talks to accounts, never to brokers; every trade needs explicit approval. Brokers: CCXT (100+ crypto exchanges), Alpaca, Interactive Brokers [web: [OpenAlice on GitHub](https://github.com/TraderAlice/OpenAlice)]. This matches the vault.

### 7.4 Safety patterns worth copying
1. **Fail-closed guard layer on every order path**, configured as numbers (max contracts, trades per day, daily loss, trading window, news blackouts, max correlation, max drawdown) `[vault]`. Include a timezone; the vault's RiskGate example omitted one.
2. **Staged, annotated, hash-addressable order history** (OpenAlice's "trading as git") so every decision is reviewable.
3. **Human approval before execution**, snapshots, an equity curve, and paper trading before any real money.
4. **Keep the LLM out of the order path.** Let it propose, let deterministic code check and place.

### 7.5 Base rates
- **Prop-firm challenges:** about 5–14% of purchased challenges end funded; about 7% of buyers ever receive a payout, and roughly 1–2% of all clients get paid [web: [Finance Magnates](https://financemagnates.com/forex/analysis/exclusive-only-7-of-300000-prop-trading-accounts-achieved-payouts), [Track360](https://track360.io/blog/prop-trading-industry-statistics-2026)]. The vault's ORB simulation (19% CAGR, Sharpe 2.16, 50% Topstep pass) `[vault]` is a strong strategy and still only a coin flip, which fits.
- **Polymarket:** an on-chain study of 95M transactions found 0.51% of wallets above $1,000 profit; 36.8% of active wallets were up for the week and the median PnL was about -$3. Makers pay no fees and get 20–25% of taker fees back; the effective taker fee can reach ~1.56% at the midpoint, which kills thin-edge bot strategies [web, secondary: [analysis](https://digitechbytes.com/emerging-consumer-tech-explained/are-polymarket-trading-bots-actually-profitable-the-math-behind-2026-s-predictio/), [fee changes](https://dev.to/emilanielsen/polymarkets-new-fee-structure-why-many-trading-bots-may-no-longer-be-profitable-2j3j)]. The vault's "$50 to $5,000" and 53–55% accuracy claims should be read against this.
- **Market making:** queue position and latency dominate. The vault's simulator itself shows 24% rejection at 200 ms and a rebate only exchange market makers get `[vault]`; retail has no realistic path there `[bg]`.
- **Spoofing** (orders placed to cancel) is illegal market manipulation in regulated markets `[bg]`.

---

## 8. Scorecard of vault claims

| Claim from the vault | Verdict |
|---|---|
| Cursor agent built a browser in ~7 days with little human help | **Mostly true, mis-framed.** Hundreds of agents (not one), close to a week, planner/worker/judge, fresh restarts needed, output experimental. The "no human intervention" wording is too strong. |
| DeepSeek V4 Flash "13B" means active experts | **True.** 284B total, 13B active. |
| 80 GB quant on a 256 GB Xeon gives ~3 tok/s | **Plausible, unconfirmed.** No published 80 GB file; ceiling arithmetic allows ~3 tok/s. |
| Qwen3.8 is 27B dense, 256K context, 30 tok/s | **Roughly true** for 27B (26–44 tok/s at BF16/FP8 on one user); context and Opus-parity claims unchecked. |
| Qwen3.8 is "on par with Opus" | **Unsupported.** No shared benchmark found; Opus 5.5 leads independent indexes. |
| MiroFish simulates public reaction; offline edition is fully local | **Real project; accuracy unvalidated; "fully local" doubtful** (agent memory uses Zep Cloud). |
| OpenAlice has a guard pipeline, trading-as-git and approval | **Confirmed** from its repo description. |
| Nautilus Trader v2 is Rust-native with PyO3 bindings | **Confirmed**; note live-adapter gaps in v2. |
| AXI replaces MCP with token-efficient CLIs | **Real project** (10 principles, TOON format); affiliation unchecked; "replacement" is the author's pitch. |
| Hermes is a Python layer connecting LLMs to devices | **Partial.** A real, different thing exists called Hermes Agent. |
| Claude Code has loops | **True** (`/loop`), but session-scoped and 3-day expiring; "worksheets" are not a documented feature. |
| Cascades cut cost ~5× at 82% correctness | **Plausible**; same order as RouteLLM and FrugalGPT. |
| Self-reported confidence ≥0.8 as escalation rule | **Weak** by the research; the vault already corrected itself toward verifiers. |
| $50 to $5,000, $3k/month and similar bot income | **No evidence**, and the base rates (7.5) argue against it. |
| 137-agent company map | **Unsupported**; shows task names only. |
| 59,470 strategies/day with 70/30 validation | **Contradictory** without a multiple-testing correction. |
| GLM-5.3 ban, OpenAI distillation ban | **Not verified.** |
| Fastest scraper, 774× speed, bypasses Cloudflare | **Unverifiable**, and bypass breaks site terms. |

---

## 9. Vault open questions: answered and still open

**Answered here**
- *Why do local speeds differ 30× (3 vs 100 tok/s)?* Different memory bandwidth and active size: datacenter GPU versus a CPU with used DDR4 (5.1, 5.3).
- *What hardware does a 100% local MiroFish need?* Unstated by the project; its agents use an LLM, so it scales with whichever model you point it at, and Zep Cloud is a dependency.
- *Is a classifier layer worth it?* Only for high-volume repeated decisions; verifier-based escalation beats confidence (4.2).
- *Would an index plus grep beat embeddings for a vault?* For a few thousand notes, agentic grep plus an index is the default in Claude Code and Cursor; add embeddings when scale or meaning-matching demands it (6.2).
- *Which correction for multiple comparisons?* Deflated Sharpe, plus probability of backtest overfitting (7.2).
- *What does an autonomous multi-day run cost?* Cursor gave no figure; outside commentary guessed several million dollars for the browser. Anthropic's multiplier is ~15× chat. Budget with tokens, not days.
- *Are the Nautilus claims accurate?* Yes (7.3).

**Still open**
- Exact format and owner of a "worksheet" in the loops pattern.
- How much correctness loss a cascade should accept: this depends on your own evaluation set.
- Whether Alpha Ledger-style copy trading is permitted by its terms, and whether it performs.
- Which Xeon platform produces the claimed 3 tok/s.
- Real-world accuracy of simulated public reaction.
- Qwen3.8 versus current frontier models on a shared benchmark.

---

## 10. What this means for your setup

1. **infoStacker stays a workflow.** Its current split is sound: local embeddings, Codex for drafts and ratings, Claude Opus for page synthesis. Add a verifier before any cloud escalation if you ever route per task.
2. **For the planned AI workstation:** a harness with a router (rules → small local model → cloud agent), a markdown vault as shared memory searched with grep first, per-role models, approval gates on anything that sends, spends or publishes, tracing on every tool call, and a 20-query evaluation set built before the first agent.
3. **Local models:** Qwen3.5-4B for volume classification and drafts, Qwen3.5-9B when quality needs it, `bge-m3` for embeddings, faster-whisper for speech. Do not buy hardware for DeepSeek V4 Flash unless you are willing to put 110 GB or more of RAM behind it.
4. **A Jarvis prototype** is feasible on this PC: openWakeWord + Silero VAD + faster-whisper + a 4B local model for simple commands + Kokoro, escalating to a cloud agent for real work. Expect 3–4 s responses at first; spend effort on streaming the first sentence.
5. **Trading:** if you build anything, build the guard layer, the paper-trading loop and the validation harness first, keep the LLM out of order placement, and treat every income claim in the vault as marketing.

## 11. Gaps and limits of this dossier
- Sources are search summaries and a few fetched pages; I did not read full papers. Where only a secondary summary was available I said so.
- Model names, prices and rankings are from third-party aggregators dated October 2026 and change monthly.
- I did not verify: GLM-5.3, Jev/TypeSafe, Ultron's components, Alpha Ledger, Freqtrade/vectorbt status, or the qualifications of any reel author.
- Hardware bandwidth figures marked `[bg]` come from my background knowledge, not from a source I opened today.
