package plain

import (
	"fmt"
	"strings"

	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/scan"
)

func (s *session) system() error {
	for {
		if !s.checked {
			if err := s.checkSystem(); err != nil {
				return err
			}
		}
		if s.discovered {
			s.systemResults()
		} else {
			s.lines("Continuing with manual choices. Managed policy still applies.")
		}
		if err := s.waitForEnter("Press Enter to continue."); err != errBack {
			return err
		}
		s.checked = false
	}
}

func (s *session) checkSystem() error {
	s.lines("Check local hardware, tools on PATH and managed hook settings.")
	s.lines("Skip discovery to choose your tools and stacks manually.")
	for {
		answer, err := s.ask("Run the local system check? [Y/n]")
		if err != nil {
			return err
		}
		switch strings.ToLower(answer) {
		case "", "y", "yes":
			s.discovered = true
		case "n", "no", "skip":
			s.discovered = false
		default:
			s.lines("Choose y to check or n to continue manually.")
			continue
		}
		result, rules, err := s.preflight(s.discovered)
		if err != nil {
			return err
		}
		s.w.SetScan(result)
		s.w.SetPolicy(rules)
		s.checked = true
		return nil
	}
}

func (s *session) systemResults() {
	r := s.w.Scan
	s.lines(fmt.Sprintf("System check complete. %s/%s, %d CPU threads, %.0f GB RAM. Tier %s.", r.OS, r.Arch, r.CPUs, r.RAMGB, r.Tier))
	for _, tool := range []struct{ fact, label string }{
		{"claude_code", "Claude Code"}, {"codex", "Codex"}, {"gemini", "Gemini CLI"},
	} {
		status := "not on PATH"
		if p := r.Find(tool.fact); p != nil {
			status = "on PATH"
			if p.Version != "" {
				status += " (" + p.Version + ")"
			}
		}
		s.lines("  " + tool.label + " - " + status)
	}
	var found []string
	for _, p := range r.Programs {
		switch p.Fact {
		case "claude_code", "codex", "gemini":
			continue
		}
		if p.Version != "" {
			found = append(found, p.Name+" "+p.Version)
		} else if p.Available() {
			found = append(found, p.Name)
		}
	}
	if len(found) > 0 {
		s.lines("Other tools on PATH: " + strings.Join(found, ", "))
	}
	for _, reason := range r.TierWhy {
		s.lines(reason)
	}
}

func (s *session) preflight(discover bool) (*scan.Result, policy.Policy, error) {
	check := s.cfg.Preflight
	if check == nil {
		return s.w.Scan, s.w.Policy, nil
	}
	if discover {
		s.lines("Checking this machine...")
	}
	return check(discover)
}
