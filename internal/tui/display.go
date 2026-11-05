package tui

import (
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Display is how the screen may draw: which frame lines, which marks, and whether colour
// is allowed. tcell prints "?" for box characters it can't encode, so the ASCII fallback
// has to be chosen up front.
type Display struct {
	Lines string // double, single or ascii
	Color bool
}

// Detect picks the display for this terminal: ASCII when it can't show UTF-8 (or when
// forced), single lines on the Linux console, no colour when NO_COLOR is set.
func Detect(forceASCII bool) Display {
	d := Display{Lines: "double", Color: os.Getenv("NO_COLOR") == ""}
	switch {
	case forceASCII || !utf8Terminal():
		d.Lines = "ascii"
	case os.Getenv("TERM") == "linux":
		d.Lines = "single"
	}
	return d
}

// utf8Locale is the check outside Windows: the locale must say UTF-8.
func utf8Locale() bool {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			v = strings.ToLower(v)
			return strings.Contains(v, "utf-8") || strings.Contains(v, "utf8")
		}
	}
	return false
}

func (d Display) apply() {
	b := &tview.Borders
	set := func(h, v, tl, tr, bl, br rune) {
		b.Horizontal, b.Vertical, b.TopLeft, b.TopRight, b.BottomLeft, b.BottomRight = h, v, tl, tr, bl, br
		b.HorizontalFocus, b.VerticalFocus = h, v
		b.TopLeftFocus, b.TopRightFocus, b.BottomLeftFocus, b.BottomRightFocus = tl, tr, bl, br
	}
	switch d.Lines {
	case "double":
		set('═', '║', '╔', '╗', '╚', '╝')
	case "single":
		set('─', '│', '┌', '┐', '└', '┘')
	default:
		set('-', '|', '+', '+', '+', '+')
	}
	s := &tview.Styles
	if d.Color {
		s.PrimitiveBackgroundColor = tcell.ColorNavy
		s.ContrastBackgroundColor = tcell.ColorNavy
		s.PrimaryTextColor = tcell.ColorWhite
		s.SecondaryTextColor = tcell.ColorYellow
		s.BorderColor = tcell.ColorWhite
		s.TitleColor = tcell.ColorYellow
	} else {
		s.PrimitiveBackgroundColor = tcell.ColorDefault
		s.ContrastBackgroundColor = tcell.ColorDefault
		s.PrimaryTextColor = tcell.ColorDefault
		s.SecondaryTextColor = tcell.ColorDefault
		s.BorderColor = tcell.ColorDefault
		s.TitleColor = tcell.ColorDefault
	}
}

func (d Display) radio(on bool) string {
	switch {
	case !on:
		return "( )"
	case d.Lines == "ascii":
		return "(*)"
	}
	return "(•)"
}

func (d Display) meter(used, max, width int) string {
	full, empty := "█", "░"
	if d.Lines == "ascii" {
		full, empty = "#", "."
	}
	n := 0
	if max > 0 {
		n = used * width / max
	}
	if n > width {
		n = width
	}
	return strings.Repeat(full, n) + strings.Repeat(empty, width-n)
}

func (d Display) rule(width int) string {
	switch d.Lines {
	case "double":
		return strings.Repeat("═", width)
	case "single":
		return strings.Repeat("─", width)
	}
	return strings.Repeat("=", width)
}

// styles for the selection bar and the key bar.
func (d Display) barStyle() tcell.Style {
	if d.Color {
		return tcell.StyleDefault.Background(tcell.ColorSilver).Foreground(tcell.ColorBlack)
	}
	return tcell.StyleDefault.Reverse(true)
}

func (d Display) screenStyle() tcell.Style {
	if d.Color {
		return tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
	}
	return tcell.StyleDefault
}

// Colour roles. On the blue screen: white for normal text, yellow for prompts and
// emphasis, grey for things that are off or secondary, green/yellow/cyan for what an
// action does, red for errors.
const (
	roleAccent = "yellow"
	roleDim    = "silver"
	roleOK     = "lime"
	roleWarn   = "yellow"
	roleInfo   = "aqua"
	roleBad    = "red"
)

// tag colours s for a text with dynamic colours on; without colour it is only escaped.
func (d Display) tag(role, s string) string {
	if !d.Color {
		return tview.Escape(s)
	}
	return "[" + role + "]" + tview.Escape(s) + "[-]"
}

// bold works with NO_COLOR too: it is emphasis, not colour.
func (d Display) bold(s string) string { return "[::b]" + tview.Escape(s) + "[::-]" }

// spinner frames: plain ASCII, as DOS installers drew them.
var spinner = []string{"|", "/", "-", `\`}

func (d Display) meterRole(used, max int) string {
	switch {
	case used > max:
		return roleBad
	case used*4 > max*3:
		return roleWarn
	}
	return roleOK
}

// shadow darkens the cells right of and below a window, like DOS setup screens.
func (d Display) shadow(s tcell.Screen, x, y, w, h int) {
	if !d.Color {
		return
	}
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)
	put := func(cx, cy int) {
		r, comb, _, _ := s.GetContent(cx, cy)
		s.SetContent(cx, cy, r, comb, style)
	}
	for cy := y + 1; cy <= y+h; cy++ {
		put(x+w, cy)
		put(x+w+1, cy)
	}
	for cx := x + 2; cx < x+w; cx++ {
		put(cx, y+h)
	}
}
