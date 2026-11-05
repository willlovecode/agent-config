package tui

import (
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Display is how the screen may draw: Unicode pixel art or ASCII, and whether colour is
// allowed. tcell prints "?" for characters it can't encode, so ASCII is chosen up front.
type Display struct {
	Lines string // "unicode" or "ascii" ("double" and "single" count as unicode)
	Color bool
}

// Detect picks the display for this terminal: ASCII when it can't show UTF-8 (or when
// forced), no colour when NO_COLOR is set.
func Detect(forceASCII bool) Display {
	d := Display{Lines: "unicode", Color: os.Getenv("NO_COLOR") == ""}
	if forceASCII || !utf8Terminal() {
		d.Lines = "ascii"
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

func (d Display) ascii() bool { return d.Lines == "ascii" }

func (d Display) g() glyphs {
	if d.ascii() {
		return asciiGlyphs
	}
	return unicodeGlyphs
}

// The palette is the terminal's own: default background and text, grey for anything
// secondary, and one soft accent for the cursor, the current step and the mascot.
// Without colour, the accent becomes bold and grey becomes dim.
type palette struct{ base, dim, accent, strong, cursor, warn tcell.Style }

func (d Display) palette() palette {
	base := tcell.StyleDefault
	if !d.Color {
		return palette{base: base, dim: base.Dim(true), accent: base.Bold(true), strong: base.Bold(true),
			cursor: base.Bold(true), warn: base.Bold(true)}
	}
	pink := tcell.NewRGBColor(255, 135, 175)
	return palette{
		base:   base,
		dim:    base.Foreground(tcell.ColorGray),
		accent: base.Foreground(pink),
		strong: base.Bold(true),
		cursor: base.Foreground(pink).Bold(true),
		warn:   base.Foreground(tcell.NewRGBColor(255, 175, 95)),
	}
}
