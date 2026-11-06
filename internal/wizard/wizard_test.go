package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
	"github.com/willlovecode/agent-config/internal/scan"
)

// fakeScan is a machine with Claude Code, Node and Java but no Codex.
func fakeScan() *scan.Result {
	return &scan.Result{OS: "windows", Arch: "amd64", Tier: "A", Programs: []*scan.Program{
		{Name: "claude", Fact: "claude_code", Version: "2.1.318"},
		{Name: "codex", Fact: "codex"},
		{Name: "node", Fact: "node", Version: "24.19.0"},
		{Name: "java", Fact: "jdk", Version: "21.0.13"},
	}}
}

func newWizard(t *testing.T) *Wizard {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	return New(cat, assets, fakeScan())
}

func on(w *Wizard) map[string]bool {
	m := map[string]bool{}
	for _, l := range w.Lines() {
		m[l.Item.ID] = l.On
	}
	return m
}

func TestDefaultsFollowTheScan(t *testing.T) {
	w := newWizard(t)
	if got := strings.Join(w.Values("tools"), ","); got != "claude_code" {
		t.Errorf("tools default = %s, want only what was found", got)
	}
	if got := strings.Join(w.Values("stacks"), ","); got != "java,javascript" {
		t.Errorf("stacks default = %s", got)
	}
	m := on(w)
	if _, ok := m["codex-rules"]; ok {
		t.Error("Codex items must not show when Codex is not chosen")
	}
	if !m["git-guard"] || !m["tdd"] || !m["context7"] {
		t.Errorf("personal defaults: %v", m)
	}
	if _, ok := m["stop-checks"]; ok {
		t.Error("project-scoped items belong to project setup, not this list")
	}
}

func TestSkillViewsKeepGeneralSkillsOptional(t *testing.T) {
	w := newWizard(t)
	lines := w.Lines()
	for _, id := range []string{"caveman", "i-have-adhd", "find-skills"} {
		t.Run(id, func(t *testing.T) {
			for _, view := range []string{"general", "popular"} {
				rows := FilterLines(lines, id, view)
				if len(rows) != 1 || rows[0].Item.ID != id {
					t.Fatalf("%s view missing %s", view, id)
				}
				if rows[0].On || rows[0].Recommended || !rows[0].Item.ExplicitOnly {
					t.Errorf("%s should require selection and activation", id)
				}
				details := strings.Join(ItemDetails(rows[0]), "\n")
				if !strings.Contains(details, "2026-11-05") || !strings.Contains(details, "Website: https://") {
					t.Errorf("details lack ranking date or website:\n%s", details)
				}
			}
		})
	}
	w.Set(w.Questions(StageProfile)[0], "company")
	if reason := w.Toggle("find-skills"); reason == "" || on(w)["find-skills"] {
		t.Error("Company must require approval for external skill discovery")
	}
	if reason := w.Toggle("i-have-adhd"); reason != "" || !on(w)["i-have-adhd"] {
		t.Errorf("Company should be able to select a local style skill: %q", reason)
	}
}

func TestApplyRecoveryUsesThisRun(t *testing.T) {
	fixtureHome := t.TempDir()
	backups := filepath.Join(install.StateDir(fixtureHome), "backups")
	previous, current := filepath.Join(backups, "1"), filepath.Join(backups, "2")
	for _, dir := range []string{previous, current} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := []byte(`{"entries":[]}`)
	if err := os.WriteFile(filepath.Join(previous, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	report := SummarizeApply("Backup: " + current + "\n")
	if report.CanUndo(fixtureHome) {
		t.Fatal("an early failure should not suggest undoing an older run")
	}
	if err := os.WriteFile(filepath.Join(current, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if !report.CanUndo(fixtureHome) {
		t.Fatal("the current run has a backup to restore")
	}
}

func TestCorporateAndChoices(t *testing.T) {
	w := newWizard(t)
	use := w.Questions(StageProfile)[0]
	w.Set(use, "company")
	if len(w.Questions(StageProfile)) != 1 {
		t.Fatal("profile only asks who the setup is for")
	}
	if reason := w.Toggle("context7"); reason == "" || on(w)["context7"] {
		t.Error("company cannot enable a network service without approval")
	}
	w.SetPolicy(policy.Policy{Items: []string{"context7"}, Source: "test policy"})
	before := on(w)["context7"]
	if reason := w.Toggle("context7"); reason != "" || on(w)["context7"] == before {
		t.Errorf("approved Context7 should be selectable: %q", reason)
	}
	w.Set(use, "personal")
	if w.Blocked(use) == "" || !w.Company() {
		t.Error("a personal answer must not bypass a machine policy")
	}
	w.Set(w.Questions(StageModels)[0], "codex")
	if !w.Has("tools", "codex") || !w.Has("tools", "claude_code") {
		t.Errorf("tools = %v", w.Values("tools"))
	}
	used, max := w.Budget()
	if used <= 0 || max != 3100 {
		t.Errorf("budget %d/%d", used, max)
	}
}

func TestProfileRoundTrip(t *testing.T) {
	w := newWizard(t)
	w.Toggle("tdd")
	p := w.Profile().At(filepath.Join(t.TempDir(), "profile.yaml"))
	b, err := yaml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"2.1.318", "amd64", "21.0.13"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("profile holds scan data %q:\n%s", leak, b)
		}
	}
	loaded, err := profile.Load(p.Path)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	w2 := newWizard(t)
	if err := w2.Load(loaded); err != nil {
		t.Fatal(err)
	}
	if on(w2)["tdd"] || !on(w2)["git-guard"] {
		t.Errorf("loaded choices: %v", on(w2))
	}
}

func TestAnswersKeepExplicitlyEmptyStacks(t *testing.T) {
	w := newWizard(t)
	p := &profile.Profile{Answers: profile.Answers{Stacks: []string{}}}
	if err := w.Load(p); err != nil {
		t.Fatal(err)
	}
	if len(w.Values("stacks")) != 0 || on(w)["context7"] {
		t.Fatal("an empty stack list should override scan defaults")
	}
}

func TestModelQuestions(t *testing.T) {
	w := newWizard(t)
	qs := w.Questions(StageModels)
	if len(qs) != 5 || qs[0].ID != "tools" || qs[1].ID != "routing" || qs[1].Multi ||
		qs[2].ID != "plans" || qs[3].ID != "models" || !qs[4].ReadOnly {
		t.Fatalf("models step: %+v", qs)
	}
	a := w.Answers()
	if a.Plans != nil || a.Routing != "" || w.One("routing") != "balanced" {
		t.Errorf("defaults: plans %v routing %q", a.Plans, a.Routing)
	}
	opts := qs[1].Options
	if opts[0].Value != routing.None || opts[0].Label != "Keep current models" {
		t.Errorf("the first routing option leaves the models alone: %+v", opts[0])
	}
	var notes []string
	for _, o := range opts[1:] {
		if o.Label == "" || o.Detail == "" {
			t.Errorf("routing option %q needs a one-line description", o.Value)
		}
	}
	for _, o := range opts {
		if w.Note(qs[1], o) != "" {
			notes = append(notes, o.Value)
		}
	}
	if len(notes) != 1 || notes[0] != "balanced" {
		t.Errorf("only balanced is recommended on a personal machine, got %v", notes)
	}

	w.Set(qs[2], "claude_pro")
	w.Set(qs[1], "quality")
	models := w.Questions(StageModels)[3]
	if w.Blocked(models) == "" || len(models.Options) < 3 {
		t.Fatal("plans suggest models but do not confirm access")
	}
	w.Set(models, models.Options[1].Value)
	a = w.Answers()
	if strings.Join(a.Plans, ",") != "claude_pro" || a.Routing != "quality" || len(a.Models) != 1 || a.Models[0].ID != routing.Sonnet {
		t.Errorf("answers: plans %v routing %q", a.Plans, a.Routing)
	}
	f := a.Facts(nil)
	if f["routing"] != "quality" || strings.Join(f["plans"].([]string), ",") != "claude_pro" {
		t.Errorf("facts: %v", f)
	}
}

func TestLeaveMyModelsAlone(t *testing.T) {
	w := newWizard(t)
	routingQ := w.Questions(StageModels)[1]
	plans := w.Questions(StageModels)[2]
	w.Set(routingQ, routing.None)
	if qs := w.Questions(StageModels); len(qs) != 2 || qs[1].ID != "routing" {
		t.Fatalf("plans are only asked while routing: %+v", qs)
	}
	if a := w.Answers(); a.Routing != "" || a.Plans != nil {
		t.Errorf("none leaves routing and plans out: %+v", a)
	}
	if strings.Contains(string(mustYAML(t, w.Profile())), "routing") {
		t.Error("a profile that leaves models alone must not name a routing")
	}
	// No Claude plan is fine now: nothing is routed.
	w.values["plans"] = nil
	if w.Blocked(plans) != "" || w.Blocked(routingQ) != "" {
		t.Error("plans must not block without routing")
	}
	// Back to a preset: plans are asked and block again.
	w.Set(routingQ, "balanced")
	if len(w.Questions(StageModels)) != 5 || w.Blocked(plans) == "" {
		t.Error("routing needs its plans")
	}
}

func TestRoutingDefaultFollowsUse(t *testing.T) {
	w := newWizard(t)
	use, routingQ := w.Questions(StageProfile)[0], w.Questions(StageModels)[1]
	w.Set(use, "company")
	if got := w.One("routing"); got != routing.None {
		t.Errorf("a corporate machine keeps its models by default, got %q", got)
	}
	var notes []string
	for _, o := range routingQ.Options {
		if w.Note(routingQ, o) != "" {
			notes = append(notes, o.Value)
		}
	}
	if len(notes) != 1 || notes[0] != routing.None {
		t.Errorf("recommended: %v", notes)
	}
	w.Set(use, "personal")
	if got := w.One("routing"); got != routing.Default {
		t.Errorf("personal default %q", got)
	}
	// An active preset cannot carry into Company without approved models.
	w.Set(routingQ, "saver")
	w.Set(use, "company")
	if got := w.One("routing"); got != routing.None || len(w.Questions(StageModels)[1].Options) != 1 {
		t.Errorf("unapproved routing carried over: %q", got)
	}
}

func TestCompanyModelChoicesComeFromPolicy(t *testing.T) {
	w := newWizard(t)
	model := routing.Model{Plan: "claude_enterprise", Tool: routing.ToolClaude, ID: "approved-model", Efforts: []string{"high"}}
	w.SetPolicy(policy.Policy{Source: "test", Models: []routing.Model{model}})
	w.Set(w.Questions(StageModels)[1], "balanced")
	plans := w.Questions(StageModels)[2]
	if len(plans.Options) != 1 || plans.Options[0].Value != model.Plan {
		t.Fatalf("Company plans: %+v", plans.Options)
	}
	w.Set(plans, model.Plan)
	models := w.Questions(StageModels)[3]
	if len(models.Options) != 1 || models.Options[0].Value != model.Key() {
		t.Fatalf("Company models: %+v", models.Options)
	}
	w.Set(models, model.Key())
	if err := w.Policy.CheckRouting(w.Answers()); err != nil {
		t.Fatal(err)
	}
}

func TestModelAnswersRoundTripAndOldFiles(t *testing.T) {
	w := newWizard(t)
	p := w.Profile().At(filepath.Join(t.TempDir(), "profile.yaml"))
	p.Answers.Routing, p.Answers.Plans = "saver", []string{"claude_max", "gemini_pro"}
	b, err := yaml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "routing: saver") || !strings.Contains(string(b), "plans:") {
		t.Fatalf("profile lacks the model answers:\n%s", b)
	}
	os.WriteFile(p.Path, b, 0o644)
	loaded, err := profile.Load(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	w2 := newWizard(t)
	if err := w2.Load(loaded); err != nil {
		t.Fatal(err)
	}
	if a := w2.Answers(); a.Routing != "saver" || strings.Join(a.Plans, ",") != "claude_max,gemini_pro" {
		t.Errorf("loaded: %+v", a)
	}

	// "none" is accepted and means the same as no routing.
	none := &profile.Profile{Answers: profile.Answers{Routing: "none", Plans: []string{"copilot"}}}
	w4 := newWizard(t)
	if err := w4.Load(none); err != nil {
		t.Fatal(err)
	}
	if a := w4.Answers(); a.Routing != "" || w4.One("routing") != routing.None {
		t.Errorf("routing none: %+v, wizard value %q", a, w4.One("routing"))
	}

	// An answers file from before routing existed must not turn it on.
	old := &profile.Profile{Answers: profile.Answers{Use: "personal", Tools: []string{"codex"}}}
	w3 := newWizard(t)
	if err := w3.Load(old); err != nil {
		t.Fatal(err)
	}
	if a := w3.Answers(); a.Routing != "" || w3.One("routing") != routing.None {
		t.Errorf("old answers got routing %q", a.Routing)
	}
	out, _ := yaml.Marshal(w3.Answers())
	if strings.Contains(string(out), "routing") {
		t.Errorf("empty routing must be omitted:\n%s", out)
	}

	bad := &profile.Profile{Answers: profile.Answers{Routing: "turbo"}}
	if err := newWizard(t).Load(bad); err == nil {
		t.Error("an unknown preset must be rejected")
	}
}

func TestLegacyGeminiPlanBecomesGeminiPro(t *testing.T) {
	w := newWizard(t)
	p := &profile.Profile{Answers: profile.Answers{Plans: []string{"claude_pro", "gemini"}, Routing: "balanced"}}
	if err := w.Load(p); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(w.Answers().Plans, ","); got != "claude_pro,gemini_pro" {
		t.Errorf("plans %s", got)
	}
	both := &profile.Profile{Answers: profile.Answers{Plans: []string{"claude_pro", "gemini_pro", "gemini_plus"}}}
	if newWizard(t).Load(both) == nil {
		t.Error("the two Google plans together must be rejected")
	}
}

func TestGoogleAccountsHaveSeparateOptions(t *testing.T) {
	w := newWizard(t)
	plans := w.Questions(StageModels)[2]
	w.Set(plans, "gemini_pro")
	w.Set(plans, "gemini_plus")
	if got := strings.Join(w.Values("plans"), ","); got != "gemini_plus,gemini_pro" {
		t.Errorf("plans %s, separate accounts can be selected together", got)
	}
	w.Set(plans, "gemini_plus") // off again
	if got := strings.Join(w.Values("plans"), ","); got != "gemini_pro" {
		t.Errorf("plans %s", got)
	}
	var labels []string
	for _, o := range plans.Options {
		labels = append(labels, o.Label)
	}
	if got := strings.Join(labels, "|"); !strings.Contains(got, "Google AI Plus|Google AI Pro") {
		t.Errorf("labels %s", got)
	}
}

func TestRoutesNeedAnAccountAndConfirmedModel(t *testing.T) {
	w := newWizard(t)
	plans := w.Questions(StageModels)[2]
	if len(w.Values("plans")) != 0 || w.Blocked(plans) == "" {
		t.Errorf("no plans must be blocked with a message, plans %v", w.Values("plans"))
	}
	w.Set(plans, "copilot_free")
	if msg := w.Blocked(plans); msg != "" {
		t.Errorf("Copilot alone can provide advisory routes: %s", msg)
	}
	models := w.Questions(StageModels)[3]
	if len(models.Options) != 1 || w.Blocked(models) == "" {
		t.Fatalf("Copilot Free can confirm Auto only: %+v", models.Options)
	}
	w.Set(models, models.Options[0].Value)
	if w.Blocked(models) != "" || len(w.Questions(StageModels)[4].Options) == 0 {
		t.Error("confirming Auto should enable a route preview")
	}
	tools := w.Questions(StageModels)[0]
	w.Set(tools, "claude_code")
	if w.Blocked(tools) == "" {
		t.Error("no tools must be blocked")
	}
}

func TestModelsCanRouteCodexOnly(t *testing.T) {
	w := newWizard(t)
	tools := w.Questions(StageModels)[0]
	w.Set(tools, "claude_code")
	w.Set(tools, "codex")
	w.Set(w.Questions(StageModels)[2], "chatgpt_plus")
	models := w.Questions(StageModels)[3]
	w.Set(models, models.Options[1].Value)
	setup, _, err := w.Answers().ResolveRouting()
	if err != nil || setup == nil || setup.Default(routing.ToolCodex).Model != routing.Sol || setup.Default(routing.ToolClaude).Model != "" {
		t.Fatalf("Codex-only confirmed routing: %+v, %v", setup, err)
	}
	// Legacy presets still apply only to Claude Code.
	file := &profile.Profile{Answers: profile.Answers{Tools: []string{"codex"}, Plans: []string{"copilot"}, Routing: "quality"}}
	w2 := newWizard(t)
	if err := w2.Load(file); err != nil {
		t.Fatal(err)
	}
	if a := w2.Answers(); a.Routing != "" || a.Plans != nil || strings.Contains(string(mustYAML(t, w2.Profile())), "routing") {
		t.Errorf("routing must not be saved without Claude Code: %+v", a)
	}
	bad := &profile.Profile{Answers: profile.Answers{Tools: []string{"codex"}, Plans: []string{"free"}, Routing: "quality"}}
	if newWizard(t).Load(bad) == nil {
		t.Error("an unknown plan must be rejected without Claude Code too")
	}
	noClaude := &profile.Profile{Answers: profile.Answers{Tools: []string{"claude_code"}, Plans: []string{"copilot"}, Routing: "quality"}}
	if err := newWizard(t).Load(noClaude); err == nil || !strings.Contains(err.Error(), "claude_pro or claude_max") {
		t.Errorf("routing without a Claude plan: %v", err)
	}
}

func TestBudgetIncludesTheRoutingTable(t *testing.T) {
	w := newWizard(t)
	w.Set(w.Questions(StageModels)[2], "claude_pro")
	models := w.Questions(StageModels)[3]
	w.Set(models, models.Options[1].Value)
	with, _ := w.Budget()
	w.values["routing"] = nil
	without, _ := w.Budget()
	if with-without < 100 {
		t.Errorf("routing table share %d tokens", with-without)
	}
}

func mustYAML(t *testing.T, v any) []byte {
	t.Helper()
	b, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestToolsQuestionOffersGemini(t *testing.T) {
	w := newWizard(t)
	tools := w.Questions(StageModels)[0]
	var values []string
	for _, o := range tools.Options {
		values = append(values, o.Value)
	}
	if strings.Join(values, ",") != "claude_code,codex,gemini" {
		t.Fatalf("tools options = %v", values)
	}
	if w.Has("tools", "gemini") {
		t.Error("Gemini is on by default only when the scan finds it")
	}
	if ToolLabel("gemini") != "Gemini CLI / Antigravity" {
		t.Errorf("label %q", ToolLabel("gemini"))
	}

	found := fakeScan()
	found.Programs = append(found.Programs, &scan.Program{Name: "gemini", Fact: "gemini", Version: "0.9.0"})
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(New(cat, assets, found).Values("tools"), ","); got != "claude_code,gemini" {
		t.Errorf("tools default with gemini on PATH = %s", got)
	}

	w.Set(tools, "gemini")
	a := w.Answers()
	if !contains(a.Tools, "gemini") || !contains(a.Tools, "claude_code") {
		t.Errorf("answers tools = %v", a.Tools)
	}
	if !on(w)["instructions-base"] {
		t.Error("the working agreement is offered for Gemini too")
	}
	p := w.Profile()
	if s := SetupSummary(p); !strings.Contains(s, "Claude Code + Gemini") {
		t.Errorf("summary %q", s)
	}
	if d := strings.Join(w.ReviewDetails(p), "\n"); !strings.Contains(d, "Gemini CLI / Antigravity / user\n  Instructions: instructions-base") {
		t.Errorf("review lacks the Gemini block:\n%s", d)
	}

	// Gemini alone is a valid choice; an answers file without the tool keeps its tools.
	w.Set(tools, "claude_code")
	if msg := w.Blocked(tools); msg != "" {
		t.Errorf("Gemini alone must be allowed: %s", msg)
	}
	old := &profile.Profile{Answers: profile.Answers{Use: "personal", Tools: []string{"codex"}}}
	if err := w.Load(old); err != nil || w.Has("tools", "gemini") {
		t.Errorf("an answers file without gemini must not gain it: %v %v", err, w.Values("tools"))
	}
}
