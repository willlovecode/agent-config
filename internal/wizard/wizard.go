// Package wizard holds the setup questionnaire independent of any screen: the questions,
// their defaults from the scan, the recommended items and the user's changes to them, and
// the profile they add up to. The terminal wizard and answers-file mode share it.
package wizard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/check"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
	"github.com/willlovecode/agent-config/internal/scan"
)

type Option struct {
	Value, Label, Detail string
	Note                 string // e.g. "found: 2.1.318"
	Group                string
}

type Question struct {
	ID       string
	Stage    Stage
	Prompt   string
	Multi    bool
	ReadOnly bool
	Hint     string
	Options  []Option
	when     func(*Wizard) bool
	def      func(*Wizard) []string
}

// Stage identifies a step in the setup flow.
type Stage int

const (
	StageSystem Stage = iota
	StageProfile
	StageWork
	StageModels
	StageItems
	StageReview
	StageApply
	StageCount
)

var stageNames = [StageCount]string{"System", "Profile", "Work", "Models", "Items", "Review", "Apply"}

func (s Stage) String() string { return stageNames[s] }

type Wizard struct {
	Scan          *scan.Result
	Catalog       *catalog.Catalog
	Assets        fs.FS
	Policy        policy.Policy
	Project       string
	values        map[string][]string
	choice        map[string]bool // items the user switched on or off by hand
	answered      map[string]bool // questions the user answered, or a loaded file did, so defaults leave them alone
	legacyRouting bool
	loadedModels  []routing.Model
}

func New(cat *catalog.Catalog, assets fs.FS, s *scan.Result) *Wizard {
	w := &Wizard{Catalog: cat, Assets: assets, values: map[string][]string{}, choice: map[string]bool{}, answered: map[string]bool{}}
	w.SetScan(s)
	return w
}

func (w *Wizard) SetScan(s *scan.Result) {
	w.Scan = s
	for _, q := range questions {
		if !w.answered[q.ID] {
			w.values[q.ID] = q.def(w)
		}
	}
}

// Questions returns the questions of a stage that apply to the current answers.
func (w *Wizard) Questions(stage Stage) []*Question {
	var out []*Question
	for _, q := range questions {
		if q.Stage == stage && (q.when == nil || q.when(w)) {
			out = append(out, w.question(q))
		}
	}
	return out
}

func (w *Wizard) question(q *Question) *Question {
	page := *q
	switch q.ID {
	case "routing":
		if w.Company() && len(w.Policy.Models) == 0 {
			keep := q.Options[0]
			keep.Detail = "IT has not approved model changes. Keep the current settings."
			page.Options = []Option{keep}
		}
	case "plans":
		page.Options = w.planOptions()
	case "models":
		page.Options = w.modelOptions()
	case "routes":
		page.Options = routeOptions(w.Answers())
	}
	return &page
}

func (w *Wizard) SetPolicy(p policy.Policy) {
	w.Policy = p
	if p.Source != "" {
		w.values["use"] = []string{"company"}
		if !w.answered["routing"] || len(p.Models) == 0 {
			w.values["routing"] = routingDefault(w)
		}
	}
}

func (w *Wizard) Values(id string) []string { return w.values[id] }

func (w *Wizard) One(id string) string {
	if v := w.values[id]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (w *Wizard) Has(id, value string) bool {
	for _, v := range w.values[id] {
		if v == value {
			return true
		}
	}
	return false
}

// Set answers a single-choice question, or toggles a value of a multiple-choice one.
func (w *Wizard) Set(q *Question, value string) {
	if q.ReadOnly {
		return
	}
	w.answered[q.ID] = true
	if !q.Multi {
		w.values[q.ID] = []string{value}
		if q.ID == "use" && (!w.answered["routing"] || (w.Company() && len(w.Policy.Models) == 0)) {
			w.values["routing"] = routingDefault(w)
		}
		return
	}
	var out []string
	found := false
	for _, opt := range q.Options { // keep option order
		on := w.Has(q.ID, opt.Value)
		if opt.Value == value {
			on, found = !on, true
		}
		if on {
			out = append(out, opt.Value)
		}
	}
	if found {
		w.values[q.ID] = out
	}
}

// Note is shown next to an option, e.g. which version of a tool the scan found.
func (w *Wizard) Note(q *Question, o Option) string {
	if q.ID == "routing" && o.Value == routingDefault(w)[0] {
		return "recommended"
	}
	if q.ID == "tools" {
		if p := w.Scan.Find(o.Value); p != nil {
			if p.Version != "" {
				return "found: " + p.Version
			}
			return "on PATH"
		}
		if len(w.Scan.Programs) == 0 {
			return "PATH not checked"
		}
		return "install later"
	}
	return o.Note
}

// Blocked says why a question cannot be left as answered, or "" when it can.
func (w *Wizard) Blocked(q *Question) string {
	switch q.ID {
	case "use":
		if w.Policy.Source != "" && w.One("use") != "company" {
			return "This machine is managed. Select Company to continue."
		}
	case "tools":
		if len(w.values["tools"]) == 0 {
			return "Select at least one agent tool to continue."
		}
	case "plans":
		if !routingOn(w) {
			return ""
		}
		if w.Company() && len(w.Policy.Models) == 0 {
			return "IT has not approved any models. Go back and keep current models."
		}
		if len(w.selectedPlans()) == 0 {
			return "Select a plan or go back and keep current models."
		}
	case "models":
		a := w.Answers()
		if len(a.Models) == 0 {
			return "Confirm an available model or go back and keep current models."
		}
		if _, _, err := a.ResolveRouting(); err != nil {
			return err.Error()
		}
	}
	return ""
}

// Answers records the exact access selected for routing. Leaving the preset off
// omits routing fields entirely.
func (w *Wizard) Answers() profile.Answers {
	a := profile.Answers{
		Use: w.One("use"), Tools: w.values["tools"], Stacks: append(append([]string(nil), w.values["stacks"]...), w.values["frameworks"]...), Work: w.values["work"],
	}
	if routingOn(w) {
		if !w.legacyRouting {
			a.Models = w.confirmedModels()
		}
		if w.legacyRouting || len(a.Models) > 0 {
			a.Plans, a.Routing = w.selectedPlans(), w.One("routing")
		}
	}
	return a
}

func (w *Wizard) confirmedModels() []routing.Model {
	var out []routing.Model
	for _, m := range w.models() {
		if w.Has("models", m.Key()) {
			out = append(out, m)
		}
	}
	return out
}

// Budget is the always-loaded token cost of the chosen items against the context budget.
// It includes the larger of the clients' routing tables.
func (w *Wizard) Budget() (used, limit int) {
	for _, l := range w.Lines() {
		if l.On {
			used += l.Cost
		}
	}
	if r, _, err := w.Answers().ResolveRouting(); err == nil {
		used += max(catalog.Tokens(r.Section(routing.ToolClaude)), catalog.Tokens(r.Section(routing.ToolCodex)))
	}
	return used, check.Budget.Instructions + check.Budget.SkillTokens + check.Budget.AgentTokens
}

// ProfilePath is where the answers are saved, so `agtk apply` can repeat them.
func ProfilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agtk", "profile.yaml"), nil
}

// Profile is the setup the answers and choices add up to.
func (w *Wizard) Profile() *profile.Profile {
	p := &profile.Profile{Description: "Set up with the agtk wizard", Answers: w.Answers(), Items: []string{}}
	project := profile.ProjectItems{Project: w.Project}
	for _, l := range w.Lines() {
		if l.On {
			if l.Item.Scope == "project" {
				project.Items = append(project.Items, l.Item.ID)
			} else {
				p.Items = append(p.Items, l.Item.ID)
			}
		}
	}
	if len(project.Items) > 0 {
		p.ProjectItems = []profile.ProjectItems{project}
	}
	return p
}

// Load fills the answers and choices from a saved profile or an answers file.
func (w *Wizard) Load(p *profile.Profile) error {
	a := p.Answers
	if w.Policy.Company(a) {
		w.values["use"] = []string{"company"}
	} else if a.Use != "" {
		w.values["use"] = []string{a.Use}
	}
	if a.Tools == nil {
		a.Tools = w.values["tools"]
	}
	if err := w.Policy.CheckRouting(a); err != nil {
		return err
	}
	setup, _, err := a.ResolveRouting()
	if err != nil {
		return err
	}
	// Loading older files must not enable routing they did not request.
	set := map[string][]string{"tools": a.Tools, "stacks": a.Stacks, "work": a.Work, "plans": routing.NormalizePlans(a.Plans),
		"routing": {routing.None}}
	w.loadedModels, w.legacyRouting = a.Models, a.Models == nil
	for _, m := range a.Models {
		set["models"] = append(set["models"], m.Key())
	}
	w.answered["routing"] = true
	if setup != nil {
		set["routing"] = []string{setup.Preset.ID}
	}
	for id, v := range set {
		if v != nil {
			w.values[id] = v
		}
	}
	if p.Items == nil {
		return nil // keep the recommendations
	}
	chosen := map[string]bool{}
	for _, id := range p.Items {
		it, ok := w.Catalog.Get(id)
		if !ok {
			return fmt.Errorf("item %q is not in the catalog", id)
		}
		if reason := w.Policy.ItemBlock(a, it); reason != "" {
			return fmt.Errorf("%s: %s", id, reason)
		}
		chosen[id] = true
	}
	for _, l := range w.Lines() {
		w.choice[l.Item.ID] = chosen[l.Item.ID]
	}
	return nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
