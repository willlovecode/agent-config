package screen

import (
	"strconv"
	"strings"
)

// Env is what a page is drawn for: the terminal size and whether it takes escape
// sequences for bold and dim text.
type Env struct {
	Width, Height int
	Color         bool
}

const (
	minWidth, minHeight = 60, 16
	maxContent          = 100
	keyIndent           = 7 // "  (1)  "
	maxKeys             = 9 // one keypress picks a row, so a page holds nine
)

type cmd struct {
	key   rune
	label string
	do    func()
}

// row is one line of a list, with the lines that belong under it.
type row struct {
	group    string
	text     string
	extra    []string
	numbered bool // takes a key 1-9 on its page
	name     string
	info     func() []string // what "?" shows
	do       func()
}

// page is one screen: everything a layout needs, and the keys that act on it.
type page struct {
	header   string
	title    []string // bold, centered
	hint     []string // centered
	rows     []row
	static   []string // verbatim lines after the rows; never paged
	cmds     []cmd
	nav      []cmd // back, restart, quit: after the paging keys
	enter    rune  // the command key Enter stands for
	enterRow int   // the row Enter picks when enter is 0; -1 for none
	compact  bool  // commands share lines
	message  string
	final    bool // the last screen: no height limit, no prompt
	noPrompt bool // no prompt line either
}

// laid is a page drawn for one size.
type laid struct {
	lines      []string // with style markers
	shown      []int    // the rows keys 1.. pick
	first      int      // the first row of this page
	pages      int
	next, prev int // the first row of the neighbouring pages, -1 for none
}

func contentWidth(env Env) int { return max(20, min(env.Width-1, maxContent)) }

// keys lists the commands of one page of a layout: the page's own, paging, then nav.
func (p *page) keys(paging, next, prev bool) []cmd {
	out := append([]cmd{}, p.cmds...)
	if paging && next {
		out = append(out, cmd{key: 'n', label: "Next page."})
	}
	if paging && prev {
		out = append(out, cmd{key: 'p', label: "Previous page."})
	}
	return append(out, p.nav...)
}

// layout draws the page. Commands get a line each, as in p10k, unless that would need
// more than one page.
func (p *page) layout(env Env, start int) laid {
	lay := p.layoutAs(env, start, p.compact)
	if !p.compact && lay.pages > 1 {
		lay = p.layoutAs(env, start, true)
	}
	if len(lay.lines) > env.Height && !p.final && len(p.hint) > 0 { // no room for the hint
		bare := *p
		bare.hint = nil
		lay = bare.layoutAs(env, start, p.compact || lay.pages > 1)
	}
	return lay
}

func (p *page) layoutAs(env Env, start int, compact bool) laid {
	cw := contentWidth(env)
	var top []string
	for _, t := range p.title {
		for _, l := range wrap(t, cw-4, 0) {
			top = append(top, bold(center(l, cw)))
		}
	}
	if len(p.hint) > 0 && (env.Height >= 26 || p.final) {
		top = append(top, "")
	}
	for _, h := range p.hint {
		for _, l := range wrap(h, cw-4, 0) {
			top = append(top, center(l, cw))
		}
	}
	lead, gap := 0, 0 // the header and its blank line, the blank line after the top
	if p.header != "" {
		lead = 2
	}
	if len(top) > 0 {
		gap = 1
	}
	room := func(paging bool) int {
		if p.final {
			return 1 << 20
		}
		return env.Height - lead - len(top) - gap - len(p.bottom(cw, compact, p.keys(paging, true, true), "x"))
	}
	starts := paginate(p.rows, room(false))
	paging := len(starts) > 1
	if paging {
		starts = paginate(p.rows, room(true))
	}
	if start < 0 { // not paged yet: show the page with the default
		start = max(p.enterRow, 0)
	}
	cur := 0
	for i, st := range starts {
		if st <= start {
			cur = i
		}
	}
	lay := laid{next: -1, prev: -1}
	first, end := 0, len(p.rows)
	if len(starts) > 0 {
		first = starts[cur]
	}
	lay.first, lay.pages = first, len(starts)
	if cur+1 < len(starts) {
		end = starts[cur+1]
		lay.next = end
	}
	if cur > 0 {
		lay.prev = starts[cur-1]
	}

	var body []string
	group, enterOn := "", 0
	capacity := room(paging)
	for i := first; i < end; i++ {
		r := p.rows[i]
		heading := 0
		if r.group != "" && r.group != group {
			body = append(body, "  "+bold(r.group))
			heading = 1
		}
		group = r.group
		if i == first && !p.final && 1+len(r.extra)+heading > capacity { // a row taller than the page loses its tail
			keep := max(capacity-1-heading-1, 0)
			r.extra = append(append([]string{}, r.extra[:min(keep, len(r.extra))]...), dim("..."))
		}
		line := strings.TrimRight("  "+r.text, " ")
		if r.numbered {
			lay.shown = append(lay.shown, i)
			if i == p.enterRow {
				enterOn = len(lay.shown)
			}
			line = "  " + bold("("+strconv.Itoa(len(lay.shown))+")") + "  " + r.text
		}
		body = append(body, line)
		for _, e := range r.extra {
			body = append(body, strings.Repeat(" ", keyIndent)+e)
		}
	}

	header := p.header
	if paging {
		header += " | page " + strconv.Itoa(cur+1) + " of " + strconv.Itoa(len(starts))
	}
	cmds := p.keys(paging, lay.next >= 0, lay.prev >= 0)
	var lines []string
	if lead > 0 {
		lines = append(lines, center(dim(header), cw), "")
	}
	lines = append(lines, top...)
	if gap > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, body...)
	lines = append(lines, p.static...)
	lines = append(lines, p.bottom(cw, compact, cmds, p.promptLine(lay, cmds, enterOn))...)
	if p.final {
		lines = append(lines, "")
	}
	lay.lines = collapse(lines)
	return lay
}

// bottom is what follows the rows: commands, a message, the prompt.
func (p *page) bottom(cw int, compact bool, cmds []cmd, prompt string) []string {
	var out []string
	if len(cmds) > 0 {
		out = append(out, "")
		out = append(out, p.cmdLines(cmds, cw, compact)...)
		if !compact && !p.noPrompt && !p.final {
			out = append(out, "")
		}
	}
	if p.message != "" {
		for _, l := range wrap(p.message, cw-4, 0) {
			out = append(out, "  "+bold(l))
		}
	}
	if !p.noPrompt && !p.final {
		out = append(out, prompt)
	}
	return out
}

func (p *page) cmdLines(cmds []cmd, cw int, compact bool) []string {
	var out []string
	cur := ""
	for _, c := range cmds {
		text := bold("("+string(c.key)+")") + "  " + c.label
		switch {
		case !compact:
			out = append(out, "  "+text)
		case cur == "":
			cur = "  " + text
		case width(cur)+3+width(text) <= cw:
			cur += "   " + text
		default:
			out = append(out, cur)
			cur = "  " + text
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// promptLine is "Choice [12ynq]: ", listing exactly the keys that do something here.
func (p *page) promptLine(lay laid, cmds []cmd, enterOn int) string {
	var keys strings.Builder
	switch n := len(lay.shown); {
	case n == 1:
		keys.WriteString("1")
	case n == 2:
		keys.WriteString("12")
	case n > 2:
		keys.WriteString("1-" + strconv.Itoa(n))
	}
	for _, c := range cmds {
		keys.WriteRune(c.key)
	}
	out := "Choice [" + keys.String() + "]"
	switch {
	case p.enter != 0:
		out += " (Enter = " + string(p.enter) + ")"
	case enterOn > 0:
		out += " (Enter = " + strconv.Itoa(enterOn) + ")"
	case p.enterRow >= 0:
		out += " (Enter keeps the default)"
	}
	return out + ": "
}

// paginate returns the first row of each page. A page holds as many rows as fit in
// capacity lines, at most maxKeys numbered ones; a group heading costs a line each page.
func paginate(rows []row, capacity int) []int {
	var starts []int
	for i := 0; i < len(rows); {
		starts = append(starts, i)
		used, keys, group := 0, 0, ""
		j := i
		for ; j < len(rows); j++ {
			r := rows[j]
			h := 1 + len(r.extra)
			if r.group != "" && r.group != group {
				h++
			}
			if j > i && (used+h > capacity || (r.numbered && keys >= maxKeys)) {
				break
			}
			used += h
			group = r.group
			if r.numbered {
				keys++
			}
		}
		i = j
	}
	return starts
}

// collapse turns runs of blank lines into one, and drops a blank line at the very top.
func collapse(lines []string) []string {
	out := lines[:0:0]
	for _, l := range lines {
		if l == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, l)
	}
	return out
}
