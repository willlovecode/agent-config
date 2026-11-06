package wizard

import (
	"cmp"
	"slices"
	"strings"

	"github.com/willlovecode/agent-config/internal/routing"
)

func SetupStages() []Stage {
	return []Stage{StageSystem, StageProfile, StageModels, StageItems, StageReview, StageApply}
}

// SetupQuestions groups the choices for the installer. Access comes
// before the preset so each route can be previewed against confirmed models.
func (w *Wizard) SetupQuestions(stage Stage) []*Question {
	var out []*Question
	switch stage {
	case StageProfile:
		out = append(w.Questions(StageProfile), w.Questions(StageWork)...)
	case StageModels:
		ids := []string{"tools"}
		if !w.Company() || len(w.Policy.Models) > 0 {
			ids = append(ids, "plans")
			if len(w.selectedPlans()) > 0 {
				ids = append(ids, "models")
				if len(w.confirmedModels()) > 0 {
					ids = append(ids, "routing")
				}
			}
		}
		for _, id := range ids {
			for _, q := range questions {
				if q.ID == id {
					out = append(out, w.question(q))
					break
				}
			}
		}
	}
	for _, q := range out {
		q.Options = groupOptions(q.Options, q.ID == "stacks" || q.ID == "frameworks")
		switch q.ID {
		case "tools":
			q.Hint = "Select clients to configure now. You can install them later."
			if len(w.Scan.Programs) > 0 {
				q.Hint = "Detected clients start checked. Select others to install later."
			}
			if w.Company() && len(w.Policy.Models) == 0 {
				q.Hint += "\nCompany policy keeps the current model settings."
			}
		case "plans":
			q.Hint = "Select accounts, or continue with none to keep current models."
		case "models":
			q.Hint = "Confirm available models, or continue with none to keep your settings."
		case "routing":
			q.Hint = "Preview each preset's routes, costs, limits and speeds."
		}
	}
	return out
}

func groupOptions(options []Option, sortLabels bool) []Option {
	var order []string
	groups := map[string][]Option{}
	for _, o := range options {
		if _, ok := groups[o.Group]; !ok {
			order = append(order, o.Group)
		}
		groups[o.Group] = append(groups[o.Group], o)
	}
	out := make([]Option, 0, len(options))
	for _, group := range order {
		rows := groups[group]
		if sortLabels {
			slices.SortStableFunc(rows, func(a, b Option) int { return cmp.Compare(a.Label, b.Label) })
		}
		out = append(out, rows...)
	}
	return out
}

// ConfirmSetup keeps the chosen defaults when the user revisits the system page.
func (w *Wizard) ConfirmSetup(q *Question) string {
	if msg := w.SetupBlocked(q); msg != "" {
		return msg
	}
	w.answered[q.ID] = true
	return ""
}

func (w *Wizard) SetupBlocked(q *Question) string {
	switch q.ID {
	case "plans", "models":
		// Access is optional. A preset appears only after model confirmation.
	case "routing":
		a := w.Answers()
		if err := w.Policy.CheckRouting(a); err != nil {
			return err.Error()
		}
		if _, _, err := a.ResolveRouting(); err != nil {
			return err.Error()
		}
	default:
		return w.Blocked(q)
	}
	return ""
}

func (w *Wizard) previewRouting(preset string) (*routing.Setup, error) {
	setup, _, err := routing.ResolveSetup(preset, w.selectedPlans(), w.confirmedModels(), w.values["tools"])
	return setup, err
}

func (w *Wizard) SetupDetails(q *Question, option Option) []string {
	var lines []string
	if option.Detail != "" {
		lines = strings.Split(option.Detail, "\n")
	}
	switch q.ID {
	case "plans":
		return append(lines, routing.PlanMetrics(option.Value)...)
	case "models":
		for _, model := range w.models() {
			if model.Key() == option.Value {
				return append(lines, routing.ModelMetrics(model.Choice(""))...)
			}
		}
	case "routing":
		setup, err := w.previewRouting(option.Value)
		if err != nil {
			return append(lines, err.Error())
		}
		if setup == nil {
			return lines
		}
		var models []routing.Choice
		for _, row := range setup.Rows() {
			route := row.Use.String()
			models = appendChoice(models, row.Use)
			if len(row.Fallbacks) > 0 {
				route += " -> " + row.Fallbacks[0].String()
				models = appendChoice(models, row.Fallbacks[0])
			}
			lines = append(lines, shortTask(row.Task)+": "+route)
		}
		for _, model := range models {
			lines = append(lines, "", describeRoute(model))
			lines = append(lines, routing.ModelMetrics(model)...)
		}
	}
	return lines
}

func appendChoice(choices []routing.Choice, c routing.Choice) []routing.Choice {
	if slices.Contains(choices, c) {
		return choices
	}
	return append(choices, c)
}

func shortTask(t routing.Task) string {
	switch t.ID {
	case "explore":
		return "Find code"
	case "routine":
		return "Coding"
	case "hard":
		return "Hard tasks"
	case "review":
		return "Review"
	case "mechanical":
		return "Mechanical"
	case "research":
		return "Research"
	}
	return t.Label
}
