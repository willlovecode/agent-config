// Package plain is the line-mode wizard for screen readers, small terminals and CI:
// numbered questions in the style of the Debian installer's text front end. ENTER keeps
// the default, < goes back one step. No screen drawing, no colours.
package plain

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/wizard"
)

type Config struct {
	Plan  func(*profile.Profile) ([]install.Change, error)
	Apply func(p *profile.Profile, save bool, out io.Writer) error
}

type session struct {
	w   *wizard.Wizard
	cfg Config
	in  *bufio.Scanner
	out io.Writer
}

var errBack = fmt.Errorf("back")
var errQuit = fmt.Errorf("quit")

// Run asks the questions on in/out and installs when the user confirms. It returns
// whether anything was installed.
func Run(w *wizard.Wizard, cfg Config, in io.Reader, out io.Writer) (bool, error) {
	s := &session{w: w, cfg: cfg, in: bufio.NewScanner(in), out: out}
	fmt.Fprintln(out, "Agent Toolkit Setup (plain mode). ENTER keeps the value in [brackets], < goes back, q quits.")
	fmt.Fprintln(out, "Nothing is written until you confirm at the end.")
	steps := []func() (bool, error){s.scan, func() (bool, error) { return false, s.questions("use", 2) },
		func() (bool, error) { return false, s.questions("work", 3) }, s.recommend, s.review}
	for i := 0; i < len(steps); {
		done, err := steps[i]()
		switch {
		case err == errBack:
			if i > 0 {
				i--
			}
		case err == errQuit:
			fmt.Fprintln(out, "Left setup. Nothing was written.")
			return false, nil
		case err != nil:
			return false, err
		case done:
			return true, nil
		default:
			i++
		}
	}
	return false, nil
}

func (s *session) title(i int) {
	fmt.Fprintf(s.out, "\nStep %d of %d: %s\n", i+1, len(wizard.Steps), wizard.Steps[i])
}

// ask reads one line; EOF counts as quitting.
func (s *session) ask(prompt string) (string, error) {
	fmt.Fprint(s.out, prompt)
	if !s.in.Scan() {
		fmt.Fprintln(s.out)
		return "", errQuit
	}
	a := strings.TrimSpace(s.in.Text())
	switch a {
	case "<":
		return "", errBack
	case "q", "Q":
		return "", errQuit
	}
	return a, nil
}

func (s *session) scan() (bool, error) {
	s.title(1)
	r := s.w.Scan
	fmt.Fprintf(s.out, "System: %s/%s, %d CPU threads, %.0f GB RAM. Tier %s.\n", r.OS, r.Arch, r.CPUs, r.RAMGB, r.Tier)
	var found []string
	for _, p := range r.Programs {
		if p.Version != "" {
			found = append(found, p.Name+" "+p.Version)
		}
	}
	fmt.Fprintf(s.out, "Found: %s\n", strings.Join(found, ", "))
	_, err := s.ask("Press ENTER to continue: ")
	return false, err
}

func (s *session) questions(step string, n int) error {
	s.title(n)
	for qi := 0; qi < len(s.w.Questions(step)); qi++ {
		q := s.w.Questions(step)[qi]
		fmt.Fprintln(s.out, q.Prompt)
		var current []string
		for i, o := range q.Options {
			mark := "  "
			if s.w.Has(q.ID, o.Value) {
				mark = "* "
				current = append(current, strconv.Itoa(i+1))
			}
			line := fmt.Sprintf("  %s%d) %s", mark, i+1, o.Label)
			if note := s.w.Note(q, o); note != "" {
				line += "  (" + note + ")"
			}
			fmt.Fprintln(s.out, line)
		}
		prompt := fmt.Sprintf("Choose one [%s]: ", strings.Join(current, ""))
		if q.Multi {
			prompt = fmt.Sprintf("Numbers to switch on/off, separated by spaces [keep %s]: ", strings.Join(current, " "))
		}
		a, err := s.ask(prompt)
		if err == errBack && qi > 0 {
			qi -= 2 // back to the previous question of this step
			continue
		}
		if err != nil {
			return err
		}
		nums, ok := numbers(a, len(q.Options))
		if !ok || (!q.Multi && len(nums) > 1) {
			fmt.Fprintln(s.out, "Please type option numbers from the list.")
			qi--
			continue
		}
		for _, i := range nums {
			s.w.Set(q, q.Options[i].Value)
		}
	}
	return nil
}

// numbers parses "1 3" into zero-based indexes; empty input is valid (keep the default).
func numbers(a string, max int) ([]int, bool) {
	var out []int
	for _, f := range strings.FieldsFunc(a, func(r rune) bool { return r == ' ' || r == ',' }) {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > max {
			return nil, false
		}
		out = append(out, n-1)
	}
	return out, true
}

func (s *session) recommend() (bool, error) {
	s.title(4)
	for {
		lines := s.w.Lines()
		for i, l := range lines {
			mark := "[ ]"
			if l.On {
				mark = "[X]"
			}
			note := l.Item.Summary
			if !l.On && !l.Recommended && l.Reason != "" {
				note = "off: " + l.Reason
			}
			fmt.Fprintf(s.out, "  %2d) %s %-18s %s\n", i+1, mark, l.Item.ID, note)
		}
		used, max := s.w.Budget()
		fmt.Fprintf(s.out, "Always loaded: ~%d tokens of %d budget.\n", used, max)
		a, err := s.ask("Numbers to switch on/off, ?N explains item N [ENTER continues]: ")
		if err != nil {
			return false, err
		}
		if a == "" {
			return false, nil
		}
		if strings.HasPrefix(a, "?") {
			if n, ok := numbers(strings.TrimPrefix(a, "?"), len(lines)); ok && len(n) == 1 {
				l := lines[n[0]]
				fmt.Fprintf(s.out, "%s: %s\n", l.Item.ID, l.Reason)
				if l.Item.RecommendWhen != nil {
					fmt.Fprintf(s.out, "  recommend when %s\n", l.Item.RecommendWhen)
				}
				if l.Item.NeverWhen != nil {
					fmt.Fprintf(s.out, "  never when %s\n", l.Item.NeverWhen)
				}
				if l.Item.Source.Repo != "" {
					fmt.Fprintf(s.out, "  source %s@%s (%s)\n", l.Item.Source.Repo, l.Item.Source.Commit[:7], l.Item.Source.License)
				}
				fmt.Fprintf(s.out, "  ~%d tokens in every session\n", l.Cost)
			}
			continue
		}
		nums, ok := numbers(a, len(lines))
		if !ok {
			fmt.Fprintln(s.out, "Please type item numbers from the list.")
			continue
		}
		for _, i := range nums {
			s.w.Toggle(lines[i].Item.ID)
		}
	}
}

func (s *session) review() (bool, error) {
	s.title(5)
	p, err := s.w.Profile()
	if err != nil {
		return false, err
	}
	fmt.Fprintln(s.out, "Working out what will change...")
	plan, err := s.cfg.Plan(p)
	if err != nil {
		return false, err
	}
	if len(plan) == 0 {
		fmt.Fprintln(s.out, "Everything is already in place.")
	}
	for _, c := range plan {
		fmt.Fprintf(s.out, "  %-7s %s\n", c.Action, c.What)
	}
	a, err := s.ask(fmt.Sprintf("Save your answers to %s (no machine details)? [Y/n]: ", p.Path))
	if err != nil {
		return false, err
	}
	save := !strings.HasPrefix(strings.ToLower(a), "n")
	a, err = s.ask("Install now? Each changed file is backed up first. [y/N]: ")
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(strings.ToLower(a), "y") {
		return false, errQuit
	}
	if err := s.cfg.Apply(p, save, s.out); err != nil {
		fmt.Fprintf(s.out, "Setup failed: %v\nFiles changed so far are backed up; `agtk undo` restores them.\n", err)
		return false, err
	}
	return true, nil
}
