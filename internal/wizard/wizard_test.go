package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
)

// fakeScan is a machine with Claude Code, Node and Java but no Codex.
func fakeScan() *scan.Result {
	return &scan.Result{OS: "windows", Arch: "amd64", Tier: "A", Programs: []*scan.Program{
		{Name: "claude", Fact: "claude_code", Version: "2.1.318"},
		{Name: "codex", Fact: "codex"},
		{Name: "node", Fact: "node", Version: "24.19.0"},
		{Name: "java", Fact: "jdk", Version: "21.0.13"},
	}}
}

func newWizard(t *testing.T) *Wizard {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	return New(cat, assets, fakeScan())
}

func on(w *Wizard) map[string]bool {
	m := map[string]bool{}
	for _, l := range w.Lines() {
		m[l.Item.ID] = l.On
	}
	return m
}

func TestDefaultsFollowTheScan(t *testing.T) {
	w := newWizard(t)
	if got := strings.Join(w.Values("tools"), ","); got != "claude_code" {
		t.Errorf("tools default = %s, want only what was found", got)
	}
	if got := strings.Join(w.Values("stacks"), ","); got != "java,javascript" {
		t.Errorf("stacks default = %s", got)
	}
	m := on(w)
	if _, ok := m["codex-rules"]; ok {
		t.Error("Codex items must not show when Codex is not chosen")
	}
	if !m["git-guard"] || !m["tdd"] || !m["context7"] {
		t.Errorf("personal defaults: %v", m)
	}
	if _, ok := m["stop-checks"]; ok {
		t.Error("project-scoped items belong to project setup, not this list")
	}
}

func TestCorporateAndChoices(t *testing.T) {
	w := newWizard(t)
	use := w.Questions("use")[0]
	w.Set(use, "corporate_gateway")
	if len(w.Questions("use")) != 3 {
		t.Fatal("corporate use should ask about outside services")
	}
	if on(w)["context7"] {
		t.Error("context7 must be off for corporate by default")
	}
	w.Toggle("context7") // the user knows better
	if !on(w)["context7"] {
		t.Error("toggle must win over the recommendation")
	}
	w.Set(w.Questions("use")[1], "codex") // multi: add Codex
	if !w.Has("tools", "codex") || !w.Has("tools", "claude_code") {
		t.Errorf("tools = %v", w.Values("tools"))
	}
	used, max := w.Budget()
	if used <= 0 || max != 3100 {
		t.Errorf("budget %d/%d", used, max)
	}
}

func TestProfileRoundTrip(t *testing.T) {
	w := newWizard(t)
	w.Toggle("tdd")
	p, err := w.Profile()
	if err != nil {
		t.Fatal(err)
	}
	p.Path = filepath.Join(t.TempDir(), "agtk", "profile.yaml")
	p.Dir = filepath.Dir(p.Path)
	if err := SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.Path)
	for _, leak := range []string{"2.1.318", "amd64", "21.0.13"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("profile holds scan data %q:\n%s", leak, b)
		}
	}
	loaded, err := profile.Load(p.Path)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	w2 := newWizard(t)
	if err := w2.Load(loaded); err != nil {
		t.Fatal(err)
	}
	if on(w2)["tdd"] || !on(w2)["git-guard"] {
		t.Errorf("loaded choices: %v", on(w2))
	}
}
