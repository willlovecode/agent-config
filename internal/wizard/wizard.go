// Package wizard holds the setup questionnaire independent of any screen: the questions,
// their defaults from the scan, the recommended items and the user's changes to them, and
// the profile they add up to. The console UI (internal/tui) and the line mode
// (internal/plain) both drive it.
package wizard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/check"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
)

type Option struct {
	Value, Label string
	Note         string // e.g. "found: 2.1.318"
}

type Question struct {
	ID      string
	Step    string // use, work
	Prompt  string
	Multi   bool
	Options []Option
	when    func(*Wizard) bool
	def     func(*Wizard) []string
}

// Steps in screen order; Use and Work hold the questions.
var Steps = []string{"Welcome", "Scan", "How will you use it?", "What do you work on?", "Recommended setup", "Review and install"}

type Wizard struct {
	Scan    *scan.Result
	Catalog *catalog.Catalog
	Assets  fs.FS
	values  map[string][]string
	choice  map[string]bool // items the user switched on or off by hand
}

func New(cat *catalog.Catalog, assets fs.FS, s *scan.Result) *Wizard {
	w := &Wizard{Scan: s, Catalog: cat, Assets: assets, values: map[string][]string{}, choice: map[string]bool{}}
	for _, q := range questions {
		w.values[q.ID] = q.def(w)
	}
	return w
}

var questions = []*Question{
	{ID: "use", Step: "use", Prompt: "Who is this machine for?", Options: []Option{
		{Value: "personal", Label: "Personal"},
		{Value: "corporate_gateway", Label: "Corporate, through a company LLM gateway"},
		{Value: "corporate_key", Label: "Corporate, with my own API key"},
	}, def: fixed("personal")},
	{ID: "tools", Step: "use", Prompt: "Which agent tools will you use?", Multi: true, Options: []Option{
		{Value: "claude_code", Label: "Claude Code"},
		{Value: "codex", Label: "Codex"},
	}, def: func(w *Wizard) []string {
		var out []string
		for _, t := range []string{"claude_code", "codex"} {
			if w.Scan.Found(t) != "" {
				out = append(out, t)
			}
		}
		if len(out) == 0 {
			out = []string{"claude_code", "codex"}
		}
		return out
	}},
	{ID: "allow_external_mcp", Step: "use", Prompt: "May tools send queries to outside services (e.g. Context7)?", Options: []Option{
		{Value: "no", Label: "No, keep company data inside"},
		{Value: "yes", Label: "Yes, my company allows it"},
	}, when: func(w *Wizard) bool { return w.One("use") != "personal" }, def: fixed("no")},
	{ID: "stacks", Step: "work", Prompt: "Which languages and stacks?", Multi: true, Options: []Option{
		{Value: "python", Label: "Python"},
		{Value: "java", Label: "Java"},
		{Value: "spring", Label: "Spring Boot"},
		{Value: "javascript", Label: "JavaScript / TypeScript"},
		{Value: "go", Label: "Go"},
		{Value: "dotnet", Label: "C# / .NET"},
	}, def: func(w *Wizard) []string {
		var out []string
		for fact, stack := range map[string]string{"python": "python", "jdk": "java", "go": "go", "node": "javascript"} {
			if w.Scan.Found(fact) != "" {
				out = append(out, stack)
			}
		}
		sort.Strings(out)
		return out
	}},
	{ID: "work", Step: "work", Prompt: "What kind of work?", Multi: true, Options: []Option{
		{Value: "greenfield", Label: "New code"},
		{Value: "legacy_migration", Label: "Legacy code and migrations"},
		{Value: "research", Label: "Research and literature"},
		{Value: "ui", Label: "User interfaces"},
		{Value: "data", Label: "Data and analysis"},
		{Value: "ops", Label: "Operations and scripts"},
	}, def: fixed("greenfield")},
}

func fixed(v ...string) func(*Wizard) []string { return func(*Wizard) []string { return v } }

// Questions returns the questions of a step that apply to the current answers.
func (w *Wizard) Questions(step string) []*Question {
	var out []*Question
	for _, q := range questions {
		if q.Step == step && (q.when == nil || q.when(w)) {
			out = append(out, q)
		}
	}
	return out
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
	if !q.Multi {
		w.values[q.ID] = []string{value}
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
	if q.ID == "tools" {
		if v := w.Scan.Found(o.Value); v != "" {
			return "found: " + v
		}
		return "not found"
	}
	return o.Note
}

func (w *Wizard) Answers() profile.Answers {
	return profile.Answers{
		Use: w.One("use"), Tools: w.values["tools"], Stacks: w.values["stacks"], Work: w.values["work"],
		AllowExternalMCP: w.One("use") != "personal" && w.One("allow_external_mcp") == "yes",
	}
}

// Line is one item on the recommendations screen.
type Line struct {
	catalog.Decision
	On   bool // what will be installed
	Cost int  // always-loaded tokens when on
}

// Lines lists the user-scope items for the current answers: recommended ones on, unless
// the user switched them. Project items are set up per project, not here.
func (w *Wizard) Lines() []Line {
	var out []Line
	for _, d := range w.Catalog.Recommend(w.Answers().Facts(w.Scan.Facts())) {
		if d.Item.Scope != "user" {
			continue
		}
		on := d.Recommended
		if c, ok := w.choice[d.Item.ID]; ok {
			on = c
		}
		out = append(out, Line{Decision: d, On: on, Cost: catalog.Cost(w.Assets, d.Item)})
	}
	return out
}

func (w *Wizard) Toggle(id string) {
	for _, l := range w.Lines() {
		if l.Item.ID == id {
			w.choice[id] = !l.On
		}
	}
}

// Budget is the always-loaded token cost of the chosen items against the context budget.
func (w *Wizard) Budget() (used, max int) {
	for _, l := range w.Lines() {
		if l.On {
			used += l.Cost
		}
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
func (w *Wizard) Profile() (*profile.Profile, error) {
	p := &profile.Profile{Description: "Set up with the agtk wizard", Answers: w.Answers()}
	for _, l := range w.Lines() {
		if l.On {
			p.Items = append(p.Items, l.Item.ID)
		}
	}
	path, err := ProfilePath()
	if err != nil {
		return nil, err
	}
	p.Path, p.Dir = path, filepath.Dir(path)
	return p, nil
}

// SaveProfile writes the profile (answers and chosen items only: no machine details).
func SaveProfile(p *profile.Profile) error {
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	head := "# Saved by the agtk wizard. Re-apply with: agtk apply --profile " + p.Path + "\n"
	return os.WriteFile(p.Path, append([]byte(head), b...), 0o644)
}

// Load fills the answers and choices from a saved profile or an answers file.
func (w *Wizard) Load(p *profile.Profile) error {
	a := p.Answers
	set := map[string][]string{"use": {a.Use}, "tools": a.Tools, "stacks": a.Stacks, "work": a.Work}
	if a.AllowExternalMCP {
		set["allow_external_mcp"] = []string{"yes"}
	}
	for id, v := range set {
		if len(v) > 0 && v[0] != "" {
			w.values[id] = v
		}
	}
	if len(p.Items) == 0 {
		return nil // keep the recommendations
	}
	chosen := map[string]bool{}
	for _, id := range p.Items {
		if _, ok := w.Catalog.Get(id); !ok {
			return fmt.Errorf("item %q is not in the catalog", id)
		}
		chosen[id] = true
	}
	for _, l := range w.Lines() {
		w.choice[l.Item.ID] = chosen[l.Item.ID]
	}
	return nil
}
