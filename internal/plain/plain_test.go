package plain

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

func run(t *testing.T, input string) (installed bool, saved bool, items []string, out string) {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{
		{Name: "claude", Fact: "claude_code", Version: "2.1.318"}, {Name: "codex", Fact: "codex", Version: "0.166.0"}}}
	cfg := Config{
		Plan: func(*profile.Profile, bool) ([]install.Change, error) {
			return []install.Change{{Action: "create", What: "~/.codex/AGENTS.md"}}, nil
		},
		Apply: func(p *profile.Profile, save bool, w io.Writer) error {
			saved, items = save, p.Items
			fmt.Fprintln(w, "Done (tier A).")
			return nil
		},
	}
	var b strings.Builder
	installed, err = Run(wizard.New(cat, assets, s), cfg, strings.NewReader(input), &b)
	if err != nil {
		t.Fatal(err)
	}
	return installed, saved, items, b.String()
}

func TestPlainInstall(t *testing.T) {
	// Consent and results, Profile, back from Work, then choose a language.
	input := "\n\n\n<\n\n\n/Python\n1\n" +
		// Frameworks, clients, plans. Search for an item and turn it off.
		"\n\n\n/codex-rules\n?1\n1\n\nn\ny\n"
	installed, saved, items, out := run(t, input)
	if !installed || saved {
		t.Fatalf("installed=%v saved=%v\n%s", installed, saved, out)
	}
	for _, want := range []string{"[1/6] System", "[2/6] Profile", "Which languages and runtimes?", "Choices 1-9 of 13", "1) [x] Personal", "found: 2.1.318", "Use for:", "Claude Code / user", "Skills:", "create  ~/.codex/AGENTS.md", "[6/6] Apply", "Done (tier A)."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	first := "codex-rules"
	if strings.Contains(strings.Join(items, ","), first) {
		t.Errorf("item 1 was switched off but installed: %v", items)
	}
	if !strings.Contains(strings.Join(items, ","), "context7") {
		t.Errorf("a stack was chosen, context7 should be in: %v", items)
	}
}

func TestPlainQuitAndBadInput(t *testing.T) {
	installed, _, _, out := run(t, "\n\n9\n1\nq\n")
	if installed || !strings.Contains(out, "Please type option numbers") || !strings.Contains(out, "Nothing was written") {
		t.Fatalf("installed=%v\n%s", installed, out)
	}
	for _, answer := range []string{"", "\n", "N\n", "q\n", "unexpected\n"} {
		installed, _, _, out := run(t, strings.Repeat("\n", 10)+answer)
		if installed || !strings.Contains(out, "Install now? [y/N]") {
			t.Fatalf("confirmation %q: installed=%v\n%s", answer, installed, out)
		}
	}
}

func TestPlainGeneralAndPopularViews(t *testing.T) {
	input := strings.Repeat("\n", 8) + "popular\n?1\ngeneral\n/i-have-adhd\n1\n\n\ny\n"
	installed, _, items, out := run(t, input)
	if !installed || !strings.Contains(strings.Join(items, ","), "i-have-adhd") {
		t.Fatalf("General skill was not installed: %v\n%s", items, out)
	}
	for _, want := range []string{"| popular |", "| general |", "Popularity:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestPlainModelAnswers(t *testing.T) {
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{{Name: "claude", Fact: "claude_code", Version: "2.1.318"}}}
	w := wizard.New(cat, assets, s)
	var got profile.Answers
	cfg := Config{
		Plan:  func(*profile.Profile, bool) ([]install.Change, error) { return nil, nil },
		Apply: func(p *profile.Profile, _ bool, _ io.Writer) error { got = p.Answers; return nil },
	}
	// Select Claude Max and confirm Sonnet before previewing and choosing Quality.
	input := strings.Repeat("\n", 7) + "/Claude Max\n1\n2\n?4\n4\n\nn\ny\n"
	var b strings.Builder
	if installed, err := Run(w, cfg, strings.NewReader(input), &b); err != nil || !installed {
		t.Fatalf("installed=%v err=%v\n%s", installed, err, b.String())
	}
	if strings.Join(got.Plans, ",") != "claude_max" || got.Routing != "quality" || len(got.Models) != 1 {
		t.Errorf("answers: plans %v routing %q\n%s", got.Plans, got.Routing, b.String())
	}
	if !strings.Contains(b.String(), "[3/6] Models") {
		t.Errorf("no models step:\n%s", b.String())
	}
	for _, want := range []string{"Anthropic", "Confirm available models", "Details / Quality", "143 output tokens/s", "128,000 max output"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q:\n%s", want, b.String())
		}
	}
}

// modelsRun feeds input to a wizard on a machine with Claude Code and returns the answers.
func modelsRun(t *testing.T, input string) (profile.Answers, string) {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{{Name: "claude", Fact: "claude_code", Version: "2.1.318"}}}
	var got profile.Answers
	cfg := Config{
		Plan:  func(*profile.Profile, bool) ([]install.Change, error) { return nil, nil },
		Apply: func(p *profile.Profile, _ bool, _ io.Writer) error { got = p.Answers; return nil },
	}
	var b strings.Builder
	if installed, err := Run(wizard.New(cat, assets, s), cfg, strings.NewReader(input), &b); err != nil || !installed {
		t.Fatalf("installed=%v err=%v\n%s", installed, err, b.String())
	}
	return got, b.String()
}

func TestPlainLeaveModelsAlone(t *testing.T) {
	got, out := modelsRun(t, strings.Repeat("\n", 9)+"n\ny\n")
	if got.Routing != "" || got.Plans != nil {
		t.Errorf("answers %+v", got)
	}
	if strings.Contains(out, "How should tasks be routed?") {
		t.Errorf("routing should be skipped without access:\n%s", out)
	}

	// Going back from models to clear the plan keeps the current settings.
	got, out = modelsRun(t, strings.Repeat("\n", 7)+"/Claude Pro\n1\n<\n/Claude Pro\n1\n\nn\ny\n")
	if got.Routing != "" || got.Plans != nil || !strings.Contains(out, "keep current models") {
		t.Errorf("answers %+v\n%s", got, out)
	}

	// Keeping current models also works after selecting an account and a model.
	got, out = modelsRun(t, strings.Repeat("\n", 7)+"/Claude Pro\n1\n2\n1\n\nn\ny\n")
	if got.Routing != "" || got.Plans != nil || got.Models != nil || !strings.Contains(out, "How should tasks be routed?") {
		t.Errorf("answers %+v\n%s", got, out)
	}
}

func TestPlainCompanyBlocksExternalItems(t *testing.T) {
	installed, _, items, out := run(t, "\n\n2\n\n\n\n\n/context7\n1\n\nn\ny\n")
	if !installed || slices.Contains(items, "context7") || !strings.Contains(out, "Company policy requires IT approval") || strings.Contains(out, "Which AI plans") {
		t.Fatalf("installed=%v items=%v\n%s", installed, items, out)
	}
}

func TestPlainCanRouteCodexWithoutClaudeCode(t *testing.T) {
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	s := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{{Name: "codex", Fact: "codex", Version: "0.166.0"}}}
	w := wizard.New(cat, assets, s)
	var got profile.Answers
	cfg := Config{
		Plan:  func(*profile.Profile, bool) ([]install.Change, error) { return nil, nil },
		Apply: func(p *profile.Profile, _ bool, _ io.Writer) error { got = p.Answers; return nil },
	}
	// Balanced, search plans for ChatGPT Plus, confirm the second Codex model.
	input := strings.Repeat("\n", 7) + "/ChatGPT Plus\n1\n2\n\n\nn\ny\n"
	var b strings.Builder
	if installed, err := Run(w, cfg, strings.NewReader(input), &b); err != nil || !installed {
		t.Fatalf("installed=%v err=%v\n%s", installed, err, b.String())
	}
	if got.Routing != "balanced" || len(got.Models) != 1 || got.Models[0].Tool != "codex" || !strings.Contains(b.String(), "Default model: Codex") {
		t.Errorf("answers %+v\n%s", got, b.String())
	}
}

func TestPlainBackWithCodexOnly(t *testing.T) {
	installed, _, _, out := run(t, strings.Repeat("\n", 6)+"1\n\nb\nq\n")
	if installed || strings.Count(out, "Which AI plans") != 2 || strings.Contains(out, "How should tasks be routed?") {
		t.Fatalf("Back should return to access with routing off:\n%s", out)
	}
}

func TestPlainPlanErrorCanRetry(t *testing.T) {
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	var saved []bool
	cfg := Config{Plan: func(_ *profile.Profile, save bool) ([]install.Change, error) {
		saved = append(saved, save)
		if len(saved) == 1 {
			return nil, errors.New("could not read settings.json")
		}
		return []install.Change{
			{Action: "run", What: "npm install sample"},
			{Action: "create", What: "~/.codex/AGENTS.md"},
		}, nil
	}}
	// Take defaults through Items, turn saving off, retry, then decline to apply.
	input := strings.Repeat("\n", 6) + "2\n\n\nn\n\nN\n"
	var out strings.Builder
	installed, err := Run(wizard.New(cat, assets, &scan.Result{}), cfg, strings.NewReader(input), &out)
	if installed || err != nil || len(saved) != 2 || saved[0] || saved[1] {
		t.Fatalf("installed=%v err=%v save choices=%v\n%s", installed, err, saved, &out)
	}
	got := out.String()
	file, command := strings.Index(got, "create  ~/.codex/AGENTS.md"), strings.Index(got, "npm install sample")
	if !strings.Contains(got, "press Enter to retry") || file < 0 || command < file {
		t.Fatalf("expected the retry prompt and file changes before commands:\n%s", got)
	}
}

func TestSystemCheckWaitsForConsent(t *testing.T) {
	for _, tc := range []struct {
		input    string
		calls    int
		discover bool
	}{
		{"q\n", 0, false},
		{"n\nq\n", 1, false},
		{"y\nq\n", 1, true},
		{"n\nb\ny\nq\n", 2, true},
	} {
		w := wizard.New(nil, nil, &scan.Result{})
		var calls int
		var discovered bool
		cfg := Config{
			Preflight: func(discover bool) (*scan.Result, policy.Policy, error) {
				calls++
				discovered = discover
				return &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{
					{Name: "gemini", Fact: "gemini", Version: "1.0"},
				}}, policy.Policy{}, nil
			}}
		var out strings.Builder
		if _, err := Run(w, cfg, strings.NewReader(tc.input), &out); err != nil {
			t.Fatal(err)
		}
		if calls != tc.calls || discovered != tc.discover {
			t.Errorf("input %q called the check %d times (discover=%v)", tc.input, calls, discovered)
		}
		if tc.discover && (strings.Count(out.String(), "Checking this machine...") != 1 || !strings.Contains(out.String(), "Gemini CLI - on PATH (1.0)")) {
			t.Errorf("expected a static status and Gemini discovery:\n%s", &out)
		}
	}
}

func TestPlainDetailsOnDemand(t *testing.T) {
	installed, _, _, out := run(t, "\n\n?2\nq\n")
	if installed || strings.Contains(out, "Details / Personal") || !strings.Contains(out, "Details / Company") {
		t.Fatalf("only requested details should appear:\n%s", out)
	}
}

func TestConsoleWizardAt60Columns(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	scan := &scan.Result{OS: "windows", Tier: "A", Programs: []*scan.Program{{Name: "codex", Fact: "codex", Version: "1.0"}}}
	// Preview Company, ignore a modified arrow key, and accept Personal. Page to
	// Research, toggle it, then find Python using line input within the key flow.
	input := "y\r?2\x1b[1;2A\rn1p\r/Python\n1\r\r\r\r" +
		// Pick General, find a skill, and go back from Apply to keep Save off.
		"v3/i-have-adhd\n1\rnb\ry"
	in := bufio.NewReader(strings.NewReader(input))
	home := t.TempDir()
	var out strings.Builder
	var got *profile.Profile
	var plans []bool
	s := session{w: wizard.New(cat, assets, scan), in: in, out: &out, terminal: true, save: true,
		key: func() (string, error) { return readKey(in) }, cfg: Config{Home: home, Width: 60,
			Plan: func(_ *profile.Profile, save bool) ([]install.Change, error) {
				plans = append(plans, save)
				return []install.Change{{Action: "create", What: filepath.Join(home, ".codex", "AGENTS.md")}}, nil
			},
			Apply: func(p *profile.Profile, save bool, _ io.Writer) error {
				if save {
					t.Error("Back from confirmation lost the save choice")
				}
				got = p
				return nil
			},
		}}
	installed, err := s.run()
	if err != nil || !installed || !slices.Equal(plans, []bool{false, false}) {
		t.Fatalf("installed=%v err=%v plan save choices=%v\n%s", installed, err, plans, &out)
	}
	if a := got.Answers; a.Use != "personal" || a.Routing != "" ||
		!slices.Equal(a.Work, []string{"greenfield", "research"}) || !slices.Equal(a.Stacks, []string{"python"}) ||
		!slices.Contains(got.Items, "i-have-adhd") {
		t.Fatalf("profile %+v\n%s", got, &out)
	}
	for _, want := range []string{"Details / Company", "Choices 10-13 of 13", "1) [x] Research", "| general |", "(b) back  (q) quit", "create  ~/.codex/AGENTS.md"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q", want)
		}
	}
	for line := range strings.SplitSeq(out.String(), "\n") {
		if len(line) > 60 || strings.ContainsFunc(line, func(r rune) bool { return r < ' ' || r > '~' }) {
			t.Errorf("expected a line of at most 60 ASCII columns: %q", line)
		}
	}
}

func TestConsoleCancelKeys(t *testing.T) {
	for _, key := range []string{"q", "\x03", "\x04", "\x1a", ""} {
		in := bufio.NewReader(strings.NewReader(key))
		var out strings.Builder
		s := session{w: wizard.New(nil, nil, &scan.Result{}), in: in, out: &out,
			key: func() (string, error) { return readKey(in) }}
		if installed, err := s.run(); installed || err != nil || !strings.Contains(out.String(), "Nothing was written") {
			t.Errorf("key %q: installed=%v err=%v\n%s", key, installed, err, &out)
		}
	}
}

func TestPlainOffersGemini(t *testing.T) {
	// Only Claude Code was found; option 3 turns Gemini CLI / Antigravity on.
	got, out := modelsRun(t, strings.Repeat("\n", 6)+"3\n\n\nn\ny\n")
	if strings.Join(got.Tools, ",") != "claude_code,gemini" {
		t.Errorf("tools %v\n%s", got.Tools, out)
	}
	for _, want := range []string{"1) [x] Claude Code", "2) [ ] Codex", "3) [ ] Gemini CLI / Antigravity", "Gemini CLI / Antigravity / user"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
