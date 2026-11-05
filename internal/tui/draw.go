package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/willlovecode/agent-config/internal/wizard"
)

// Screen layout in the 80x25 frame, after Turbo Vision and the MS-DOS editor: a title
// bar, a step list on the left (as classic installers have), the current step in a
// double-line dialog with a shaded drop shadow and Back/Next buttons, and a key bar.
const (
	sideX, sideW   = 1, 20
	dlgX, dlgW     = 23, 54
	bodyTop        = 2
	sideH, dlgH    = 21, 20
	innerW         = dlgW - 4 // usable width inside the dialog
	contentTop     = 5        // rows inside the dialog taken by the mascot and its bubble
	buttonRowInset = 2        // buttons sit this many rows above the dialog's bottom edge
)

// seg is a run of text in one style; a line is a list of segments, drawn from column x
// inside the dialog. A focused line is shown in reverse video across the full width,
// and a line with an action can be clicked.
type seg struct {
	text  string
	style tcell.Style
}

type line struct {
	x     int
	segs  []seg
	focus bool
	act   func()
}

// hit is a clickable screen area recorded while drawing.
type hit struct {
	x, y, w int
	act     func()
}

func (u *ui) put(x, y int, s string, st tcell.Style) int {
	w, _ := u.s.Size()
	for _, r := range s {
		if x >= w {
			break
		}
		if r == 0xa0 {
			r = 0x20
		}
		u.s.SetContent(x, y, r, nil, st)
		x++
	}
	return x
}

// putLine draws l at (x, y), padding a focused line to width so the bar spans it.
func (u *ui) putLine(x, y, width int, l line) {
	start := x
	x += l.x
	for _, sg := range l.segs {
		st := sg.style
		if l.focus {
			st = st.Reverse(true)
		}
		room := start + width - x
		text := []rune(sg.text)
		if room <= 0 {
			break
		}
		if len(text) > room {
			text = append(text[:room-1], []rune(u.ellipsis())...)
		}
		x = u.put(x, y, string(text), st)
	}
	if l.focus {
		for ; x < start+width; x++ {
			u.s.SetContent(x, y, ' ', nil, u.pal.base.Reverse(true))
		}
	}
	if l.act != nil {
		u.hits = append(u.hits, hit{start, y, width, l.act})
	}
}

func (u *ui) tilde(p string) string {
	if u.cfg.Home != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(u.cfg.Home)) {
		return "~" + filepath.ToSlash(p[len(u.cfg.Home):])
	}
	return p
}

// frame draws a box; double lines for the active dialog, single for panels.
func (u *ui) frame(x, y, w, h int, title string, double bool, st tcell.Style) {
	g := u.g
	tl, tr, bl, br, hz, vt := g.tl, g.tr, g.bl, g.br, g.h, g.v
	if double && !u.d.ascii() {
		tl, tr, bl, br, hz, vt = "╔", "╗", "╚", "╝", "═", "║"
	}
	if double && u.d.ascii() {
		hz = "="
	}
	for row := 1; row < h-1; row++ {
		u.put(x+1, y+row, strings.Repeat(" ", w-2), u.pal.base)
		u.put(x, y+row, vt, st)
		u.put(x+w-1, y+row, vt, st)
	}
	top := tl + strings.Repeat(hz, w-2) + tr
	u.put(x, y, top, st)
	if title != "" {
		u.put(x+2, y, " "+title+" ", u.pal.strong)
	}
	u.put(x, y+h-1, bl+strings.Repeat(hz, w-2)+br, st)
}

// shadow shades the cells right of and below a box, the way DOS dialogs cast one.
func (u *ui) shadow(x, y, w, h int) {
	shade := "░"
	if u.d.ascii() {
		shade = ":"
	}
	for row := y + 1; row <= y+h; row++ {
		u.put(x+w, row, shade+shade, u.pal.dim)
	}
	u.put(x+2, y+h, strings.Repeat(shade, w-2), u.pal.dim)
}

func (u *ui) draw() {
	s := u.s
	s.Clear()
	u.hits = nil
	w, h := s.Size()
	u.small = w < MinWidth || h < MinHeight
	if u.small {
		for i, l := range []string{
			fmt.Sprintf("The window is %dx%d; setup needs at least %dx%d.", w, h, MinWidth, MinHeight),
			"Make the window larger, or press P for plain line mode.",
			"Q or ESC exits. Nothing has been written.",
		} {
			u.put(0, i, l, u.pal.base)
		}
		return
	}
	ox := (w - MinWidth) / 2 // centre the 80-column layout on wider screens
	u.titleBar(w)
	u.sidebar(ox+sideX, bodyTop)
	u.dialog(ox+dlgX, bodyTop)
	u.keyBar(w, h)
	switch u.overlay {
	case helpOverlay:
		u.help(w, h)
	case quitOverlay:
		u.box(w, h, "Quit setup?", []line{
			{segs: []seg{{"Nothing has been written yet.", u.pal.base}}},
			{},
			{segs: []seg{{"[ Y ] Quit", u.pal.strong}, {"     ", u.pal.base}, {"[ N ] Keep going", u.pal.strong}}},
		})
	case infoOverlay:
		u.info(w, h)
	}
}

// titleBar is one reverse-video line, like the menu bar of the MS-DOS editor.
func (u *ui) titleBar(w int) {
	bar := u.pal.base.Reverse(true)
	u.put(0, 0, strings.Repeat(" ", w), bar)
	u.put(1, 0, "agtk setup", bar.Bold(true))
	u.put(13, 0, "v"+u.cfg.Version, bar)
	right := "F1 Help   F10 Quit"
	u.put(w-len(right)-2, 0, right, bar)
	u.hits = append(u.hits, hit{w - len(right) - 2, 0, 7, func() { u.overlay = helpOverlay }},
		hit{w - 9 - 2, 0, 8, func() { u.overlay = quitOverlay }})
}

// sidebar: the pixel logo, then every step: done ones ticked (click to go back), the
// current one marked, later ones dim.
func (u *ui) sidebar(x, y int) {
	u.frame(x, y, sideW, sideH, "", false, u.pal.dim)
	lg := logo
	if u.d.ascii() {
		lg = logoASCII
	}
	for i, l := range lg {
		u.put(x+2, y+1+i, l, u.pal.accent)
	}
	u.put(x+2, y+4, "setup", u.pal.dim)
	ps := u.pages()
	tick, here := "✓", "▸"
	if u.d.ascii() {
		tick, here = "+", ">"
	}
	for i, p := range ps {
		row := y + 6 + i
		name := stepName(p)
		switch {
		case i < u.cur || (u.finished && u.result.Err == nil):
			u.put(x+2, row, tick+" "+name, u.pal.base)
			if !u.installing && !u.finished {
				i := i
				u.hits = append(u.hits, hit{x + 1, row, sideW - 2, func() { u.cur = i }})
			}
		case i == u.cur:
			u.putLine(x+1, row, sideW-2, line{x: 1, segs: []seg{{here + " " + name, u.pal.strong}}, focus: true})
		default:
			u.put(x+2, row, "  "+name, u.pal.dim)
		}
	}
	u.put(x+2, y+sideH-2, fmt.Sprintf("step %d of %d", u.cur+1, len(ps)), u.pal.dim)
}

func stepName(p page) string {
	switch p.kind {
	case pageScan:
		return "Welcome"
	case pageItems:
		return "Pick items"
	case pageReview:
		return "Review"
	case pageInstall:
		return "Install"
	}
	return map[string]string{"use": "Who it's for", "tools": "Agent tools", "allow_external_mcp": "Outside help",
		"stacks": "Languages", "work": "Kind of work"}[p.id]
}

// dialog is the current step: the mascot asks, the content answers, buttons go on.
func (u *ui) dialog(x, y int) {
	p := u.current()
	u.shadow(x, y, dlgW, dlgH)
	u.frame(x, y, dlgW, dlgH, stepName(p), true, u.pal.base)

	m, say := u.mood()
	for i, l := range mascot(m, u.d.ascii(), u.tick%23 == 0) {
		u.put(x+2, y+1+i, l, u.pal.accent)
	}
	u.bubble(x+11, y+1, innerW-9, say)

	top := y + contentTop
	bottom := y + dlgH - buttonRowInset - 1
	for i, l := range u.content(p, bottom-top) {
		u.putLine(x+2, top+i, innerW, l)
	}
	u.buttons(x, y+dlgH-buttonRowInset, p)
}

// bubble is a three-row speech bubble pointing left at the mascot; long text wraps to
// two lines inside it.
func (u *ui) bubble(x, y, w int, say string) {
	g := u.g
	lines := wrap(say, w-4)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	h := len(lines) + 2
	bw := 0
	for _, l := range lines {
		bw = max(bw, len([]rune(l))+4)
	}
	u.put(x, y, g.tl+strings.Repeat(g.h, bw-2)+g.tr, u.pal.dim)
	for i, l := range lines {
		u.put(x, y+1+i, g.v+" "+l+strings.Repeat(" ", bw-4-len([]rune(l)))+" "+g.v, u.pal.dim)
		u.put(x+2, y+1+i, l, u.pal.base)
	}
	u.put(x, y+h-1, g.bl+strings.Repeat(g.h, bw-2)+g.br, u.pal.dim)
	point := "◂"
	if u.d.ascii() {
		point = "<"
	}
	u.put(x-1, y+1, point, u.pal.dim)
}

func wrap(s string, w int) []string {
	var out []string
	cur := ""
	for _, word := range strings.Split(s, " ") { // not Fields: it would split on nbsp too
		if word == "" {
			continue
		}
		if cur != "" && len([]rune(cur))+1+len([]rune(word)) > w {
			out = append(out, cur)
			cur = word
			continue
		}
		if cur != "" {
			cur += " "
		}
		cur += word
	}
	return append(out, cur)
}

// buttons: Back on the left, the main action on the right, each with its key printed on
// it so the mouse and the keyboard teach each other.
func (u *ui) buttons(x, y int, p page) {
	type button struct {
		label string
		on    bool
		act   func()
	}
	back := button{"< Back  Esc", u.cur > 0 && !u.installing && !u.finished, u.back}
	next := button{"Next >  Enter", !u.installing, u.next}
	switch {
	case u.finished:
		next = button{"Exit  Enter", true, func() { u.quit = true }}
	case u.installing:
		next = button{"Installing...", false, nil}
	case p.kind == pageReview:
		next = button{"Install  Enter", !u.planning && u.planErr == nil, u.next}
	}
	draw := func(bx int, b button) {
		label := "[ " + b.label + " ]"
		st := u.pal.strong
		if !b.on {
			st = u.pal.dim
		}
		u.put(bx, y, label, st)
		if b.on && b.act != nil {
			u.hits = append(u.hits, hit{bx, y, len([]rune(label)), b.act})
		}
	}
	draw(x+3, back)
	nl := len([]rune("[ " + next.label + " ]"))
	draw(x+dlgW-3-nl, next)
}

func (u *ui) mood() (mood, string) {
	p := u.current()
	switch {
	case u.finished && u.result.Err != nil:
		return moodOops, speech["failed"]
	case u.finished:
		return moodHappy, speech["done"]
	case u.installing || (p.kind == pageReview && u.planning):
		return moodWork, speech["install"]
	case p.kind == pageScan:
		return moodHi, speech["scan"]
	case p.kind == pageItems:
		return moodThink, speech["items"]
	}
	if s, ok := speech[p.id]; ok {
		return moodThink, s
	}
	return moodThink, p.q.Prompt
}

// keyBar lists the keys for this step on the bottom line.
func (u *ui) keyBar(w, h int) {
	type kv struct{ k, v string }
	p := u.current()
	up := "↑↓"
	if u.d.ascii() {
		up = "Up/Dn"
	}
	var keys []kv
	switch {
	case u.finished:
		keys = []kv{{"Enter", "Exit"}}
	case u.installing:
		keys = []kv{{"", "Installing, please wait..."}}
	case p.kind == pageScan:
		keys = []kv{{"Enter", "Next"}, {"F1", "Help"}, {"F10", "Quit"}}
	case p.kind == pageQuestion && p.q.Multi:
		keys = []kv{{up, "Move"}, {"Space", "Tick"}, {"Enter", "Next"}, {"Esc", "Back"}, {"F1", "Help"}, {"F10", "Quit"}}
	case p.kind == pageQuestion:
		keys = []kv{{up, "Move"}, {"1-" + fmt.Sprint(len(p.q.Options)), "Pick"}, {"Enter", "Choose"}, {"Esc", "Back"}, {"F1", "Help"}}
	case p.kind == pageItems:
		keys = []kv{{up, "Move"}, {"Space", "Tick"}, {"I", "Info"}, {"Enter", "Next"}, {"Esc", "Back"}, {"F1", "Help"}}
	case p.kind == pageReview:
		keys = []kv{{up, "Scroll"}, {"Space", "Save on/off"}, {"Enter", "Install"}, {"Esc", "Back"}}
	}
	x := 1
	for i, k := range keys {
		if i > 0 {
			x = u.put(x, h-1, "  ", u.pal.dim)
		}
		if k.k != "" {
			x = u.put(x, h-1, k.k, u.pal.strong)
			x = u.put(x, h-1, " ", u.pal.dim)
		}
		x = u.put(x, h-1, k.v, u.pal.dim)
	}
}

// content is the body of the current step, at most budget lines, x from the dialog edge.
func (u *ui) content(p page, budget int) []line {
	pal, g := u.pal, u.g
	at := func(segs ...seg) line { return line{segs: segs} }
	var out []line
	switch p.kind {
	case pageScan:
		s := u.w.Scan
		var found, missing []string
		seen := map[string]bool{}
		for _, pr := range s.Programs {
			if seen[pr.Fact] {
				continue
			}
			seen[pr.Fact] = true
			if v := s.Found(pr.Fact); v != "" {
				found = append(found, pr.Name+nbsp+v)
			} else {
				missing = append(missing, pr.Name)
			}
		}
		row := func(k, v string, st tcell.Style) {
			for i, l := range wrap(v, innerW-10) {
				label := ""
				if i == 0 {
					label = k
				}
				out = append(out, at(seg{fmt.Sprintf("%-10s", label), pal.dim}, seg{l, st}))
			}
		}
		row("System", fmt.Sprintf("%s/%s, %d threads, %.0f GB RAM", s.OS, s.Arch, s.CPUs, s.RAMGB), pal.base)
		row("Found", strings.Join(found, ", "), pal.base) // nbsp keeps "name version" together
		if len(missing) > 0 {
			row("Missing", strings.Join(missing, ", "), pal.dim)
		}
		if s.Tier == "B" {
			row("Hooks", "blocked by policy; rules are used instead", pal.warn)
		} else {
			row("Hooks", "allowed", pal.base)
		}
		out = append(out, line{}, at(seg{"Nothing is written until the last step.", pal.dim}))

	case pageQuestion:
		c := u.cursor[p.id]
		for i, o := range p.q.Options {
			i := i
			on := u.w.Has(p.q.ID, o.Value)
			mark := g.radioOff
			if p.q.Multi {
				mark = g.off
			}
			if on {
				mark = g.radioOn
				if p.q.Multi {
					mark = g.on
				}
			}
			l := at(seg{fmt.Sprintf(" %d  ", i+1), pal.dim}, seg{mark + " ", pal.base}, seg{fmt.Sprintf("%-28s", o.Label), pal.base})
			if note := u.w.Note(p.q, o); note != "" {
				l.segs = append(l.segs, seg{note, pal.dim})
			}
			l.focus = i == c
			l.act = func() { u.cursor[p.id] = i; u.pick(p, i) }
			out = append(out, l)
		}

	case pageItems:
		lines := u.w.Lines()
		listRows := budget - 4
		c := u.cursor[p.id]
		first := u.window(p.id, c, len(lines), listRows)
		for i := first; i < len(lines) && i < first+listRows; i++ {
			i := i
			l := lines[i]
			mark, st := g.off, pal.dim
			if l.On {
				mark, st = g.on, pal.base
			}
			cost := ""
			if l.Cost > 0 {
				cost = fmt.Sprintf("~%d tok", l.Cost)
			}
			out = append(out, line{segs: []seg{{" " + mark + " ", st}, {fmt.Sprintf("%-19s", l.Item.ID), st},
				{fmt.Sprintf("%-17s", kindLabel(l.Item.Kind, l.Item.ExplicitOnly)), pal.dim}, {fmt.Sprintf("%9s", cost), pal.dim}},
				focus: i == c, act: func() { u.cursor[p.id] = i; u.pick(p, i) }})
		}
		more := ""
		if len(lines) > listRows {
			more = fmt.Sprintf("%d-%d of %d", first+1, min(first+listRows, len(lines)), len(lines))
		}
		out = append(out, at(seg{fmt.Sprintf("%*s", innerW, more), pal.dim}))
		if c < len(lines) {
			l := lines[c]
			verdict := "Recommended: "
			if !l.Recommended {
				verdict = "Not recommended: "
			}
			out = append(out, at(seg{l.Item.Summary, pal.strong}), at(seg{verdict + l.Reason, pal.dim}))
		}
		used, max := u.w.Budget()
		out = append(out, at(seg{"Context ", pal.dim}, seg{u.bar(used, max, 24), pal.base}, seg{fmt.Sprintf(" ~%d/%d tokens", used, max), pal.dim}))

	case pageReview:
		if u.planning {
			out = append(out, at(seg{u.spin() + " ", pal.accent}, seg{"Working out what will change...", pal.dim}))
			break
		}
		if u.planErr != nil {
			out = append(out, at(seg{"Can't install: " + u.planErr.Error(), pal.warn}), at(seg{"Nothing was written.", pal.dim}))
			break
		}
		counts := map[string]int{}
		for _, c := range u.plan {
			counts[c.Action]++
		}
		if len(u.plan) == 0 {
			out = append(out, at(seg{"Everything is already in place.", pal.base}))
		} else {
			out = append(out, at(seg{fmt.Sprintf("%d new, %d changed, %d commands. ", counts["create"], counts["change"], counts["run"]), pal.base},
				seg{"Backups first.", pal.dim}))
		}
		listRows := budget - 3
		c := u.cursor[p.id]
		first := u.window(p.id, min(c, len(u.plan)-1), len(u.plan), listRows)
		for i := first; i < len(u.plan) && i < first+listRows; i++ {
			ch := u.plan[i]
			what := ch.What
			if ch.Action != "run" {
				what = u.tilde(what)
			}
			ast := pal.dim
			if ch.Action == "change" {
				ast = pal.warn
			}
			out = append(out, at(seg{fmt.Sprintf(" %-7s", ch.Action), ast}, seg{what, pal.base}))
		}
		for len(out) < budget-1 {
			out = append(out, line{})
		}
		pr, _ := u.w.Profile()
		mark := g.off
		if u.save {
			mark = g.on
		}
		path := ""
		if pr != nil {
			path = u.tilde(pr.Path)
		}
		out = append(out, line{segs: []seg{{" " + mark + " Save answers to " + path, pal.base}},
			focus: c >= len(u.plan), act: func() { u.save = !u.save }})

	case pageInstall:
		u.mu.Lock()
		log := u.log.String()
		u.mu.Unlock()
		logLines := strings.Split(strings.TrimRight(log, "\n"), "\n")
		if u.installing {
			done, total := len(logLines), len(u.plan)+12
			done = min(done, total-1)
			out = append(out, at(seg{u.spin() + " ", pal.accent}, seg{u.bar(done, total, 30), pal.base},
				seg{fmt.Sprintf(" %d%%", done*100/total), pal.dim}), line{})
			out = append(out, tail(logLines, budget-2, at, pal.dim)...)
			break
		}
		out = append(out, u.summary(log, at)...)
		var rest []string // the log without the lines the summary already shows
		for _, l := range logLines {
			if !strings.HasPrefix(l, "To do: ") && !strings.HasPrefix(l, "Backup: ") {
				rest = append(rest, l)
			}
		}
		if n := budget - len(out) - 1; n > 0 {
			out = append(out, line{})
			out = append(out, tail(rest, n, at, pal.dim)...)
		}
	}
	if len(out) > budget {
		out = out[:budget]
	}
	return out
}

// summary leads the result: the outcome, what is left for the user, the backup.
func (u *ui) summary(log string, at func(...seg) line) []line {
	pal, g := u.pal, u.g
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
	if u.result.Err != nil {
		return []line{at(seg{"Setup stopped: " + u.result.Err.Error(), pal.warn}),
			at(seg{"Changed files are backed up; ", pal.dim}, seg{"agtk undo", pal.strong}, seg{" restores them.", pal.dim})}
	}
	out := []line{at(seg{g.heart + " ", pal.accent}, seg{"Setup complete.", pal.strong})}
	for _, t := range todo {
		out = append(out, at(seg{"To do   ", pal.dim}, seg{t, pal.base}))
	}
	if backup != "" {
		out = append(out, at(seg{"Backup  ", pal.dim}, seg{backup, pal.base}), at(seg{"        agtk undo restores it", pal.dim}))
	}
	return out
}

func tail(lines []string, n int, at func(...seg) line, st tcell.Style) []line {
	if n <= 0 {
		return nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var out []line
	for _, l := range lines {
		out = append(out, at(seg{strings.TrimRight(l, "\r"), st}))
	}
	return out
}

// window keeps the cursor row visible in a scrolled list.
func (u *ui) window(id string, cursor, n, rows int) int {
	first := u.scroll[id]
	if cursor < first {
		first = cursor
	}
	if cursor >= first+rows {
		first = cursor - rows + 1
	}
	if first > n-rows {
		first = n - rows
	}
	if first < 0 {
		first = 0
	}
	u.scroll[id] = first
	return first
}

func (u *ui) spin() string {
	f := spinPixel
	if u.d.ascii() {
		f = spinASCII
	}
	return f[u.tick%len(f)]
}

// bar is a block progress bar, like old installers drew.
func (u *ui) bar(used, max, width int) string {
	n := 0
	if max > 0 {
		n = used * width / max
	}
	n = min(n, width)
	full, empty := "█", "░"
	if u.d.ascii() {
		full, empty = "#", "."
	}
	return strings.Repeat(full, n) + strings.Repeat(empty, width-n)
}

func kindLabel(kind string, explicit bool) string {
	label := map[string]string{"instructions": "instructions", "hook": "hook", "rules": "codex rules",
		"agents": "subagents", "skill": "skill", "mcp": "mcp server", "plugin": "plugin"}[kind]
	if explicit {
		label += ", by name"
	}
	return label
}

// box is a centred dialog with a shadow, for help, quit and item details.
func (u *ui) box(w, h int, title string, lines []line) {
	bw := 30
	for _, l := range lines {
		n := 0
		for _, s := range l.segs {
			n += len([]rune(s.text))
		}
		bw = max(bw, n+6)
	}
	bw = min(bw, w-6)
	bh := len(lines) + 4
	x, y := (w-bw)/2, (h-bh)/2
	u.shadow(x, y, bw, bh)
	u.frame(x, y, bw, bh, title, true, u.pal.base)
	for i, l := range lines {
		u.putLine(x+3, y+2+i, bw-6, l)
	}
}

func (u *ui) help(w, h int) {
	pal := u.pal
	up, back := "Up Down  or  K J", "Esc  Left  Backspace"
	row := func(k, v string) line {
		return line{segs: []seg{{fmt.Sprintf("%-22s", k), pal.strong}, {v, pal.base}}}
	}
	u.box(w, h, "Keys", []line{
		row(up, "move"),
		row("Space", "tick or untick"),
		row("Enter", "choose and go on"),
		row(back, "go back a step"),
		row("1 - 9", "pick an answer directly"),
		row("I", "details of an item"),
		row("Mouse", "click answers, buttons, done steps"),
		row("F1  ?", "this help"),
		row("F10  Q", "quit (nothing written before Install)"),
		{},
		{segs: []seg{{"Any key closes this.", pal.dim}}},
	})
}

// info explains the highlighted item: why, the rules, source, audit and cost.
func (u *ui) info(w, h int) {
	ls := u.w.Lines()
	c := u.cursor["items"]
	if c >= len(ls) {
		u.overlay = noOverlay
		return
	}
	var l wizard.Line = ls[c]
	pal, it := u.pal, l.Item
	row := func(k, v string) line { return line{segs: []seg{{fmt.Sprintf("%-8s", k), pal.dim}, {v, pal.base}}} }
	verdict := "recommended"
	if !l.Recommended {
		verdict = "not recommended"
	}
	lines := []line{{segs: []seg{{it.Summary, pal.strong}}}, {}}
	for i, s := range wrap(verdict+": "+l.Reason, 56) {
		k := ""
		if i == 0 {
			k = "Why"
		}
		lines = append(lines, row(k, s))
	}
	lines = append(lines, row("Kind", kindLabel(it.Kind, it.ExplicitOnly)))
	if it.RecommendWhen != nil {
		lines = append(lines, row("Rule", "recommend when "+it.RecommendWhen.String()))
	}
	if it.NeverWhen != nil {
		lines = append(lines, row("", "never when "+it.NeverWhen.String()))
	}
	if it.Source.Repo != "" {
		lines = append(lines, row("Source", fmt.Sprintf("%s@%s, %s", it.Source.Repo, it.Source.Commit[:7], it.Source.License)))
	}
	if it.Audit != "" {
		for i, s := range wrap(it.Audit, 56) {
			k := ""
			if i == 0 {
				k = "Audit"
			}
			lines = append(lines, row(k, s))
		}
	}
	lines = append(lines, row("Cost", fmt.Sprintf("~%d tokens in every session", l.Cost)), line{},
		line{segs: []seg{{"Any key closes this.", pal.dim}}})
	u.box(w, h, it.ID, lines)
}

func (u *ui) ellipsis() string {
	if u.d.ascii() {
		return "~"
	}
	return "…"
}

// nbsp joins words that wrap must keep on one line; put shows it as a space.
const nbsp = " "
