// Package install applies a profile to Claude Code and Codex. It only adds and updates:
// it never deletes skills, agents, hooks or plugins it did not install. Every changed file
// is backed up first, so `agtk undo` can restore it. Safe to re-run.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/ojson"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
)

type Options struct {
	Home        string // user home; tests point it at a scratch folder
	Assets      fs.FS
	Catalog     *catalog.Catalog
	Profile     *profile.Profile
	Policy      policy.Policy
	Tier        string // A or B
	Exe         string // this binary; copied to the state folder so hooks have a stable path
	SaveProfile bool   // write the selected profile through the same backup session
	SkipTools   bool   // don't run claude/npm commands (MCP add, plugins, npm packages)
	DryRun      bool   // change nothing; Plan reports what Apply would do
	Version     string
	Out         io.Writer
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
	gemini        string
	exe           string // installed agtk binary
	useClaude     bool
	useCodex      bool
	useGemini     bool // only when chosen: older profiles without the tool keep their files
	route         *routing.Setup
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

// Plan reports file changes and possible tool commands without executing those commands.
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
	if err := o.Policy.Check(o.Profile, o.Catalog, items); err != nil {
		return nil, err
	}
	a := &applier{Options: o, codex: filepath.Join(o.Home, ".codex"), claude: filepath.Join(o.Home, ".claude"), gemini: filepath.Join(o.Home, ".gemini")}
	tools := o.Profile.Answers.Tools
	a.useClaude = len(tools) == 0 || contains(tools, "claude_code")
	a.useCodex = len(tools) == 0 || contains(tools, "codex")
	a.useGemini = contains(tools, "gemini")
	route, skipped, err := o.Profile.Answers.ResolveRouting()
	if err != nil {
		return nil, err
	}
	a.route = route
	if a.s, err = newSession(StateDir(o.Home), o.DryRun); err != nil {
		return nil, err
	}
	if o.Profile.Path == "" {
		a.say("Profile: %s (one-time setup)", o.Profile.Description)
	} else {
		a.say("Profile: %s (%s)", o.Profile.Path, o.Profile.Description)
	}
	a.say("Tier: %s", o.Tier)
	if skipped {
		a.say("routing skipped: needs Claude Code")
	}
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
	if o.SaveProfile {
		b, err := yaml.Marshal(o.Profile)
		if err != nil {
			return nil, err
		}
		if err := a.writeFile(o.Profile.Path, b); err != nil {
			return nil, fmt.Errorf("save profile: %w", err)
		}
		a.say("Saved profile: %s", o.Profile.Path)
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
	a.say("Done (tier %s). Start new sessions in the clients you configured.", o.Tier)
	for _, f := range a.followUps {
		a.say("To do: %s", f)
	}
	a.pathHint()
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

const utf8BOM = "\xef\xbb\xbf"

// generatedNotice heads each instruction file agtk writes.
const generatedNotice = "<!-- Generated by agtk. Edit the profile's instructions and re-run `agtk apply`; changes made here are overwritten. -->"

// geminiHandoff is the inline handoff procedure in GEMINI.md: Gemini gets no skills from agtk.
// The handoff skill (packs/core/skills/handoff) says the same.
const geminiHandoff = "## Writing a handoff\n" +
	"To hand work to another agent (usage limit, switching tools): run `git status` and `git diff --stat`, then write `.agent-handoff/latest.md` " +
	"in the repo root with the sections Goal, Done, Left, Files touched, Decisions and gotchas, How to run checks, Next step, and a summary of that git output. " +
	"Make sure `.agent-handoff/` is in `.git/info/exclude`; never commit it or put secrets in it. " +
	"Then give me this line for the next agent: \"Read .agent-handoff/latest.md, then run git status and git diff, then continue with Next step.\"\n"

// InstructionTexts builds the Claude Code and Codex instruction files for legacy plan-only routing.
func InstructionTexts(parts []string, route *routing.Preset, plans []string) (claude, codex string) {
	var setup *routing.Setup
	if route != nil {
		setup = &routing.Setup{Preset: route, Plans: plans}
	}
	claude, codex, _ = SetupInstructionTexts(parts, setup)
	return claude, codex
}

// SetupInstructionTexts builds the three instruction files. GEMINI.md is the plain text
// of AGENTS.md (no routing table) plus the handoff procedure.
func SetupInstructionTexts(parts []string, route *routing.Setup) (claude, codex, gemini string) {
	trimmed := make([]string, len(parts))
	for i, p := range parts {
		trimmed[i] = strings.TrimRight(strings.TrimPrefix(p, utf8BOM), "\r\n \t")
	}
	codex = generatedNotice + "\n\n" + strings.Join(trimmed, "\n\n") + "\n"
	claude = codex
	gemini = codex + "\n" + geminiHandoff
	if section := route.Section(routing.ToolClaude); section != "" {
		claude = routing.Splice(claude, section)
	}
	if section := route.Section(routing.ToolCodex); section != "" {
		codex = routing.Splice(codex, section)
	}
	return claude, codex, gemini
}

func (a *applier) instructions(items []*catalog.Item) error {
	var parts []string
	for _, it := range has(items, catalog.KindInstructions) {
		b, err := fs.ReadFile(a.Assets, it.Vendored)
		if err != nil {
			return err
		}
		parts = append(parts, string(b))
	}
	if a.Profile.Instructions != "" {
		b, err := a.Profile.File(a.Profile.Instructions)
		if err != nil {
			return err
		}
		parts = append(parts, string(b))
	}
	if len(parts) == 0 {
		return nil
	}
	claudeText, codexText, geminiText := SetupInstructionTexts(parts, a.route)
	a.say("Instructions")
	if a.useClaude {
		if err := a.writeFile(filepath.Join(a.claude, "CLAUDE.md"), []byte(claudeText)); err != nil {
			return err
		}
		a.say("  ~/.claude/CLAUDE.md (%d tokens approx.)", catalog.Tokens(claudeText))
	}
	if a.useCodex {
		if err := a.writeFile(filepath.Join(a.codex, "AGENTS.md"), []byte(codexText)); err != nil {
			return err
		}
		a.say("  ~/.codex/AGENTS.md (%d tokens approx.)", catalog.Tokens(codexText))
	}
	if a.useGemini {
		if err := a.writeFile(filepath.Join(a.gemini, "GEMINI.md"), []byte(geminiText)); err != nil {
			return err
		}
		a.say("  ~/.gemini/GEMINI.md (%d tokens approx.)", catalog.Tokens(geminiText))
	}
	if a.route != nil {
		a.say("  model routing guidance: preset %s", a.route.Preset.ID)
	}
	return nil
}

func (a *applier) codexConfig(items []*catalog.Item) error {
	if !a.useCodex {
		return nil
	}
	a.say("Codex")
	if c := a.route.Default(routing.ToolCodex); c.Model != "" && a.Profile.CodexDefaults == "" {
		vals := map[string]string{"model": strconv.Quote(c.Model), "model_reasoning_effort": ""}
		if c.Effort != "" {
			vals["model_reasoning_effort"] = strconv.Quote(c.Effort)
		}
		if err := a.editTOML(func(text string) string {
			text, _ = setTOMLTopKeys(text, []string{"model", "model_reasoning_effort"}, vals)
			return text
		}); err != nil {
			return err
		}
		a.say("  model from routing: %s", c.String())
	}
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
	guard := false
	for _, it := range has(items, catalog.KindHook) {
		guard = guard || it.ID == "git-guard"
	}
	codexHook := guard && a.useCodex && a.Tier == "A"
	if !a.useClaude {
		if codexHook {
			return a.codexHook()
		}
		return nil
	}

	for _, it := range has(items, catalog.KindAgents) {
		files, _ := fs.Glob(a.Assets, path.Join(it.Vendored, "*.md"))
		var names []string
		for _, f := range files {
			b, _ := fs.ReadFile(a.Assets, f)
			name := strings.TrimSuffix(path.Base(f), ".md")
			if c, ok := a.agentRouting(name); ok {
				b = []byte(routing.SetFrontmatter(string(b), c.Model, c.Effort))
			} else if a.Policy.Company(a.Profile.Answers) || (a.route != nil && a.route.Models != nil) {
				b = []byte(routing.SetFrontmatter(string(b), "inherit", ""))
			}
			if err := a.writeFile(filepath.Join(a.claude, "agents", path.Base(f)), b); err != nil {
				return err
			}
			names = append(names, name)
		}
		a.say("Claude Code subagents: %s", strings.Join(names, ", "))
	}
	model := a.route.Default(routing.ToolClaude).Model
	if !guard && a.Profile.ClaudeModelSettings == "" && model == "" {
		return nil
	}

	sp := filepath.Join(a.claude, "settings.json")
	s, err := a.readJSON(sp)
	if err != nil {
		return err
	}
	if model != "" {
		// Other models' entries, and other keys of these entries, are kept.
		s.Set("model", model)
		var desc []string
		for _, me := range a.route.ClaudeEfforts() {
			s.Child("modelSettings").Child(me.Model).Set("effortLevel", me.Effort)
			desc = append(desc, me.Model+" "+me.Effort)
		}
		a.say("  model: %s (preset %s)", model, a.route.Preset.ID)
		a.say("  modelSettings from preset: %s", strings.Join(desc, ", "))
	}
	if a.Profile.ClaudeModelSettings != "" {
		// modelSettings is merged per model id; entries not in the profile are kept.
		b, err := a.Profile.File(a.Profile.ClaudeModelSettings)
		if err != nil {
			return err
		}
		ms, err := ojson.Parse(b)
		if err != nil {
			return fmt.Errorf("%s: %w", a.Profile.ClaudeModelSettings, err)
		}
		if a.route != nil {
			diffs, err := routing.ModelDifferences(a.route.ClaudeEfforts(), b)
			if err != nil {
				return fmt.Errorf("%s: %w", a.Profile.ClaudeModelSettings, err)
			}
			for _, d := range diffs {
				a.say("  note: profile model-settings sets %s to %s, preset %s says %s; profile wins", d.Model, d.Profile, a.route.Preset.ID, d.Preset)
			}
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

	if guard {
		a.say("Enforcement")
		if a.Tier == "A" {
			// Fail closed when the hook cannot start or times out.
			setHookGroup(s, "PreToolUse", "git-guard", ojson.Obj("matcher", "Bash|PowerShell", "hooks", []any{
				ojson.Obj("type", "command", "command", a.exe, "args", []string{"hook", "git-guard"}, "timeout", 10, "onFailure", "block")}))
			a.say("  Claude Code: git-guard PreToolUse hook")
		} else {
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
	if err := a.writeJSON(sp, s); err != nil {
		return err
	}
	if codexHook {
		return a.codexHook()
	}
	return nil
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
		origin, short := "agtk", "authored" // an authored skill has no upstream pin
		if !it.Authored {
			origin, short = it.Source.Repo+"@"+it.Source.Commit, it.Source.Repo+"@"+it.Source.Commit[:7]
		}
		want := fmt.Sprintf("%s explicit_only=%s files=%s", origin, psBool(it.ExplicitOnly), hash)
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
			a.say("  install  %s  (%s%s)", it.ID, short, note)
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

var errPlanProbe = errors.New("tool state is checked during apply")

// run checks for a program on PATH and only executes it during apply.
func (a *applier) run(name string, args ...string) (out string, ok bool, err error) {
	if a.SkipTools {
		return "", false, nil
	}
	p, lerr := exec.LookPath(name)
	if lerr != nil {
		return "", false, nil
	}
	if a.DryRun {
		return "", true, errPlanProbe
	}
	b, err := exec.Command(p, args...).CombinedOutput()
	return string(b), true, err
}

// change runs a command that changes another program's setup, and records it for the
// backup (undo can't reverse it) or, in a dry run, only records it.
func (a *applier) change(name string, args ...string) error {
	describe := strings.Join(append([]string{name}, args...), " ")
	if err := a.s.external(describe); err != nil {
		return err
	}
	if !a.DryRun {
		if _, ok, err := a.run(name, args...); err != nil {
			return fmt.Errorf("%s: %w", describe, err)
		} else if !ok {
			return fmt.Errorf("%s disappeared from PATH during apply", name)
		}
	}
	return nil
}

func (a *applier) claudeMCP(it *catalog.Item) error {
	if _, ok, err := a.run("claude", "mcp", "get", it.ID); !ok {
		a.say("  Claude Code MCP: skipped %s (claude not run)", it.ID)
		return nil
	} else if err == nil {
		a.say("  Claude Code MCP: ok %s", it.ID)
		return nil
	}
	if err := a.change("claude", "mcp", "add", "--scope", "user", "--transport", "http", it.ID, it.MCP.URL); err != nil {
		return err
	}
	a.say("  Claude Code MCP: added %s", it.ID)
	return nil
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
	if err := a.change("npm", "install", "-g", pkg, "--no-fund", "--no-audit"); err != nil {
		return err
	}
	a.say("  npm: installed %s", pkg)
	return nil
}

func (a *applier) claudePlugin(it *catalog.Item) error {
	mk := it.Plugin.ID[strings.Index(it.Plugin.ID, "@")+1:]
	out, ok, _ := a.run("claude", "plugin", "marketplace", "list")
	if !ok {
		a.say("  Claude plugin: skipped %s (claude not run)", it.Plugin.ID)
		return nil
	}
	if !strings.Contains(out, mk) {
		if err := a.change("claude", "plugin", "marketplace", "add", it.Plugin.Marketplace); err != nil {
			return err
		}
	}
	if out, _, _ := a.run("claude", "plugin", "list"); strings.Contains(out, it.Plugin.ID) {
		a.say("  Claude plugin: ok %s", it.Plugin.ID)
		return nil
	}
	if err := a.change("claude", "plugin", "install", it.Plugin.ID); err != nil {
		return err
	}
	a.say("  Claude plugin: installed %s", it.Plugin.ID)
	return nil
}

// agentRouting is the preset's model and effort for a subagent, when routing is on.
func (a *applier) agentRouting(name string) (routing.Choice, bool) {
	if a.route == nil {
		return routing.Choice{}, false
	}
	return a.route.Agent(name)
}
