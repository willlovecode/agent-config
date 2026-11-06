package plain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/willlovecode/agent-config/internal/wizard"
)

type menuRow struct {
	group, label, mark, note string
	details                  func() []string
}

type menuState struct {
	filter string
	page   int
}

func (s *session) questions(stage wizard.Stage, backward bool) error {
	qi := 0
	if backward {
		qi = len(s.w.SetupQuestions(stage)) - 1
	}
	state := menuState{}
	for {
		questions := s.w.SetupQuestions(stage)
		if qi < 0 || qi >= len(questions) {
			return nil
		}
		q := questions[qi]
		options := wizard.FilterOptions(q.Options, state.filter)
		rows := make([]menuRow, 0, len(options))
		for _, option := range options {
			note := s.w.Note(q, option)
			if q.ID == "models" {
				note = "" // the account is already shown as the group
			}
			rows = append(rows, menuRow{
				group: option.Group, label: option.Label, mark: mark(s.w.Has(q.ID, option.Value)), note: note,
				details: func() []string { return s.w.SetupDetails(q, option) },
			})
		}
		fmt.Fprintln(s.out)
		hint := q.Hint
		if hint == "" {
			if q.ID == "use" {
				hint = "Your profile sets which components and models are available."
			} else {
				hint = "These choices help agtk recommend components for your work."
			}
		}
		s.lines(hint)
		s.lines(q.Prompt)
		s.menu(rows, state)
		if q.Multi {
			if s.key != nil {
				s.lines("Numbers toggle choices. Enter accepts them.")
			} else {
				s.lines("Enter keeps choices. Numbers toggle choices and continue.")
			}
		} else {
			s.lines("Enter keeps the selected choice. Choose a number to change it.")
		}
		answer, err := s.ask("")
		if err == errBack && qi > 0 {
			qi--
			state = menuState{}
			continue
		}
		if err != nil {
			return err
		}
		if handled, err := s.menuCommand(rows, &state, answer); handled {
			if err != nil {
				return err
			}
			continue
		}
		nums, ok := s.menuNumbers(answer, state, len(rows))
		if !ok || (!q.Multi && len(nums) > 1) {
			fmt.Fprintln(s.out, "Please type option numbers from the list.")
			continue
		}
		for _, i := range nums {
			s.w.Set(q, options[i].Value)
		}
		if q.Multi && s.key != nil && len(nums) > 0 {
			continue
		}
		if msg := s.w.ConfirmSetup(q); msg != "" {
			s.lines(msg)
			continue
		}
		qi++
		state = menuState{}
	}
}

func (s *session) items() error {
	state := menuState{}
	subset := "all"
	views := wizard.ItemViews()
	var names []string
	for _, view := range views {
		names = append(names, view.ID)
	}
	for {
		all := s.w.Lines()
		selected := 0
		for _, l := range all {
			if l.On {
				selected++
			}
		}
		lines := wizard.FilterLines(all, state.filter, subset)
		var rows []menuRow
		for _, l := range lines {
			status, note := mark(l.On), ""
			if l.Blocked != "" {
				status, note = "[-]", "blocked"
			}
			rows = append(rows, menuRow{
				group: wizard.ItemGroup(l.Item), label: l.Item.ID, mark: status, note: note,
				details: func() []string { return wizard.ItemDetails(l) },
			})
		}
		used, limit := s.w.Budget()
		fmt.Fprintln(s.out)
		s.lines("Recommended components start selected.")
		s.lines("Choose what to install.")
		s.lines(fmt.Sprintf("%d selected | %s | ~%d of %d context tokens", selected, subset, used, limit))
		s.menu(rows, state)
		s.lines("Enter reviews the setup. Numbers toggle items.")
		if s.key != nil {
			s.lines("(v) change item view")
		} else {
			s.lines("Views: " + strings.Join(names, ", ") + ".")
		}
		answer, err := s.ask("")
		if err != nil {
			return err
		}
		if answer == "" {
			return nil
		}
		view := strings.ToLower(answer)
		if view == "v" {
			view, err = s.itemView(views, subset)
			if err != nil {
				return err
			}
		}
		if slices.Contains(names, view) {
			subset = view
			state.page = 0
			continue
		}
		if handled, err := s.menuCommand(rows, &state, answer); handled {
			if err != nil {
				return err
			}
			continue
		}
		nums, ok := s.menuNumbers(answer, state, len(rows))
		if !ok {
			fmt.Fprintln(s.out, "Please type item numbers from the list.")
			continue
		}
		for _, i := range nums {
			if blocked := s.w.Toggle(lines[i].Item.ID); blocked != "" {
				s.lines(blocked)
			}
		}
		state.page = nums[len(nums)-1] / choicesPerPage
	}
}

func (s *session) itemView(views []wizard.ItemView, current string) (string, error) {
	s.lines("Which items would you like to see?")
	for i, view := range views {
		s.lines(fmt.Sprintf("  %d) %s %s", i+1, mark(view.ID == current), view.Label))
	}
	for {
		answer, err := s.ask("Choose a view. Enter keeps the current one.")
		if err == errBack || (err == nil && answer == "") {
			return current, nil
		}
		if err != nil {
			return "", err
		}
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(views) {
			return views[n-1].ID, nil
		}
		s.lines("Choose a number from the list.")
	}
}

func mark(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

// numbers parses "1 3" into zero-based indexes. Empty input keeps the defaults.
func numbers(a string, max int) ([]int, bool) {
	fields := strings.FieldsFunc(a, func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
	if len(fields) == 0 {
		return nil, a == ""
	}
	var out []int
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > max {
			return nil, false
		}
		out = append(out, n-1)
	}
	return out, true
}
