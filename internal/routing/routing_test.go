package routing

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func preset(t *testing.T, id string) *Preset {
	t.Helper()
	p, ok := Get(id)
	if !ok {
		t.Fatalf("no preset %q", id)
	}
	return p
}

func row(rows []Row, task string) Row {
	for _, r := range rows {
		if r.Task.ID == task {
			return r
		}
	}
	return Row{}
}

func TestPresets(t *testing.T) {
	if _, ok := Get(Default); !ok {
		t.Fatal("default preset missing")
	}
	for id, c := range map[string]struct {
		model string
		tasks map[string]Choice
	}{
		"saver": {Sonnet, map[string]Choice{
			"explore": claude(Haiku, "medium"), "routine": claude(Sonnet, "high"), "hard": claude(Sonnet, "high"),
			"review": claude(Sonnet, "high"), "mechanical": claude(Haiku, "medium"), "research": claude(Sonnet, "high"),
		}},
		"balanced": {Sonnet, map[string]Choice{
			"explore": claude(Haiku, "medium"), "routine": claude(Sonnet, "high"), "hard": claude(Opus, "medium"),
			"review": claude(Opus, "medium"), "mechanical": claude(Haiku, "medium"), "research": claude(Sonnet, "high"),
		}},
		"quality": {Opus, map[string]Choice{
			"explore": claude(Haiku, "medium"), "routine": claude(Opus, "medium"), "hard": claude(Opus, "medium"),
			"review": claude(Opus, "medium"), "mechanical": claude(Haiku, "medium"), "research": claude(Sonnet, "high"),
		}},
	} {
		p := preset(t, id)
		if p.DefaultModel != c.model {
			t.Errorf("%s default model %s, want %s", id, p.DefaultModel, c.model)
		}
		for task, want := range c.tasks {
			if got := row(p.Rows(nil), task).Use; got != want {
				t.Errorf("%s %s = %+v, want %+v", id, task, got, want)
			}
		}
	}
	if s := preset(t, "saver"); s.Summary != "Sonnet everywhere, Opus only when you escalate" {
		t.Errorf("saver summary %q", s.Summary)
	}
}

// The benchmark evidence in profiles/home/AGENTS.md allows exactly Opus at medium, Sonnet at
// high and Haiku at medium, and profiles/home/claude/model-settings.json holds them.
func TestPresetsOnlyUseTheEvidenceSettings(t *testing.T) {
	b, err := os.ReadFile("../../profiles/home/claude/model-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]struct{ EffortLevel string }
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{Opus: "medium", Sonnet: "high", Haiku: "medium"}
	for model, effort := range want {
		if file[model].EffortLevel != effort {
			t.Fatalf("model-settings.json has %s at %q, this test and the evidence say %s", model, file[model].EffortLevel, effort)
		}
	}
	for _, p := range Presets() {
		if len(p.ModelEfforts) != len(want) {
			t.Errorf("%s: modelSettings %v", p.ID, p.ModelEfforts)
		}
		for _, me := range p.ModelEfforts {
			if want[me.Model] != me.Effort {
				t.Errorf("%s: modelSettings has %s at %s", p.ID, me.Model, me.Effort)
			}
		}
		for _, r := range p.Rows(nil) {
			if want[r.Use.Model] != r.Use.Effort {
				t.Errorf("%s: %s uses %s at %s", p.ID, r.Task.ID, r.Use.Model, r.Use.Effort)
			}
		}
		if diffs, err := p.Differences(b); err != nil || len(diffs) != 0 {
			t.Errorf("%s differs from model-settings.json: %v %v", p.ID, diffs, err)
		}
	}
	for _, r := range preset(t, "saver").Rows(nil) {
		if r.Use.Model == Opus {
			t.Errorf("saver uses Opus for %s; Opus is for escalating by hand", r.Task.ID)
		}
	}
}

func TestDifferences(t *testing.T) {
	b := preset(t, "balanced")
	got, err := b.Differences([]byte(`{"claude-sonnet-5-5": {"effortLevel": "low"}, "claude-opus-5-5": {"effortLevel": "medium"}, "claude-haiku-5-5": {}, "claude-other-1": {"effortLevel": "max"}}`))
	if err != nil || len(got) != 1 || got[0] != (EffortDiff{Sonnet, "low", "high"}) {
		t.Errorf("differences %+v %v", got, err)
	}
	if got, err := b.Differences([]byte("{}")); err != nil || len(got) != 0 {
		t.Errorf("an empty file differs from nothing: %+v %v", got, err)
	}
	if _, err := b.Differences([]byte("{")); err == nil {
		t.Error("broken JSON must be an error")
	}
}

func TestEveryRowModelHasAnEffort(t *testing.T) {
	// Every model a row uses must have a modelSettings entry, or its effort is unset.
	for _, p := range Presets() {
		efforts := map[string]string{}
		for _, me := range p.ModelEfforts {
			efforts[me.Model] = me.Effort
		}
		for _, r := range p.Rows(nil) {
			if _, ok := efforts[r.Use.Model]; !ok {
				t.Errorf("%s: %s uses %s, which has no modelSettings entry", p.ID, r.Task.ID, r.Use.Model)
			}
		}
	}
}

func TestFallbacksFollowPlans(t *testing.T) {
	b := preset(t, "balanced")
	if fb := row(b.Rows([]string{"claude_pro"}), "hard").Fallbacks; len(fb) != 0 {
		t.Errorf("a Claude-only user has no fallback, got %v", fb)
	}
	all := []string{"claude_pro", "copilot", "gemini_pro", "chatgpt"}
	hard := row(b.Rows(all), "hard").Fallbacks
	want := []Choice{{Tool: ToolCopilot, Model: Opus}, {Tool: ToolGemini, Plan: GeminiPro}, {Tool: ToolCodex, Model: Sol, Effort: "xhigh"}}
	if len(hard) != len(want) {
		t.Fatalf("hard fallbacks %+v", hard)
	}
	for i := range want {
		if hard[i] != want[i] {
			t.Errorf("hard fallback %d = %+v, want %+v", i, hard[i], want[i])
		}
	}
	if got := row(b.Rows(all), "routine").Fallbacks[2]; got != (Choice{Tool: ToolCodex, Model: Sol, Effort: "medium"}) {
		t.Errorf("routine codex fallback %+v", got)
	}
	if got := row(b.Rows(all), "research").Fallbacks[0].Tool; got != ToolGemini {
		t.Errorf("research tries %s first, want gemini", got)
	}
	only := row(b.Rows([]string{"chatgpt"}), "review").Fallbacks
	if len(only) != 1 || only[0].Tool != ToolCodex {
		t.Errorf("chatgpt only: %+v", only)
	}
}

func TestGooglePlansDecideTheGeminiFallback(t *testing.T) {
	b := preset(t, "balanced")
	for _, c := range []struct {
		plans []string
		want  string
	}{
		{[]string{"claude_pro", "gemini_pro"}, "Antigravity (AI Pro)"},
		{[]string{"claude_pro", "gemini"}, "Antigravity (AI Pro)"}, // legacy name
		{[]string{"claude_pro", "gemini_plus"}, "Antigravity (confirm access)"},
	} {
		rows := b.Rows(c.plans)
		for _, r := range rows {
			if len(r.Fallbacks) != 1 || r.Fallbacks[0].Tool != ToolGemini || r.Fallbacks[0].String() != c.want {
				t.Errorf("%v, %s: fallbacks %v, want only %q", c.plans, r.Task.ID, r.Fallbacks, c.want)
			}
		}
		if table := b.Table(c.plans); !strings.Contains(table, "| "+c.want+" |") {
			t.Errorf("%v: table lacks %q:\n%s", c.plans, c.want, table)
		}
	}
	// Research lists Gemini before the other fallbacks on either plan.
	for _, g := range []string{"gemini_pro", "gemini_plus"} {
		fb := row(b.Rows([]string{"claude_pro", "copilot", g, "chatgpt"}), "research").Fallbacks
		if len(fb) != 3 || fb[0].Tool != ToolGemini || fb[1].Tool != ToolCopilot {
			t.Errorf("%s research fallbacks: %v", g, fb)
		}
	}
	if strings.Contains(b.Table([]string{"claude_pro", "gemini_plus"}), "AI Pro") {
		t.Error("AI Plus must not claim AI Pro limits")
	}
}

func TestCodexModelMatchesProfile(t *testing.T) {
	b, err := os.ReadFile("../../profiles/home/codex/default.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `model = "`+Sol+`"`) {
		t.Errorf("codex default.toml does not use %s", Sol)
	}
}

func TestAgent(t *testing.T) {
	b := preset(t, "balanced")
	for name, want := range map[string]Choice{
		"scout": claude(Haiku, "medium"), "mechanic": claude(Haiku, "medium"),
		"implement": claude(Sonnet, "high"), "review": claude(Opus, "medium"),
	} {
		if got, ok := b.Agent(name); !ok || got != want {
			t.Errorf("agent %s = %+v %v, want %+v", name, got, ok, want)
		}
	}
	if _, ok := b.Agent("other"); ok {
		t.Error("unknown agent must not match")
	}
}

func TestCheck(t *testing.T) {
	if err := Check("", nil); err != nil {
		t.Errorf("empty routing is valid: %v", err)
	}
	for _, plans := range [][]string{{"claude_pro", "gemini_pro"}, {"claude_max", "gemini_plus"}, {"claude_pro", "gemini"}} {
		if err := Check("balanced", plans); err != nil {
			t.Errorf("%v: %v", plans, err)
		}
	}
	if Check("fast", []string{"claude_pro"}) == nil || Check("balanced", []string{"claude_pro", "free"}) == nil || Check("", []string{"free"}) == nil {
		t.Error("unknown preset and plan must be rejected")
	}
	for _, plans := range [][]string{nil, {}, {"copilot", "chatgpt"}} {
		err := Check("balanced", plans)
		if err == nil || !strings.Contains(err.Error(), "claude_pro or claude_max") {
			t.Errorf("routing without a Claude plan (%v): %v", plans, err)
		}
	}
	if err := Check("", []string{"copilot"}); err != nil {
		t.Errorf("no routing needs no Claude plan: %v", err)
	}
	for _, plans := range [][]string{nil, {"copilot"}, {"claude_pro"}} {
		if err := Check(None, plans); err != nil {
			t.Errorf("%q needs no Claude plan (%v): %v", None, plans, err)
		}
	}
	if Check("balanced", []string{"claude_pro", "gemini_pro", "gemini_plus"}) == nil || Check("", []string{"gemini", "gemini_plus"}) == nil {
		t.Error("the two Google plans together must be rejected")
	}
}

func TestResolve(t *testing.T) {
	plans := []string{"claude_pro"}
	if id, skipped, err := Resolve("balanced", plans, true); id != "balanced" || skipped || err != nil {
		t.Errorf("with Claude Code: %q %v %v", id, skipped, err)
	}
	if id, skipped, err := Resolve("balanced", plans, false); id != "" || !skipped || err != nil {
		t.Errorf("without Claude Code the preset is skipped: %q %v %v", id, skipped, err)
	}
	if id, skipped, err := Resolve("", plans, false); id != "" || skipped || err != nil {
		t.Errorf("no preset: %q %v %v", id, skipped, err)
	}
	// "none" is routing off, exactly as the empty answer.
	if id, skipped, err := Resolve(None, nil, true); id != "" || skipped || err != nil {
		t.Errorf("none: %q %v %v", id, skipped, err)
	}
	if id, skipped, err := Resolve(None, plans, false); id != "" || skipped || err != nil {
		t.Errorf("none without Claude Code is not a skip: %q %v %v", id, skipped, err)
	}
	if _, _, err := Resolve(None, []string{"free"}, true); err == nil {
		t.Error("plan names are checked with none too")
	}
	// Bad names are rejected whether or not routing applies; a missing Claude plan only when it does.
	if _, _, err := Resolve("balanced", []string{"free"}, false); err == nil {
		t.Error("unknown plan must be rejected without Claude Code too")
	}
	if _, _, err := Resolve("turbo", plans, false); err == nil {
		t.Error("unknown preset must be rejected without Claude Code too")
	}
	if _, _, err := Resolve("balanced", []string{"copilot"}, true); err == nil {
		t.Error("a Claude plan is required")
	}
	if _, _, err := Resolve("balanced", []string{"copilot"}, false); err != nil {
		t.Errorf("no Claude Code, no Claude plan needed: %v", err)
	}
}

func TestNormalizePlans(t *testing.T) {
	if got := NormalizePlans([]string{"gemini", "copilot", "gemini_pro"}); strings.Join(got, ",") != "gemini_pro,copilot" {
		t.Errorf("got %v", got)
	}
	if NormalizePlans(nil) != nil || NormalizePlans([]string{}) == nil {
		t.Error("nil must stay nil and empty must stay empty")
	}
}

func TestTable(t *testing.T) {
	b := preset(t, "balanced")
	table := b.Table([]string{"claude_pro", "copilot", "chatgpt"})
	lines := strings.Split(table, "\n")
	if len(lines) != 2+len(Tasks) || lines[0] != "| Task | Use | Fallback |" {
		t.Fatalf("table:\n%s", table)
	}
	want := "| Multi-file refactor, ambiguous design | Claude Opus 5.5 medium | Copilot Opus 5.5 then Codex GPT-6.1 Sol xhigh |"
	if !strings.Contains(table, want) {
		t.Errorf("table lacks %q:\n%s", want, table)
	}
	if !strings.Contains(table, "Claude Haiku 5.5 medium (`scout` agent)") {
		t.Errorf("table should name the subagent:\n%s", table)
	}
	if strings.Contains(table, "Gemini") {
		t.Error("Gemini is not an owned plan")
	}
	if !strings.Contains(b.Table(nil), "| none |") {
		t.Error("no plans, no fallback")
	}
}

func TestSectionAndSplice(t *testing.T) {
	b := preset(t, "balanced")
	sec := b.Section([]string{"copilot"})
	if !strings.HasPrefix(sec, Begin) || !strings.HasSuffix(sec, End) || !strings.Contains(sec, "## Model routing") {
		t.Fatalf("section:\n%s", sec)
	}
	doc := "# Mine\n\nText — kept.\n"
	once := Splice(doc, sec)
	if !strings.HasPrefix(once, doc[:len(doc)-1]) || !strings.HasSuffix(once, End+"\n") {
		t.Fatalf("appended section:\n%s", once)
	}
	if again := Splice(once, sec); again != once {
		t.Errorf("same section twice changed the document:\n%s", again)
	}
	other := preset(t, "saver").Section(nil)
	swapped := Splice(once+"\nAfter.\n", other)
	if strings.Count(swapped, Begin) != 1 || strings.Contains(swapped, "Preset balanced") || !strings.Contains(swapped, "Preset saver") {
		t.Errorf("replaced section:\n%s", swapped)
	}
	if !strings.HasSuffix(swapped, "\nAfter.\n") || !strings.HasPrefix(swapped, "# Mine\n\nText — kept.") {
		t.Errorf("text around the section must stay:\n%s", swapped)
	}
}

func TestSpliceNeverLosesTextAroundStrayMarkers(t *testing.T) {
	sec := preset(t, "balanced").Section([]string{"claude_pro"})
	tail := "## Later\nText after — kept.\n"
	orphan := "# Mine\n\n" + Begin + "\nmid text\n\n"
	for _, c := range []struct {
		name, doc string
		kept      []string
		gone      []string
	}{
		{"begin without end", orphan + tail, []string{"mid text"}, nil},
		{"begin without end, CRLF", strings.ReplaceAll(orphan+tail, "\n", "\r\n"), []string{"mid text"}, nil},
		{"end without begin", "# Mine\n" + End + "\n" + tail, nil, nil},
		{"begin inside a line", "# Mine\nsee " + Begin + " here\n" + tail, []string{"see  here"}, nil},
		{"orphan begin, then a full section", orphan + Begin + "\nold table\n" + End + "\n" + tail, []string{"mid text"}, []string{"old table"}},
	} {
		got := Splice(c.doc, sec)
		if strings.Count(got, Begin) != 1 || strings.Count(got, End) != 1 {
			t.Errorf("%s: want exactly one section:\n%s", c.name, got)
		}
		for _, keep := range append([]string{"# Mine", "Text after — kept."}, c.kept...) {
			if !strings.Contains(got, keep) {
				t.Errorf("%s: lost %q:\n%s", c.name, keep, got)
			}
		}
		for _, drop := range c.gone {
			if strings.Contains(got, drop) {
				t.Errorf("%s: %q should be replaced:\n%s", c.name, drop, got)
			}
		}
		if again := Splice(got, sec); again != got {
			t.Errorf("%s: not idempotent:\n%s\n---\n%s", c.name, got, again)
		}
	}
}

func TestSetFrontmatterBOM(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	in := bom + "---\nname: scout\nmodel: haiku\n---\nBody\n"
	want := bom + "---\nname: scout\nmodel: claude-opus-5-5\neffort: high\n---\nBody\n"
	if got := SetFrontmatter(in, Opus, "high"); got != want {
		t.Errorf("BOM file: %q", got)
	}
	if got := SetFrontmatter(strings.ReplaceAll(in, "\n", "\r\n"), Opus, "high"); got != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("BOM and CRLF: %q", got)
	}
	if got := SetFrontmatter(bom+"no frontmatter\n", Opus, "high"); got != bom+"no frontmatter\n" {
		t.Errorf("BOM, no frontmatter: %q", got)
	}
}

func TestSetFrontmatter(t *testing.T) {
	in := "---\nname: scout\ndescription: Finds — things.\ntools: Read, Grep\nmodel: haiku\neffort: medium\ncolor: cyan\n---\n\nBody with model: x\n---\neffort: keep\n"
	out := SetFrontmatter(in, Opus, "high")
	want := "---\nname: scout\ndescription: Finds — things.\ntools: Read, Grep\nmodel: claude-opus-5-5\neffort: high\ncolor: cyan\n---\n\nBody with model: x\n---\neffort: keep\n"
	if out != want {
		t.Errorf("got:\n%s", out)
	}
	if SetFrontmatter(out, Opus, "high") != out {
		t.Error("not idempotent")
	}

	crlf := strings.ReplaceAll(in, "\n", "\r\n")
	if got := SetFrontmatter(crlf, Opus, "high"); got != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("CRLF not kept:\n%q", got)
	}

	missing := "---\nname: x\n---\nBody\n"
	if got := SetFrontmatter(missing, Sonnet, "high"); got != "---\nname: x\nmodel: claude-sonnet-5-5\neffort: high\n---\nBody\n" {
		t.Errorf("missing keys: %q", got)
	}
	for _, plain := range []string{"no frontmatter\n", "---\nnever closed\n", ""} {
		if got := SetFrontmatter(plain, Opus, "high"); got != plain {
			t.Errorf("%q changed to %q", plain, got)
		}
	}
}

func TestSectionsTellToRunTheHandoffAtALimit(t *testing.T) {
	if got := preset(t, "balanced").Section([]string{"claude_pro"}); !strings.Contains(got, HandoffLine+"\n\n| Task |") {
		t.Errorf("the line follows the intro of the preset section:\n%s", got)
	}
	models := Candidates([]string{"claude_pro"})
	setup, _, err := ResolveSetup("balanced", []string{"claude_pro"}, models[:1], []string{"claude_code"})
	if err != nil {
		t.Fatal(err)
	}
	if got := setup.Section(ToolClaude); !strings.Contains(got, HandoffLine+"\n\n| Task |") {
		t.Errorf("the line follows the intro of the confirmed-models section:\n%s", got)
	}
	if n := len(HandoffLine) / 4; n > 30 {
		t.Errorf("the line costs about %d tokens, want at most 30", n)
	}
}
