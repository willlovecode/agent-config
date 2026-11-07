package wizard

import (
	"fmt"
	"strings"

	"github.com/willlovecode/agent-config/internal/scan"
)

// SystemLines describes the result of a system check: hardware, the clients on PATH and
// other tools found.
func SystemLines(r *scan.Result) []string {
	lines := []string{fmt.Sprintf("System check complete. %s/%s, %d CPU threads, %.0f GB RAM. Tier %s.", r.OS, r.Arch, r.CPUs, r.RAMGB, r.Tier)}
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
		lines = append(lines, "  "+tool.label+" - "+status)
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
		lines = append(lines, "Other tools on PATH: "+strings.Join(found, ", "))
	}
	return append(lines, r.TierWhy...)
}
