// Stop hook for Claude Code and Codex: runs a project's checks before the agent may finish.
// Usage in a project's hook config: node <agent-config>/hooks/stop-checks.mjs "<check command>" [timeoutSeconds]
// The command runs in cmd.exe on Windows. Skips when HEAD and the uncommitted work are unchanged
// since the last pass. Blocks at most MAX_BLOCKS times in a row, then lets the agent stop and report.

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const MAX_BLOCKS = 2;
const [checkCmd, timeoutArg] = process.argv.slice(2);
const timeoutMs = (Number(timeoutArg) || 300) * 1000;

function git(args, cwd) {
  const r = spawnSync("git", args, { cwd, encoding: "utf8", timeout: 20000, maxBuffer: 64 * 1024 * 1024 });
  return r.status === 0 ? r.stdout : null;
}

function fingerprint(cwd) {
  const head = git(["rev-parse", "HEAD"], cwd);
  const diff = git(["diff", "HEAD"], cwd);
  const untracked = git(["ls-files", "--others", "--exclude-standard", "-z"], cwd);
  if (head === null || diff === null || untracked === null) return null;
  const h = createHash("sha256").update(checkCmd).update(head).update(diff);
  for (const f of untracked.split("\0").filter(Boolean)) {
    h.update(f);
    try {
      const p = join(cwd, f);
      if (statSync(p).size <= 5 * 1024 * 1024) h.update(readFileSync(p));
    } catch {}
  }
  return h.digest("hex");
}

function readState(file) {
  try {
    return JSON.parse(readFileSync(file, "utf8"));
  } catch {
    return {};
  }
}

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (d) => (raw += d));
process.stdin.on("end", () => {
  let input = {};
  try {
    input = JSON.parse(raw || "{}");
  } catch {}
  if (!checkCmd) process.exit(0);
  const cwd = input.cwd || process.cwd();

  const dir = join(tmpdir(), "agent-config-stop-checks");
  mkdirSync(dir, { recursive: true });
  const stateFile = join(dir, createHash("sha256").update(cwd.toLowerCase()).digest("hex").slice(0, 16) + ".json");
  const state = readState(stateFile);
  const fp = fingerprint(cwd);
  if (fp && state.passed === fp) process.exit(0);
  // A fresh stop (not one we caused) resets the block counter.
  const blocks = input.stop_hook_active ? state.blocks || 0 : 0;

  const r = spawnSync(checkCmd, { cwd, shell: true, encoding: "utf8", timeout: timeoutMs, maxBuffer: 64 * 1024 * 1024 });
  if (r.status === 0) {
    writeFileSync(stateFile, JSON.stringify({ passed: fp, blocks: 0 }));
    process.exit(0);
  }
  const out = `${r.stdout || ""}${r.stderr || ""}`.trim().split(/\r?\n/).slice(-40).join("\n");
  const why = r.error?.code === "ETIMEDOUT" ? `timed out after ${timeoutMs / 1000}s` : `exit ${r.status}`;
  if (blocks >= MAX_BLOCKS) {
    writeFileSync(stateFile, JSON.stringify({ blocks: 0 }));
    process.exit(0); // give up blocking; the agent has already been told twice
  }
  writeFileSync(stateFile, JSON.stringify({ blocks: blocks + 1 }));
  process.stdout.write(
    JSON.stringify({
      decision: "block",
      reason: `Project checks failed (${checkCmd}, ${why}). Fix them, or tell the user why they cannot pass.\n${out}`,
    })
  );
  process.exit(0);
});
