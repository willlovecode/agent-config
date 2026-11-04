// Checks every vendored skill against skills/baseline.json: present, frontmatter name
// matches the folder, a description within the spec limit, a full commit pin.
import { readFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const repo = join(dirname(fileURLToPath(import.meta.url)), "..");
const baseline = JSON.parse(readFileSync(join(repo, "skills", "baseline.json"), "utf8"));
let failures = 0;
const fail = (msg) => { failures++; console.log(`FAIL ${msg}`); };

for (const s of baseline.skills) {
  const md = join(repo, "packs", s.pack, "skills", s.name, "SKILL.md");
  if (!existsSync(md)) { fail(`${s.name}: missing ${md}`); continue; }
  const text = readFileSync(md, "utf8");
  const fm = text.match(/^---\r?\n([\s\S]*?)\r?\n---/)?.[1] ?? "";
  const name = fm.match(/^name:\s*(.+)$/m)?.[1].trim();
  const desc = fm.match(/^description:\s*([\s\S]*?)(?=^\S[\w-]*:|$(?![\s\S]))/m)?.[1].trim() ?? "";
  if (name !== s.name) fail(`${s.name}: frontmatter name is "${name}"`);
  if (!desc) fail(`${s.name}: no description`);
  if (desc.length > 1024) fail(`${s.name}: description ${desc.length} chars (> 1024)`);
  if (!/^[0-9a-f]{40}$/.test(s.commit)) fail(`${s.name}: commit pin is not a full SHA`);
  if (!existsSync(join(repo, "packs", s.pack))) fail(`${s.name}: pack ${s.pack} missing`);
}
console.log(`${baseline.skills.length} skills checked`);
process.exit(failures ? 1 : 0);
