package plain

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const choicesPerPage = 9

func (s *session) menu(rows []menuRow, state menuState) {
	if state.filter != "" {
		s.lines("Filter: " + state.filter)
	}
	if len(rows) == 0 {
		s.lines("No choices in this view.")
	} else {
		s.menuPage(rows, state.page)
	}
	if s.key != nil {
		s.lines("(?) details  (/) filter")
	} else {
		s.lines("?N shows details. /text filters. / clears the filter.")
	}
}

func (s *session) menuPage(rows []menuRow, page int) {
	first := page * choicesPerPage
	last := min(first+choicesPerPage, len(rows))
	if len(rows) > choicesPerPage {
		s.lines(fmt.Sprintf("Choices %d-%d of %d", first+1, last, len(rows)))
	}
	number := 1
	if s.key == nil {
		number = first + 1
	}
	group := ""
	for i, row := range rows[first:last] {
		if row.group != group {
			group = row.group
			if group != "" {
				s.lines("  " + group)
			}
		}
		label := fmt.Sprintf("  %d) %s %s", number+i, row.mark, row.label)
		if row.note != "" {
			label += " - " + row.note
		}
		s.lines(label)
	}
	if last < len(rows) {
		s.lines("(n) next page")
	}
	if first > 0 {
		s.lines("(p) previous page")
	}
}

func (s *session) menuNumbers(answer string, state menuState, count int) ([]int, bool) {
	if s.key == nil {
		return numbers(answer, count)
	}
	first := state.page * choicesPerPage
	indices, ok := numbers(answer, min(choicesPerPage, count-first))
	for i := range indices {
		indices[i] += first
	}
	return indices, ok
}

func (s *session) menuCommand(rows []menuRow, state *menuState, answer string) (bool, error) {
	switch strings.ToLower(answer) {
	case "n":
		if (state.page+1)*choicesPerPage < len(rows) {
			state.page++
		}
		return true, nil
	case "p":
		state.page = max(state.page-1, 0)
		return true, nil
	}
	if query, ok := strings.CutPrefix(answer, "/"); ok {
		if s.key != nil {
			var err error
			query, err = s.readAnswer("Filter text. Press Enter to submit or clear.", nil)
			if err == errBack {
				return true, nil
			}
			if err != nil {
				return true, err
			}
		}
		*state = menuState{filter: strings.TrimSpace(query)}
		return true, nil
	}
	if choice, ok := strings.CutPrefix(answer, "?"); ok {
		return true, s.preview(rows, *state, strings.TrimSpace(choice))
	}
	return false, nil
}

func (s *session) preview(rows []menuRow, state menuState, choice string) error {
	if choice == "" && s.key != nil && len(rows) > 0 {
		var err error
		choice, err = s.ask("Choose a number to preview.")
		if err == errBack {
			return nil
		}
		if err != nil {
			return err
		}
	}
	indices, ok := s.menuNumbers(choice, state, len(rows))
	if !ok || len(indices) != 1 {
		s.lines("Choose one number to preview.")
		return nil
	}
	row := rows[indices[0]]
	fmt.Fprintln(s.out)
	s.lines("Details / " + row.label)
	for _, line := range row.details() {
		if _, url, ok := strings.Cut(line, "https://"); ok && !strings.ContainsRune(url, ' ') && strings.IndexFunc(line, unicode.IsControl) < 0 {
			// Let the terminal wrap the link so it remains copyable.
			fmt.Fprintln(s.out, line)
		} else {
			s.lines(line)
		}
	}
	return nil
}

func (s *session) lines(text string) {
	for _, line := range strings.Split(text, "\n") {
		for _, part := range wrap(line, s.cfg.Width) {
			fmt.Fprintln(s.out, part)
		}
	}
}

func cellWidth(s string) int { return uniseg.StringWidth(s) }

func wrap(line string, width int) []string {
	line = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, line)
	width = max(4, width)
	var out []string
	for cellWidth(line) > width {
		end, used := 0, 0
		g := uniseg.NewGraphemes(line)
		for g.Next() && used+g.Width() <= width {
			_, end = g.Positions()
			used += g.Width()
		}
		if split := strings.LastIndexByte(line[:end], ' '); split > 0 && strings.TrimSpace(line[:split]) != "" {
			end = split
		}
		out = append(out, line[:end])
		line = strings.TrimLeft(line[end:], " ")
	}
	return append(out, line)
}
