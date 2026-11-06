package install

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/routing"
)

var allPlans = []string{"claude_pro", "copilot", "gemini_pro", "chatgpt"}

// frontmatter splits an agent file into its header lines and everything after the closing ---.
func frontmatter(t *testing.T, text string) (head, body string) {
	t.Helper()
	parts := strings.SplitN(text, "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("no frontmatter:\n%s", text)
	}
	return parts[0], parts[1]
}

func asset(t *testing.T, name string) string {
	t.Helper()
	return read(t, filepath.Join("..", "..", "packs", "core", "agents", name))
}

func routed(t *testing.T, tools, preset string) Options {
	t.Helper()
	o, _ := setup(t, "A", tools)
	o.Profile.Answers.Routing, o.Profile.Answers.Plans = preset, allPlans
	return o
}

func TestRoutingWritesModelsAgentsAndTable(t *testing.T) {
	o := routed(t, "claude_code, codex", "balanced")
	o.Profile.ClaudeModelSettings = "" // the preset alone; TestProfileModelSettingsFileWinsOverPreset covers both
	sp := filepath.Join(o.Home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(sp), 0o755)
	os.WriteFile(sp, []byte(`{
  "model": "old",
  "note": "kept — with an em dash",
  "modelSettings": {
    "claude-other-1": {"effortLevel": "low"},
    "claude-sonnet-5-5": {"effortLevel": "low", "extra": true}
  }
}
`), 0o644)
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}

	s := read(t, sp)
	for _, want := range []string{`"model": "claude-sonnet-5-5"`, `"claude-other-1": {`, `"extra": true`, "kept — with an em dash"} {
		if !strings.Contains(s, want) {
			t.Errorf("settings.json lacks %q:\n%s", want, s)
		}
	}
	var parsed struct{ ModelSettings map[string]map[string]any }
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatal(err)
	}
	for model, want := range map[string]string{"claude-opus-5-5": "medium", "claude-sonnet-5-5": "high", "claude-haiku-5-5": "medium", "claude-other-1": "low"} {
		if got := parsed.ModelSettings[model]["effortLevel"]; got != want {
			t.Errorf("%s effort %v, want %s", model, got, want)
		}
	}
	if strings.Contains(s, "â€") || strings.Contains(s, `\u2014`) {
		t.Errorf("settings.json text was re-encoded:\n%s", s)
	}
	if strings.Index(s, `"model"`) > strings.Index(s, `"note"`) {
		t.Errorf("model must keep its place:\n%s", s)
	}

	for name, want := range map[string]string{
		"scout.md": "model: claude-haiku-5-5\neffort: medium", "mechanic.md": "model: claude-haiku-5-5\neffort: medium",
		"implement.md": "model: claude-sonnet-5-5\neffort: high", "review.md": "model: claude-opus-5-5\neffort: medium",
	} {
		got := read(t, filepath.Join(o.Home, ".claude", "agents", name))
		gotHead, gotBody := frontmatter(t, got)
		origHead, origBody := frontmatter(t, asset(t, name))
		if !strings.Contains(gotHead, want) {
			t.Errorf("%s header:\n%s", name, gotHead)
		}
		if gotBody != origBody {
			t.Errorf("%s: body changed", name)
		}
		// Only the model and effort lines may differ.
		g, w := strings.Split(gotHead, "\n"), strings.Split(origHead, "\n")
		if len(g) != len(w) {
			t.Fatalf("%s: header lines %d, was %d", name, len(g), len(w))
		}
		for i := range g {
			if g[i] != w[i] && !strings.HasPrefix(g[i], "model:") && !strings.HasPrefix(g[i], "effort:") {
				t.Errorf("%s: line %q became %q", name, w[i], g[i])
			}
		}
	}

	claudeMD := read(t, filepath.Join(o.Home, ".claude", "CLAUDE.md"))
	if strings.Count(claudeMD, routing.Begin) != 1 || !strings.Contains(claudeMD, "Preset balanced") ||
		!strings.Contains(claudeMD, "Antigravity (AI Pro)") || !strings.Contains(claudeMD, "Codex GPT-6.1 Sol xhigh") {
		t.Errorf("CLAUDE.md routing section:\n%s", claudeMD)
	}
	if !strings.Contains(claudeMD, "Model choice") {
		t.Error("the profile's own instructions must stay")
	}
	cfg := read(t, filepath.Join(o.Home, ".codex", "config.toml"))
	if !strings.Contains(cfg, `model = "gpt-6.1-sol"`) {
		t.Errorf("codex defaults must still apply:\n%s", cfg)
	}

	// Re-running changes nothing; a different preset replaces the section and the models.
	before := Backups(StateDir(o.Home))
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if after := Backups(StateDir(o.Home)); len(after) != len(before) {
		t.Errorf("a re-run that changes nothing must keep no backup: %v", after)
	}
	o.Profile.Answers.Routing = "quality"
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	claudeMD = read(t, filepath.Join(o.Home, ".claude", "CLAUDE.md"))
	if strings.Count(claudeMD, routing.Begin) != 1 || strings.Contains(claudeMD, "Preset balanced") || !strings.Contains(claudeMD, "Preset quality") {
		t.Errorf("section not replaced:\n%s", claudeMD)
	}
	if s := read(t, sp); !strings.Contains(s, `"model": "claude-opus-5-5"`) {
		t.Errorf("quality default model:\n%s", s)
	}
	// Quality gives routine work to Opus at medium; balanced had Sonnet at high.
	if h, _ := frontmatter(t, read(t, filepath.Join(o.Home, ".claude", "agents", "implement.md"))); !strings.Contains(h, "model: claude-opus-5-5\neffort: medium") {
		t.Errorf("quality implement agent:\n%s", h)
	}
}

func TestProfileModelSettingsFileWinsOverPreset(t *testing.T) {
	o := routed(t, "claude_code", "saver")
	settings := func() map[string]struct{ EffortLevel string } {
		t.Helper()
		var s struct {
			ModelSettings map[string]struct{ EffortLevel string }
		}
		if err := json.Unmarshal([]byte(read(t, filepath.Join(o.Home, ".claude", "settings.json"))), &s); err != nil {
			t.Fatal(err)
		}
		return s.ModelSettings
	}

	// The profile's file agrees with the preset: nothing to report.
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if out := o.Out.(*bytes.Buffer).String(); strings.Contains(out, "note:") {
		t.Errorf("file and preset agree, but apply made a note:\n%s", out)
	}

	// It disagrees for one model: the file wins, and apply says so.
	file := filepath.Join(o.Profile.Dir, "claude", "model-settings.json")
	os.WriteFile(file, []byte(`{
  "claude-opus-5-5": { "effortLevel": "medium" },
  "claude-sonnet-5-5": { "effortLevel": "low" },
  "claude-haiku-5-5": { "effortLevel": "medium" }
}`), 0o644)
	o.Out.(*bytes.Buffer).Reset()
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if got := settings()["claude-sonnet-5-5"].EffortLevel; got != "low" {
		t.Errorf("profile modelSettings should win over the preset, sonnet is at %q", got)
	}
	out := o.Out.(*bytes.Buffer).String()
	want := "note: profile model-settings sets claude-sonnet-5-5 to low, preset saver says high; profile wins"
	if !strings.Contains(out, want) || strings.Count(out, "note:") != 1 {
		t.Errorf("want exactly the line %q in:\n%s", want, out)
	}

	// Without routing there is no preset to differ from.
	o.Profile.Answers.Routing = ""
	o.Out.(*bytes.Buffer).Reset()
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(o.Out.(*bytes.Buffer).String(), "note:") {
		t.Error("no routing, but apply compared the file with a preset")
	}
}

func TestNoRoutingLeavesModelsAlone(t *testing.T) {
	for _, answer := range []string{"", routing.None} {
		o, _ := setup(t, "A", "claude_code, codex")
		o.Profile.Answers.Routing, o.Profile.Answers.Plans = answer, allPlans
		checkModelsUntouched(t, o)
	}
}

func checkModelsUntouched(t *testing.T, o Options) {
	t.Helper()
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if s := read(t, filepath.Join(o.Home, ".claude", "settings.json")); strings.Contains(s, `"model":`) {
		t.Errorf("no routing, but settings.json sets a model:\n%s", s)
	}
	if got := read(t, filepath.Join(o.Home, ".claude", "agents", "scout.md")); got != asset(t, "scout.md") {
		t.Error("no routing, but a subagent file changed")
	}
	for _, f := range []string{".claude/CLAUDE.md", ".codex/AGENTS.md"} {
		if a := read(t, filepath.Join(o.Home, f)); strings.Contains(a, routing.Begin) {
			t.Errorf("no routing, but %s has a routing section", f)
		}
	}
}

func TestRoutingIsClaudeCodeOnly(t *testing.T) {
	o := routed(t, "codex", "balanced")
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(o.Home, ".claude")) {
		t.Error("Codex-only setup wrote into ~/.claude")
	}
	if a := read(t, filepath.Join(o.Home, ".codex", "AGENTS.md")); strings.Contains(a, routing.Begin) {
		t.Error("the table routes Claude Code; Codex-only setups don't get it")
	}
	if out := o.Out.(*bytes.Buffer).String(); !strings.Contains(out, "routing skipped: needs Claude Code") {
		t.Errorf("apply should say routing was skipped:\n%s", out)
	}
	// Names are still checked, and a Claude plan is not needed when nothing is routed.
	o = routed(t, "codex", "balanced")
	o.Profile.Answers.Plans = []string{"copilot"}
	if err := Apply(o); err != nil {
		t.Errorf("Codex-only needs no Claude plan: %v", err)
	}
	o.Profile.Answers.Plans = []string{"free"}
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "free") {
		t.Errorf("unknown plan without Claude Code: %v", err)
	}
}

func TestRoutingRejectsBadAnswers(t *testing.T) {
	o := routed(t, "claude_code", "turbo")
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "turbo") {
		t.Errorf("unknown preset: %v", err)
	}
	o = routed(t, "claude_code", "balanced")
	o.Profile.Answers.Plans = []string{"free"}
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "free") {
		t.Errorf("unknown plan: %v", err)
	}
	o.Profile.Answers.Routing = "" // names are checked even when nothing is routed
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "free") {
		t.Errorf("unknown plan without routing: %v", err)
	}
	o = routed(t, "claude_code", "balanced")
	o.Profile.Answers.Plans = []string{"copilot", "chatgpt"}
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "claude_pro or claude_max") {
		t.Errorf("no Claude plan: %v", err)
	}
	if exists(filepath.Join(o.Home, ".claude")) {
		t.Error("a rejected profile must write nothing")
	}
}

func TestGooglePlanChangesTheFallbackRow(t *testing.T) {
	o := routed(t, "claude_code", "balanced")
	table := func() string {
		t.Helper()
		if err := Apply(o); err != nil {
			t.Fatal(err)
		}
		return read(t, filepath.Join(o.Home, ".claude", "CLAUDE.md"))
	}
	if got := table(); !strings.Contains(got, "Antigravity (AI Pro)") || strings.Contains(got, "Gemini CLI") {
		t.Errorf("gemini_pro:\n%s", got)
	}
	// Switching plans is one edit of the list and a re-run.
	o.Profile.Answers.Plans = []string{"claude_pro", "copilot", "gemini_plus", "chatgpt"}
	if got := table(); !strings.Contains(got, "Antigravity (confirm access)") || strings.Contains(got, "Gemini CLI") || strings.Count(got, routing.Begin) != 1 {
		t.Errorf("gemini_plus:\n%s", got)
	}
	// The old name still works and means gemini_pro.
	o.Profile.Answers.Plans = []string{"claude_pro", "gemini"}
	if got := table(); !strings.Contains(got, "Antigravity (AI Pro)") {
		t.Errorf("legacy gemini:\n%s", got)
	}
}

func TestInstructionFilesAreSeparateAndStandalone(t *testing.T) {
	o := routed(t, "claude_code, codex", "balanced")
	// An earlier install left CLAUDE.md as an import of the Codex file.
	cm := filepath.Join(o.Home, ".claude", "CLAUDE.md")
	os.MkdirAll(filepath.Dir(cm), 0o755)
	os.WriteFile(cm, []byte("@"+slash(filepath.Join(o.Home, ".codex", "AGENTS.md"))+"\n"), 0o644)
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	claudeMD, agentsMD := read(t, cm), read(t, filepath.Join(o.Home, ".codex", "AGENTS.md"))
	for name, text := range map[string]string{"CLAUDE.md": claudeMD, "AGENTS.md": agentsMD} {
		if !strings.HasPrefix(text, "<!-- Generated by agtk.") || !strings.Contains(strings.SplitN(text, "\n", 2)[0], "agtk apply") {
			t.Errorf("%s lacks the generated-file notice:\n%.200s", name, text)
		}
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "@") {
				t.Errorf("%s imports another file: %q", name, line)
			}
		}
		if strings.HasPrefix(text, "\xef\xbb\xbf") {
			t.Errorf("%s must be UTF-8 without a BOM", name)
		}
		if !strings.Contains(text, "Model choice") || !strings.Contains(text, "Environment (home)") {
			t.Errorf("%s misses the base or profile text", name)
		}
	}
	if !strings.Contains(claudeMD, routing.Begin) || strings.Contains(agentsMD, routing.Begin) || strings.Contains(agentsMD, "Model routing") {
		t.Error("the routing table belongs in CLAUDE.md only")
	}
	if claudeMD != agentsMD && !strings.HasPrefix(claudeMD, strings.TrimSuffix(agentsMD, "\n")) {
		t.Error("CLAUDE.md should be the Codex text plus the table")
	}
	if backups := Backups(StateDir(o.Home)); len(backups) != 1 {
		t.Errorf("backups: %v", backups)
	}

	// Idempotent: a second run changes nothing.
	before := Backups(StateDir(o.Home))
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if after := Backups(StateDir(o.Home)); len(after) != len(before) {
		t.Errorf("re-run kept a backup: %v", after)
	}
	if read(t, cm) != claudeMD {
		t.Error("CLAUDE.md changed on re-run")
	}

	// Undo restores the old import.
	if err := Undo(StateDir(o.Home), io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := read(t, cm); !strings.HasPrefix(got, "@") || exists(filepath.Join(o.Home, ".codex", "AGENTS.md")) {
		t.Errorf("undo should restore the one-line import and remove AGENTS.md: %q", got)
	}
}

func TestEachToolGetsOnlyItsOwnFolder(t *testing.T) {
	o := routed(t, "claude_code", "balanced")
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(o.Home, ".codex")) {
		t.Error("Claude-only setup created ~/.codex")
	}
	if !strings.Contains(read(t, filepath.Join(o.Home, ".claude", "CLAUDE.md")), routing.Begin) {
		t.Error("Claude-only CLAUDE.md lacks the table")
	}
}

func TestInstructionBytesStayIntact(t *testing.T) {
	o := routed(t, "claude_code, codex", "balanced")
	// A profile text with non-ASCII, CRLF-free, and a BOM in front of it.
	extra := filepath.Join(o.Profile.Dir, "extra.md")
	os.WriteFile(extra, []byte("\xef\xbb\xbf## Notes\nCafé — 日本語 ✓\n"), 0o644)
	o.Profile.Instructions = "extra.md"
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".claude/CLAUDE.md", ".codex/AGENTS.md"} {
		raw, err := os.ReadFile(filepath.Join(o.Home, f))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("\xef\xbb\xbf")) || !bytes.Contains(raw, []byte("Café — 日本語 ✓\n")) {
			t.Errorf("%s: non-ASCII text changed or a BOM is left:\n%s", f, raw)
		}
	}
}

func TestSettingsRoundTripKeepsBytes(t *testing.T) {
	o := routed(t, "claude_code", "balanced")
	sp := filepath.Join(o.Home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(sp), 0o755)
	os.WriteFile(sp, []byte("{\n  \"note\": \"em dash — and 日本語\"\n}\n"), 0o644)
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	first := read(t, sp)
	if !strings.Contains(first, "em dash — and 日本語") || strings.Contains(first, "â") {
		t.Fatalf("text re-encoded:\n%s", first)
	}
	// Writing the result back and applying again must reproduce it byte for byte.
	o.Out = &bytes.Buffer{}
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if second := read(t, sp); second != first {
		t.Errorf("second run changed settings.json:\n%s\n---\n%s", first, second)
	}
	raw, _ := os.ReadFile(sp)
	if bytes.HasPrefix(raw, []byte("\xef\xbb\xbf")) {
		t.Error("settings.json must be UTF-8 without a BOM")
	}
}
