package screen

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

// Styles are carried in the text as marker bytes and turned into escape sequences (or
// dropped) last, so wrapping and centering never count them.
const (
	mBold  = "\x01"
	mDim   = "\x02"
	mReset = "\x03"
)

var markers = strings.NewReplacer(mBold, "", mDim, "", mReset, "")

func bold(s string) string { return mBold + s + mReset }
func dim(s string) string  { return mDim + s + mReset }

// clean replaces control characters, which would break the layout or the markers.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func width(s string) int { return uniseg.StringWidth(markers.Replace(s)) }

// sgr turns the markers into escape sequences, or removes them when there is no colour.
func sgr(s string, color bool) string {
	if !color {
		return markers.Replace(s)
	}
	return strings.NewReplacer(mBold, "\x1b[1m", mDim, "\x1b[2m", mReset, "\x1b[0m").Replace(s)
}

// wrap breaks text into lines of at most w columns. Leading spaces carry over to the
// continuation lines, plus hang more.
func wrap(text string, w, hang int) []string {
	text = clean(text)
	indent := len(text) - len(strings.TrimLeft(text, " "))
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	w = max(w, indent+hang+8)
	var out []string
	line, lineW, fresh := strings.Repeat(" ", indent), indent, true // fresh: no word on the line yet
	newline := func() {
		out = append(out, line)
		line, lineW, fresh = strings.Repeat(" ", indent+hang), indent+hang, true
	}
	for _, word := range words {
		if !fresh && lineW+1+width(word) > w {
			newline()
		}
		for width(word) > w-lineW { // longer than a line: only happens on a fresh one
			var head string
			head, word = cut(word, w-lineW)
			line += head
			newline()
		}
		if !fresh {
			line, lineW = line+" ", lineW+1
		}
		line, lineW, fresh = line+word, lineW+width(word), false
	}
	return append(out, line)
}

// cut splits s after at most w columns.
func cut(s string, w int) (head, rest string) {
	end, used := 0, 0
	g := uniseg.NewGraphemes(s)
	for g.Next() && (used+g.Width() <= w || end == 0) {
		_, end = g.Positions()
		used += g.Width()
	}
	return s[:end], s[end:]
}

// truncate shortens s to w columns, ending in "..." when it was cut.
func truncate(s string, w int) string {
	s = clean(s)
	if width(s) <= w {
		return s
	}
	head, _ := cut(s, max(w-3, 0))
	return strings.TrimRight(head, " ") + "..."
}

func center(s string, w int) string {
	if pad := (w - width(s)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
