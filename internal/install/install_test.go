package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/profile"
)

const userSettings = `{
  "zeta": "kept — with non-ASCII",
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "node", "args": ["C:\\old\\.agent-config\\hooks\\git-guard.mjs"]}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "my-own-hook"}]}
    ]
  },
  "alpha": 1
}
`

func setup(t *testing.T, tier string, tools string) (Options, string) {
	t.Helper()
	root := "../.."
	assets := os.DirFS(root)
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	pdir := t.TempDir()
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".git", "info"), 0o755)
	for _, f := range []string{"AGENTS.md", "claude/model-settings.json", "codex/default.toml", "codex/deep.config.toml"} {
		b, err := os.ReadFile(filepath.Join(root, "profiles", "home", f))
		if err != nil {
			t.Fatal(err)
		}
		os.MkdirAll(filepath.Dir(filepath.Join(pdir, f)), 0o755)
		os.WriteFile(filepath.Join(pdir, f), b, 0o644)
	}
	yaml := `description: test
answers: {use: personal, tools: [` + tools + `]}
items: [instructions-base, git-guard, codex-rules, subagents, tdd, grilling, context7, pyright-lsp]
project_items:
  - project: '` + project + `'
    items: [spring-boot-skill]
instructions: AGENTS.md
claude_model_settings: claude/model-settings.json
codex_defaults: codex/default.toml
codex_profiles: [codex/deep.config.toml]
`
	os.WriteFile(filepath.Join(pdir, "profile.yaml"), []byte(yaml), 0o644)
	p, err := profile.Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "agtk.exe")
	os.WriteFile(exe, []byte("binary"), 0o755)
	return Options{Home: home, Assets: assets, Catalog: cat, Profile: p, Tier: tier, Exe: exe,
		SkipTools: true, Version: "test", Out: &bytes.Buffer{}}, project
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyMigrateRerunUndo(t *testing.T) {
	o, project := setup(t, "A", "claude_code, codex")
	sp := filepath.Join(o.Home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(sp), 0o755)
	os.WriteFile(sp, []byte(userSettings), 0o644)
	toml := filepath.Join(o.Home, ".codex", "config.toml")
	os.MkdirAll(filepath.Dir(toml), 0o755)
	origTOML := "model = \"old\"\n\n[projects.x]\ntrust_level = \"trusted\"\n"
	os.WriteFile(toml, []byte(origTOML), 0o644)

	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	s := read(t, sp)
	if strings.Contains(s, "git-guard.mjs") || !strings.Contains(s, "my-own-hook") || strings.Count(s, "git-guard") != 1 {
		t.Errorf("old guard must be replaced, other hooks kept:\n%s", s)
	}
	if strings.Index(s, `"zeta"`) > strings.Index(s, `"alpha"`) || !strings.Contains(s, "kept — with non-ASCII") {
		t.Errorf("user keys must keep order and text:\n%s", s)
	}
	if !strings.Contains(s, `"claude-opus-5-5"`) {
		t.Error("modelSettings missing")
	}
	c := read(t, toml)
	if !strings.Contains(c, `model = "gpt-6.1-sol"`) || !strings.Contains(c, "[projects.x]") || !strings.Contains(c, "[mcp_servers.context7]") {
		t.Errorf("config.toml:\n%s", c)
	}
	for _, p := range []string{
		".codex/AGENTS.md", ".claude/CLAUDE.md", ".codex/rules/agent-config.rules", ".codex/deep.config.toml",
		".claude/agents/scout.md", ".agents/skills/tdd/SKILL.md", ".claude/skills/tdd/SKILL.md", ".agtk/bin/agtk.exe",
	} {
		if _, err := os.Stat(filepath.Join(o.Home, p)); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	if g := read(t, filepath.Join(o.Home, ".agents/skills/grilling/SKILL.md")); !strings.Contains(g, "disable-model-invocation: true") {
		t.Error("explicit-only skill not hidden from Claude Code")
	}
	if _, err := os.Stat(filepath.Join(project, ".agents/skills/spring-boot-skill/SKILL.md")); err != nil {
		t.Error("project skill missing")
	}
	if ex := read(t, filepath.Join(project, ".git/info/exclude")); !strings.Contains(ex, ".agents/skills/") {
		t.Error("project skills not git-excluded")
	}
	var hooks struct {
		Hooks map[string][]struct{ Hooks []struct{ Command string } }
	}
	json.Unmarshal([]byte(read(t, filepath.Join(o.Home, ".codex/hooks.json"))), &hooks)
	if cmd := hooks.Hooks["PreToolUse"][0].Hooks[0].Command; !strings.HasSuffix(cmd, `agtk.exe" hook git-guard`) {
		t.Errorf("codex hook command %q", cmd)
	}
	var claudeHooks struct {
		Hooks map[string][]struct{ Hooks []struct{ OnFailure string } }
	}
	json.Unmarshal([]byte(read(t, filepath.Join(o.Home, ".claude/settings.json"))), &claudeHooks)
	if g := claudeHooks.Hooks["PreToolUse"]; len(g) == 0 || g[len(g)-1].Hooks[0].OnFailure != "block" {
		t.Error("claude git-guard hook must fail closed (onFailure: block)")
	}

	// A second run changes nothing, so its backup records nothing.
	before := Backups(StateDir(o.Home))
	o.Out = &bytes.Buffer{}
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if out := o.Out.(*bytes.Buffer).String(); strings.Contains(out, "install ") || strings.Contains(out, "/hooks") {
		t.Errorf("re-run should find everything in place:\n%s", out)
	}
	if after := Backups(StateDir(o.Home)); len(after) != len(before) {
		t.Errorf("a re-run that changes nothing must keep no backup: %v", after)
	}

	var w bytes.Buffer
	if err := Undo(StateDir(o.Home), &w); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadState(o.Home); err == nil {
		t.Error("state.json should be gone once its apply is undone")
	}
	if got := read(t, sp); got != userSettings {
		t.Errorf("settings.json not restored:\n%s", got)
	}
	if got := read(t, toml); got != origTOML {
		t.Errorf("config.toml not restored:\n%s", got)
	}
	for _, p := range []string{".codex/AGENTS.md", ".agents/skills/tdd", ".claude/skills/tdd", ".agtk/bin/agtk.exe"} {
		if exists(filepath.Join(o.Home, p)) {
			t.Errorf("%s should be gone after undo", p)
		}
	}
	if exists(filepath.Join(project, ".agents/skills/spring-boot-skill")) {
		t.Error("project skill should be gone after undo")
	}
}

func TestTierBAndToolChoice(t *testing.T) {
	o, _ := setup(t, "B", "claude_code")
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	s := read(t, filepath.Join(o.Home, ".claude", "settings.json"))
	if strings.Contains(s, `"hooks"`) || !strings.Contains(s, "Bash(git push:*)") {
		t.Errorf("tier B must use deny rules, not hooks:\n%s", s)
	}
	for _, p := range []string{".codex/hooks.json", ".codex/rules", ".codex/config.toml", ".agents/skills"} {
		if exists(filepath.Join(o.Home, p)) {
			t.Errorf("%s written although Codex is not used", p)
		}
	}
	if _, err := os.Stat(filepath.Join(o.Home, ".claude/skills/tdd/SKILL.md")); err != nil {
		t.Error("Claude-only user should get skills in ~/.claude/skills")
	}
}

func TestPreexistingSkillIsNotReplaced(t *testing.T) {
	o, _ := setup(t, "A", "claude_code, codex")
	mine := filepath.Join(o.Home, ".agents", "skills", "tdd")
	os.MkdirAll(mine, 0o755)
	os.WriteFile(filepath.Join(mine, "SKILL.md"), []byte("my own"), 0o644)
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "not installed by agtk") {
		t.Fatalf("want refusal, got %v", err)
	}
	if read(t, filepath.Join(mine, "SKILL.md")) != "my own" {
		t.Error("user's skill was changed")
	}
}

func TestReviewCases(t *testing.T) {
	o, project := setup(t, "A", "claude_code, codex")
	// config.toml with a BOM, as Windows PowerShell 5 writes it.
	toml := filepath.Join(o.Home, ".codex", "config.toml")
	os.MkdirAll(filepath.Dir(toml), 0o755)
	os.WriteFile(toml, []byte("\ufeffmodel = \"old\"\n"), 0o644)
	// A user hook that only mentions git-guard in a path must survive.
	sp := filepath.Join(o.Home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(sp), 0o755)
	os.WriteFile(sp, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"C:/tools/git-guard/mine.sh"}]}]}}`), 0o644)
	// The project is a worktree: .git is a file.
	os.RemoveAll(filepath.Join(project, ".git"))
	os.WriteFile(filepath.Join(project, ".git"), []byte("gitdir: /nonexistent\n"), 0o644)

	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if c := read(t, toml); strings.Count(c, "model =") != 1 || strings.Contains(c, "\ufeff") {
		t.Errorf("BOM config.toml: %q", c)
	}
	if s := read(t, sp); !strings.Contains(s, "mine.sh") || !strings.Contains(s, `"hook"`) {
		t.Errorf("user hook lost or ours missing:\n%s", s)
	}

	// Switching to Claude Code only: ~/.claude/skills/tdd is now a junction to replace.
	o.Profile.Answers.Tools = []string{"claude_code"}
	o.Profile.Items = append(o.Profile.Items, "diagnosing-bugs")
	o.Out = &bytes.Buffer{}
	os.WriteFile(filepath.Join(o.Home, ".agents", "skills", "tdd", ".agent-config-pin"), []byte("stale"), 0o644)
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(filepath.Join(o.Home, ".claude", "skills", "tdd")); err != nil || isLink(st) {
		t.Errorf("tdd should be a real folder for a Claude-only user: %v", err)
	}
	if _, err := os.Stat(filepath.Join(o.Home, ".agents", "skills", "tdd", "SKILL.md")); err != nil {
		t.Error("replacing the junction must not delete its target")
	}
	var w bytes.Buffer
	if err := Undo(StateDir(o.Home), &w); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(filepath.Join(o.Home, ".claude", "skills", "tdd")); err != nil || !isLink(st) {
		t.Errorf("undo should restore the junction: %v\n%s", err, w.String())
	}
}

func TestSetTOMLTopKeys(t *testing.T) {
	in := "# c\r\nmodel = \"a\"\r\n[t]\r\nmodel = \"keep\"\r\n"
	out, changes := setTOMLTopKeys(in, []string{"model", "model_reasoning_effort"},
		map[string]string{"model": `"b"`, "model_reasoning_effort": `"medium"`})
	want := "model_reasoning_effort = \"medium\"\r\n# c\r\nmodel = \"b\"\r\n[t]\r\nmodel = \"keep\"\r\n"
	if out != want || len(changes) != 2 {
		t.Errorf("got %q %v", out, changes)
	}
}
