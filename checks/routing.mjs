// The model routing table as agtk writes it into ~/.claude/CLAUDE.md, ported from
// internal/routing/routing.go so the Node budget check can size that file without Go.
// internal/routing/node_test.go compares this output with the Go one for every preset and
// plan set, so a change to one side that misses the other fails `go test`.
const OPUS = "claude-opus-5-5", SONNET = "claude-sonnet-5-5", HAIKU = "claude-haiku-5-5", SOL = "gpt-6.1-sol";
const NAMES = { [OPUS]: "Opus 5.5", [SONNET]: "Sonnet 5.5", [HAIKU]: "Haiku 5.5", [SOL]: "GPT-6.1 Sol" };
const HANDOFF_LINE = "When a limit is hit, run the handoff skill, then continue in the fallback tool.";
const GEMINI_PRO = "gemini_pro", GEMINI_PLUS = "gemini_plus";

export const PLANS = ["claude_pro", "claude_max", "chatgpt", "copilot", GEMINI_PRO, GEMINI_PLUS];
const LEGACY_PLANS = { gemini: GEMINI_PRO };

const TASKS = [
  { id: "explore", label: "Find code", agent: "scout" },
  { id: "routine", label: "Implement, tests, bug fixes", agent: "implement" },
  { id: "hard", label: "Multi-file refactor, ambiguous design", agent: "" },
  { id: "review", label: "Review before commit", agent: "review" },
  { id: "mechanical", label: "Renames, boilerplate", agent: "mechanic" },
  { id: "research", label: "Long documents, literature", agent: "" },
];
const CODEX_EFFORT = { explore: "low", mechanical: "low", routine: "medium", review: "medium", research: "medium", hard: "xhigh" };

const claude = (model, effort) => ({ tool: "claude", model, effort });
const opus = claude(OPUS, "medium"), sonnet = claude(SONNET, "high"), haiku = claude(HAIKU, "medium");

export const PRESETS = {
  saver: { explore: haiku, routine: sonnet, hard: sonnet, review: sonnet, mechanical: haiku, research: sonnet },
  balanced: { explore: haiku, routine: sonnet, hard: opus, review: opus, mechanical: haiku, research: sonnet },
  quality: { explore: haiku, routine: opus, hard: opus, review: opus, mechanical: haiku, research: sonnet },
};

export function normalizePlans(plans) {
  if (!plans) return plans;
  return [...new Set(plans.map((p) => LEGACY_PLANS[p] ?? p))];
}

// Mirrors routing.Resolve: returns the preset id that applies ("" for none) or throws on a
// bad answer. Without Claude Code nothing is routed, but names are still checked.
export function resolve(preset, plans, claudeCode) {
  preset = !preset || preset === "none" ? "" : preset;
  plans = normalizePlans(plans) ?? [];
  if (preset && !(preset in PRESETS)) throw new Error(`unknown routing preset "${preset}" (saver, balanced or quality)`);
  for (const pl of plans) if (!PLANS.includes(pl)) throw new Error(`unknown plan "${pl}" (${PLANS.join(", ")})`);
  if (plans.includes(GEMINI_PRO) && plans.includes(GEMINI_PLUS)) throw new Error(`plans lists ${GEMINI_PRO} and ${GEMINI_PLUS}; keep the one you have`);
  if (!claudeCode && preset) return "";
  if (preset && !plans.includes("claude_pro") && !plans.includes("claude_max")) {
    throw new Error(`routing "${preset}" needs a Claude plan: add claude_pro or claude_max to plans`);
  }
  return preset;
}

const name = (model) => NAMES[model] ?? model;

function choiceString(c) {
  let s = "";
  if (c.tool === "claude") s = `Claude ${name(c.model)}`;
  else if (c.tool === "copilot") s = `Copilot ${name(c.model)}`;
  else if (c.tool === "gemini") s = c.plan === GEMINI_PRO ? "Antigravity (AI Pro)" : "Antigravity (confirm access)";
  else if (c.tool === "codex") s = `Codex ${name(c.model)}`;
  return c.effort ? `${s} ${c.effort}` : s;
}

function rows(preset, plans) {
  plans = normalizePlans(plans) ?? [];
  const gemini = { tool: "gemini", plan: plans.includes(GEMINI_PRO) ? GEMINI_PRO : GEMINI_PLUS };
  return TASKS.map((task) => {
    const use = PRESETS[preset][task.id];
    const order = task.id === "research" ? ["gemini", "copilot", "codex"] : ["copilot", "gemini", "codex"];
    const fallbacks = [];
    for (const tool of order) {
      if (tool === "copilot" && plans.includes("copilot")) fallbacks.push({ tool, model: use.model });
      else if (tool === "gemini" && (plans.includes(GEMINI_PRO) || plans.includes(GEMINI_PLUS))) fallbacks.push(gemini);
      else if (tool === "codex" && plans.includes("chatgpt")) fallbacks.push({ tool, model: SOL, effort: CODEX_EFFORT[task.id] });
    }
    return { task, use, fallbacks };
  });
}

export const BEGIN = "<!-- agtk:model-routing:begin -->";
export const END = "<!-- agtk:model-routing:end -->";

export function section(preset, plans) {
  const lines = ["| Task | Use | Fallback |", "|---|---|---|"];
  for (const r of rows(preset, plans)) {
    const use = choiceString(r.use) + (r.task.agent ? ` (\`${r.task.agent}\` agent)` : "");
    const fb = r.fallbacks.length ? r.fallbacks.map(choiceString).join(" then ") : "none";
    lines.push(`| ${r.task.label} | ${use} | ${fb} |`);
  }
  return `${BEGIN}\n## Model routing\nPreset ${preset}. Use the row for the task; take the fallback when the first choice hits its usage limit. ${HANDOFF_LINE}\n\n${lines.join("\n")}\n${END}`;
}
