package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

type harness struct {
	t     *testing.T
	sim   tcell.SimulationScreen
	done  chan Result
	err   error
	saved bool
	items []string
}

func start(t *testing.T, w, h int, d Display) *harness {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Arch: "amd64", CPUs: 12, RAMGB: 16, Tier: "A", Programs: []*scan.Program{
		{Name: "claude", Fact: "claude_code", Version: "2.1.318"},
		{Name: "codex", Fact: "codex", Version: "0.166.0"},
		{Name: "node", Fact: "node", Version: "24.19.0"},
	}}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	hs := &harness{t: t, sim: sim, done: make(chan Result, 1)}
	cfg := Config{Version: "test", Display: d, Home: `C:\Users\x`, Screen: sim,
		Plan: func(p *profile.Profile) ([]install.Change, error) {
			return []install.Change{{Action: "create", What: `C:\Users\x\.codex\AGENTS.md`}, {Action: "run", What: "npm install -g pyright@1.1.414"}}, nil
		},
		Apply: func(p *profile.Profile, save bool, out io.Writer) error {
			hs.saved, hs.items = save, p.Items
			fmt.Fprintln(out, "Done (tier A).")
			return nil
		}}
	go func() {
		r, err := Run(wizard.New(cat, assets, s), cfg)
		hs.err = err
		hs.done <- r
	}()
	// The size takes effect once the running app sees a resize event.
	time.Sleep(50 * time.Millisecond)
	sim.SetSize(w, h)
	sim.PostEvent(tcell.NewEventResize(w, h))
	return hs
}

func (h *harness) screen() string {
	cells, w, _ := h.sim.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteRune(' ')
		}
		if (i+1)%w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// waitFor polls the screen until it shows s.
func (h *harness) waitFor(s string) {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.screen(), s) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("screen never showed %q:\n%s", s, h.screen())
}

func (h *harness) key(k tcell.Key) { h.sim.InjectKey(k, 0, tcell.ModNone) }
func (h *harness) rune(r rune)     { h.sim.InjectKey(tcell.KeyRune, r, tcell.ModNone) }

func (h *harness) finish() Result {
	h.t.Helper()
	select {
	case r := <-h.done:
		return r
	case <-time.After(3 * time.Second):
		h.t.Fatalf("wizard did not exit:\n%s", h.screen())
	}
	return Result{}
}

func TestWizardWalkthrough(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "double", Color: true})
	h.waitFor("Step 1 of 6: Welcome")
	h.waitFor("╔")
	h.key(tcell.KeyEnter)
	h.waitFor("Step 2 of 6: Scan")
	h.waitFor("claude 2.1.318")
	h.key(tcell.KeyEnter)
	h.waitFor("Who is this machine for?")
	h.waitFor("(•) Personal")
	h.waitFor("found: 0.166.0")
	h.key(tcell.KeyEnter)
	h.waitFor("Step 4 of 6")
	h.key(tcell.KeyEscape) // back works
	h.waitFor("Step 3 of 6")
	h.key(tcell.KeyEnter)
	h.waitFor("Step 4 of 6")
	h.key(tcell.KeyEnter)
	h.waitFor("Step 5 of 6: Recommended setup")
	h.waitFor("[X] codex-rules")
	h.waitFor("Always loaded: ~")
	h.key(tcell.KeyF1)
	h.waitFor("Why codex-rules?")
	h.waitFor("recommend when always")
	h.key(tcell.KeyEscape)
	h.waitFor("[X] codex-rules")
	h.rune(' ') // switch codex-rules off
	h.waitFor("[ ] codex-rules")
	h.key(tcell.KeyEnter)
	h.waitFor("Step 6 of 6: Review and install")
	h.waitFor("create  ~/.codex/AGENTS.md")
	h.waitFor("run     npm install -g pyright")
	h.waitFor("[X] Save my answers")
	h.rune(' ')
	h.waitFor("[ ] Save my answers")
	h.key(tcell.KeyEnter)
	h.waitFor("Setup complete")
	h.waitFor("Done (tier A).")
	h.key(tcell.KeyEnter)
	r := h.finish()
	if h.err != nil || !r.Installed || !strings.Contains(r.Log, "Done (tier A).") {
		t.Fatalf("result %+v err %v", r, h.err)
	}
	if h.saved {
		t.Error("save was switched off")
	}
	joined := strings.Join(h.items, ",")
	if strings.Contains(joined, "codex-rules") || !strings.Contains(joined, "git-guard") {
		t.Errorf("installed items %s", joined)
	}
}

func TestTooSmallOffersPlainMode(t *testing.T) {
	h := start(t, 60, 20, Display{Lines: "double", Color: true})
	h.waitFor("setup needs at least 80x25")
	h.rune('p')
	h.finish()
	if h.err != ErrPlain {
		t.Fatalf("want ErrPlain, got %v", h.err)
	}
}

func TestASCIIAndNoColour(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "ascii", Color: false})
	h.waitFor("- Step 1 of 6: Welcome -")
	for _, r := range "═║╔╗╚╝•█░" {
		if strings.ContainsRune(h.screen(), r) {
			t.Errorf("ASCII mode drew %q", r)
		}
	}
	h.key(tcell.KeyEnter)
	h.key(tcell.KeyEnter)
	h.waitFor("(*) Personal")
	cells, _, _ := h.sim.GetContents()
	for _, c := range cells {
		if fg, bg, _ := c.Style.Decompose(); fg == tcell.ColorNavy || bg == tcell.ColorNavy {
			t.Fatal("NO_COLOR mode used colour")
		}
	}
	h.key(tcell.KeyF3)
	if r := h.finish(); r.Installed {
		t.Error("F3 must not install")
	}
}
