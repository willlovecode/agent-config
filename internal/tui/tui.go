// Package tui is the setup wizard's screen: a small pixel mascot asks one question at a
// time, finished answers stay visible in a step gutter (as in Clack and create-astro),
// the terminal keeps its own colours with one soft accent (as in Charm's gum), and the
// keys are the usual ones: arrows or j/k, space, enter, esc, numbers, ? and q.
// It draws internal/wizard on plain tcell and calls the engine through Config.
package tui

import (
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// MinWidth and MinHeight are the smallest screen the wizard draws on.
const MinWidth, MinHeight = 80, 25

// ErrPlain means the user asked for line mode (the screen was too small).
var ErrPlain = errors.New("plain mode requested")

type Config struct {
	Version string
	Display Display
	Home    string                                                   // shown as ~ in paths
	Plan    func(*profile.Profile) ([]install.Change, error)         // what would change
	Apply   func(p *profile.Profile, save bool, out io.Writer) error // install
	Screen  tcell.Screen                                             // tests use a simulation screen
}

// Result says what happened, so the caller can print the log after the screen closes.
type Result struct {
	Installed bool
	Log       string
	Err       error
}

type pageKind int

const (
	pageScan pageKind = iota
	pageQuestion
	pageItems
	pageReview
	pageInstall
)

type page struct {
	kind  pageKind
	id    string // question id, or the page name
	label string
	q     *wizard.Question
}

type overlay int

const (
	noOverlay overlay = iota
	helpOverlay
	quitOverlay
	infoOverlay
)

// Short labels for the gutter, and what the mascot says on each page.
var labels = map[string]string{"use": "who for", "tools": "tools", "allow_external_mcp": "outside",
	"stacks": "stacks", "work": "work"}

var speech = map[string]string{
	"scan":               "hi! i had a quick look at this machine. nothing leaves it.",
	"use":                "who's this computer for?",
	"tools":              "which agent tools do you use?",
	"allow_external_mcp": "may tools ask outside services, like Context7?",
	"stacks":             "what do you code in? pick all that fit.",
	"work":               "and what kind of work?",
	"items":              "here's what i'd set up. toggle anything you like.",
	"review":             "last look. nothing is written until you press enter.",
	"install":            "installing... hold tight!",
	"done":               "all set! start a new session and have fun.",
	"failed":             "oops. it stopped. nothing is lost, see below.",
}

type ui struct {
	cfg Config
	d   Display
	g   glyphs
	pal palette
	w   *wizard.Wizard
	s   tcell.Screen

	cur         int
	cursor      map[string]int
	scroll      map[string]int
	overlay     overlay
	save        bool
	tick        int // animation frame
	small       bool
	hits        []hit // clickable areas from the last draw
	lastButtons tcell.ButtonMask

	planning bool
	planGen  int
	plan     []install.Change
	planErr  error

	installing bool
	finished   bool
	mu         sync.Mutex
	log        strings.Builder

	result Result
	quit   bool
	plain  bool
}

// Run shows the wizard until the user installs or leaves.
func Run(w *wizard.Wizard, cfg Config) (Result, error) {
	s := cfg.Screen
	if s == nil {
		var err error
		if s, err = tcell.NewScreen(); err != nil {
			return Result{}, err
		}
	}
	if err := s.Init(); err != nil {
		return Result{}, err
	}
	defer s.Fini()
	u := &ui{cfg: cfg, d: cfg.Display, g: cfg.Display.g(), pal: cfg.Display.palette(), w: w, s: s,
		cursor: map[string]int{}, scroll: map[string]int{}, save: true}
	s.SetStyle(u.pal.base)
	s.HideCursor()
	s.EnableMouse()

	stop := make(chan struct{})
	defer close(stop)
	go func() { // animation: spinner, blinking, progress
		t := time.NewTicker(180 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				u.post(func() { u.tick++ })
			}
		}
	}()

	for !u.quit {
		u.draw()
		s.Show()
		switch ev := s.PollEvent().(type) {
		case nil:
			u.quit = true
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			u.key(ev)
		case *tcell.EventMouse:
			u.mouse(ev)
		case *tcell.EventInterrupt:
			if f, ok := ev.Data().(func()); ok {
				f()
			}
		}
	}
	if u.plain {
		return u.result, ErrPlain
	}
	u.mu.Lock()
	u.result.Log = u.log.String()
	u.mu.Unlock()
	return u.result, nil
}

// post runs f on the event loop (from another goroutine).
func (u *ui) post(f func()) { u.s.PostEvent(tcell.NewEventInterrupt(f)) }

// pages lists the steps for the current answers (a follow-up question can appear).
func (u *ui) pages() []page {
	out := []page{{kind: pageScan, id: "scan", label: "scan"}}
	for _, step := range []string{"use", "work"} {
		for _, q := range u.w.Questions(step) {
			out = append(out, page{kind: pageQuestion, id: q.ID, label: labels[q.ID], q: q})
		}
	}
	return append(out, page{kind: pageItems, id: "items", label: "items"},
		page{kind: pageReview, id: "review", label: "review"},
		page{kind: pageInstall, id: "install", label: "install"})
}

func (u *ui) current() page {
	ps := u.pages()
	if u.cur >= len(ps) {
		u.cur = len(ps) - 1
	}
	return ps[u.cur]
}

// ---- keys ----

func (u *ui) key(e *tcell.EventKey) {
	if u.small {
		switch {
		case e.Key() == tcell.KeyRune && (e.Rune() == 'p' || e.Rune() == 'P'):
			u.plain, u.quit = true, true
		case e.Key() == tcell.KeyEscape, e.Key() == tcell.KeyCtrlC, e.Key() == tcell.KeyRune && e.Rune() == 'q':
			u.quit = true
		}
		return
	}
	switch u.overlay {
	case quitOverlay:
		if e.Key() == tcell.KeyRune && (e.Rune() == 'y' || e.Rune() == 'Y') || e.Key() == tcell.KeyEnter {
			u.quit = true
		}
		u.overlay = noOverlay
		return
	case helpOverlay, infoOverlay:
		u.overlay = noOverlay
		return
	}
	if u.finished {
		if e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyCtrlC || e.Rune() == 'q' {
			u.quit = true
		}
		return
	}
	if u.installing {
		return // nothing to do but wait
	}
	r := rune(0)
	if e.Key() == tcell.KeyRune {
		r = e.Rune()
	}
	switch {
	case e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyF10 || r == 'q':
		u.overlay = quitOverlay
		return
	case r == '?' || e.Key() == tcell.KeyF1:
		u.overlay = helpOverlay
		return
	case e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyBackspace || e.Key() == tcell.KeyBackspace2 || e.Key() == tcell.KeyLeft:
		u.back()
		return
	}
	p := u.current()
	n := u.rows(p)
	c := u.cursor[p.id]
	switch {
	case e.Key() == tcell.KeyUp || r == 'k':
		u.move(p, c-1, n)
	case e.Key() == tcell.KeyDown || r == 'j':
		u.move(p, c+1, n)
	case e.Key() == tcell.KeyHome || r == 'g':
		u.move(p, 0, n)
	case e.Key() == tcell.KeyEnd || r == 'G':
		u.move(p, n-1, n)
	case e.Key() == tcell.KeyPgUp:
		u.move(p, c-8, n)
	case e.Key() == tcell.KeyPgDn:
		u.move(p, c+8, n)
	case r >= '1' && r <= '9' && p.kind == pageQuestion:
		if i := int(r - '1'); i < n {
			u.cursor[p.id] = i
			u.pick(p, i)
		}
	case r == ' ':
		u.pick(p, c)
	case (r == 'i' || r == 'I') && p.kind == pageItems:
		u.overlay = infoOverlay
	case e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyRight:
		if p.kind == pageQuestion && !p.q.Multi {
			u.pick(p, c) // enter takes the highlighted choice, as in gum
		}
		u.next()
	}
}

// rows is how many selectable rows a page has.
func (u *ui) rows(p page) int {
	switch p.kind {
	case pageQuestion:
		return len(p.q.Options)
	case pageItems:
		return len(u.w.Lines())
	case pageReview:
		return len(u.plan) + 1 // the changes scroll; the last row is the save toggle
	}
	return 0
}

func (u *ui) move(p page, to, n int) {
	if n == 0 {
		return
	}
	if to < 0 {
		to = 0
	}
	if to >= n {
		to = n - 1
	}
	u.cursor[p.id] = to
}

func (u *ui) pick(p page, i int) {
	switch p.kind {
	case pageQuestion:
		if i < len(p.q.Options) {
			u.w.Set(p.q, p.q.Options[i].Value)
		}
	case pageItems:
		if ls := u.w.Lines(); i < len(ls) {
			u.w.Toggle(ls[i].Item.ID)
		}
	case pageReview:
		u.save = !u.save
	}
}

func (u *ui) back() {
	if u.cur > 0 {
		u.cur--
	}
}

func (u *ui) next() {
	p := u.current()
	switch p.kind {
	case pageReview:
		if !u.planning && u.planErr == nil {
			u.install()
		}
		return
	case pageInstall:
		return
	}
	u.cur++
	if u.current().kind == pageReview {
		u.startPlan()
	}
}

func (u *ui) startPlan() {
	pr, err := u.w.Profile()
	u.planGen++
	gen := u.planGen
	u.planning, u.plan, u.planErr = true, nil, err
	if err != nil {
		u.planning = false
		return
	}
	go func() {
		plan, err := u.cfg.Plan(pr)
		u.post(func() {
			if gen == u.planGen {
				u.planning, u.plan, u.planErr = false, plan, err
				u.cursor["review"] = len(plan) // start on the save toggle
			}
		})
	}()
}

func (u *ui) install() {
	pr, err := u.w.Profile()
	if err != nil {
		u.planErr = err
		return
	}
	u.cur = len(u.pages()) - 1
	u.installing = true
	save := u.save
	go func() {
		err := u.cfg.Apply(pr, save, writerFunc(func(b []byte) {
			u.mu.Lock()
			u.log.Write(b)
			u.mu.Unlock()
		}))
		u.post(func() {
			u.installing, u.finished = false, true
			u.result.Installed, u.result.Err = err == nil, err
		})
	}()
}

type writerFunc func([]byte)

func (f writerFunc) Write(b []byte) (int, error) { f(b); return len(b), nil }

// mouse: a click runs what was drawn there (an answer, a button, a done step); the
// wheel moves the cursor. Any click closes help and item details.
func (u *ui) mouse(e *tcell.EventMouse) {
	b := e.Buttons()
	pressed := b&tcell.Button1 != 0 && u.lastButtons&tcell.Button1 == 0
	u.lastButtons = b
	if u.small || u.installing {
		return
	}
	if b&(tcell.WheelUp|tcell.WheelDown) != 0 && u.overlay == noOverlay && !u.finished {
		p := u.current()
		d := 1
		if b&tcell.WheelUp != 0 {
			d = -1
		}
		u.move(p, u.cursor[p.id]+d, u.rows(p))
		return
	}
	if !pressed {
		return
	}
	if u.overlay == helpOverlay || u.overlay == infoOverlay {
		u.overlay = noOverlay
		return
	}
	if u.overlay != noOverlay {
		return // quit asks for Y or N
	}
	x, y := e.Position()
	for _, h := range u.hits {
		if y == h.y && x >= h.x && x < h.x+h.w {
			h.act()
			return
		}
	}
}
