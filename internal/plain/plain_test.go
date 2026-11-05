package plain

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

func run(t *testing.T, input string) (installed bool, saved bool, items []string, out string) {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{
		{Name: "claude", Fact: "claude_code", Version: "2.1.318"}, {Name: "codex", Fact: "codex", Version: "0.166.0"}}}
	cfg := Config{
		Plan: func(*profile.Profile) ([]install.Change, error) {
			return []install.Change{{Action: "create", What: "~/.codex/AGENTS.md"}}, nil
		},
		Apply: func(p *profile.Profile, save bool, w io.Writer) error {
			saved, items = save, p.Items
			fmt.Fprintln(w, "Done (tier A).")
			return nil
		},
	}
	var b strings.Builder
	installed, err = Run(wizard.New(cat, assets, s), cfg, strings.NewReader(input), &b)
	if err != nil {
		t.Fatal(err)
	}
	return installed, saved, items, b.String()
}

func TestPlainInstall(t *testing.T) {
	// scan, use (personal), back at tools, use again, tools (keep), stacks: Python,
	// work (keep), explain item 1, switch item 1 off, continue, no save, install.
	input := "\n\n<\n\n\n1\n\n?1\n1\n\nn\ny\n"
	installed, saved, items, out := run(t, input)
	if !installed || saved {
		t.Fatalf("installed=%v saved=%v\n%s", installed, saved, out)
	}
	for _, want := range []string{"Step 3 of 6", "* 1) Personal", "found: 2.1.318", "recommend when", "create  ~/.codex/AGENTS.md", "Done (tier A)."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	first := "codex-rules" // item 1, alphabetically
	if strings.Contains(strings.Join(items, ","), first) {
		t.Errorf("item 1 was switched off but installed: %v", items)
	}
	if !strings.Contains(strings.Join(items, ","), "context7") {
		t.Errorf("a stack was chosen, context7 should be in: %v", items)
	}
}

func TestPlainQuitAndBadInput(t *testing.T) {
	installed, _, _, out := run(t, "\n9\n1\nq\n")
	if installed || !strings.Contains(out, "Please type option numbers") || !strings.Contains(out, "Nothing was written") {
		t.Fatalf("installed=%v\n%s", installed, out)
	}
	installed, _, _, _ = run(t, "\n\n\n\n\n\n\nN\n") // declines at the end (EOF after)
	if installed {
		t.Error("declining must not install")
	}
}
