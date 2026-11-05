// Package install applies a profile to Claude Code and Codex. It only adds and updates:
// it never deletes skills, agents, hooks or plugins it did not install. Every changed file
// is backed up first, so `agtk undo` can restore it. Safe to re-run.
package install

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/ojson"
	"github.com/willlovecode/agent-config/internal/profile"
)

type Options struct {
	Home      string // user home; tests point it at a scratch folder
	Assets    fs.FS
	Catalog   *catalog.Catalog
	Profile   *profile.Profile
	Tier      string // A or B
	Exe       string // this binary; copied to the state folder so hooks have a stable path
	SkipTools bool   // don't run claude/npm commands (MCP add, plugins, npm packages)
	DryRun    bool   // change nothing; Plan reports what Apply would do
	Version   string
	Out       io.Writer
}

// StateDir is where agtk keeps its own binary, backups and state.
func StateDir(home string) string { return filepath.Join(home, ".agtk") }

type State struct {
	Profile   string    `json:"profile"`
	Tier      string    `json:"tier"`
	Version   string    `json:"version"`
	AppliedAt time.Time `json:"applied_at"`
	Backup    string    `json:"backup"`
	Hook      string    `json:"hook_command,omitempty"`
}

func ReadState(home string) (*State, error) {
	b, err := os.ReadFile(filepath.Join(StateDir(home), "state.json"))
	if err != nil {
		return nil, err
	}
	var s State
	return &s, json.Unmarshal(b, &s)
}

type applier struct {
	Options
	s             *session
	codex, claude string
	exe           string // installed agtk binary
	useClaude     bool
	useCodex      bool
	followUps     []string
}

func (a *applier) say(format string, args ...any) { fmt.Fprintf(a.Out, format+"\n", args...) }

// Apply installs the profile. On error the files changed so far stay backed up, and
// `agtk undo` restores them.
func Apply(o Options) error {
	o.DryRun = false
	_, err := run(o)
	return err
}

// Change is one thing Apply would do: create or change a path, or run a command.
type Change struct {
	Action string // create, change, run
	What   string
}

// Plan reports what Apply would change, without writing anything.
func Plan(o Options) ([]Change, error) {
	o.DryRun = true
	a, err := run(o)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, e := range a.s.Entries {
		switch {
		case e.Kind == "external":
			out = append(out, Change{"run", e.Describe})
		case e.Existed:
			out = append(out, Change{"change", e.Path})
		default:
			out = append(out, Change{"create", e.Path})
		}
	}
	return out, nil
}

func run(o Options) (*applier, error) {
	items, err := o.Profile.Resolve(o.Catalog)
	if err != nil {
		return nil, err
	}
	a := &applier{Options: o, codex: filepath.Join(o.Home, ".codex"), claude: filepath.Join(o.Home, ".claude")}
	tools := o.Profile.Answers.Tools
	a.useClaude = len(tools) == 0 || contains(tools, "claude_code")
	a.useCodex = len(tools) == 0 || contains(tools, "codex")
	if a.s, err = newSession(StateDir(o.Home), o.DryRun); err != nil {
		return nil, err
	}
	a.say("Profile: %s (%s)", o.Profile.Path, o.Profile.Description)
	a.say("Tier: %s", o.Tier)
	if !o.DryRun {
		a.say("Backup: %s", a.s.dir)
	}

	steps := []func([]*catalog.Item) error{a.binary, a.instructions, a.codexConfig, a.claudeConfig, a.skills, a.tools}
	for _, step := range steps {
		if err := step(items); err != nil {
			return nil, err
		}
	}
	if err := a.projectSkills(); err != nil {
		return nil, err
	}
	if o.DryRun {
		return a, nil
	}
	backup := a.s.finish()
	if backup == "" {
		a.say("Nothing changed; no backup kept.")
		if prev, err := ReadState(o.Home); err == nil {
			backup = prev.Backup // still the one undo restores
		}
	}
	st := State{Profile: o.Profile.Path, Tier: o.Tier, Version: o.Version, AppliedAt: time.Now().UTC(), Backup: backup, Hook: a.exe}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(filepath.Join(StateDir(o.Home), "state.json"), b, 0o644); err != nil {
		return nil, err
	}
	a.say("Done (tier %s). Start new Claude Code and Codex sessions to pick up the changes.", o.Tier)
	for _, f := range a.followUps {
		a.say("To do: %s", f)
	}
	return a, nil
}

func has(items []*catalog.Item, kind string) []*catalog.Item {
	var out []*catalog.Item
	for _, it := range items {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

// binary copies agtk into ~/.agtk/bin so hook commands keep working when the download moves.
func (a *applier) binary(items []*catalog.Item) error {
	name := "agtk"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	a.exe = filepath.Join(StateDir(a.Home), "bin", name)
	if len(has(items, catalog.KindHook)) == 0 {
		return nil
	}
	self, err := filepath.EvalSymlinks(a.Exe)
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Clean(self), filepath.Clean(a.exe)) {
		return nil
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(a.exe); err == nil && string(old) == string(data) {
		return nil
	}
	if err := a.s.remember(a.exe, "file"); err != nil {
		return err
	}
	a.say("Binary\n  %s", a.exe)
	if a.DryRun {
		return nil
	}
	return replaceFile(a.exe, data, 0o755)
}

func (a *applier) instructions(items []*catalog.Item) error {
	var parts []string
	for _, it := range has(items, catalog.KindInstructions) {
		b, err := fs.ReadFile(a.Assets, it.Vendored)
		if err != nil {
			return err
		}
		parts = append(parts, strings.TrimRight(string(b), "\r\n \t"))
	}
	if a.Profile.Instructions != "" {
		b, err := a.Profile.File(a.Profile.Instructions)
		if err != nil {
			return err
		}
		parts = append(parts, strings.TrimRight(string(b), "\r\n \t"))
	}
	if len(parts) == 0 {
		return nil
	}
	text := strings.Join(parts, "\n\n") + "\n"
	a.say("Instructions")
	agents := filepath.Join(a.codex, "AGENTS.md")
	if err := a.writeFile(agents, []byte(text)); err != nil {
		return err
	}
	a.say("  ~/.codex/AGENTS.md (%d tokens approx.)", catalog.Tokens(text))
	if a.useClaude {
		if err := a.writeFile(filepath.Join(a.claude, "CLAUDE.md"), []byte("@"+slash(agents)+"\n")); err != nil {
			return err
		}
		a.say("  ~/.claude/CLAUDE.md imports %s", slash(agents))
	}
	return nil
}

func (a *applier) codexConfig(items []*catalog.Item) error {
	if !a.useCodex {
		return nil
	}
	a.say("Codex")
	if a.Profile.CodexDefaults != "" {
		b, err := a.Profile.File(a.Profile.CodexDefaults)
		if err != nil {
			return err
		}
		keys, vals := tomlTopValues(string(b), "model", "model_reasoning_effort")
		if err := a.editTOML(func(text string) string {
			text, changes := setTOMLTopKeys(text, keys, vals)
			for _, c := range changes {
				a.say("  config.toml: %s", c)
			}
			return text
		}); err != nil {
			return err
		}
	}
	for _, p := range a.Profile.CodexProfiles {
		b, err := a.Profile.File(p)
		if err != nil {
			return err
		}
		if err := a.writeFile(filepath.Join(a.codex, filepath.Base(p)), b); err != nil {
			return err
		}
		a.say("  profile %s", filepath.Base(p))
	}
	for _, it := range has(items, catalog.KindRules) {
		b, err := fs.ReadFile(a.Assets, it.Vendored)
		if err != nil {
			return err
		}
		if err := a.writeFile(filepath.Join(a.codex, "rules", path.Base(it.Vendored)), b); err != nil {
			return err
		}
		a.say("  execpolicy rules: %s", it.Summary)
	}
	return nil
}

func (a *applier) editTOML(edit func(string) string) error {
	p := filepath.Join(a.codex, "config.toml")
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// A BOM (Windows PowerShell 5 writes one) would hide the first key from the edit.
	return a.writeFile(p, []byte(edit(strings.TrimPrefix(string(b), "\xef\xbb\xbf"))))
}

func (a *applier) claudeConfig(items []*catalog.Item) error {
	a.say("Claude Code")
	if a.useClaude {
		for _, it := range has(items, catalog.KindAgents) {
			files, _ := fs.Glob(a.Assets, path.Join(it.Vendored, "*.md"))
			var names []string
			for _, f := range files {
				b, _ := fs.ReadFile(a.Assets, f)
				if err := a.writeFile(filepath.Join(a.claude, "agents", path.Base(f)), b); err != nil {
					return err
				}
				names = append(names, strings.TrimSuffix(path.Base(f), ".md"))
			}
			a.say("  subagents: %s", strings.Join(names, ", "))
		}
	}
	sp := filepath.Join(a.claude, "settings.json")
	s, err := a.readJSON(sp)
	if err != nil {
		return err
	}
	if a.useClaude && a.Profile.ClaudeModelSettings != "" {
		// modelSettings is merged per model id; entries not in the profile are kept.
		b, err := a.Profile.File(a.Profile.ClaudeModelSettings)
		if err != nil {
			return err
		}
		ms, err := ojson.Parse(b)
		if err != nil {
			return fmt.Errorf("%s: %w", a.Profile.ClaudeModelSettings, err)
		}
		dst := s.Child("modelSettings")
		var desc []string
		for _, k := range ms.Keys() {
			v, _ := ms.Get(k)
			dst.Set(k, v)
			if vo, ok := v.(*ojson.Object); ok {
				e, _ := vo.Get("effortLevel")
				desc = append(desc, fmt.Sprintf("%s %v", k, e))
			}
		}
		a.say("  modelSettings: %s", strings.Join(desc, ", "))
	}

	var guard *catalog.Item
	for _, it := range has(items, catalog.KindHook) {
		if it.ID == "git-guard" {
			guard = it
		}
	}
	if guard != nil {
		a.say("Enforcement")
		if a.Tier == "A" {
			if a.useClaude {
				// A hook that can't start (blocked exe, missing file, timeout) lets the call through
				// unless onFailure is "block" (Claude Code 2.1.295+).
				setHookGroup(s, "PreToolUse", "git-guard", ojson.Obj("matcher", "Bash|PowerShell", "hooks", []any{
					ojson.Obj("type", "command", "command", a.exe, "args", []string{"hook", "git-guard"}, "timeout", 10, "onFailure", "block")}))
				a.say("  Claude Code: git-guard PreToolUse hook")
			}
			if a.useCodex {
				if err := a.codexHook(); err != nil {
					return err
				}
			}
		} else if a.useClaude {
			// No hooks: deny rules in Claude Code's permissions instead (prefix match, weaker).
			perms := s.Child("permissions")
			deny := perms.List("deny")
			for _, r := range []string{"Bash(git push:*)", "Bash(git reset --hard:*)", "PowerShell(git push:*)", "PowerShell(git reset --hard:*)"} {
				if !containsAny(deny, r) {
					deny = append(deny, r)
				}
			}
			perms.Set("deny", deny)
			a.say("  Claude Code: permissions.deny for git push / reset --hard (no hooks in tier B)")
		}
	}
	if !a.useClaude && !exists(sp) {
		return nil
	}
	return a.writeJSON(sp, s)
}

func (a *applier) codexHook() error {
	hp := filepath.Join(a.codex, "hooks.json")
	h, err := a.readJSON(hp)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("%q hook git-guard", slash(a.exe))
	before := ojson.Mentions(h, cmd)
	setHookGroup(h, "PreToolUse", "git-guard", ojson.Obj("matcher", "^Bash$", "hooks", []any{
		ojson.Obj("type", "command", "command", cmd, "timeout", 10, "statusMessage", "git-guard")}))
	if err := a.writeJSON(hp, h); err != nil {
		return err
	}
	a.say("  Codex: git-guard PreToolUse hook")
	if !before {
		a.followUps = append(a.followUps, "open Codex and run /hooks to trust the git-guard hook (Codex skips untrusted hooks silently)")
	}
	return nil
}

// setHookGroup replaces the hook groups under event that mention marker, or appends one.
// A group is ours when it runs hook `name`: the old .mjs script, or agtk with args
// ["hook", name] (Claude Code) or a command line "... hook name" (Codex).
func setHookGroup(settings *ojson.Object, event, name string, group any) {
	hooks := settings.Child("hooks")
	var keep []any
	for _, g := range hooks.List(event) {
		ours := ojson.Mentions(g, name+".mjs") || ojson.Mentions(g, "hook "+name) || ojson.ContainsJSON(g, `"hook","`+name+`"`)
		if !ours {
			keep = append(keep, g)
		}
	}
	hooks.Set(event, append(keep, group))
}

func containsAny(l []any, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

var allowedToolsRe = regexp.MustCompile(`(?m)^allowed-tools:.*\r?\n`)
var frontStartRe = regexp.MustCompile(`\A---\r?\n`)

func (a *applier) skills(items []*catalog.Item) error {
	list := has(items, catalog.KindSkill)
	if len(list) == 0 {
		return nil
	}
	a.say("Skills")
	return a.installSkills(list, a.Home)
}

// installSkills puts shared skills in <root>/.agents/skills (Codex reads it) with a
// junction per skill in <root>/.claude/skills; Claude-only skills go there directly.
func (a *applier) installSkills(list []*catalog.Item, root string) error {
	shared := filepath.Join(root, ".agents", "skills")
	claude := filepath.Join(root, ".claude", "skills")
	for _, it := range list {
		toCodex := it.Supports("codex") && a.useCodex
		toClaude := it.Supports("claude_code") && a.useClaude
		if !toCodex && !toClaude {
			continue
		}
		home := claude
		if toCodex {
			home = shared
		}
		dest := filepath.Join(home, it.ID)
		pin := filepath.Join(dest, ".agent-config-pin")
		hash, err := treeHash(a.Assets, it.Vendored)
		if err != nil {
			return err
		}
		want := fmt.Sprintf("%s@%s explicit_only=%s files=%s", it.Source.Repo, it.Source.Commit, psBool(it.ExplicitOnly), hash)
		if have, err := os.ReadFile(pin); err == nil && strings.TrimSpace(string(have)) == want {
			a.say("  ok       %s", it.ID)
		} else {
			if exists(dest) && !exists(pin) {
				return fmt.Errorf("%s: %s exists but was not installed by agtk; move it away first", it.ID, dest)
			}
			if err := a.s.remember(dest, "dir"); err != nil {
				return err
			}
			if !a.DryRun {
				if err := os.RemoveAll(dest); err != nil {
					return err
				}
				if err := copyFromAssets(a.Assets, it.Vendored, dest); err != nil {
					return err
				}
				if err := adjustSkill(dest, it.ExplicitOnly); err != nil {
					return err
				}
				if err := os.WriteFile(pin, []byte(want), 0o644); err != nil {
					return err
				}
			}
			note := ""
			if it.ExplicitOnly {
				note = ", explicit-only"
			}
			a.say("  install  %s  (%s@%s%s)", it.ID, it.Source.Repo, it.Source.Commit[:7], note)
		}
		if toClaude && home == shared {
			link := filepath.Join(claude, it.ID)
			if !exists(link) {
				if !a.DryRun {
					if err := os.MkdirAll(claude, 0o755); err != nil {
						return err
					}
				}
				if err := a.link(link, dest); err != nil {
					return err
				}
				a.say("  linked   %s -> Claude Code", it.ID)
			}
		}
	}
	return nil
}

func psBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// adjustSkill strips pre-approved tools (never wanted from a third-party skill) and, for
// explicit-only skills, hides them from automatic invocation in both tools.
func adjustSkill(dir string, explicitOnly bool) error {
	md := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(md)
	if err != nil {
		return err
	}
	text := allowedToolsRe.ReplaceAllString(string(b), "")
	if explicitOnly && !regexp.MustCompile(`(?m)^disable-model-invocation:`).MatchString(text) {
		if loc := frontStartRe.FindStringIndex(text); loc != nil {
			text = "---\ndisable-model-invocation: true\n" + text[loc[1]:]
		}
	}
	if err := os.WriteFile(md, []byte(text), 0o644); err != nil {
		return err
	}
	if !explicitOnly {
		return nil
	}
	yml := filepath.Join(dir, "agents", "openai.yaml")
	y, _ := os.ReadFile(yml)
	if strings.Contains(string(y), "allow_implicit_invocation") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(yml), 0o755); err != nil {
		return err
	}
	return os.WriteFile(yml, append(y, []byte("policy:\n  allow_implicit_invocation: false\n")...), 0o644)
}

func (a *applier) projectSkills() error {
	for _, pi := range a.Profile.ProjectItems {
		if _, err := os.Stat(pi.Project); err != nil {
			a.say("Project %s not found, skipped", pi.Project)
			continue
		}
		a.say("Project skills in %s", pi.Project)
		var list []*catalog.Item
		for _, id := range pi.Items {
			it, _ := a.Catalog.Get(id)
			list = append(list, it)
		}
		if err := a.installSkills(list, pi.Project); err != nil {
			return err
		}
		if err := a.gitExclude(pi.Project, ".agents/skills/", ".claude/skills/"); err != nil {
			return err
		}
	}
	return nil
}

// gitExclude adds lines to the project's .git/info/exclude so nothing personal gets committed.
func (a *applier) gitExclude(project string, lines ...string) error {
	st, err := os.Lstat(filepath.Join(project, ".git"))
	if err != nil {
		return nil // not a git checkout
	}
	p := filepath.Join(project, ".git", "info", "exclude")
	if !st.IsDir() {
		// A worktree or submodule: .git is a file pointing at the real git folder.
		out, err := exec.Command("git", "-C", project, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude").Output()
		if err != nil {
			a.say("  could not find the git folder of %s; add %s to its exclude file by hand", project, strings.Join(lines, ", "))
			return nil
		}
		p = filepath.FromSlash(strings.TrimSpace(string(out)))
	}
	b, _ := os.ReadFile(p)
	have := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	text := string(b)
	for _, l := range lines {
		if !contains(have, l) {
			if text != "" && !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			text += l + "\n"
		}
	}
	return a.writeFile(p, []byte(text))
}

// tools registers MCP servers and plugins with the agent programs themselves.
func (a *applier) tools(items []*catalog.Item) error {
	mcps, plugins := has(items, catalog.KindMCP), has(items, catalog.KindPlugin)
	if len(mcps)+len(plugins) == 0 {
		return nil
	}
	a.say("Tools")
	for _, it := range mcps {
		if a.useCodex && it.Supports("codex") {
			// Codex: a [mcp_servers.<id>] table with an allowlist of tools.
			header := "[mcp_servers." + it.ID + "]"
			err := a.editTOML(func(text string) string {
				if regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(header)).MatchString(text) {
					a.say("  Codex MCP: ok %s", it.ID)
					return text
				}
				nl := "\n"
				if strings.Contains(text, "\r\n") {
					nl = "\r\n"
				}
				var quoted []string
				for _, t := range it.MCP.CodexEnabledTools {
					quoted = append(quoted, fmt.Sprintf("%q", t))
				}
				a.say("  Codex MCP: added %s", it.ID)
				return text + nl + header + nl + fmt.Sprintf("url = %q", it.MCP.URL) + nl +
					"enabled_tools = [" + strings.Join(quoted, ", ") + "]" + nl
			})
			if err != nil {
				return err
			}
		}
		if a.useClaude && it.Supports("claude_code") {
			if err := a.claudeMCP(it); err != nil {
				return err
			}
		}
	}
	for _, it := range plugins {
		for _, pkg := range it.NPM {
			if err := a.npmGlobal(pkg); err != nil {
				return err
			}
		}
		if a.useClaude {
			if err := a.claudePlugin(it); err != nil {
				return err
			}
		}
	}
	return nil
}

// run executes an outside program; ok is false when it is missing or tools are skipped.
func (a *applier) run(name string, args ...string) (out string, ok bool, err error) {
	if a.SkipTools {
		return "", false, nil
	}
	p, lerr := exec.LookPath(name)
	if lerr != nil {
		return "", false, nil
	}
	b, err := exec.Command(p, args...).CombinedOutput()
	return string(b), true, err
}

// change runs a command that changes another program's setup, and records it for the
// backup (undo can't reverse it) or, in a dry run, only records it.
func (a *applier) change(describe, name string, args ...string) error {
	if !a.DryRun {
		if out, _, err := a.run(name, args...); err != nil {
			return fmt.Errorf("%s %s: %v %s", name, strings.Join(args, " "), err, out)
		}
	}
	return a.s.external(describe)
}

func (a *applier) claudeMCP(it *catalog.Item) error {
	if _, ok, err := a.run("claude", "mcp", "get", it.ID); !ok {
		a.say("  Claude Code MCP: skipped %s (claude not run)", it.ID)
		return nil
	} else if err == nil {
		a.say("  Claude Code MCP: ok %s", it.ID)
		return nil
	}
	a.say("  Claude Code MCP: added %s", it.ID)
	return a.change("claude mcp add --scope user "+it.ID+" (remove with: claude mcp remove --scope user "+it.ID+")",
		"claude", "mcp", "add", "--scope", "user", "--transport", "http", it.ID, it.MCP.URL)
}

func (a *applier) npmGlobal(pkg string) error {
	name := pkg[:strings.LastIndex(pkg, "@")]
	out, ok, _ := a.run("npm", "ls", "-g", "--depth=0", name)
	if !ok {
		a.say("  npm: skipped %s (npm not run)", pkg)
		return nil
	}
	if strings.Contains(out, pkg) {
		a.say("  npm: ok %s", pkg)
		return nil
	}
	a.say("  npm: installed %s", pkg)
	return a.change("npm install -g "+pkg, "npm", "install", "-g", pkg, "--no-fund", "--no-audit")
}

func (a *applier) claudePlugin(it *catalog.Item) error {
	mk := it.Plugin.ID[strings.Index(it.Plugin.ID, "@")+1:]
	out, ok, _ := a.run("claude", "plugin", "marketplace", "list")
	if !ok {
		a.say("  Claude plugin: skipped %s (claude not run)", it.Plugin.ID)
		return nil
	}
	if !strings.Contains(out, mk) {
		if err := a.change("claude plugin marketplace add "+it.Plugin.Marketplace,
			"claude", "plugin", "marketplace", "add", it.Plugin.Marketplace); err != nil {
			return err
		}
	}
	if out, _, _ := a.run("claude", "plugin", "list"); strings.Contains(out, it.Plugin.ID) {
		a.say("  Claude plugin: ok %s", it.Plugin.ID)
		return nil
	}
	a.say("  Claude plugin: installed %s", it.Plugin.ID)
	return a.change("claude plugin install "+it.Plugin.ID, "claude", "plugin", "install", it.Plugin.ID)
}
