package wizard

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/willlovecode/agent-config/internal/catalog"
)

type Line struct {
	catalog.Decision
	On      bool
	Cost    int
	Blocked string
}

var taskGroups = []string{"Setup", "General", "Planning", "Testing & debugging", "Review & security", "Frontend & accessibility", "Backend", "Data & databases", "Infrastructure", "Documentation", "Research"}

type ItemView struct{ ID, Label string }

func ItemViews() []ItemView {
	return []ItemView{
		{"all", "All"}, {"recommended", "Recommended"}, {"general", "General"},
		{"popular", "Popular"}, {"blocked", "Blocked"},
	}
}

func ItemGroup(it *catalog.Item) string {
	if it.Task == "" {
		return taskGroups[0]
	}
	return it.Task
}

func FilterOptions(options []Option, query string) []Option {
	if query == "" {
		return options
	}
	var out []Option
	for _, o := range options {
		if matches(query, o.Label, o.Detail, o.Value, o.Group) {
			out = append(out, o)
		}
	}
	return out
}

func FilterLines(lines []Line, query, subset string) []Line {
	var out []Line
	for _, l := range lines {
		switch {
		case subset == "recommended" && (!l.Recommended || l.Blocked != ""):
			continue
		case subset == "general" && (l.Item.Kind != catalog.KindSkill || ItemGroup(l.Item) != "General"):
			continue
		case subset == "popular" && l.Item.Popularity == nil:
			continue
		case subset == "blocked" && l.Blocked == "":
			continue
		}
		if matches(query, l.Item.ID, l.Item.Summary, l.Item.Purpose, ItemGroup(l.Item), l.Item.Kind) {
			out = append(out, l)
		}
	}
	return out
}

func matches(query string, fields ...string) bool {
	return strings.Contains(strings.ToLower(strings.Join(fields, " ")), strings.ToLower(strings.TrimSpace(query)))
}

func (w *Wizard) Lines() []Line {
	a := w.Answers()
	var out []Line
	groups := map[string]bool{}
	for _, d := range w.Catalog.Recommend(a.Facts(w.Scan.Facts())) {
		if d.Item.Scope == "project" && d.Item.Kind != catalog.KindSkill {
			continue
		}
		blocked := w.Policy.ItemBlock(a, d.Item)
		if blocked == "" && d.Item.Scope == "project" && w.Project == "" {
			blocked = "Project skill. Start setup with --project DIR to select it."
		}
		on := d.Recommended
		if c, ok := w.choice[d.Item.ID]; ok {
			on = c
		}
		if d.Recommended && blocked == "" {
			groups[ItemGroup(d.Item)] = true
		}
		out = append(out, Line{Decision: d, On: on && blocked == "", Cost: catalog.Cost(w.Assets, d.Item), Blocked: blocked})
	}
	slices.SortStableFunc(out, func(a, b Line) int {
		ga, gb := ItemGroup(a.Item), ItemGroup(b.Item)
		if ga != gb {
			if groups[ga] != groups[gb] {
				if groups[ga] {
					return -1
				}
				return 1
			}
			if order := cmp.Compare(slices.Index(taskGroups, ga), slices.Index(taskGroups, gb)); order != 0 {
				return order
			}
			return cmp.Compare(ga, gb)
		}
		return cmp.Compare(a.Item.ID, b.Item.ID)
	})
	return out
}

func (w *Wizard) Toggle(id string) string {
	for _, l := range w.Lines() {
		if l.Item.ID == id {
			if l.Blocked == "" {
				w.choice[id] = !l.On
			}
			return l.Blocked
		}
	}
	return ""
}

func ToolLabel(tool string) string {
	switch tool {
	case "claude_code":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "gemini":
		return "Gemini CLI / Antigravity"
	}
	return tool
}

func ItemDetails(l Line) []string {
	it := l.Item
	purpose := it.Purpose
	if purpose == "" {
		purpose = it.Summary
	}
	tools := make([]string, 0, len(it.Tools))
	for _, tool := range it.Tools {
		tools = append(tools, ToolLabel(tool))
	}
	var lines []string
	if l.Blocked != "" {
		lines = append(lines, "Unavailable: "+l.Blocked)
	}
	lines = append(lines,
		"Use for: "+purpose,
		"Why: "+l.Reason,
	)
	if p := it.Popularity; p != nil {
		lines = append(lines, fmt.Sprintf("Popularity: %s (%s)", p.Label, p.Checked))
		if p.URL != it.Website {
			lines = append(lines, "Ranking source: "+p.URL)
		}
	}
	if it.Website != "" {
		lines = append(lines, "Website: "+it.Website)
	}
	lines = append(lines, fmt.Sprintf("Task: %s | %s scope | %s", ItemGroup(it), it.Scope, strings.Join(tools, " + ")))
	if it.Needs != "" {
		lines = append(lines, "Needs: "+it.Needs)
	}
	if it.Access != "" {
		lines = append(lines, "Access: "+it.Access)
	}
	if it.MCP != nil {
		lines = append(lines, "External endpoint: "+it.MCP.URL)
	}
	if link := it.SourceURL(); link != "" {
		lines = append(lines, "Pinned source: "+link, "License: "+it.Source.License)
	}
	if it.Audit != "" {
		lines = append(lines, "Audit: "+it.Audit)
	}
	if it.ExplicitOnly {
		lines = append(lines, "Activate by name when you need it.")
	}
	return append(lines, fmt.Sprintf("Always loaded: ~%d tokens", l.Cost))
}
