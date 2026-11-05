// Package tui is the DOS-style setup wizard: blue screen, double-line frames with a drop
// shadow, a step strip, a key bar, keyboard only. It draws internal/wizard and calls the
// engine through Config.
package tui

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// MinWidth and MinHeight are the smallest screen the wizard draws on.
const MinWidth, MinHeight = 80, 25

// Window size inside the 80x25 screen; the shadow takes one more row and two columns.
const winWidth, winHeight = 72, 20

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

// Short step names for the strip under the header.
var stripNames = []string{"Welcome", "Scan", "Use", "Work", "Items", "Install"}

type ui struct {
	cfg    Config
	d      Display
	w      *wizard.Wizard
	app    *tview.Application
	window *tview.Flex // holds the current frame
	strip  *tview.TextView
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
	title := u.d.bold("Agent Toolkit Setup")
	header := tview.NewTextView().SetDynamicColors(true).
		SetText(fmt.Sprintf(" %s%s%s", title, strings.Repeat(" ", MinWidth-4-len("Agent Toolkit Setup")-len(cfg.Version)), u.d.tag(roleDim, "v"+cfg.Version)))
	rule := tview.NewTextView().SetText(" " + u.d.rule(MinWidth-2))
	u.strip = tview.NewTextView().SetDynamicColors(true)
	u.window = tview.NewFlex().SetDirection(tview.FlexRow)
	u.keys = keyBar(u.d)
	u.keys.SetDynamicColors(true)
	centre := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).AddItem(u.window, winHeight, 0, true).AddItem(nil, 0, 1, false), winWidth, 0, true).
		AddItem(nil, 0, 1, false)
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).AddItem(rule, 1, 0, false).AddItem(u.strip, 1, 0, false).
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
	u.app.SetAfterDrawFunc(func(s tcell.Screen) {
		if !u.small {
			x, y, w, h := u.window.GetRect()
			u.d.shadow(s, x, y, w, h)
		}
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

// setKeys writes the key bar: key names in bold, "KEY=Action" pairs separated by spaces.
func (u *ui) setKeys(keys string) {
	var parts []string
	for _, kv := range strings.Fields(keys) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			parts = append(parts, u.d.bold(k)+"="+tview.Escape(v))
		} else {
			parts = append(parts, tview.Escape(kv))
		}
	}
	u.keys.SetText(" " + strings.Join(parts, "  "))
}

// setStrip marks the current step: done steps plain, the current one highlighted,
// later ones grey.
func (u *ui) setStrip(current int) {
	var b strings.Builder
	b.WriteString("   ")
	for i, n := range stripNames {
		label := fmt.Sprintf(" %d %s ", i+1, n)
		switch {
		case i == current && u.d.Color:
			b.WriteString("[black:silver:b]" + tview.Escape(label) + "[-:-:-]")
		case i == current:
			b.WriteString("[::r]" + tview.Escape(label) + "[::-]")
		case i < current:
			b.WriteString(tview.Escape(label))
		default:
			b.WriteString(u.d.tag(roleDim, label))
		}
		if i < len(stripNames)-1 {
			b.WriteString(u.d.tag(roleDim, ">"))
		}
	}
	u.strip.SetText(b.String())
}

// screen puts a frame in the window and sets the key bar, key handler and focus order.
func (u *ui) screen(title string, body tview.Primitive, keys string, onKey func(*tcell.EventKey) *tcell.EventKey, focus ...tview.Primitive) {
	u.window.Clear()
	u.window.AddItem(frame(title, body), 0, 1, true)
	u.setKeys(keys)
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
	u.setStrip(step)
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

// spin animates a spinner in front of label in t until the returned stop is called.
func (u *ui) spin(t *tview.TextView, label string) (stop func()) {
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(120 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			frame := spinner[i%len(spinner)]
			u.app.QueueUpdateDraw(func() { t.SetText(u.d.tag(roleAccent, frame) + " " + tview.Escape(label)) })
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}

func (u *ui) welcome() {
	d := u.d
	var b strings.Builder
	b.WriteString("Setup configures " + d.bold("Claude Code") + " and " + d.bold("Codex") + " on this computer: " +
		"instructions, skills, hooks, MCP servers and plugins, chosen for your machine and how you work.\n\n")
	steps := []string{"Look at this machine (nothing is stored or sent)", "Ask how you will use it",
		"Ask what you work on", "Recommend a setup you can change", "Show every change, then install"}
	for i, s := range steps {
		fmt.Fprintf(&b, "  %s  %s\n", d.tag(roleAccent, fmt.Sprintf("%d.", i+2)), tview.Escape(s))
	}
	b.WriteString("\n" + d.tag(roleOK, "Nothing is written until you confirm in step 6.") +
		" Every changed file is backed up first; " + d.bold("agtk undo") + " restores it.\n\n")
	b.WriteString("Press " + d.bold("ENTER") + " to begin.")
	u.screen(u.stepTitle(0), text(b.String()), "ENTER=Continue F3=Exit", u.nav)
}

func (u *ui) scan() {
	d, s := u.d, u.w.Scan
	var b strings.Builder
	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", d.tag(roleAccent, fmt.Sprintf("%-10s", label)), value)
	}
	row("System", tview.Escape(fmt.Sprintf("%s/%s, %d CPU threads, %.0f GB RAM", s.OS, s.Arch, s.CPUs, s.RAMGB)))
	b.WriteString("\n")
	// Two columns of programs: name and version, or "not found" in grey.
	seen := map[string]bool{}
	var cells []string
	for _, p := range s.Programs {
		if seen[p.Fact] {
			continue
		}
		v := s.Found(p.Fact)
		name := p.Name
		for _, q := range s.Programs { // python3/python: show the one that was found
			if q.Fact == p.Fact && q.Version != "" {
				name = q.Name
			}
		}
		seen[p.Fact] = true
		if v != "" {
			cells = append(cells, fmt.Sprintf("%-9s %s", tview.Escape(name), d.tag(roleOK, fmt.Sprintf("%-13s", v))))
		} else {
			cells = append(cells, fmt.Sprintf("%-9s %s", tview.Escape(name), d.tag(roleDim, fmt.Sprintf("%-13s", "not found"))))
		}
	}
	b.WriteString(d.tag(roleAccent, "Programs") + "\n")
	for i := 0; i < len(cells); i += 2 {
		line := "  " + cells[i]
		if i+1 < len(cells) {
			line += "     " + cells[i+1]
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	if len(s.Policies) == 0 {
		row("Policies", "none found")
	}
	for _, p := range s.Policies {
		row("Policy", tview.Escape(p))
	}
	if s.Tier == "B" {
		row("Tier", d.tag(roleWarn, "B: rules and permissions only")+" ("+tview.Escape(strings.Join(s.TierWhy, "; "))+")")
	} else {
		row("Tier", d.tag(roleOK, "A: hooks allowed"))
	}
	b.WriteString("\n" + d.tag(roleDim, "Nothing of this scan is stored or sent."))
	u.screen(u.stepTitle(1), text(b.String()), "ENTER=Continue ESC=Back F3=Exit", u.nav)
}

// questions shows a step's questions as radio groups and checklists, TAB between them.
// The focused question's prompt is marked, and only it shows the selection bar.
func (u *ui) questions(step string) {
	d := u.d
	body := tview.NewFlex().SetDirection(tview.FlexRow)
	var lists []tview.Primitive
	pointer := "►"
	if d.Lines == "ascii" {
		pointer = ">"
	}
	for _, q := range u.w.Questions(step) {
		q := q
		prompt := tview.NewTextView().SetDynamicColors(true)
		setPrompt := func(focused bool) {
			if focused {
				prompt.SetText(d.tag(roleAccent, pointer+" "+q.Prompt))
			} else {
				prompt.SetText("  " + tview.Escape(q.Prompt))
			}
		}
		setPrompt(len(lists) == 0)
		body.AddItem(prompt, 1, 0, false)
		list := newChoiceList(d, func(l *choiceList) {
			for _, o := range q.Options {
				on := u.w.Has(q.ID, o.Value)
				mark := d.radio(on)
				if q.Multi {
					mark = "[ ]"
					if on {
						mark = "[X]"
					}
				}
				label := fmt.Sprintf("    %s %-26s", tview.Escape(mark), tview.Escape(o.Label))
				if !on {
					label = d.tag(roleDim, fmt.Sprintf("    %s %-26s", mark, o.Label))
				}
				note := u.w.Note(q, o)
				switch {
				case note == "":
				case strings.HasPrefix(note, "found"):
					label += " " + d.tag(roleOK, note)
				default:
					label += " " + d.tag(roleDim, note)
				}
				l.addTagged(label)
			}
		}, nil)
		list.SetFocusFunc(func() { setPrompt(true) })
		list.SetBlurFunc(func() { setPrompt(false) })
		list.toggle = func(row int) {
			before := len(u.w.Questions(step))
			u.w.Set(q, q.Options[row].Value)
			if len(u.w.Questions(step)) != before { // a follow-up question appeared or went
				fi := u.fi
				u.questions(step)
				if fi < len(u.focus) {
					u.fi = fi
					u.app.SetFocus(u.focus[fi])
				}
			}
		}
		body.AddItem(list, len(q.Options), 0, len(lists) == 0)
		body.AddItem(nil, 1, 0, false)
		lists = append(lists, list)
	}
	keys := "ENTER=Continue SPACE=Select ESC=Back F3=Exit"
	if len(lists) > 1 {
		keys = "ENTER=Continue SPACE=Select TAB=Next ESC=Back F3=Exit"
	}
	u.screen(u.stepTitle(u.step), body, keys, u.nav, lists...)
}

// recommend lists the items with their cost; the panel below describes the highlighted
// one, so long descriptions never get cut off.
func (u *ui) recommend() {
	d := u.d
	var lines []wizard.Line
	meter := tview.NewTextView().SetDynamicColors(true)
	desc := text("")
	refresh := func() {
		used, max := u.w.Budget()
		meter.SetText(fmt.Sprintf("Always loaded: ~%d tokens of %d budget  %s", used, max,
			d.tag(d.meterRole(used, max), d.meter(used, max, 20))))
	}
	describe := func(i int) {
		if i < 0 || i >= len(lines) {
			return
		}
		l := lines[i]
		verdict := d.tag(roleOK, "Recommended:")
		if !l.Recommended {
			verdict = d.tag(roleDim, "Not recommended:")
		}
		desc.SetText(d.bold(l.Item.Summary) + "\n" + verdict + " " + tview.Escape(l.Reason) + "\n" +
			d.tag(roleDim, "F1 shows the rule, source and audit."))
	}
	list := newChoiceList(d, func(l *choiceList) {
		lines = u.w.Lines()
		for _, ln := range lines {
			mark := "[ ]"
			if ln.On {
				mark = "[X]"
			}
			cost := "-"
			if ln.Cost > 0 {
				cost = fmt.Sprintf("~%d tok", ln.Cost)
			}
			row := fmt.Sprintf(" %s %-20s %-24s %9s", mark, ln.Item.ID, kindLabel(ln.Item.Kind, ln.Item.ExplicitOnly), cost)
			if ln.On {
				l.addTagged(tview.Escape(row))
			} else {
				l.addTagged(d.tag(roleDim, row))
			}
		}
	}, nil)
	list.toggle = func(row int) { u.w.Toggle(lines[row].Item.ID); refresh() }
	list.SetChangedFunc(func(i int, _, _ string, _ rune) { describe(i) })
	refresh()
	describe(0)
	sep := tview.NewTextView().SetText(d.rule(winWidth - 6))
	sep.SetTextColor(tview.Styles.SecondaryTextColor)
	if d.Color {
		sep.SetTextColor(tcell.ColorSilver)
	}
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).AddItem(sep, 1, 0, false).AddItem(desc, 3, 0, false).
		AddItem(nil, 1, 0, false).AddItem(meter, 1, 0, false)
	u.screen(u.stepTitle(4), body, "ENTER=Continue SPACE=Toggle F1=Why? ESC=Back F3=Exit",
		func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == tcell.KeyF1 && len(lines) > 0 {
				u.why(lines[list.GetCurrentItem()], list.GetCurrentItem())
				return nil
			}
			return u.nav(e)
		}, list)
}

func kindLabel(kind string, explicit bool) string {
	label := map[string]string{"instructions": "instructions", "hook": "hook", "rules": "Codex rules",
		"agents": "subagents", "skill": "skill", "mcp": "MCP server", "plugin": "plugin"}[kind]
	if explicit {
		label += " (call by name)"
	}
	return label
}

// why explains one item: the recommendation, the rules behind it, source and cost.
func (u *ui) why(l wizard.Line, row int) {
	d, it := u.d, l.Item
	var b strings.Builder
	field := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", d.tag(roleAccent, fmt.Sprintf("%-11s", label)), value)
	}
	if l.Recommended {
		b.WriteString(d.tag(roleOK, "Recommended") + "  " + tview.Escape(l.Reason) + "\n\n")
	} else {
		b.WriteString(d.tag(roleDim, "Not recommended") + "  " + tview.Escape(l.Reason) + "\n\n")
	}
	field("What", tview.Escape(it.Summary))
	field("Kind", tview.Escape(kindLabel(it.Kind, it.ExplicitOnly)))
	if it.RecommendWhen != nil {
		field("Rule", "recommend when "+tview.Escape(it.RecommendWhen.String()))
	}
	if it.NeverWhen != nil {
		field("", "never when "+tview.Escape(it.NeverWhen.String()))
	}
	if it.Source.Repo != "" {
		field("Source", tview.Escape(fmt.Sprintf("%s@%s", it.Source.Repo, it.Source.Commit[:7]))+d.tag(roleDim, " "+it.Source.License))
	}
	if it.Audit != "" {
		field("Audit", tview.Escape(it.Audit))
	}
	field("Cost", fmt.Sprintf("~%d tokens in every session", l.Cost))
	u.screen("Why "+it.ID+"?", text(b.String()), "ESC=Back", func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyF1 {
			u.recommend()
			if list, ok := u.focus[0].(*choiceList); ok {
				list.SetCurrentItem(row)
			}
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

var actionRole = map[string]string{"create": roleOK, "change": roleWarn, "run": roleInfo}

func (u *ui) review() {
	d := u.d
	p, err := u.w.Profile()
	if err != nil {
		u.fail(err)
		return
	}
	changes := text("")
	changes.SetWrap(false).SetScrollable(true)
	saveRow := newChoiceList(d, func(l *choiceList) {
		mark := "[ ]"
		if u.save {
			mark = "[X]"
		}
		l.add(fmt.Sprintf("%s Save my answers to %s", mark, u.tilde(p.Path)))
	}, func(int) { u.save = !u.save })
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(changes, 0, 1, false).AddItem(nil, 1, 0, false).AddItem(saveRow, 1, 0, true)
	ready := false
	u.screen(u.stepTitle(5), body, "ENTER=Install SPACE=Toggle TAB=Scroll ESC=Back F3=Exit",
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
	stop := u.spin(changes, "Working out what will change...")
	go func() {
		plan, err := u.cfg.Plan(p)
		stop()
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			if err != nil {
				u.fail(err)
				return
			}
			var b strings.Builder
			counts := map[string]int{}
			for _, c := range plan {
				counts[c.Action]++
			}
			if len(plan) == 0 {
				b.WriteString(d.tag(roleOK, "Everything is already in place.") + " Installing changes nothing.\n")
			} else {
				fmt.Fprintf(&b, "%s to create, %s to change, %s to run\n"+d.tag(roleDim, "Each changed file is backed up first.")+"\n\n",
					d.tag(roleOK, fmt.Sprint(counts["create"])), d.tag(roleWarn, fmt.Sprint(counts["change"])), d.tag(roleInfo, fmt.Sprint(counts["run"])))
			}
			for _, c := range plan {
				what := c.What
				if c.Action != "run" {
					what = u.tilde(what)
				}
				fmt.Fprintf(&b, "%s %s\n", d.tag(actionRole[c.Action], fmt.Sprintf("%-7s", c.Action)), tview.Escape(what))
			}
			changes.SetText(b.String()).ScrollToBeginning()
			ready = true
		})
	}()
}

// install runs the engine with a spinner, then shows what was done and what is left.
func (u *ui) install(p *profile.Profile) {
	status := tview.NewTextView().SetDynamicColors(true)
	out := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	out.SetChangedFunc(func() { out.ScrollToEnd() })
	body := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(status, 2, 0, false).AddItem(out, 0, 1, false)
	u.screen("Installing", body, "Please wait...", nil)
	u.setStrip(5)
	u.busy = true
	stop := u.spin(status, "Installing...")
	go func() {
		err := u.cfg.Apply(p, u.save, writerFunc(func(b []byte) {
			u.mu.Lock()
			u.log.Write(b)
			u.mu.Unlock()
			s := string(b)
			u.app.QueueUpdateDraw(func() { fmt.Fprint(out, u.d.tag(roleDim, s)) })
		}))
		stop()
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			u.result.Installed = err == nil
			u.result.Err = err
			u.mu.Lock()
			log := u.log.String()
			u.mu.Unlock()
			u.done(log, err)
		})
	}()
}

// done is the result screen: the outcome, the steps left for the user, the backup, then
// the full log to scroll through.
func (u *ui) done(log string, err error) {
	d := u.d
	var todo []string
	backup := ""
	for _, l := range strings.Split(log, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "To do: "):
			todo = append(todo, strings.TrimPrefix(l, "To do: "))
		case strings.HasPrefix(l, "Backup: "):
			backup = u.tilde(strings.TrimPrefix(l, "Backup: "))
		}
	}
	var b strings.Builder
	title := "Setup complete"
	if err != nil {
		title = "Setup failed"
		b.WriteString(d.tag(roleBad, "Setup stopped: "+err.Error()) + "\n")
		b.WriteString("Files changed so far are backed up; " + d.bold("agtk undo") + " restores them.\n\n")
	} else {
		b.WriteString(d.tag(roleOK, "Claude Code and Codex are set up.") + " Start new sessions to use it.\n\n")
		if len(todo) > 0 {
			b.WriteString(d.tag(roleAccent, "Still to do") + "\n")
			for _, t := range todo {
				b.WriteString("  - " + tview.Escape(t) + "\n")
			}
			b.WriteString("\n")
		}
		if backup != "" {
			b.WriteString(d.tag(roleAccent, "Backup") + "  " + tview.Escape(backup) + d.tag(roleDim, "  (agtk undo restores it)") + "\n")
		}
	}
	s := strings.TrimRight(b.String(), "\n") + "\n"
	summary := text(s)
	lines := strings.Count(s, "\n") + 1 // and one blank row before the log
	logView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetText(d.tag(roleDim, log))
	logRule := tview.NewTextView().SetDynamicColors(true).SetText(d.tag(roleDim, "Log "+d.rule(winWidth-10)))
	body := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(summary, lines, 0, false).
		AddItem(logRule, 1, 0, false).AddItem(logView, 0, 1, true)
	u.window.Clear()
	u.window.AddItem(frame(title, body), 0, 1, true)
	u.setKeys("ENTER=Exit UP/DOWN=Scroll")
	u.app.SetFocus(logView)
	u.onKey = func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEnter || e.Key() == tcell.KeyEscape {
			u.app.Stop()
			return nil
		}
		return e
	}
}

func (u *ui) fail(err error) {
	u.result.Err = err
	u.screen("Error", text(u.d.tag(roleBad, err.Error())+"\n\nNothing was written."), "ESC=Back F3=Exit",
		func(e *tcell.EventKey) *tcell.EventKey {
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
