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
		{Name: "docker", Fact: "docker"},
	}}
	sim := tcell.NewSimulationScreen("UTF-8")
	hs := &harness{t: t, sim: sim, done: make(chan Result, 1)}
	cfg := Config{Version: "test", Display: d, Home: `C:\Users\x`, Screen: sim,
		Plan: func(p *profile.Profile) ([]install.Change, error) {
			return []install.Change{{Action: "create", What: `C:\Users\x\.codex\AGENTS.md`}, {Action: "change", What: `C:\Users\x\.claude\settings.json`},
				{Action: "run", What: "npm install -g pyright@1.1.414"}}, nil
		},
		Apply: func(p *profile.Profile, save bool, out io.Writer) error {
			hs.saved, hs.items = save, p.Items
			fmt.Fprintln(out, `Backup: C:\Users\x\.agtk\backups\1`)
			fmt.Fprintln(out, "To do: open Codex and run /hooks")
			fmt.Fprintln(out, "Done (tier A).")
			return nil
		}}
	go func() {
		r, err := Run(wizard.New(cat, assets, s), cfg)
		hs.err = err
		hs.done <- r
	}()
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
	h := start(t, 80, 25, Display{Lines: "unicode", Color: true})
	h.waitFor("▄▀█ █▀▀ ▀█▀ █▄▀")
	h.waitFor("hi! i had a quick look")
	h.waitFor("claude 2.1.318, codex 0.166.0")
	h.waitFor("Missing   docker")
	h.key(tcell.KeyEnter)
	h.waitFor("who's this computer for?")
	h.waitFor("1  ● Personal")
	h.rune('2') // number keys choose directly
	h.waitFor("2  ● Corporate")
	h.rune('1')
	h.waitFor("1  ● Personal")
	h.key(tcell.KeyEnter)
	h.waitFor("which agent tools do you use?")
	h.waitFor("✓ Who it's for")
	h.waitFor("found: 0.166.0")
	h.key(tcell.KeyEnter)
	h.waitFor("what do you code in?")
	h.key(tcell.KeyEscape) // back works
	h.waitFor("which agent tools do you use?")
	h.key(tcell.KeyEnter)
	h.key(tcell.KeyEnter)
	h.waitFor("and what kind of work?")
	h.key(tcell.KeyEnter)
	h.waitFor("here's what i'd set up")
	h.waitFor("[x] codex-rules")
	h.waitFor("Context █")
	h.rune('i')
	h.waitFor("recommend when always")
	h.rune('x') // any key closes
	h.rune(' ')
	h.waitFor("[ ] codex-rules")
	h.rune('?')
	h.waitFor("choose and go on")
	h.key(tcell.KeyEnter)
	h.key(tcell.KeyEnter)
	h.waitFor("last look")
	h.waitFor("1 new, 1 changed, 1 commands")
	h.waitFor("create ~/.codex/AGENTS.md")
	h.waitFor("[x] Save answers to")
	h.rune(' ')
	h.waitFor("[ ] Save answers to")
	h.key(tcell.KeyEnter)
	h.waitFor("Setup complete.")
	h.waitFor("To do   open Codex and run /hooks")
	h.waitFor("Backup  ~/.agtk/backups/1")
	h.waitFor("all set!")
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

func TestQuitAsksFirst(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "unicode", Color: true})
	h.waitFor("hi! i had a quick look")
	h.rune('q')
	h.waitFor("Nothing has been written yet.")
	h.rune('n')
	h.key(tcell.KeyEnter)
	h.waitFor("who's this computer for?")
	h.key(tcell.KeyF10)
	h.waitFor("Quit setup?")
	h.rune('y')
	if r := h.finish(); r.Installed || h.err != nil {
		t.Fatalf("quit must not install: %+v %v", r, h.err)
	}
}

func TestTooSmallOffersPlainMode(t *testing.T) {
	h := start(t, 60, 20, Display{Lines: "unicode", Color: true})
	h.waitFor("setup needs at least 80x25")
	h.rune('p')
	h.finish()
	if h.err != ErrPlain {
		t.Fatalf("want ErrPlain, got %v", h.err)
	}
}

func TestASCIIAndNoColour(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "ascii", Color: false})
	h.waitFor("( o o )")
	h.key(tcell.KeyEnter)
	h.waitFor("1  (*) Personal")
	scr := h.screen()
	for _, r := range "▄▀█▸●╭╮╰╯╔╗╚╝═║░◂✓" {
		if strings.ContainsRune(scr, r) {
			t.Errorf("ASCII mode drew %q", r)
		}
	}
	cells, _, _ := h.sim.GetContents()
	for _, c := range cells {
		if fg, bg, _ := c.Style.Decompose(); fg != tcell.ColorDefault || bg != tcell.ColorDefault {
			t.Fatalf("NO_COLOR mode used colour %v/%v", fg, bg)
		}
	}
	h.rune('q')
	h.rune('y')
	if r := h.finish(); r.Installed {
		t.Error("quit must not install")
	}
}

func TestOneAccentColour(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "unicode", Color: true})
	h.key(tcell.KeyEnter)
	h.waitFor("1  ● Personal")
	colours := map[tcell.Color]bool{}
	cells, _, _ := h.sim.GetContents()
	for _, c := range cells {
		fg, bg, _ := c.Style.Decompose()
		if bg != tcell.ColorDefault {
			t.Fatalf("background colour %v: the terminal's own background must stay", bg)
		}
		colours[fg] = true
	}
	delete(colours, tcell.ColorDefault)
	delete(colours, tcell.ColorGray)
	if len(colours) != 1 {
		t.Errorf("want exactly one accent colour, got %v", colours)
	}
	h.rune('q')
	h.rune('y')
	h.finish()
}

// TestDumpScreens writes each screen as text when AGTK_DUMP names a file (for reviewing
// the layout by eye); it is skipped otherwise.
func TestDumpScreens(t *testing.T) {
	path := os.Getenv("AGTK_DUMP")
	if path == "" {
		t.Skip("set AGTK_DUMP=<file> to write the screens")
	}
	h := start(t, 80, 25, Display{Lines: "unicode", Color: true})
	var b strings.Builder
	shot := func(wait string) {
		h.waitFor(wait)
		time.Sleep(100 * time.Millisecond)
		b.WriteString(h.screen() + "\n")
	}
	shot("hi! i had")
	h.key(tcell.KeyEnter)
	shot("who's this")
	h.key(tcell.KeyEnter)
	h.key(tcell.KeyEnter)
	h.key(tcell.KeyEnter)
	shot("and what kind of work?")
	h.key(tcell.KeyEnter)
	shot("here's what")
	h.rune('i')
	shot("recommend when")
	h.rune('x')
	h.rune('?')
	shot("choose and go on")
	h.rune('x')
	h.key(tcell.KeyEnter)
	shot("Save answers")
	h.key(tcell.KeyEnter)
	shot("Setup complete")
	h.key(tcell.KeyEnter)
	h.finish()
	os.WriteFile(path, []byte(b.String()), 0o644)
}

// click presses and releases the left button on the first cell showing s.
func (h *harness) click(s string) {
	h.t.Helper()
	h.waitFor(s)
	for y, l := range strings.Split(h.screen(), "\n") {
		if i := strings.Index(l, s); i >= 0 {
			x := len([]rune(l[:i]))
			h.sim.InjectMouse(x, y, tcell.Button1, tcell.ModNone)
			h.sim.InjectMouse(x, y, tcell.ButtonNone, tcell.ModNone)
			return
		}
	}
}

func TestMouse(t *testing.T) {
	h := start(t, 80, 25, Display{Lines: "unicode", Color: true})
	h.click("Next >  Enter")
	h.waitFor("who's this computer for?")
	h.click("Corporate, with my own API key")
	h.waitFor("3  ● Corporate, with my own API key")
	h.click("✓ Welcome") // done steps in the sidebar take you back
	h.waitFor("hi! i had a quick look")
	h.click("F1 Help")
	h.waitFor("choose and go on")
	h.click("Keys") // any click closes help
	h.click("F10 Quit")
	h.waitFor("Quit setup?")
	h.rune('y')
	if r := h.finish(); r.Installed {
		t.Error("quit must not install")
	}
}
