package routing

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	ToolAntigravity = "antigravity"
	ToolGeminiCLI   = "gemini-cli"
)

type PlanOption struct {
	ID, Label, Tool, Detail string
	Company, Metered        bool
}

// Plan names describe how a user signs in or is billed. Model access is confirmed
// separately in the user's client.
var planOptions = []PlanOption{
	{"claude_pro", "Claude Pro", ToolClaude, "Personal subscription", false, false},
	{"claude_max", "Claude Max", ToolClaude, "Personal subscription with higher limits", false, false},
	{"claude_ultra", "Claude Ultra", ToolClaude, "Personal subscription", false, false},
	{"claude_team", "Claude Team", ToolClaude, "Company seat with organization controls", true, false},
	{"claude_enterprise", "Claude Enterprise", ToolClaude, "Company seat with organization controls", true, false},
	{"claude_api", "Claude API / cloud provider", ToolClaude, "Console, Bedrock, Vertex or Foundry billing", true, true},
	{"chatgpt_free", "ChatGPT Free", ToolCodex, "Luna in the desktop app, when available", false, false},
	{"chatgpt_go", "ChatGPT Go", ToolCodex, "Luna in the desktop app, when available", false, false},
	{"chatgpt_plus", "ChatGPT Plus", ToolCodex, "Personal subscription with CLI and IDE access", false, false},
	{"chatgpt_pro", "ChatGPT Pro", ToolCodex, "Personal subscription with higher usage limits", false, false},
	{"chatgpt_business", "ChatGPT Business", ToolCodex, "Company workspace and seat controls", true, false},
	{"chatgpt_edu", "ChatGPT Edu", ToolCodex, "Education workspace and seat controls", true, false},
	{"chatgpt_enterprise", "ChatGPT Enterprise", ToolCodex, "Company workspace and seat controls", true, false},
	{"codex_api", "Codex API", ToolCodex, "Billed to an API project", true, true},
	{"copilot_free", "Copilot Free", ToolCopilot, "Automatic model selection", false, false},
	{"copilot_student", "Copilot Student", ToolCopilot, "Automatic model selection", false, false},
	{"copilot_pro", "Copilot Pro", ToolCopilot, "Individual model choices and included AI credits", false, false},
	{"copilot_pro_plus", "Copilot Pro+", ToolCopilot, "Higher AI credit allowance and premium model access", false, false},
	{"copilot_max", "Copilot Max", ToolCopilot, "Highest individual AI credit allowance", false, false},
	{"copilot_business", "Copilot Business", ToolCopilot, "Models depend on organization policy", true, false},
	{"copilot_enterprise", "Copilot Enterprise", ToolCopilot, "Models depend on organization policy", true, false},
	{"copilot_access", "Copilot Access", ToolCopilot, "Enterprise model access with metered usage", true, true},
	{"antigravity_free", "Antigravity (baseline)", ToolAntigravity, "Confirm available models in Antigravity", false, false},
	{GeminiPlus, "Google AI Plus", ToolAntigravity, "Confirm Antigravity access separately", false, false},
	{GeminiPro, "Google AI Pro", ToolAntigravity, "Confirm Antigravity models in your account", false, false},
	{"gemini_ultra", "Google AI Ultra", ToolAntigravity, "Confirm Antigravity models in your account", false, false},
	{"gemini_enterprise", "Antigravity for Gemini Enterprise", ToolAntigravity, "Company Google Cloud access", true, true},
	{"gemini_api", "Gemini CLI API / Vertex AI", ToolGeminiCLI, "Separate API or Google Cloud access", true, true},
	{"company_gateway", "Company gateway", "", "Approved client and model IDs from IT", true, true},
	{"local_model", "Local model", "", "A model already configured in a compatible client", true, false},
}

func PlanOptions() []PlanOption { return planOptions }

func PlanByID(id string) (PlanOption, bool) {
	for _, p := range planOptions {
		if p.ID == id {
			return p, true
		}
	}
	switch id {
	case "chatgpt":
		return PlanOption{ID: id, Label: "ChatGPT", Tool: ToolCodex}, true
	case "copilot":
		return PlanOption{ID: id, Label: "GitHub Copilot", Tool: ToolCopilot}, true
	}
	return PlanOption{}, false
}

// Model records the exact client model the user confirmed. Custom endpoints can
// declare their own IDs, capability class, and supported reasoning efforts.
type Model struct {
	Plan    string   `yaml:"plan"`
	Tool    string   `yaml:"tool"`
	ID      string   `yaml:"id"`
	Class   string   `yaml:"class,omitempty"` // fast, standard, deep
	Efforts []string `yaml:"efforts,omitempty"`
}

func (m Model) Key() string { return m.Plan + "|" + m.Tool + "|" + m.ID }

var builtInModels = []Model{
	{Tool: ToolClaude, ID: Haiku, Class: "fast", Efforts: []string{"medium"}},
	{Tool: ToolClaude, ID: Sonnet, Class: "standard", Efforts: []string{"high"}},
	{Tool: ToolClaude, ID: Opus, Class: "deep", Efforts: []string{"medium"}},
	{Tool: ToolCodex, ID: "gpt-6-luna", Class: "fast"},
	{Tool: ToolCodex, ID: Sol, Class: "standard", Efforts: []string{"medium", "xhigh"}},
	{Tool: ToolCodex, ID: "gpt-6.1-terra", Class: "standard"},
	{Tool: ToolCodex, ID: "gpt-6-astra", Class: "deep", Efforts: []string{"low", "medium", "xhigh"}},
	{Tool: ToolCopilot, ID: "auto", Class: "standard"},
	{Tool: ToolCopilot, ID: Haiku, Class: "fast"},
	{Tool: ToolCopilot, ID: Sonnet, Class: "standard"},
	{Tool: ToolCopilot, ID: Opus, Class: "deep"},
	{Tool: ToolCopilot, ID: Sol, Class: "standard"},
	{Tool: ToolCopilot, ID: "gemini-4-argon", Class: "deep"},
	{Tool: ToolAntigravity, ID: "auto", Class: "standard"},
	{Tool: ToolAntigravity, ID: "gemini-4-argon", Class: "deep"},
	{Tool: ToolGeminiCLI, ID: "auto", Class: "standard"},
}

func Candidates(plans []string) []Model {
	var out []Model
	for _, id := range NormalizePlans(plans) {
		p, ok := PlanByID(id)
		if !ok {
			continue
		}
		for _, m := range builtInModels {
			if m.Tool == p.Tool && offered(id, m.ID) {
				m.Plan = p.ID
				out = append(out, m)
			}
		}
		if id == "local_model" {
			for _, tool := range []string{ToolClaude, ToolCodex} {
				out = append(out, Model{Plan: id, Tool: tool, ID: "auto", Class: "standard"})
			}
		}
	}
	return out
}

func offered(plan, model string) bool {
	switch plan {
	case "copilot_free", "copilot_student":
		return model == "auto"
	case "chatgpt_free", "chatgpt_go":
		return model == "gpt-6-luna"
	case "antigravity_free", GeminiPlus:
		return model == "auto"
	}
	return true
}

var modelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/@-]{0,127}$`)

func ValidateModel(m Model) error {
	p, ok := PlanByID(m.Plan)
	if !ok || (p.Tool != "" && p.Tool != m.Tool) || !modelID.MatchString(m.ID) {
		return fmt.Errorf("invalid model, plan or client")
	}
	switch m.Tool {
	case ToolClaude, ToolCodex, ToolCopilot, ToolAntigravity, ToolGeminiCLI:
	default:
		return fmt.Errorf("unknown model client")
	}
	if m.Class != "" && !slices.Contains([]string{"fast", "standard", "deep"}, m.Class) {
		return fmt.Errorf("model class must be fast, standard or deep")
	}
	for _, e := range m.Efforts {
		if !slices.Contains(effortOrder, e) {
			return fmt.Errorf("unsupported reasoning effort")
		}
	}
	return nil
}

var effortOrder = []string{"minimal", "low", "medium", "high", "xhigh", "max", "ultra"}

func (m Model) Choice(effort string) Choice {
	return Choice{Tool: m.Tool, Model: m.ID, Plan: m.Plan, Effort: effort}
}

// Setup supports older profiles with a preset and plan names only. New profiles
// choose from confirmed models and may route Codex independently of Claude Code.
type Setup struct {
	Preset *Preset
	Plans  []string
	Models []Model
	Tools  []string
}

func ResolveSetup(preset string, plans []string, models []Model, tools []string) (*Setup, bool, error) {
	plans = NormalizePlans(plans)
	if models == nil {
		id, skipped, err := Resolve(preset, plans, configures(tools, ToolClaude))
		if err != nil || id == "" {
			return nil, skipped, err
		}
		p, _ := Get(id)
		return &Setup{Preset: p, Plans: plans, Tools: tools}, false, nil
	}
	if Active(preset) == "" {
		return nil, false, nil
	}
	p, ok := Get(preset)
	if !ok {
		return nil, false, fmt.Errorf("unknown routing preset")
	}
	for _, pl := range plans {
		if _, ok := PlanByID(pl); !ok {
			return nil, false, fmt.Errorf("unknown AI plan")
		}
	}
	if len(models) == 0 {
		return nil, false, fmt.Errorf("confirm at least one available model to use routing")
	}
	seen := map[string]bool{}
	for _, m := range models {
		if err := ValidateModel(m); err != nil {
			return nil, false, err
		}
		if !slices.Contains(plans, m.Plan) || seen[m.Key()] {
			return nil, false, fmt.Errorf("each confirmed model needs a selected plan and may appear only once")
		}
		if !offered(m.Plan, m.ID) {
			return nil, false, fmt.Errorf("the selected plan does not offer that model choice")
		}
		seen[m.Key()] = true
	}
	return &Setup{Preset: p, Plans: plans, Models: models, Tools: tools}, false, nil
}

func configures(tools []string, tool string) bool {
	if tool == ToolClaude {
		return len(tools) == 0 || slices.Contains(tools, "claude_code")
	}
	if tool == ToolCodex {
		return len(tools) == 0 || slices.Contains(tools, "codex")
	}
	return false
}

func (s *Setup) preference(t Task) int {
	switch {
	case t.ID == "explore" || t.ID == "mechanical":
		return 0
	case s.Preset.ID == "quality" && t.ID != "research":
		return 2
	case s.Preset.ID == "balanced" && (t.ID == "hard" || t.ID == "review"):
		return 2
	}
	return 1
}

func capability(m Model) int {
	switch m.Class {
	case "fast":
		return 0
	case "deep":
		return 2
	default:
		return 1
	}
}

func (m Model) configurable() bool {
	return m.ID != "auto" && m.Plan != "chatgpt_free" && m.Plan != "chatgpt_go"
}

func (s *Setup) ranked(t Task, tool string) []Model {
	var models []Model
	for _, m := range s.Models {
		if tool == "" || (tool == m.Tool && m.configurable()) {
			models = append(models, m)
		}
	}
	want := s.preference(t)
	score := func(m Model) int {
		level := capability(m)
		p, _ := PlanByID(m.Plan)
		fit := 6 * (want - level)
		if level > want {
			fit = 3 * (level - want)
		}
		if !configures(s.Tools, m.Tool) {
			fit++
		}
		if m.ID == "auto" {
			fit++
		}
		if p.Metered {
			fit++
			if s.Preset.ID == "saver" {
				fit += 20
			}
		}
		return fit
	}
	slices.SortStableFunc(models, func(a, b Model) int { return cmp.Compare(score(a), score(b)) })
	return models
}

func (s *Setup) choice(t Task, m Model) Choice {
	if len(m.Efforts) == 0 {
		return m.Choice("")
	}
	effort := m.Efforts[0]
	deep := s.preference(t) == 2
	for _, e := range m.Efforts[1:] {
		if order := cmp.Compare(slices.Index(effortOrder, e), slices.Index(effortOrder, effort)); (deep && order > 0) || (!deep && order < 0) {
			effort = e
		}
	}
	return m.Choice(effort)
}

func (s *Setup) Rows() []Row {
	if s.Models == nil {
		return s.Preset.Rows(s.Plans)
	}
	var rows []Row
	for _, task := range Tasks {
		models := s.ranked(task, "")
		use := s.choice(task, models[0])
		row := Row{Task: task, Use: use}
		for _, m := range models[1:] {
			if m.Tool != use.Tool || m.ID != use.Model {
				row.Fallbacks = []Choice{s.choice(task, m)}
				break
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *Setup) Default(tool string) Choice {
	if s == nil || !configures(s.Tools, tool) {
		return Choice{}
	}
	if s.Models == nil {
		if tool == ToolClaude {
			return Choice{Tool: tool, Model: s.Preset.DefaultModel}
		}
		return Choice{}
	}
	models := s.ranked(Task{ID: "routine"}, tool)
	if len(models) == 0 {
		return Choice{}
	}
	return s.choice(Task{ID: "routine"}, models[0])
}

func (s *Setup) ClaudeEfforts() []ModelEffort {
	if s.Models == nil {
		return s.Preset.ModelEfforts
	}
	var out []ModelEffort
	seen := map[string]bool{}
	for _, m := range s.Models {
		if m.Tool == ToolClaude && m.configurable() && !seen[m.ID] {
			if c := s.choice(Task{ID: "routine"}, m); c.Effort != "" {
				out = append(out, ModelEffort{m.ID, c.Effort})
			}
			seen[m.ID] = true
		}
	}
	return out
}

func (s *Setup) Agent(name string) (Choice, bool) {
	if s.Models == nil {
		return s.Preset.Agent(name)
	}
	for _, t := range Tasks {
		if t.Agent == name {
			models := s.ranked(t, ToolClaude)
			if len(models) > 0 {
				return s.choice(t, models[0]), true
			}
		}
	}
	return Choice{}, false
}

func (s *Setup) Section(tool string) string {
	if s == nil || !configures(s.Tools, tool) {
		return ""
	}
	if s.Models == nil {
		if tool == ToolClaude {
			return s.Preset.Section(s.Plans)
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(Begin + "\n## Model routing\n")
	fmt.Fprintf(&b, "%s. Switch clients or accounts manually when needed. %s\n\n", s.Preset.Label, HandoffLine)
	b.WriteString(table(s.Rows(), false))
	b.WriteString("\n" + End)
	return b.String()
}

func (s *Setup) Reason(t Task) string {
	var reason string
	switch s.preference(t) {
	case 0:
		reason = "Prefer fast feedback for search and mechanical work."
	case 2:
		reason = "Prefer deeper reasoning for this task."
	default:
		reason = "Prefer a general-purpose model for this task."
	}
	if s.Preset.ID == "saver" {
		reason = "Prefer included usage. " + reason
	}
	if len(s.Models) > 0 {
		picked := s.ranked(t, "")[0]
		if capability(picked) != s.preference(t) {
			reason += " This preset uses the best fit you confirmed."
		}
	}
	return reason
}
