package wizard

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
)

// SetupSummary describes the choices behind an install plan.
func SetupSummary(p *profile.Profile) string {
	a := p.Answers
	var tools []string
	models := "Keep current models"
	if len(a.Tools) == 0 || contains(a.Tools, "claude_code") {
		tools = append(tools, "Claude Code")
	}
	if len(a.Tools) == 0 || contains(a.Tools, "codex") {
		tools = append(tools, "Codex")
	}
	if contains(a.Tools, "gemini") {
		tools = append(tools, "Gemini")
	}
	if preset, ok := routing.Get(a.Routing); ok {
		models = preset.Label + " routing"
	}
	mode := "Personal"
	if a.Company() {
		mode = "Company"
	}
	count := len(p.Items)
	for _, project := range p.ProjectItems {
		count += len(project.Items)
	}
	return fmt.Sprintf("%s | %s | %s | %d items", mode, strings.Join(tools, " + "), models, count)
}

func (w *Wizard) ReviewDetails(p *profile.Profile) []string {
	var out []string
	a := p.Answers
	items, _ := p.Resolve(w.Catalog)
	for _, tool := range []string{"claude_code", "codex", "gemini"} {
		if (len(a.Tools) > 0 || tool == "gemini") && !contains(a.Tools, tool) {
			continue
		}
		out = append(out, ToolLabel(tool)+" / user")
		for _, kind := range []struct{ id, label string }{
			{catalog.KindSkill, "Skills"}, {catalog.KindInstructions, "Instructions"}, {catalog.KindAgents, "Agents"},
			{catalog.KindHook, "Guards"}, {catalog.KindRules, "Rules"}, {catalog.KindMCP, "MCP"}, {catalog.KindPlugin, "Plugins"},
		} {
			var ids []string
			for _, it := range items {
				if it.Kind == kind.id && it.Supports(tool) {
					ids = append(ids, it.ID)
				}
			}
			if len(ids) > 0 {
				out = append(out, "  "+kind.label+": "+strings.Join(ids, ", "))
			}
		}
	}
	for _, project := range p.ProjectItems {
		out = append(out, "Project: "+project.Project, "  Skills: "+strings.Join(project.Items, ", "))
	}
	if len(w.Scan.Programs) > 0 {
		for _, tool := range a.Tools {
			if w.Scan.Find(tool) == nil {
				out = append(out, "Install later: "+ToolLabel(tool)+" was missing from PATH. Save this profile and run agtk apply once the client is installed.")
				for _, it := range items {
					if tool == "claude_code" && it.Supports(tool) && (it.Kind == catalog.KindMCP || it.Kind == catalog.KindPlugin) {
						out = append(out, "  Needs the Claude Code command: "+it.ID)
					}
				}
			}
		}
	}
	if w.Policy.Company(a) {
		source := w.Policy.Source
		if source == "" {
			source = "local defaults; external capabilities need IT approval"
		}
		out = append(out, "Company policy: "+source)
	}
	if w.Scan.Tier == "B" {
		out = append(out, "Guard mode: rules and permissions; managed settings restrict hooks.")
	}
	if r, _, err := a.ResolveRouting(); err == nil && r != nil {
		for _, tool := range []string{routing.ToolClaude, routing.ToolCodex} {
			if model := r.Default(tool); model.Model != "" {
				out = append(out, "Default model: "+describeRoute(model))
			}
		}
		out = append(out, "Task routes are guidance; switch clients or accounts manually.")
		for _, o := range routeOptions(a) {
			detail, _, _ := strings.Cut(o.Detail, "\n")
			out = append(out, "  "+o.Label+": "+detail)
		}
	}
	for _, it := range items {
		if it.MCP != nil {
			out = append(out, "External destination: "+it.ID+" -> "+it.MCP.URL)
		}
		if len(it.NPM) > 0 {
			out = append(out, "Possible global npm packages: "+strings.Join(it.NPM, ", "))
		}
		if it.Access == "network" && it.MCP == nil {
			out = append(out, "External access when used: "+it.ID+" ("+it.Needs+")")
		}
	}
	for _, l := range w.Lines() {
		if l.Blocked != "" {
			out = append(out, "Unavailable: "+l.Item.ID+" - "+l.Blocked)
		}
	}
	return out
}

type ReviewPlan struct {
	Files, Commands []install.Change
	New, Edited     int
}

// GroupChanges keeps file and command order within each group.
func GroupChanges(changes []install.Change) ReviewPlan {
	var plan ReviewPlan
	for _, c := range changes {
		if c.Action == "run" {
			plan.Commands = append(plan.Commands, c)
			continue
		}
		plan.Files = append(plan.Files, c)
		switch c.Action {
		case "create":
			plan.New++
		case "change":
			plan.Edited++
		}
	}
	return plan
}

type ApplyReport struct {
	Backup    string
	Next, Log []string
}

func SummarizeApply(log string) ApplyReport {
	var report ApplyReport
	log = strings.TrimRight(log, "\n")
	if log == "" {
		return report
	}
	for line := range strings.SplitSeq(log, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if path, ok := strings.CutPrefix(line, "Backup: "); ok {
			report.Backup = path
		} else if next, ok := strings.CutPrefix(line, "To do: "); ok {
			report.Next = append(report.Next, next)
		} else {
			if line == "Nothing changed; no backup kept." {
				report.Backup = ""
			}
			report.Log = append(report.Log, line)
		}
	}
	return report
}

const UndoHint = "Run agtk undo to restore the files from this run."

// Undo picks the newest manifest. A failure before the first write can leave an
// empty backup directory and undo would then pick a previous run.
func (r ApplyReport) CanUndo(home string) bool {
	if home == "" || r.Backup == "" {
		return false
	}
	backups := install.Backups(install.StateDir(home))
	return len(backups) > 0 && backups[0] == filepath.Clean(r.Backup)
}
