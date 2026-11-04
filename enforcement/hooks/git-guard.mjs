// PreToolUse guard for Claude Code (Bash, PowerShell) and Codex (Bash).
// Denies git push, git reset --hard and git clean (dry runs allowed).
// A best-effort tripwire, not a sandbox: git aliases, scripts written to disk
// and commands computed at run time get past it.

const SEP = new Set([";", "&&", "||", "|", "&", "(", ")", "{", "}", "\n"]);
// Words that may stand before a command without changing which program runs.
const PREFIX = new Set(["sudo", "env", "command", "exec", "nohup", "time", "xargs", "call", "start", "."]);
const GIT_OPTS_WITH_ARG = new Set(["-c", "-C", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env"]);

// Splits like a shell: adjacent quoted and bare pieces join into one word (git p"ush" -> push).
function tokenize(s) {
  const tokens = [];
  const re = /((?:"(?:\\.|[^"\\])*"|'[^']*'|[^\s;&|(){}"'])+)|(&&|\|\||[;&|(){}\n])/g;
  let m;
  while ((m = re.exec(s))) {
    if (m[2]) {
      tokens.push({ text: m[2], sep: true });
      continue;
    }
    const word = m[1];
    const text = word.replace(/"((?:\\.|[^"\\])*)"|'([^']*)'/g, (_, d, q) => (d ?? q));
    tokens.push({ text, quoted: /^["']/.test(word), sep: false });
  }
  return tokens;
}

const isGit = (t) => /(^|[\\/])git(\.exe)?$/i.test(t);
const isShellFlag = (t) => /^-[a-z]*c$/i.test(t) || /^\/c$/i.test(t) || /^-com(mand)?$/i.test(t) || /^(iex|invoke-expression)$/i.test(t);
const isEncodedFlag = (t) => /^-e(c|nc|ncodedcommand)?$/i.test(t);

function cleanIsDryRun(rest) {
  for (let i = 0; i < rest.length; i++) {
    const a = rest[i];
    if (a === "--") break;
    if (a === "-e" || a === "--exclude") { i++; continue; }
    if (a.startsWith("--exclude=")) continue;
    if (a === "--dry-run") return true;
    if (/^-[a-zA-Z]+$/.test(a)) {
      const flags = a.slice(1).split("e")[0]; // after -e comes its value
      if (flags.includes("n")) return true;
    }
  }
  return false;
}

function check(command, depth = 0) {
  if (depth > 3) return null;
  const tokens = tokenize(command);
  let cmdPos = true; // is the current token where a command name goes?
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i];
    if (t.sep) { cmdPos = true; continue; }
    const prev = tokens[i - 1];
    // Re-scan strings handed to another shell: sh -c, bash -lc, powershell -Command, cmd /c, iex.
    if (prev && !prev.sep && isShellFlag(prev.text) && t.quoted) {
      const hit = check(t.text, depth + 1);
      if (hit) return hit;
    }
    if (prev && !prev.sep && isEncodedFlag(prev.text) && /^[A-Za-z0-9+/=]{8,}$/.test(t.text)) {
      const hit = check(Buffer.from(t.text, "base64").toString("utf16le"), depth + 1);
      if (hit) return hit;
    }
    if (!cmdPos) continue;
    if (PREFIX.has(t.text.toLowerCase()) || /^\w+=/.test(t.text) || t.text.startsWith("-")) continue;
    cmdPos = false;
    if (!isGit(t.text)) continue;
    const args = [];
    for (let j = i + 1; j < tokens.length && !tokens[j].sep; j++) args.push(tokens[j].text);
    let k = 0;
    while (k < args.length && args[k].startsWith("-")) {
      const opt = args[k].split("=")[0];
      k += GIT_OPTS_WITH_ARG.has(opt) && !args[k].includes("=") ? 2 : 1;
    }
    const sub = (args[k] || "").toLowerCase();
    const rest = args.slice(k + 1);
    if (sub === "push") return "git push";
    if (sub === "reset" && rest.includes("--hard")) return "git reset --hard";
    if (sub === "clean" && !cleanIsDryRun(rest)) return "git clean";
  }
  return null;
}

function deny(reason) {
  process.stdout.write(
    JSON.stringify({
      hookSpecificOutput: { hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: reason },
    })
  );
  process.exit(0);
}

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (d) => (raw += d));
process.stdin.on("end", () => {
  let input;
  try {
    input = JSON.parse(raw);
  } catch {
    deny("agent-config git-guard could not read the hook input, so it blocked the command. Ask the user.");
  }
  let cmd = input?.tool_input?.command;
  if (Array.isArray(cmd)) cmd = cmd.join(" ");
  if (typeof cmd !== "string") process.exit(0); // not a shell call
  const hit = check(cmd);
  if (hit) deny(`Blocked by agent-config git-guard: ${hit} is not allowed for agents. Ask the user to run it themselves.`);
  process.exit(0);
});
