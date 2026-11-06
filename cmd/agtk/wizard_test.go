package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/wizard"
)

func TestAnswersPreserveProfile(t *testing.T) {
	root := t.TempDir()
	home, config := filepath.Join(root, "home"), filepath.Join(root, "config")
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	answers := filepath.Join(root, "answers.yaml")
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	input := profile.Profile{
		Description:  "Fixture",
		Answers:      profile.Answers{Use: "personal", Tools: []string{"codex"}},
		Items:        []string{"instructions-base"},
		Instructions: "overlay.md",
		ProjectItems: []profile.ProjectItems{{Project: project, Items: []string{"spring-boot-skill"}}},
	}
	b, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	write(answers, string(b))
	write(filepath.Join(root, input.Instructions), "Fixture profile instructions.\n")

	if err := runWizard([]string{"--answers", answers, "--save", "--skip-tools"}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(home, ".codex", "AGENTS.md")
	if b, err := os.ReadFile(out); err != nil || !strings.Contains(string(b), "Fixture profile instructions.") {
		t.Fatalf("profile instructions missing from %s: %v", out, err)
	}
	skill := filepath.Join(project, ".agents", "skills", "spring-boot-skill", "SKILL.md")
	if _, err := os.Stat(skill); err != nil {
		t.Fatal(err)
	}
	path, err := wizard.ProfilePath()
	if err != nil {
		t.Fatal(err)
	}
	state, err := install.ReadState(home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profile != path {
		t.Fatalf("remembered %q, want %q", state.Profile, path)
	}
	if err := runApply([]string{"--skip-tools"}); err != nil {
		t.Fatalf("reapplying the saved profile: %v", err)
	}

	write(answers, "answers:\n  tools: [codex]\nitems: []\n")
	emptyHome := filepath.Join(root, "empty")
	t.Setenv("USERPROFILE", emptyHome)
	t.Setenv("HOME", emptyHome)
	if err := runWizard([]string{"--answers", answers, "--skip-tools"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(emptyHome, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("explicit empty item list should create no Codex config, got %v", err)
	}
}

func TestAnswersFileModelRouting(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	answers := filepath.Join(root, "answers.yaml")
	apply := func(name, content string) string {
		t.Helper()
		home := filepath.Join(root, name)
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOME", home)
		if err := os.WriteFile(answers, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runWizard([]string{"--answers", answers, "--skip-tools"}); err != nil {
			t.Fatal(err)
		}
		return home
	}

	home := apply("routed", "answers:\n  tools: [claude_code]\n  plans: [claude_pro, copilot]\n  routing: saver\n")
	settings, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil || !strings.Contains(string(settings), `"model": "claude-sonnet-5-5"`) {
		t.Fatalf("saver should set the default model: %v\n%s", err, settings)
	}
	review, err := os.ReadFile(filepath.Join(home, ".claude", "agents", "review.md"))
	if err != nil || !strings.Contains(string(review), "model: claude-sonnet-5-5\neffort: high") {
		t.Fatalf("saver reviews with Sonnet at high: %v\n%s", err, review)
	}
	claudeMD, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil || !strings.Contains(string(claudeMD), "Preset saver") || !strings.Contains(string(claudeMD), "Copilot Sonnet 5.5") {
		t.Fatalf("routing table missing: %v\n%s", err, claudeMD)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Errorf("a Claude-only setup must not create ~/.codex: %v", err)
	}

	// The same file without routing, as written before it existed, changes no model.
	home = apply("plain", "answers:\n  tools: [claude_code]\n")
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); err == nil && strings.Contains(string(b), `"model"`) {
		t.Errorf("no routing in the answers, but a model was set:\n%s", b)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil || strings.Contains(string(b), "Model routing") {
		t.Errorf("no routing in the answers, but CLAUDE.md has a table: %v", err)
	}

	// "none" is accepted explicitly and does what a missing routing does.
	home = apply("none", "answers:\n  tools: [claude_code]\n  plans: [claude_pro]\n  routing: none\n")
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); err == nil && strings.Contains(string(b), `"model"`) {
		t.Errorf("routing none, but a model was set:\n%s", b)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil || strings.Contains(string(b), "Model routing") {
		t.Errorf("routing none, but CLAUDE.md has a table: %v", err)
	}
	orig, err := os.ReadFile(filepath.Join("..", "..", "packs", "core", "agents", "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "agents", "review.md")); err != nil || string(b) != string(orig) {
		t.Errorf("routing none, but the review agent was rewritten: %v", err)
	}

	// Routing without Claude Code is dropped: nothing under ~/.claude, no table in AGENTS.md.
	home = apply("codex", "answers:\n  tools: [codex]\n  plans: [copilot]\n  routing: saver\n")
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Errorf("a Codex-only setup must not create ~/.claude: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md")); err != nil || strings.Contains(string(b), "Model routing") {
		t.Errorf("Codex-only AGENTS.md must have no table: %v", err)
	}
	path, err := wizard.ProfilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := runWizard([]string{"--answers", answers, "--skip-tools", "--save"}); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(path); err != nil || strings.Contains(string(saved), "routing") {
		t.Errorf("the saved profile must not keep routing without Claude Code: %v\n%s", err, saved)
	}

	home = apply("confirmed", `answers:
  tools: [codex]
  routing: quality
  plans: [chatgpt_plus]
  models:
    - {plan: chatgpt_plus, tool: codex, id: gpt-6.1-sol, class: standard, efforts: [medium]}
`)
	if b, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), `model = "gpt-6.1-sol"`) {
		t.Errorf("confirmed Codex model was not installed: %v\n%s", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md")); err != nil || !strings.Contains(string(b), "Codex GPT-6.1 Sol medium") {
		t.Errorf("confirmed Codex routing table is missing: %v\n%s", err, b)
	}
}

func TestCompanyAnswersCannotEnableExternalMCP(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	path := filepath.Join(root, "company.yaml")
	b := []byte("answers: {use: company, tools: [codex], allow_external_mcp: true}\nitems: [context7]\n")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, run := range []func() error{
		func() error { return runWizard([]string{"--answers", path, "--save", "--skip-tools"}) },
		func() error { return runApply([]string{"--profile", path, "--skip-tools"}) },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "IT approval") {
			t.Fatalf("unapproved Context7 must be blocked: %v", err)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("blocked setup should not create the home directory: %v", err)
	}
}

func TestAnswersFileGemini(t *testing.T) {
	root := t.TempDir()
	config, home := filepath.Join(root, "config"), filepath.Join(root, "home")
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	answers := filepath.Join(root, "answers.yaml")
	content := "answers:\n  tools: [claude_code, gemini]\n  plans: [claude_pro, copilot]\n  routing: saver\nitems: [instructions-base, handoff]\n"
	if err := os.WriteFile(answers, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runWizard([]string{"--answers", answers, "--skip-tools"}); err != nil {
		t.Fatal(err)
	}
	gemini, err := os.ReadFile(filepath.Join(home, ".gemini", "GEMINI.md"))
	if err != nil || strings.Contains(string(gemini), "Model routing") || !strings.Contains(string(gemini), ".agent-handoff/latest.md") {
		t.Fatalf("GEMINI.md: want the handoff and no table: %v\n%s", err, gemini)
	}
	claude, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil || !strings.Contains(string(claude), "run the handoff skill") {
		t.Fatalf("CLAUDE.md routing section lacks the handoff line: %v\n%s", err, claude)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "handoff", "SKILL.md")); err != nil {
		t.Errorf("handoff skill not installed for Claude Code: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Errorf("codex was not chosen: %v", err)
	}
}
