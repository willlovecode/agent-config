// Package hooks holds the hook programs Claude Code and Codex call as `agtk hook <name>`.
package hooks

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf16"
)

// Git guard: a PreToolUse hook for Claude Code (Bash, PowerShell) and Codex (Bash).
// Denies git push, git reset --hard and git clean (dry runs allowed).
// A best-effort tripwire, not a sandbox: git aliases, scripts written to disk
// and commands computed at run time get past it.

// Words that may stand before a command without changing which program runs.
var guardPrefix = map[string]bool{"sudo": true, "env": true, "command": true, "exec": true, "nohup": true,
	"time": true, "xargs": true, "call": true, "start": true, ".": true}

var gitOptsWithArg = map[string]bool{"-c": true, "-C": true, "--git-dir": true, "--work-tree": true,
	"--namespace": true, "--exec-path": true, "--config-env": true}

type token struct {
	text   string
	quoted bool
	sep    bool
}

var (
	// JavaScript's \s: Go's \s lacks \v and the Unicode spaces PowerShell also splits on.
	tokenRe   = regexp.MustCompile(`((?:"(?:\\.|[^"\\])*"|'[^']*'|[^\s\v\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff};&|(){}"'])+)|(&&|\|\||[;&|(){}\n])`)
	quoteRe   = regexp.MustCompile(`"((?:\\.|[^"\\])*)"|'([^']*)'`)
	gitRe     = regexp.MustCompile(`(?i)(^|[\\/])git(\.exe)?$`)
	shellRe   = regexp.MustCompile(`(?i)^(-[a-z]*c|/c|-com(mand)?|iex|invoke-expression)$`)
	encodedRe = regexp.MustCompile(`(?i)^-e(c|nc|ncodedcommand)?$`)
	base64Re  = regexp.MustCompile(`^[A-Za-z0-9+/=]{8,}$`)
	assignRe  = regexp.MustCompile(`^\w+=`)
	shortRe   = regexp.MustCompile(`^-[a-zA-Z]+$`)
)

// tokenize splits like a shell: adjacent quoted and bare pieces join into one word (git p"ush" -> push).
func tokenize(s string) []token {
	var out []token
	for _, m := range tokenRe.FindAllStringSubmatch(s, -1) {
		if m[2] != "" {
			out = append(out, token{text: m[2], sep: true})
			continue
		}
		word := m[1]
		text := quoteRe.ReplaceAllStringFunc(word, func(q string) string { return q[1 : len(q)-1] })
		out = append(out, token{text: text, quoted: word[0] == '"' || word[0] == '\''})
	}
	return out
}

func cleanIsDryRun(rest []string) bool {
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			return false
		case a == "-e" || a == "--exclude":
			i++
		case strings.HasPrefix(a, "--exclude="):
		case a == "--dry-run":
			return true
		case shortRe.MatchString(a):
			flags, _, _ := strings.Cut(a[1:], "e") // after -e comes its value
			if strings.Contains(flags, "n") {
				return true
			}
		}
	}
	return false
}

func decodeUTF16LE(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// CheckGit returns what a command would run that the guard forbids ("git push", ...), or "".
func CheckGit(command string) string { return checkGit(command, 0) }

func checkGit(command string, depth int) string {
	if depth > 3 {
		return ""
	}
	tokens := tokenize(command)
	cmdPos := true // is the current token where a command name goes?
	wrapper := false
	for i, t := range tokens {
		if t.sep {
			cmdPos, wrapper = true, false
			continue
		}
		if i > 0 && !tokens[i-1].sep {
			prev := tokens[i-1].text
			// Re-scan strings handed to another shell: sh -c, bash -lc, powershell -Command, cmd /c, iex.
			if (shellRe.MatchString(prev) || wrapper) && t.quoted {
				if hit := checkGit(t.text, depth+1); hit != "" {
					return hit
				}
			}
			if encodedRe.MatchString(prev) && base64Re.MatchString(t.text) {
				if b, err := base64.StdEncoding.DecodeString(t.text); err == nil {
					if hit := checkGit(decodeUTF16LE(b), depth+1); hit != "" {
						return hit
					}
				}
			}
		}
		if !cmdPos && !wrapper {
			continue
		}
		if cmdPos {
			if guardPrefix[strings.ToLower(t.text)] || assignRe.MatchString(t.text) || strings.HasPrefix(t.text, "-") {
				continue
			}
			cmdPos = false
			if wrapperRe.MatchString(t.text) {
				// wsl, ssh, docker...: the command comes after options of their own, so
				// every later word of this segment may be the program that runs.
				wrapper = true
				continue
			}
		}
		isDel := delRe.MatchString(t.text)
		if !isDel && !gitRe.MatchString(t.text) {
			continue
		}
		var args []string
		for j := i + 1; j < len(tokens) && !tokens[j].sep; j++ {
			args = append(args, tokens[j].text)
		}
		if isDel {
			if hit := checkDelete(args); hit != "" {
				return hit
			}
			continue
		}
		k := 0
		for k < len(args) && strings.HasPrefix(args[k], "-") {
			opt, _, hasValue := strings.Cut(args[k], "=")
			if gitOptsWithArg[opt] && !hasValue {
				k += 2
			} else {
				k++
			}
		}
		sub, rest := "", []string(nil)
		if k < len(args) {
			sub, rest = strings.ToLower(args[k]), args[k+1:]
		}
		switch {
		case sub == "push":
			return "git push"
		case sub == "reset" && contains(rest, "--hard"):
			return "git reset --hard"
		case sub == "clean" && !cleanIsDryRun(rest):
			return "git clean"
		}
	}
	return ""
}

// Delete guard: recursive deletes are denied when a target is not a plain, literal path inside
// a project: a variable or substitution (it can expand to something else than intended, or
// to nothing, and wsl.exe even expands it once more before a script runs), or a place whose
// loss is not recoverable (a root, a home directory, a drive, a system folder, a bare * or .).
var (
	delRe     = regexp.MustCompile(`(?i)(^|[\\/])(rm|rmdir|rd|del|erase|ri|remove-item)(\.exe)?$`)
	wrapperRe = regexp.MustCompile(`(?i)(^|[\\/])(wsl|ssh|docker|podman|kubectl|nsenter|chroot)(\.exe)?$`)
	// rm -rf, rm -r, rm --recursive, Remove-Item -Recurse (and any other flag with an r: a false
	// positive only matters when a target is risky), cmd's rd /s and del /s.
	recurseRe   = regexp.MustCompile(`^(-[A-Za-z]*[rR][A-Za-z]*|--recursive|/[sS])$`)
	cmdFlagRe   = regexp.MustCompile(`^/[A-Za-z]$`)
	percentRe   = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)
	protectedRe = regexp.MustCompile(`^(` +
		`|\.|\.\.|\*|\.\*|~|~[^/]*` + // empty = a filesystem root
		`|/(home|root|usr|etc|var|bin|sbin|lib|lib64|opt|mnt|boot|srv|dev|proc|sys|users|windows)` +
		`|/home/[^/]+|/mnt/[a-z]|/mnt/[a-z]/(users(/[^/]+)?|windows|programdata|program files( \(x86\))?)` +
		`|[a-z]:|[a-z]:/(users(/[^/]+)?|windows|programdata|program files( \(x86\))?)` +
		`)$`)
)

// checkDelete returns a description of a forbidden recursive delete, or "".
func checkDelete(args []string) string {
	recursive := false
	var targets []string
	for _, a := range args {
		switch {
		case recurseRe.MatchString(a):
			recursive = true
		case strings.HasPrefix(a, "-") || cmdFlagRe.MatchString(a):
		default:
			targets = append(targets, a)
		}
	}
	if !recursive {
		return ""
	}
	for _, t := range targets {
		if strings.ContainsAny(t, "$`") || percentRe.MatchString(t) {
			return fmt.Sprintf("a recursive delete of %q, whose path contains a variable or substitution", t)
		}
		p := strings.ToLower(strings.ReplaceAll(t, `\`, "/"))
		for {
			q := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(p, "/"), "/*"), "/.")
			if q == p {
				break
			}
			p = q
		}
		if protectedRe.MatchString(p) {
			return fmt.Sprintf("a recursive delete of %q, a root, home, drive or system folder", t)
		}
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// GitGuard reads a PreToolUse hook input and writes a deny decision when the command is forbidden.
func GitGuard(in io.Reader, out io.Writer) {
	deny := func(reason string) {
		writeJSON(out, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": reason}})
	}
	var input struct {
		ToolInput struct {
			Command json.RawMessage `json:"command"`
		} `json:"tool_input"`
	}
	raw, err := io.ReadAll(in)
	if err == nil {
		err = json.Unmarshal(raw, &input)
	}
	if err != nil {
		deny("agtk git-guard could not read the hook input, so it blocked the command. Ask the user.")
		return
	}
	var cmd string
	var parts []any
	candidates := []string{}
	if json.Unmarshal(input.ToolInput.Command, &cmd) == nil {
		candidates = append(candidates, cmd)
	} else if json.Unmarshal(input.ToolInput.Command, &parts) == nil {
		// An argv array: check it joined, and each element alone, since
		// ["bash", "-lc", "git push"] hands its last element to a shell.
		var words []string
		for _, p := range parts {
			words = append(words, fmt.Sprint(p))
		}
		candidates = append(append(candidates, strings.Join(words, " ")), words...)
	} else {
		return // not a shell call
	}
	for _, c := range candidates {
		if hit := CheckGit(c); hit != "" {
			if strings.HasPrefix(hit, "a recursive delete") {
				deny("Blocked by agtk git-guard: " + hit + ". Print the resolved path first (echo, ls), " +
					"then delete only that literal path inside the project or scratch folder, or ask the user.")
				return
			}
			deny("Blocked by agtk git-guard: " + hit + " is not allowed for agents. Ask the user to run it themselves.")
			return
		}
	}
}

// writeJSON writes v without HTML escaping, so commands in messages stay readable.
func writeJSON(w io.Writer, v any) {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	e.Encode(v)
}
