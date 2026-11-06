// Package routing decides which tool, model and effort handles which kind of task, and
// how that choice is written down: Claude Code settings, subagent frontmatter and a short
// table in the personal instructions. It holds data and pure functions only.
package routing

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ToolClaude  = "claude"
	ToolCopilot = "copilot"
	ToolGemini  = "gemini"
	ToolCodex   = "codex"
)

// Claude and Codex model ids, as profiles/home/claude/model-settings.json and
// profiles/home/codex/default.toml spell them.
const (
	Opus   = "claude-opus-5-5"
	Sonnet = "claude-sonnet-5-5"
	Haiku  = "claude-haiku-5-5"
	Sol    = "gpt-6.1-sol"
)

const (
	GeminiPro  = "gemini_pro"  // Google AI Pro
	GeminiPlus = "gemini_plus" // Google AI Plus
)

// legacyPlans are older spellings, read as the plan they became.
var legacyPlans = map[string]string{"gemini": GeminiPro}

// NormalizePlans replaces legacy plan names and drops duplicates. A nil list stays nil.
func NormalizePlans(plans []string) []string {
	if plans == nil {
		return nil
	}
	out := make([]string, 0, len(plans))
	for _, pl := range plans {
		if n, ok := legacyPlans[pl]; ok {
			pl = n
		}
		if !contains(out, pl) {
			out = append(out, pl)
		}
	}
	return out
}

// Choice is one tool with a model and a reasoning effort. Effort is empty where the tool
// does not let you pick one; Model is empty for Gemini, whose tools pick their own. Plan is
// the Google plan behind a Gemini choice, which decides the limits.
type Choice struct {
	Tool, Model, Effort, Plan string
}

// Task is a kind of work. Agent names the Claude Code subagent that does it, if any.
type Task struct {
	ID, Label, Agent string
}

var Tasks = []Task{
	{"explore", "Find code", "scout"},
	{"routine", "Implement, tests, bug fixes", "implement"},
	{"hard", "Multi-file refactor, ambiguous design", ""},
	{"review", "Review before commit", "review"},
	{"mechanical", "Renames, boilerplate", "mechanic"},
	{"research", "Long documents, literature", ""},
}

// Preset is a complete routing: a primary choice per task and the Claude Code defaults.
type Preset struct {
	ID, Label, Summary string
	DefaultModel       string        // top-level "model" in Claude Code's settings.json
	ModelEfforts       []ModelEffort // modelSettings: the effort each model starts with
	primary            map[string]Choice
}

type ModelEffort struct{ Model, Effort string }

func claude(model, effort string) Choice {
	return Choice{Tool: ToolClaude, Model: model, Effort: effort}
}

// The only Claude settings the benchmark evidence in profiles/home/AGENTS.md ("Model
// choice") finds worth using: each other effort is beaten on score and cost by a
// neighbouring one. Every preset uses these and no others, so presets differ only in which
// model takes which task.
var (
	opus   = claude(Opus, "medium")
	sonnet = claude(Sonnet, "high")
	haiku  = claude(Haiku, "medium")
)

// efforts is what modelSettings gets in every preset; it equals
// profiles/home/claude/model-settings.json.
func efforts() []ModelEffort {
	return []ModelEffort{{Opus, "medium"}, {Sonnet, "high"}, {Haiku, "medium"}}
}

var presets = []*Preset{
	{
		ID: "saver", Label: "Saver", Summary: "Sonnet everywhere, Opus only when you escalate",
		DefaultModel: Sonnet, ModelEfforts: efforts(),
		primary: map[string]Choice{
			"explore": haiku, "routine": sonnet, "hard": sonnet,
			"review": sonnet, "mechanical": haiku, "research": sonnet,
		},
	},
	{
		ID: "balanced", Label: "Balanced", Summary: "Sonnet daily, Opus for hard tasks and review",
		DefaultModel: Sonnet, ModelEfforts: efforts(),
		primary: map[string]Choice{
			"explore": haiku, "routine": sonnet, "hard": opus,
			"review": opus, "mechanical": haiku, "research": sonnet,
		},
	},
	{
		ID: "quality", Label: "Quality", Summary: "Opus by default and for all code work",
		DefaultModel: Opus, ModelEfforts: efforts(),
		primary: map[string]Choice{
			"explore": haiku, "routine": opus, "hard": opus,
			"review": opus, "mechanical": haiku, "research": sonnet,
		},
	},
}

const (
	// None is the routing answer that leaves the user's models alone; an empty answer
	// means the same.
	None = "none"
	// Default is the preset the wizard proposes to a personal machine.
	Default = "balanced"
)

// Active returns the preset id, or "" when routing is off (empty or None).
func Active(preset string) string {
	if preset == None {
		return ""
	}
	return preset
}

func Presets() []*Preset { return presets }

func Get(id string) (*Preset, bool) {
	for _, p := range presets {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

// HasClaudePlan reports whether plans include Claude Pro or Max; the presets route
// Claude Code work and need one.
func HasClaudePlan(plans []string) bool {
	return contains(plans, "claude_pro") || contains(plans, "claude_max")
}

// BothGooglePlans reports whether plans list the two Google plans together.
func BothGooglePlans(plans []string) bool {
	plans = NormalizePlans(plans)
	return contains(plans, GeminiPro) && contains(plans, GeminiPlus)
}

func checkNames(preset string, plans []string) error {
	if preset = Active(preset); preset != "" {
		if _, ok := Get(preset); !ok {
			return fmt.Errorf("unknown routing preset %q (saver, balanced or quality)", preset)
		}
	}
	for _, pl := range NormalizePlans(plans) {
		if _, ok := PlanByID(pl); !ok {
			return fmt.Errorf("unknown plan %q", pl)
		}
	}
	if BothGooglePlans(plans) {
		return fmt.Errorf("plans lists %s and %s; keep the one you have", GeminiPro, GeminiPlus)
	}
	return nil
}

// Check reports an unknown preset or plan, or a preset without a Claude plan. An empty
// preset or "none" means no routing and is valid.
func Check(preset string, plans []string) error {
	if err := checkNames(preset, plans); err != nil {
		return err
	}
	if preset = Active(preset); preset != "" && !HasClaudePlan(NormalizePlans(plans)) {
		return fmt.Errorf("routing %q needs a Claude plan: add claude_pro or claude_max to plans", preset)
	}
	return nil
}

// Resolve validates the routing answers and returns the preset that applies. Without
// Claude Code nothing is routed: id is empty and skipped is true (names are still checked).
func Resolve(preset string, plans []string, claudeCode bool) (id string, skipped bool, err error) {
	preset = Active(preset)
	if claudeCode || preset == "" {
		if err := Check(preset, plans); err != nil {
			return "", false, err
		}
		return preset, false, nil
	}
	if err := checkNames(preset, plans); err != nil {
		return "", false, err
	}
	return "", true, nil
}

// EffortDiff is a model whose effort the profile's model-settings file sets differently
// from the preset.
type EffortDiff struct{ Model, Profile, Preset string }

// Differences lists the models for which a profile's model-settings JSON (the file
// behind claude_model_settings) sets an effortLevel other than the preset's. Models the
// file does not mention, and entries without an effortLevel, are not differences.
func (p *Preset) Differences(settings []byte) ([]EffortDiff, error) {
	return ModelDifferences(p.ModelEfforts, settings)
}

func ModelDifferences(efforts []ModelEffort, settings []byte) ([]EffortDiff, error) {
	var file map[string]map[string]any
	if err := json.Unmarshal(settings, &file); err != nil {
		return nil, err
	}
	var out []EffortDiff
	for _, me := range efforts {
		if got, ok := file[me.Model]["effortLevel"].(string); ok && got != me.Effort {
			out = append(out, EffortDiff{me.Model, got, me.Effort})
		}
	}
	return out, nil
}

// Row is one task with its primary choice and the fallbacks for when that one's usage
// limit is hit, in the order to try them.
type Row struct {
	Task      Task
	Use       Choice
	Fallbacks []Choice
}

// Rows lists every task. Fallbacks come only from plans the user owns: GitHub Copilot
// (the same Claude model), then Gemini, then Codex. Research tries Gemini first, since
// long documents are what it is good at.
func (p *Preset) Rows(plans []string) []Row {
	plans = NormalizePlans(plans)
	gemini := Choice{Tool: ToolGemini, Plan: GeminiPlus}
	if contains(plans, GeminiPro) {
		gemini.Plan = GeminiPro
	}
	var rows []Row
	for _, t := range Tasks {
		use := p.primary[t.ID]
		order := []string{ToolCopilot, ToolGemini, ToolCodex}
		if t.ID == "research" {
			order = []string{ToolGemini, ToolCopilot, ToolCodex}
		}
		var fb []Choice
		for _, tool := range order {
			switch {
			case tool == ToolCopilot && contains(plans, "copilot"):
				fb = append(fb, Choice{Tool: ToolCopilot, Model: use.Model})
			case tool == ToolGemini && (contains(plans, GeminiPro) || contains(plans, GeminiPlus)):
				fb = append(fb, gemini)
			case tool == ToolCodex && contains(plans, "chatgpt"):
				fb = append(fb, Choice{Tool: ToolCodex, Model: Sol, Effort: codexEffort[t.ID]})
			}
		}
		rows = append(rows, Row{Task: t, Use: use, Fallbacks: fb})
	}
	return rows
}

var codexEffort = map[string]string{
	"explore": "low", "mechanical": "low", "routine": "medium", "review": "medium", "research": "medium", "hard": "xhigh",
}

// Agent returns the choice for a Claude Code subagent by name (scout, mechanic, implement, review).
func (p *Preset) Agent(name string) (Choice, bool) {
	for _, t := range Tasks {
		if t.Agent == name {
			return p.primary[t.ID], true
		}
	}
	return Choice{}, false
}

var names = map[string]string{
	Opus: "Opus 5.5", Sonnet: "Sonnet 5.5", Haiku: "Haiku 5.5", Sol: "GPT-6.1 Sol",
	"gpt-6-luna": "GPT-6 Luna", "gpt-6.1-terra": "GPT-6.1 Terra", "gemini-4-argon": "Gemini 4 Argon",
	"gpt-6-astra": "GPT-6 Astra",
	"auto":        "account default",
}

func (c Choice) String() string {
	var s string
	switch c.Tool {
	case ToolClaude:
		s = "Claude " + name(c.Model)
	case ToolCopilot:
		s = "Copilot " + name(c.Model)
	case ToolGemini:
		s = "Antigravity (confirm access)"
		if c.Plan == GeminiPro {
			s = "Antigravity (AI Pro)"
		}
	case ToolAntigravity:
		s = "Antigravity " + name(c.Model)
	case ToolGeminiCLI:
		s = "Gemini CLI " + name(c.Model)
	case ToolCodex:
		s = "Codex " + name(c.Model)
	}
	if c.Effort != "" {
		s += " " + c.Effort
	}
	return s
}

func name(model string) string {
	if n, ok := names[model]; ok {
		return n
	}
	return model
}

// Markers delimit the routing section so a later run replaces it instead of adding another.
const (
	Begin = "<!-- agtk:model-routing:begin -->"
	End   = "<!-- agtk:model-routing:end -->"
)

// HandoffLine follows the intro of every routing section; checks/routing.mjs repeats it.
const HandoffLine = "When a limit is hit, run the handoff skill, then continue in the fallback tool."

// Section is the delimited Markdown block for the instructions file.
func (p *Preset) Section(plans []string) string {
	var b strings.Builder
	b.WriteString(Begin + "\n## Model routing\n")
	fmt.Fprintf(&b, "Preset %s. Use the row for the task; take the fallback when the first choice hits its usage limit. %s\n\n", p.ID, HandoffLine)
	b.WriteString(p.Table(plans))
	b.WriteString("\n" + End)
	return b.String()
}

// Table is the task | use | fallback Markdown table.
func (p *Preset) Table(plans []string) string {
	return table(p.Rows(plans), true)
}

func table(rows []Row, showAgent bool) string {
	var b strings.Builder
	b.WriteString("| Task | Use | Fallback |\n|---|---|---|\n")
	for _, r := range rows {
		use := r.Use.String()
		if showAgent && r.Task.Agent != "" {
			use += " (`" + r.Task.Agent + "` agent)"
		}
		fb := "none"
		if len(r.Fallbacks) > 0 {
			var l []string
			for _, c := range r.Fallbacks {
				l = append(l, c.String())
			}
			fb = strings.Join(l, " then ")
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.Task.Label, use, fb)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Splice puts section into doc: in place of an earlier routing section, else at the end.
// A marker without its partner is dropped, so it can never swallow the text after it.
func Splice(doc, section string) string {
	for pos := 0; ; {
		j := strings.Index(doc[pos:], End)
		if j < 0 {
			break
		}
		j += pos
		if i := strings.LastIndex(doc[pos:j], Begin); i >= 0 {
			i += pos
			return dropMarkers(doc[:i]) + section + dropMarkers(doc[j+len(End):])
		}
		pos = j + len(End)
	}
	return strings.TrimRight(dropMarkers(doc), "\r\n \t") + "\n\n" + section + "\n"
}

// dropMarkers removes marker lines, and markers that sit inside other text.
func dropMarkers(s string) string {
	for _, m := range []string{Begin, End} {
		s = regexp.MustCompile(`(?m)^[ \t]*`+regexp.QuoteMeta(m)+`[ \t]*(\r?\n|\z)`).ReplaceAllString(s, "")
		s = strings.ReplaceAll(s, m, "")
	}
	return s
}

// SetFrontmatter sets model and effort in the YAML frontmatter at the top of text and
// touches nothing else: other keys, line endings, a BOM and the body keep their bytes. A
// key that is missing is added before the closing line. Text without frontmatter is returned as is.
func SetFrontmatter(text, model, effort string) string {
	if rest, ok := strings.CutPrefix(text, bom); ok {
		return bom + SetFrontmatter(rest, model, effort)
	}
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return text
	}
	eol := lines[0][len("---"):]
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return text
	}
	want := map[string]string{"model": model, "effort": effort}
	seen := map[string]bool{}
	for i := 1; i < end; i++ {
		for _, key := range []string{"model", "effort"} {
			if strings.HasPrefix(lines[i], key+":") {
				if want[key] == "" {
					lines[i] = ""
				} else {
					lines[i] = key + ": " + want[key] + lines[i][len(strings.TrimRight(lines[i], "\r\n")):]
				}
				seen[key] = true
			}
		}
	}
	var add string
	for _, key := range []string{"model", "effort"} {
		if !seen[key] && want[key] != "" {
			add += key + ": " + want[key] + eol
		}
	}
	lines[end] = add + lines[end]
	return strings.Join(lines, "")
}

const bom = "\xef\xbb\xbf"

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
