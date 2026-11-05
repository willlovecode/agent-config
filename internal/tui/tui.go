// Package tui is the DOS-style setup wizard: blue screen, double-line frames, a key bar,
// keyboard only. It draws internal/wizard and calls the engine through Config.
package tui

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

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

type ui struct {
	cfg    Config
	d      Display
	w      *wizard.Wizard
	app    *tview.Application
	window *tview.Flex // holds the current frame
	keys   *tview.TextView
	step   int
	focus  []tview.Primitive
	fi     int
	onKey  func(*tcell.EventKey) *tcell.EventKey
	busy   bool // planning or installing: keys are ignored
	small  bool
	save   bool
	plain  bool
	mu     sync.Mutex
	log    strings.Builder
	result Result
}

// Run shows the wizard until the user installs or leaves.
func Run(w *wizard.Wizard, cfg Config) (Result, error) {
	u := &ui{cfg: cfg, d: cfg.Display, w: w, app: tview.NewApplication(), save: true}
	u.d.apply()
	if cfg.Screen != nil {
		u.app.SetScreen(cfg.Screen)
	}
	header := tview.NewTextView().SetText(fmt.Sprintf(" %-*s v%s", MinWidth-4-len(cfg.Version), "Agent Toolkit Setup", cfg.Version))
	header.SetTextColor(tview.Styles.PrimaryTextColor)
	rule := tview.NewTextView().SetText(" " + u.d.rule(MinWidth-2))
	u.window = tview.NewFlex().SetDirection(tview.FlexRow)
	u.keys = keyBar(u.d)
	centre := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).AddItem(u.window, 20, 0, true).AddItem(nil, 0, 1, false), 72, 0, true).
		AddItem(nil, 0, 1, false)
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).AddItem(rule, 1, 0, false).
		AddItem(centre, 0, 1, true).AddItem(u.keys, 1, 0, false)

	// Empty layout cells are not drawn by tview; paint the whole screen first. A screen
	// below 80x25 gets a notice instead of a cut-off wizard.
	u.app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
		s.Fill(' ', u.d.screenStyle())
		wd, ht := s.Size()
		u.small = wd < MinWidth || ht < MinHeight
		if u.small {
			lines := []string{
				fmt.Sprintf("The window is %dx%d; setup needs at least %dx%d.", wd, ht, MinWidth, MinHeight),
				"Make the window larger, or press P for plain line mode.",
				"F3 or ESC exits. Nothing has been written.",
			}
			for i, l := range lines {
				for j, r := range []rune(l) {
					s.SetContent(j, i, r, nil, u.d.screenStyle())
				}
			}
			return true
		}
		return false
	})
	u.app.SetInputCapture(u.input)
	u.app.SetRoot(root, true)
	u.show(0)
	err := u.app.Run()
	if u.plain {
		return u.result, ErrPlain
	}
	u.mu.Lock()
	u.result.Log = u.log.String()
	u.mu.Unlock()
	return u.result, err
}

func (u *ui) input(e *tcell.EventKey) *tcell.EventKey {
	if u.small {
		switch {
		case e.Key() == tcell.KeyRune && (e.Rune() == 'p' || e.Rune() == 'P'):
			u.plain = true
			u.app.Stop()
		case e.Key() == tcell.KeyF3 || e.Key() == tcell.KeyEscape:
			u.app.Stop()
		}
		return nil
	}
	if u.busy {
		return nil
	}
	switch e.Key() {
	case tcell.KeyF3:
		u.app.Stop()
		return nil
	case tcell.KeyTab, tcell.KeyBacktab:
		if len(u.focus) > 1 {
			if e.Key() == tcell.KeyTab {
				u.fi = (u.fi + 1) % len(u.focus)
			} else {
				u.fi = (u.fi + len(u.focus) - 1) % len(u.focus)
			}
			u.app.SetFocus(u.focus[u.fi])
		}
		return nil
	}
	if u.onKey != nil {
		return u.onKey(e)
	}
	return e
}

// screen puts a frame in the window and sets the key bar, key handler and focus order.
func (u *ui) screen(title string, body tview.Primitive, keys string, onKey func(*tcell.EventKey) *tcell.EventKey, focus ...tview.Primitive) {
	u.window.Clear()
	u.window.AddItem(frame(title, body), 0, 1, true)
	u.keys.SetText(" " + keys)
	u.onKey, u.focus, u.fi = onKey, focus, 0
	if len(focus) > 0 {
		u.app.SetFocus(focus[0])
	} else {
		u.app.SetFocus(body)
	}
}

func (u *ui) stepTitle(i int) string {
	return fmt.Sprintf("Step %d of %d: %s", i+1, len(wizard.Steps), wizard.Steps[i])
}

// nav handles ENTER (next) and ESC (back) for the question screens.
func (u *ui) nav(e *tcell.EventKey) *tcell.EventKey {
	switch e.Key() {
	case tcell.KeyEnter:
		u.show(u.step + 1)
		return nil
	case tcell.KeyEscape:
		if u.step > 0 {
			u.show(u.step - 1)
		}
		return nil
	}
	return e
}

func (u *ui) show(step int) {
	u.step = step
	switch step {
	case 0:
		u.welcome()
	case 1:
		u.scan()
	case 2:
		u.questions("use")
	case 3:
		u.questions("work")
	case 4:
		u.recommend()
	case 5:
		u.review()
	}
}

func (u *ui) welcome() {
	body := text("Setup will configure Claude Code and Codex on this computer: " +
		"instructions, skills, hooks, MCP servers and plugins, chosen for your " +
		"machine and how you work.\n\n" +
		"Nothing is written until you confirm the summary in step 6. Every file " +
		"that changes is backed up first, and `agtk undo` restores it.\n\n" +
		"The machine scan stays in memory; it is never stored or sent.\n\n" +
		"Press ENTER to continue.")
	u.screen(u.stepTitle(0), body, "ENTER=Continue  F3=Exit", u.nav)
}

func (u *ui) scan() {
	s := u.w.Scan
	var b strings.Builder
	fmt.Fprintf(&b, "System:   %s/%s, %d CPU threads, %.0f GB RAM\n\n", s.OS, s.Arch, s.CPUs, s.RAMGB)
	var found, missing []string
	seen := map[string]bool{}
	for _, p := range s.Programs {
		if seen[p.Fact] {
			continue
		}
		if p.Version != "" {
			seen[p.Fact] = true
			found = append(found, fmt.Sprintf("%s %s", p.Name, p.Version))
		}
	}
	for _, p := range s.Programs {
		if !seen[p.Fact] {
			seen[p.Fact] = true
			missing = append(missing, p.Name)
		}
	}
	fmt.Fprintf(&b, "Found:    %s\n", strings.Join(found, ", "))
	fmt.Fprintf(&b, "Missing:  %s\n\n", strings.Join(missing, ", "))
	if len(s.Policies) == 0 {
		b.WriteString("Policies: none found\n")
	}
	for _, p := range s.Policies {
		fmt.Fprintf(&b, "Policy:   %s\n", p)
	}
	tier := "A, hooks allowed"
	if s.Tier == "B" {
		tier = "B, rules and permissions only (" + strings.Join(s.TierWhy, "; ") + ")"
	}
	fmt.Fprintf(&b, "Tier:     %s\n\nNothing of this scan is stored or sent.", tier)
	u.screen(u.stepTitle(1), text(b.String()), "ENTER=Continue  ESC=Back  F3=Exit", u.nav)
}

// questions shows a step's questions as radio groups and checklists, TAB between them.
func (u *ui) questions(step string) {
	body := tview.NewFlex().SetDirection(tview.FlexRow)
	var lists []tview.Primitive
	for _, q := range u.w.Questions(step) {
		q := q
		body.AddItem(tview.NewTextView().SetText(q.Prompt), 1, 0, false)
		list := newChoiceList(u.d, func(l *choiceList) {
			for _, o := range q.Options {
				mark := u.d.radio(u.w.Has(q.ID, o.Value))
				if q.Multi {
					mark = "[ ]"
					if u.w.Has(q.ID, o.Value) {
						mark = "[X]"
					}
				}
				line := fmt.Sprintf("%s %s", mark, o.Label)
				if n := u.w.Note(q, o); n != "" {
					line = fmt.Sprintf("%-28s %s", line, n)
				}
				l.add(line)
			}
		}, nil)
		list.toggle = func(row int) {
			before := len(u.w.Questions(step))
			u.w.Set(q, q.Options[row].Value)
			if len(u.w.Questions(step)) != before { // a follow-up question appeared or went
				fi := u.fi
				u.questions(step)
				u.fi = fi
				u.app.SetFocus(u.focus[fi])
			}
		}
		body.AddItem(list, len(q.Options), 0, len(lists) == 0)
		body.AddItem(nil, 1, 0, false)
		lists = append(lists, list)
	}
	u.screen(u.stepTitle(u.step), body, "ENTER=Continue  SPACE=Select  TAB=Next question  ESC=Back  F3=Exit", u.nav, lists...)
}

func (u *ui) recommend() {
	var lines []wizard.Line
	meter := tview.NewTextView()
	refresh := func() {
		used, max := u.w.Budget()
		meter.SetText(fmt.Sprintf("Always loaded: ~%d tokens of %d budget   %s", used, max, u.d.meter(used, max, 16)))
	}
	list := newChoiceList(u.d, func(l *choiceList) {
		lines = u.w.Lines()
		for _, ln := range lines {
			mark := "[ ]"
			if ln.On {
				mark = "[X]"
			}
			note := ln.Item.Summary
			if !ln.On && !ln.Recommended && ln.Reason != "" {
				note = "off: " + ln.Reason
			}
			l.add(fmt.Sprintf("%s %-18s %s", mark, ln.Item.ID, note))
		}
	}, nil)
	list.toggle = func(row int) { u.w.Toggle(lines[row].Item.ID); refresh() }
	refresh()
	body := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(list, 0, 1, true).AddItem(meter, 1, 0, false)
	u.screen(u.stepTitle(4), body, "ENTER=Continue  SPACE=Toggle  F1=Why this?  ESC=Back  F3=Exit",
		func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == tcell.KeyF1 && len(lines) > 0 {
				u.why(lines[list.GetCurrentItem()])
				return nil
			}
			return u.nav(e)
		}, list)
}

// why explains one item: the recommendation, the rules behind it, source and cost.
func (u *ui) why(l wizard.Line) {
	it := l.Item
	var b strings.Builder
	verdict := "Not recommended"
	if l.Recommended {
		verdict = "Recommended"
	}
	fmt.Fprintf(&b, "%s: %s\n\n", verdict, l.Reason)
	fmt.Fprintf(&b, "What:      %s (%s)\n", it.Summary, it.Kind)
	if it.RecommendWhen != nil {
		fmt.Fprintf(&b, "Rule:      recommend when %s\n", it.RecommendWhen)
	}
	if it.NeverWhen != nil {
		fmt.Fprintf(&b, "           never when %s\n", it.NeverWhen)
	}
	if it.Source.Repo != "" {
		fmt.Fprintf(&b, "Source:    %s@%s (%s)\n", it.Source.Repo, it.Source.Commit[:7], it.Source.License)
	}
	if it.Audit != "" {
		fmt.Fprintf(&b, "Audit:     %s\n", it.Audit)
	}
	if it.ExplicitOnly {
		b.WriteString("Use:       only when called by name\n")
	}
	fmt.Fprintf(&b, "Cost:      ~%d tokens in every session", l.Cost)
	u.screen("Why "+it.ID+"?", text(b.String()), "ESC=Back", func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyF1 {
			u.recommend()
			return nil
		}
		return e
	})
}

func (u *ui) tilde(p string) string {
	if u.cfg.Home != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(u.cfg.Home)) {
		return "~" + filepath.ToSlash(p[len(u.cfg.Home):])
	}
	return p
}

func (u *ui) review() {
	p, err := u.w.Profile()
	if err != nil {
		u.fail(err)
		return
	}
	changes := text("Working out what will change...")
	changes.SetWrap(false).SetScrollable(true)
	saveRow := newChoiceList(u.d, func(l *choiceList) {
		mark := "[ ]"
		if u.save {
			mark = "[X]"
		}
		l.add(fmt.Sprintf("%s Save my answers to %s", mark, u.tilde(p.Path)))
	}, func(int) { u.save = !u.save })
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(changes, 0, 1, false).AddItem(nil, 1, 0, false).AddItem(saveRow, 1, 0, true)
	ready := false
	u.screen(u.stepTitle(5), body, "ENTER=Install  SPACE=Toggle  TAB=Scroll list  ESC=Back  F3=Exit",
		func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == tcell.KeyEnter {
				if ready {
					u.install(p)
				}
				return nil
			}
			return u.nav(e)
		}, saveRow, changes)
	u.busy = true
	go func() {
		plan, err := u.cfg.Plan(p)
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			if err != nil {
				u.fail(err)
				return
			}
			var b strings.Builder
			if len(plan) == 0 {
				b.WriteString("Everything is already in place; installing changes nothing.")
			} else {
				fmt.Fprintf(&b, "%d change(s). Each changed file is backed up first.\n\n", len(plan))
			}
			for _, c := range plan {
				what := c.What
				if c.Action != "run" {
					what = u.tilde(what)
				}
				fmt.Fprintf(&b, "%-7s %s\n", c.Action, what)
			}
			changes.SetText(b.String()).ScrollToBeginning()
			ready = true
		})
	}()
}

// install runs the engine and streams its log into the window.
func (u *ui) install(p *profile.Profile) {
	out := text("")
	out.SetScrollable(true).SetChangedFunc(func() { u.app.Draw() })
	u.screen("Installing", out, "Please wait...", nil)
	u.busy = true
	go func() {
		err := u.cfg.Apply(p, u.save, writerFunc(func(b []byte) {
			u.mu.Lock()
			u.log.Write(b)
			u.mu.Unlock()
			s := string(b)
			u.app.QueueUpdateDraw(func() { fmt.Fprint(out, tview.Escape(s)) })
		}))
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			u.result.Installed = err == nil
			u.result.Err = err
			title, keys := "Setup complete", "ENTER=Exit  UP/DOWN=Scroll"
			if err != nil {
				title = "Setup failed"
				fmt.Fprintf(out, "\n%s\n\nFiles changed so far are backed up; `agtk undo` restores them.", tview.Escape(err.Error()))
			}
			u.window.Clear()
			u.window.AddItem(frame(title, out), 0, 1, true)
			u.keys.SetText(" " + keys)
			u.app.SetFocus(out)
			u.onKey = func(e *tcell.EventKey) *tcell.EventKey {
				if e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyEscape {
					u.app.Stop()
					return nil
				}
				return e
			}
		})
	}()
}

func (u *ui) fail(err error) {
	u.result.Err = err
	u.screen("Error", text(err.Error()+"\n\nNothing was written."), "ESC=Back  F3=Exit", func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape {
			u.show(4)
			return nil
		}
		return e
	})
}

type writerFunc func([]byte)

func (f writerFunc) Write(b []byte) (int, error) {
	c := make([]byte, len(b))
	copy(c, b)
	f(c)
	return len(b), nil
}
