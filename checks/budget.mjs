// Measures what every session loads before the task starts, per profile, and fails
// above the budgets in docs/portable-toolkit-design.md. Tokens are estimated as chars/4.
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const repo = join(dirname(fileURLToPath(import.meta.url)), "..");
const BUDGET = { instructions: 1200, skillCount: 15, skillTokens: 1500, agentTokens: 400 };
const tok = (s) => Math.round(s.length / 4);
const read = (...p) => readFileSync(join(repo, ...p), "utf8");

function frontmatter(md) {
  const m = md.match(/^---\r?\n([\s\S]*?)\r?\n---/);
  if (!m) return {};
  const out = {};
  let key = null;
  for (const line of m[1].split(/\r?\n/)) {
    const kv = line.match(/^([\w-]+):\s*(.*)$/);
    if (kv) { key = kv[1]; out[key] = kv[2].replace(/^>-?\s*$/, ""); }
    else if (key && /^\s+/.test(line)) out[key] = `${out[key]} ${line.trim()}`.trim();
  }
  return out;
}

const baseline = JSON.parse(read("skills", "baseline.json"));
let failed = false;
for (const profile of readdirSync(join(repo, "profiles"))) {
  const prof = JSON.parse(read("profiles", profile, "profile.json"));
  const instructions = tok(read("base", "AGENTS.md") + "\n\n" + read("profiles", profile, "AGENTS.md"));
  const listed = baseline.skills.filter((s) => prof.packs.includes(s.pack) && !s.explicit_only);
  const skillTokens = listed.reduce((n, s) => {
    const fm = frontmatter(read("packs", s.pack, "skills", s.name, "SKILL.md"));
    return n + tok(`${s.name}: ${fm.description || ""}`);
  }, 0);
  let agentTokens = 0;
  for (const pack of prof.packs) {
    const dir = join(repo, "packs", pack, "agents");
    if (!existsSync(dir)) continue;
    for (const f of readdirSync(dir).filter((f) => f.endsWith(".md"))) {
      const fm = frontmatter(readFileSync(join(dir, f), "utf8"));
      agentTokens += tok(`${fm.name}: ${fm.description || ""}`);
    }
  }
  const rows = [
    ["instructions (AGENTS.md)", instructions, BUDGET.instructions],
    ["listed skills (count)", listed.length, BUDGET.skillCount],
    ["listed skill descriptions", skillTokens, BUDGET.skillTokens],
    ["subagent descriptions", agentTokens, BUDGET.agentTokens],
  ];
  console.log(`\n${profile}`);
  for (const [name, value, max] of rows) {
    const ok = value <= max;
    if (!ok) failed = true;
    console.log(`  ${ok ? "ok  " : "OVER"} ${name.padEnd(28)} ${String(value).padStart(5)} / ${max}`);
  }
  console.log(`  total always-loaded ~${instructions + skillTokens + agentTokens} tokens`);
}
process.exit(failed ? 1 : 0);
