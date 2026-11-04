// Tests enforcement/hooks: git-guard against the case list, stop-checks through its states.
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, appendFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const repo = join(dirname(fileURLToPath(import.meta.url)), "..");
const hooks = join(repo, "enforcement", "hooks");
let failures = 0;
const run = (script, input, args = []) =>
  spawnSync(process.execPath, [join(hooks, script), ...args], { input, encoding: "utf8" }).stdout;

// git-guard: each line is "D|command" (must deny) or "A|command" (must allow).
const cases = readFileSync(join(hooks, "test", "git-guard-cases.txt"), "utf8").split(/\r?\n/).filter(Boolean);
for (const line of cases) {
  const [want, ...rest] = line.split("|");
  const command = rest.join("|");
  const got = run("git-guard.mjs", JSON.stringify({ tool_name: "Bash", tool_input: { command } })) ? "D" : "A";
  if (got !== want) { failures++; console.log(`FAIL git-guard: want ${want}, got ${got}: ${command}`); }
}
if (!run("git-guard.mjs", "not json")) { failures++; console.log("FAIL git-guard: unparsable input must deny"); }
console.log(`git-guard: ${cases.length + 1} cases`);

// stop-checks: a throwaway repo whose check fails when app.txt contains BAD.
const dir = mkdtempSync(join(tmpdir(), "stop-checks-"));
const git = (...a) => spawnSync("git", ["-C", dir, ...a], { encoding: "utf8" });
git("init", "-q");
writeFileSync(join(dir, "app.txt"), "ok\n");
writeFileSync(join(dir, "check.js"), "const s=require('fs').readFileSync('app.txt','utf8');if(/BAD/.test(s)){console.error('found BAD');process.exit(1)}");
git("add", "-A");
git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init");
const stop = (active) => run("stop-checks.mjs", JSON.stringify({ cwd: dir, stop_hook_active: active }), ["node check.js", "60"]);
const expect = (name, out, block) => {
  const blocked = /"decision":"block"/.test(out);
  if (blocked !== block) { failures++; console.log(`FAIL stop-checks: ${name}`); }
};
expect("clean tree passes", stop(false), false);
expect("unchanged tree is cached", stop(false), false);
appendFileSync(join(dir, "app.txt"), "BAD\n");
expect("failure blocks", stop(false), true);
expect("retry 1 still blocks", stop(true), true);
expect("retry 2 gives up", stop(true), false);
expect("fresh stop blocks again", stop(false), true);
writeFileSync(join(dir, "app.txt"), "ok\n");
writeFileSync(join(dir, "new.txt"), "x\n");
expect("untracked file, passing", stop(false), false);
rmSync(dir, { recursive: true, force: true });
console.log("stop-checks: 7 states");

console.log(failures ? `${failures} failure(s)` : "all hook tests passed");
process.exit(failures ? 1 : 0);
