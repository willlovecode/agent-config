// Package plain runs the wizard in the terminal scrollback using numbered prompts.
package plain

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

type Config struct {
	Home      string
	Width     int
	LineInput bool
	Preflight func(discover bool) (*scan.Result, policy.Policy, error)
	Plan      func(p *profile.Profile, save bool) ([]install.Change, error)
	Apply     func(p *profile.Profile, save bool, out io.Writer) error
}

type session struct {
	w                   *wizard.Wizard
	cfg                 Config
	in                  *bufio.Reader
	out                 io.Writer
	key                 func() (string, error)
	terminal            bool
	save                bool
	checked, discovered bool
}

var errBack = fmt.Errorf("back")
var errQuit = fmt.Errorf("quit")

// Run asks the questions on in/out and installs when the user confirms. It returns
// whether anything was installed.
func Run(w *wizard.Wizard, cfg Config, in io.Reader, out io.Writer) (bool, error) {
	s := &session{w: w, cfg: cfg, in: bufio.NewReader(in), out: out, save: true}
	if output, ok := out.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
		if width, _, err := term.GetSize(int(output.Fd())); err == nil && cfg.Width <= 0 {
			s.cfg.Width = width - 1
		}
		if input, ok := in.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
			s.terminal = true
			if !cfg.LineInput {
				s.key = func() (string, error) { return consoleKey(int(input.Fd()), s.in) }
			}
		}
	}
	return s.run()
}

func (s *session) run() (bool, error) {
	if s.cfg.Width <= 0 {
		s.cfg.Width = 80
	}
	s.lines("Agent Toolkit Setup")
	if s.key == nil {
		s.lines("Press Enter after each answer. Enter alone keeps the default.")
	} else {
		s.lines("Press a key to choose. Enter keeps the default.")
	}
	steps := wizard.SetupStages()
	backward := false
	for i := 0; i < len(steps)-1; {
		stage := steps[i]
		s.title(stage)
		var done bool
		var err error
		switch stage {
		case wizard.StageSystem:
			err = s.system()
		case wizard.StageItems:
			err = s.items()
		case wizard.StageReview:
			done, err = s.review()
		default:
			err = s.questions(stage, backward)
		}
		switch {
		case err == errBack:
			backward = true
			if i > 0 {
				i--
			}
		case err == errQuit:
			s.lines("Left setup. Nothing was written.")
			return false, nil
		case err != nil:
			return false, err
		case done:
			return true, nil
		default:
			backward = false
			i++
		}
	}
	return false, nil
}

func (s *session) title(stage wizard.Stage) {
	steps := wizard.SetupStages()
	for i, step := range steps {
		if stage == step {
			fmt.Fprintf(s.out, "\n[%d/%d] %s\n", i+1, len(steps), step)
			return
		}
	}
}

func (s *session) review() (bool, error) {
	p := s.w.Profile()
	s.lines(wizard.SetupSummary(p))
	for _, line := range s.w.ReviewDetails(p) {
		s.lines(line)
	}
	for {
		save, err := s.yesNo("Save your choices as a profile for agtk apply?", s.save)
		if err != nil {
			return false, err
		}
		s.save = save
		err = s.confirm(p)
		if err != errBack {
			return err == nil, err
		}
	}
}

func (s *session) confirm(p *profile.Profile) error {
	var plan wizard.ReviewPlan
	for {
		s.lines("Preparing your plan...")
		changes, err := s.cfg.Plan(p, s.save)
		if err == nil {
			plan = wizard.GroupChanges(changes)
			break
		}
		s.lines(fmt.Sprintf("Plan could not be prepared.\n%v", err))
		if err := s.waitForEnter("Fix the issue, then press Enter to retry."); err != nil {
			return err
		}
	}
	if len(plan.Files)+len(plan.Commands) == 0 {
		s.lines("No file or command changes in this plan.")
	}
	fmt.Fprintf(s.out, "Files (%d new, %d to edit)\n", plan.New, plan.Edited)
	for _, c := range plan.Files {
		path := c.What
		if s.cfg.Home != "" {
			if rel, err := filepath.Rel(s.cfg.Home, path); err == nil && filepath.IsLocal(rel) {
				path = "~/" + filepath.ToSlash(rel)
			}
		}
		s.lines(fmt.Sprintf("  %-7s %s", c.Action, path))
	}
	fmt.Fprintf(s.out, "Commands (%d may run)\n", len(plan.Commands))
	for _, c := range plan.Commands {
		s.lines("  " + c.What)
	}
	if len(plan.Commands) > 0 {
		s.lines("Manage command changes in the tool that runs them.")
	}
	saveStatus := "No"
	if s.save {
		saveStatus = "Yes"
	}
	fmt.Fprintf(s.out, "Save profile for agtk apply: %s\n", saveStatus)
	s.lines("Close the clients you are configuring before applying.")
	s.lines("Each changed file is backed up first.")
	apply, err := s.yesNo("Install now?", false)
	if err != nil {
		return err
	}
	if !apply {
		return errQuit
	}
	s.title(wizard.StageApply)
	var log strings.Builder
	if err := s.cfg.Apply(p, s.save, io.MultiWriter(s.out, &log)); err != nil {
		s.lines(fmt.Sprintf("Setup failed: %v", err))
		if wizard.SummarizeApply(log.String()).CanUndo(s.cfg.Home) {
			s.lines(wizard.UndoHint)
		}
		return err
	}
	return nil
}
