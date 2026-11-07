package screen

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/plain"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

func view(s *Session, w, h int) string {
	s.SetEnv(Env{Width: w, Height: h})
	return s.View()
}

func TestDefaultsEndInTheAnswersOfPlainMode(t *testing.T) {
	s, got := testSession(t, claude(), codex())
	// check, results, Personal, work, languages, frameworks, tools, plans, items, review, save
	press(s, "\r"+strings.Repeat("\r", 10)+"y")
	if !s.Done() {
		t.Fatalf("not done:\n%s", view(s, 80, 24))
	}
	if installed, err := s.Result(); !installed || err != nil {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
	if got.calls != 1 || !got.save {
		t.Fatalf("apply calls=%d save=%v", got.calls, got.save)
	}

	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	var want *profile.Profile
	var wantSave bool
	cfg := plain.Config{
		Preflight: func(bool) (*scan.Result, policy.Policy, error) {
			return machine(claude(), codex()), policy.Policy{}, nil
		},
		Plan:  func(*profile.Profile, bool) ([]install.Change, error) { return nil, nil },
		Apply: func(p *profile.Profile, save bool, _ io.Writer) error { want, wantSave = p, save; return nil },
	}
	var out strings.Builder
	if ok, err := plain.Run(wizard.New(cat, assets, &scan.Result{}), cfg, strings.NewReader(strings.Repeat("\n", 10)+"y\n"), &out); !ok || err != nil {
		t.Fatalf("plain: %v\n%s", err, &out)
	}
	a, b := want.Answers, got.profile.Answers
	if wantSave != got.save || !slices.Equal(want.Items, got.profile.Items) || a.Use != b.Use || a.Routing != b.Routing ||
		!slices.Equal(a.Tools, b.Tools) || !slices.Equal(a.Work, b.Work) || !slices.Equal(a.Stacks, b.Stacks) {
		t.Errorf("screen and plain mode disagree:\nscreen %+v %v\nplain  %+v %v", got.profile, got.save, want, wantSave)
	}
	if !slices.Equal(b.Tools, []string{"claude_code", "codex"}) || len(got.profile.Items) == 0 {
		t.Errorf("unexpected defaults: %+v", got.profile)
	}
}

func TestPagesShowOneStepAtATime(t *testing.T) {
	s, _ := testSession(t, claude(), codex())
	steps := []struct{ keys, want string }{
		{"", "This is the agtk setup wizard"},
		{"y", "System check"},
		{"\r", "Who is this setup for?"},
		{"\r", "What kind of work?"},
		{"\r", "Which languages and runtimes?"},
		{"\r", "Which frameworks and platforms?"},
		{"\r", "Which agent tools will you configure?"},
		{"\r", "Which AI plans or accounts do you have?"},
		{"\r", "Choose what to install."},
		{"\r", "Review"},
		{"\r", "Save your choices as a profile"},
		{"\r", "Ready to apply?"},
	}
	for _, st := range steps {
		press(s, st.keys)
		if v := view(s, 80, 24); !strings.Contains(v, st.want) {
			t.Fatalf("after %q want %q:\n%s", st.keys, st.want, v)
		}
	}
}

func TestBackReturnsOneScreen(t *testing.T) {
	s, _ := testSession(t, claude(), codex())
	press(s, "y\r\r\r\r") // on frameworks
	for _, want := range []string{"Which languages", "What kind of work?", "Who is this setup for?", "System check", "Check this machine first?"} {
		press(s, "b")
		if v := view(s, 80, 24); !strings.Contains(v, want) {
			t.Fatalf("back should show %q:\n%s", want, v)
		}
	}
	// Crossing stages: from the first Models question back to the last Profile one, and on from Items.
	press(s, "y\r\r\r\r\r")
	if v := view(s, 80, 24); !strings.Contains(v, "Which agent tools") {
		t.Fatalf("want tools:\n%s", v)
	}
	press(s, "b")
	if v := view(s, 80, 24); !strings.Contains(v, "Which frameworks") {
		t.Fatalf("want frameworks:\n%s", v)
	}
	press(s, "\r\r")
	if v := view(s, 80, 24); !strings.Contains(v, "Which AI plans") {
		t.Fatalf("want plans:\n%s", v)
	}
	press(s, "\rb")
	if v := view(s, 80, 24); !strings.Contains(v, "Which AI plans") {
		t.Fatalf("back from Items should land on the last Models question:\n%s", v)
	}
	// The first page has nothing to go back to or restart.
	s2, _ := testSession(t)
	if v := view(s2, 80, 24); strings.Contains(v, "(b)") || strings.Contains(v, "Restart") {
		t.Errorf("first page:\n%s", v)
	}
}

func TestRestartStartsOverWithDefaults(t *testing.T) {
	s, got := testSession(t, claude(), codex())
	press(s, "y\r\r") // on work: turn on Debugging
	press(s, "4")
	if !s.w.Has("work", "debugging") {
		t.Fatalf("the test did not change an answer: %v", s.w.Values("work"))
	}
	press(s, "r")
	if v := view(s, 80, 24); !strings.Contains(v, "This is the agtk setup wizard") {
		t.Fatalf("restart goes to the first page:\n%s", v)
	}
	if s.w.Has("work", "debugging") || len(s.w.Values("tools")) != 0 {
		t.Errorf("restart kept answers: work=%v tools=%v", s.w.Values("work"), s.w.Values("tools"))
	}
	press(s, "\r"+strings.Repeat("\r", 10)+"y")
	if got.calls != 1 || !slices.Equal(got.profile.Answers.Work, []string{"greenfield"}) {
		t.Errorf("after restart: %+v", got.profile)
	}
}

func TestQuitWritesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys string
		last Key
	}{
		{"q on the first page", "q", 0},
		{"capital Q", "Q", 0},
		{"q late", "y\r\r\r\r\r\r\r\r\r\rq", 0},
		{"Ctrl-C", "y", KeyCtrlC},
		{"end of input", "y\r", KeyEOF},
	} {
		s, got := testSession(t, claude())
		press(s, tc.keys)
		if tc.last != 0 {
			s.Press(tc.last)
		}
		installed, err := s.Result()
		if !s.Done() || installed || err != nil || got.calls != 0 {
			t.Errorf("%s: done=%v installed=%v err=%v apply calls=%d", tc.name, s.Done(), installed, err, got.calls)
		}
		if v := view(s, 80, 24); !strings.Contains(v, "Left setup. Nothing was written.") {
			t.Errorf("%s: final page:\n%s", tc.name, v)
		}
		press(s, "y\r") // keys after the end do nothing
		if got.calls != 0 {
			t.Errorf("%s: a key after quitting applied", tc.name)
		}
	}
}

func TestBlockedAnswersShowAMessageAndAllowRecovery(t *testing.T) {
	s, _ := testSession(t, claude(), codex())
	press(s, "y\r\r\r\r\r") // on tools: Claude Code and Codex are checked
	press(s, "12\r")
	v := view(s, 80, 24)
	if !strings.Contains(v, "Select at least one agent tool to continue.") || !strings.Contains(v, "Which agent tools") {
		t.Fatalf("an empty tool list must stay on the page and say why:\n%s", v)
	}
	press(s, "3") // the message goes away
	if v := view(s, 80, 24); strings.Contains(v, "Select at least one") || !strings.Contains(v, "[x] Gemini") {
		t.Fatalf("toggling clears the message:\n%s", v)
	}
	press(s, "\r")
	if v := view(s, 80, 24); !strings.Contains(v, "Which AI plans") {
		t.Fatalf("continue after fixing the selection:\n%s", v)
	}

	// Back, restart and quit still work from a blocked page.
	press(s, "b3\r") // tools empty again
	if v := view(s, 80, 24); !strings.Contains(v, "Select at least one") {
		t.Fatalf("want the message again:\n%s", v)
	}
	press(s, "b")
	if v := view(s, 80, 24); !strings.Contains(v, "Which frameworks") {
		t.Fatalf("back from a blocked page:\n%s", v)
	}
	press(s, "\r12\rr")
	if v := view(s, 80, 24); !strings.Contains(v, "This is the agtk setup wizard") {
		t.Fatalf("restart from a blocked page:\n%s", v)
	}
}

func TestManagedMachineKeepsCompany(t *testing.T) {
	s, _ := testSession(t, claude())
	s.cfg.Preflight = func(bool) (*scan.Result, policy.Policy, error) {
		return machine(claude()), policy.Policy{Source: "test policy"}, nil
	}
	press(s, "y\r")
	if v := view(s, 80, 24); !strings.Contains(v, "Company.  (default)") {
		t.Fatalf("a managed machine starts on Company:\n%s", v)
	}
	press(s, "1")
	v := view(s, 80, 24)
	if !strings.Contains(v, "This machine is managed. Select Company to continue.") || !strings.Contains(v, "Who is this setup for?") {
		t.Fatalf("Personal must be refused with a message:\n%s", v)
	}
	press(s, "2")
	if v := view(s, 80, 24); !strings.Contains(v, "What kind of work?") {
		t.Fatalf("Company goes on:\n%s", v)
	}
}

func TestSingleChoiceTakesTheKeyAtOnce(t *testing.T) {
	s, _ := testSession(t, claude())
	press(s, "y\r")
	press(s, "2")
	if s.w.One("use") != "company" {
		t.Fatalf("use = %q", s.w.One("use"))
	}
	if v := view(s, 80, 24); !strings.Contains(v, "What kind of work?") {
		t.Fatalf("a number advances without Enter:\n%s", v)
	}
	press(s, "x!") // keys that mean nothing here are ignored
	if v := view(s, 80, 24); !strings.Contains(v, "What kind of work?") {
		t.Fatalf("unknown keys must not move the page:\n%s", v)
	}
}

func TestMultiSelectTogglesAndPages(t *testing.T) {
	s, _ := testSession(t, claude())
	press(s, "y\r\r\r") // languages
	v := view(s, 60, 20)
	if !strings.Contains(v, "page 1 of") || !strings.Contains(v, "(n)  Next page.") || strings.Contains(v, "(p)") {
		t.Fatalf("first page of several:\n%s", v)
	}
	first := slices.Clone(s.w.Values("stacks"))
	press(s, "1")
	if slices.Equal(first, s.w.Values("stacks")) {
		t.Fatal("a number toggles the row")
	}
	if got := view(s, 60, 20); !strings.Contains(got, "[x]") || !strings.Contains(got, "Which languages") {
		t.Fatalf("toggling redraws the same page:\n%s", got)
	}
	press(s, "1") // toggled back
	if !slices.Equal(first, s.w.Values("stacks")) {
		t.Fatal("a second press toggles back")
	}
	page1 := view(s, 60, 20)
	press(s, "n")
	page2 := view(s, 60, 20)
	if page1 == page2 || !strings.Contains(page2, "(p)  Previous page.") || !strings.Contains(page2, "page 2 of") {
		t.Fatalf("n shows the next page:\n%s", page2)
	}
	// Keys count from 1 on each page.
	press(s, "1")
	if len(s.w.Values("stacks")) != len(first)+1 {
		t.Fatalf("a toggle on page 2: %v", s.w.Values("stacks"))
	}
	press(s, "p")
	if got := view(s, 60, 20); !strings.Contains(got, "page 1 of") {
		t.Fatalf("p goes back:\n%s", got)
	}
	press(s, "p") // past the first page nothing happens
	if got := view(s, 60, 20); !strings.Contains(got, "page 1 of") {
		t.Fatalf("p on the first page stays:\n%s", got)
	}
	press(s, strings.Repeat("n", 10))
	if got := view(s, 60, 20); strings.Contains(got, "(n)") {
		t.Fatalf("n on the last page does nothing:\n%s", got)
	}
	// A bigger terminal still shows nine rows to a page, so every row has one key.
	press(s, "p")
	if big := view(s, 100, 50); !strings.Contains(big, "(9)") || !strings.Contains(big, "page 1 of 2") {
		t.Fatalf("100x50 holds nine rows:\n%s", big)
	}
}

func TestEveryPageFitsTheTerminal(t *testing.T) {
	for _, size := range []Env{{Width: 80, Height: 24}, {Width: 60, Height: 20}, {Width: 120, Height: 40}, {Width: 60, Height: 16}} {
		s, _ := testSession(t, claude(), codex())
		s.SetEnv(size)
		check := func(when string) {
			t.Helper()
			if s.Done() {
				return
			}
			v := s.View()
			lines := strings.Split(v, "\n")
			if len(lines) > size.Height {
				t.Errorf("%dx%d %s: %d lines\n%s", size.Width, size.Height, when, len(lines), v)
			}
			for _, l := range lines {
				if width(l) >= size.Width {
					t.Errorf("%dx%d %s: line of %d columns: %q", size.Width, size.Height, when, width(l), l)
				}
			}
		}
		check("start")
		// Walk every page, paging through each and toggling the first row.
		for _, key := range "y\r\r\r\r\r\r\r\r\r\r" {
			for range 12 {
				s.Press('n')
				check("paging")
			}
			s.Press('1')
			check("toggle")
			s.Press('p')
			check("back a page")
			press(s, string(key))
			check("step " + string(key))
		}
	}
}

func TestDescribeShowsDetailsAndComesBack(t *testing.T) {
	s, _ := testSession(t, claude(), codex())
	press(s, "y\r\r\r\r\r\r") // plans
	press(s, "n")
	before := view(s, 80, 24)
	press(s, "?")
	if v := view(s, 80, 24); !strings.Contains(v, "Press the number of the choice to describe.") {
		t.Fatalf("? asks which:\n%s", v)
	}
	press(s, "1")
	v := view(s, 80, 24)
	if !strings.Contains(v, "(b)  Back to the list.") || strings.Contains(v, "[ ]") {
		t.Fatalf("details page:\n%s", v)
	}
	press(s, "b")
	if after := view(s, 80, 24); after != before {
		t.Fatalf("back from details returns to the same page:\n%s\n--- before:\n%s", after, before)
	}
	press(s, "?x") // anything but a row number cancels
	if after := view(s, 80, 24); after != before {
		t.Fatalf("cancelled ?:\n%s", after)
	}
}

func TestPlansThenModelsThenRouting(t *testing.T) {
	s, got := testSession(t, claude())
	press(s, "y\r\r\r\r\r\r") // plans
	press(s, "2\r")           // Claude Max
	v := view(s, 80, 24)
	if !strings.Contains(v, "Which models can you use") || !strings.Contains(v, "Claude Haiku 5.5") {
		t.Fatalf("models after an account:\n%s", v)
	}
	press(s, "123\r")
	v = view(s, 100, 60)
	for _, want := range []string{"How should tasks be routed?", "Keep current models.", "Saver.", "Balanced.  (default, recommended)", "Quality.", "Find code:", "Hard tasks:"} {
		if !strings.Contains(v, want) {
			t.Errorf("routing page lacks %q:\n%s", want, v)
		}
	}
	press(s, "4") // Quality
	if v := view(s, 80, 24); !strings.Contains(v, "Choose what to install.") {
		t.Fatalf("a preset advances:\n%s", v)
	}
	press(s, "\r")
	if v := view(s, 100, 60); !strings.Contains(v, "Quality routing") || !strings.Contains(v, "Default model: Claude") {
		t.Fatalf("the review states what is written:\n%s", v)
	}
	press(s, "\r\ry")
	a := got.profile.Answers
	if a.Routing != "quality" || len(a.Models) != 3 || !slices.Equal(a.Plans, []string{"claude_max"}) {
		t.Errorf("answers %+v", a)
	}
}

func TestPlansAreOptional(t *testing.T) {
	// SetupBlocked leaves access optional, so the Blocked rule for plans never traps
	// the user here: continuing with none keeps the current models.
	s, got := testSession(t, claude())
	press(s, "y\r\r\r\r\r\r\r")
	if v := view(s, 80, 24); !strings.Contains(v, "Choose what to install.") {
		t.Fatalf("continuing with no plan:\n%s", v)
	}
	press(s, "\r\r\ry")
	if got.profile.Answers.Routing != "" || got.profile.Answers.Plans != nil {
		t.Errorf("no plan, no routing: %+v", got.profile.Answers)
	}
}

func TestItemsToggleViewsAndCost(t *testing.T) {
	s, got := testSession(t, claude(), codex())
	press(s, "y\r\r\r\r\r\r\r") // items
	v := view(s, 100, 40)
	if !strings.Contains(v, "of 3100 context tokens") || !strings.Contains(v, "tok") || !strings.Contains(v, "view: all") {
		t.Fatalf("items page shows the token cost:\n%s", v)
	}
	before := len(s.w.Profile().Items)
	press(s, "1")
	if after := len(s.w.Profile().Items); after == before {
		t.Fatalf("toggle changed nothing (%d items)", before)
	}
	press(s, "v")
	if v := view(s, 100, 40); !strings.Contains(v, "view: recommended") || !strings.Contains(v, "Show general items.") {
		t.Fatalf("v shows the next view:\n%s", v)
	}
	press(s, "vvv")
	if v := view(s, 100, 40); !strings.Contains(v, "view: blocked") {
		t.Fatalf("views cycle:\n%s", v)
	}
	press(s, "vv")
	if v := view(s, 100, 40); !strings.Contains(v, "view: recommended") {
		t.Fatalf("views wrap around:\n%s", v)
	}
	press(s, "\r\r\ry")
	if got.calls != 1 {
		t.Fatalf("apply calls %d", got.calls)
	}
}

func TestPlanErrorCanRetryAndBack(t *testing.T) {
	s, got := testSession(t, claude())
	calls := 0
	s.cfg.Plan = func(_ *profile.Profile, save bool) ([]install.Change, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("could not read settings.json")
		}
		return []install.Change{{Action: "run", What: "npm install sample"}, {Action: "create", What: "~/.codex/AGENTS.md"}}, nil
	}
	press(s, "y"+strings.Repeat("\r", 9)+"n") // save: no
	v := view(s, 80, 24)
	if !strings.Contains(v, "could not read settings.json") || !strings.Contains(v, "(t)  Try again.") || strings.Contains(v, "(y)") {
		t.Fatalf("plan error page:\n%s", v)
	}
	press(s, "y") // nothing to apply yet
	if got.calls != 0 {
		t.Fatal("applied without a plan")
	}
	press(s, "t")
	v = view(s, 80, 24)
	if !strings.Contains(v, "Files (1 new, 0 to edit)") || !strings.Contains(v, "Commands (1 may run)") || !strings.Contains(v, "Save profile for agtk apply: No") {
		t.Fatalf("plan page:\n%s", v)
	}
	if strings.Index(v, "create") > strings.Index(v, "npm install sample") {
		t.Errorf("files come before commands:\n%s", v)
	}
	press(s, "b") // back to the save question, which remembers the answer
	if v := view(s, 80, 24); !strings.Contains(v, "(Enter = n)") {
		t.Fatalf("the save question remembers No:\n%s", v)
	}
	press(s, "\ry")
	if got.calls != 1 || got.save {
		t.Fatalf("apply calls=%d save=%v", got.calls, got.save)
	}
}

func TestApplyOutcome(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".agtk", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := "agtk"
	if os.PathSeparator == '\\' {
		exe += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, exe), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, _ := testSession(t, claude(), codex())
	s.cfg.Home = home
	s.cfg.Getenv = func(k string) string { return map[string]string{"PATH": "/usr/bin", "SHELL": "/bin/bash"}[k] }
	s.cfg.Apply = func(p *profile.Profile, save bool, out io.Writer) error {
		io.WriteString(out, "Done (tier A). Start new sessions in the clients you configured.\nTo do: add the MCP server with `claude mcp add x`\n")
		return nil
	}
	press(s, "y\r\r\r\r\r3\r\r\r\r\ry") // turn Gemini on in Models
	if !s.Done() {
		t.Fatalf("not done:\n%s", view(s, 80, 24))
	}
	v := view(s, 80, 24)
	for _, want := range []string{"Setup complete", "Next steps", "Start new sessions in the clients you configured.",
		"GEMINI.md", "add the MCP server with `claude mcp add x`", "is not on PATH", ".agtk", "step 6 of 6"} {
		if !strings.Contains(v, want) {
			t.Errorf("done page lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Choice [") {
		t.Errorf("the last page has no prompt:\n%s", v)
	}

	// On PATH already: no hint, and no Gemini note without Gemini.
	s2, _ := testSession(t, claude())
	s2.cfg.Home = home
	s2.cfg.Getenv = func(k string) string {
		if k == "PATH" {
			return bin
		}
		return ""
	}
	press(s2, "y"+strings.Repeat("\r", 10)+"y")
	if v := view(s2, 80, 24); strings.Contains(v, "not on PATH") || strings.Contains(v, "GEMINI") {
		t.Errorf("unexpected hint:\n%s", v)
	}

	// A failure is reported, and installed stays false.
	s3, _ := testSession(t, claude())
	s3.cfg.Apply = func(_ *profile.Profile, _ bool, out io.Writer) error {
		io.WriteString(out, "Skills\n")
		return errors.New("disk full")
	}
	press(s3, "y"+strings.Repeat("\r", 10)+"y")
	installed, err := s3.Result()
	if v := view(s3, 80, 24); installed || err == nil || !strings.Contains(v, "Setup failed") || !strings.Contains(v, "disk full") {
		t.Errorf("failure: installed=%v err=%v\n%s", installed, err, v)
	}
}

func TestPreflightErrorStopsCleanly(t *testing.T) {
	s, got := testSession(t)
	s.cfg.Preflight = func(bool) (*scan.Result, policy.Policy, error) {
		return nil, policy.Policy{}, errors.New("cannot read policy")
	}
	press(s, "y")
	installed, err := s.Result()
	if v := view(s, 80, 24); !s.Done() || installed || err == nil || got.calls != 0 || !strings.Contains(v, "cannot read policy") {
		t.Fatalf("done=%v installed=%v err=%v\n%s", s.Done(), installed, err, v)
	}
}

func TestSkippingTheCheckGoesStraightOn(t *testing.T) {
	s, _ := testSession(t, claude())
	press(s, "n")
	if v := view(s, 80, 24); !strings.Contains(v, "Who is this setup for?") {
		t.Fatalf("n skips the system check results:\n%s", v)
	}
	press(s, "b")
	if v := view(s, 80, 24); !strings.Contains(v, "Check this machine first?") {
		t.Fatalf("back from the first question asks the check again:\n%s", v)
	}
}

func TestSmallTerminalOffersPlainMode(t *testing.T) {
	for _, size := range []Env{{Width: 59, Height: 24}, {Width: 80, Height: 15}, {Width: 40, Height: 10}} {
		s, _ := testSession(t, claude())
		v := view(s, size.Width, size.Height)
		if !strings.Contains(v, "This terminal is too small") || !strings.Contains(v, "(w)  Use the plain wizard.") {
			t.Fatalf("%dx%d:\n%s", size.Width, size.Height, v)
		}
		press(s, "y1\r") // nothing else works while it is too small
		if s.Done() || s.checked {
			t.Fatalf("%dx%d: acted while too small", size.Width, size.Height)
		}
		s.SetEnv(Env{Width: 80, Height: 24}) // resized: the next page is the wizard again
		if v := s.View(); !strings.Contains(v, "This is the agtk setup wizard") {
			t.Fatalf("after resizing:\n%s", v)
		}
		s.SetEnv(size)
		press(s, "w")
		if _, err := s.Result(); !errors.Is(err, ErrPlain) || !s.Done() {
			t.Errorf("w should ask for the plain wizard: %v", err)
		}
	}
	s, _ := testSession(t, claude())
	view(s, 50, 10)
	press(s, "q")
	if installed, err := s.Result(); installed || err != nil || !s.Done() {
		t.Errorf("q while too small: %v %v", installed, err)
	}
	if got := view(s, 80, 24); !strings.Contains(got, "Nothing was written") {
		t.Errorf("q final page:\n%s", got)
	}
}

func TestUsableAndColor(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if Usable(nil, nil, env(nil)) {
		t.Error("nil files are not terminals")
	}
	if Usable(os.Stdin, os.Stdout, env(map[string]string{"TERM": "dumb"})) {
		t.Error("a dumb terminal cannot be cleared")
	}
	f, err := os.CreateTemp(t.TempDir(), "f")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if Usable(f, f, env(nil)) {
		t.Error("a file is not a terminal")
	}
	if !Color(env(nil)) || !Color(env(map[string]string{"TERM": "xterm-256color"})) {
		t.Error("colour by default")
	}
	if Color(env(map[string]string{"NO_COLOR": "1"})) || Color(env(map[string]string{"TERM": "dumb"})) {
		t.Error("NO_COLOR and dumb terminals get no escape sequences")
	}
}
