package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The few widgets the screens use, kept behind these helpers so that moving off tview
// later only touches this package.

// frame is a bordered window with a title.
func frame(title string, body tview.Primitive) *tview.Flex {
	f := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(body, 0, 1, true)
	f.SetBorder(true).SetTitle(" "+title+" ").SetBorderPadding(1, 0, 2, 2)
	return f
}

// text is a wrapped text view; s may hold colour tags, so callers escape user content.
func text(s string) *tview.TextView {
	return tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true).SetText(s)
}

// choiceList is a list for a radio group or a checklist: rows are redrawn by fill, and
// SPACE calls toggle with the current row.
type choiceList struct {
	*tview.List
	fill   func(*choiceList)
	toggle func(row int)
}

func newChoiceList(d Display, fill func(*choiceList), toggle func(int)) *choiceList {
	l := &choiceList{List: tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true).SetSelectedFocusOnly(true), fill: fill, toggle: toggle}
	bar := d.barStyle()
	fg, bg, _ := bar.Decompose()
	l.SetSelectedTextColor(fg).SetSelectedBackgroundColor(bg)
	if !d.Color {
		l.SetSelectedStyle(bar)
	}
	l.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyRune && e.Rune() == ' ' {
			l.toggle(l.GetCurrentItem())
			l.redraw()
			return nil
		}
		return e
	})
	l.redraw()
	return l
}

func (l *choiceList) redraw() {
	cur := l.GetCurrentItem()
	l.Clear()
	l.fill(l)
	if cur < l.GetItemCount() {
		l.SetCurrentItem(cur)
	}
}

// add appends a row; text is escaped so marks like [X] are not read as style tags.
func (l *choiceList) add(s string) { l.AddItem(tview.Escape(s), "", 0, nil) }

// addTagged appends a row whose text already carries colour tags (content escaped).
func (l *choiceList) addTagged(s string) { l.AddItem(s, "", 0, nil) }

// keyBar is the bottom line with the key hints.
func keyBar(d Display) *tview.TextView {
	t := tview.NewTextView()
	fg, bg, _ := d.barStyle().Decompose()
	t.SetTextColor(fg).SetBackgroundColor(bg)
	if !d.Color {
		t.SetTextStyle(d.barStyle())
	}
	return t
}
