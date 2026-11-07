package screen

import (
	"io"
	"os"
	"testing"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// machine is what the system check finds in these tests.
func machine(programs ...*scan.Program) *scan.Result {
	return &scan.Result{OS: "windows", Arch: "amd64", CPUs: 12, RAMGB: 16, Tier: "A", Programs: programs}
}

func claude() *scan.Program {
	return &scan.Program{Name: "claude", Fact: "claude_code", Version: "2.1.318"}
}
func codex() *scan.Program { return &scan.Program{Name: "codex", Fact: "codex", Version: "0.166.0"} }

// applied records what the session handed to Apply.
type applied struct {
	calls   int
	save    bool
	profile *profile.Profile
}

func testSession(t *testing.T, programs ...*scan.Program) (*Session, *applied) {
	t.Helper()
	assets := os.DirFS("../..")
	cat, err := catalog.Load(assets)
	if err != nil {
		t.Fatal(err)
	}
	got := &applied{}
	cfg := Config{
		Preflight: func(bool) (*scan.Result, policy.Policy, error) { return machine(programs...), policy.Policy{}, nil },
		Plan: func(*profile.Profile, bool) ([]install.Change, error) {
			return []install.Change{{Action: "create", What: "~/.codex/AGENTS.md"}}, nil
		},
		Apply: func(p *profile.Profile, save bool, out io.Writer) error {
			got.calls++
			got.save, got.profile = save, p
			io.WriteString(out, "Done (tier A).\n")
			return nil
		},
	}
	s := NewSession(func() *wizard.Wizard { return wizard.New(cat, assets, &scan.Result{}) }, cfg)
	return s, got
}

// press sends keys: "\r" is Enter, anything else is itself. It runs the jobs they start.
func press(s *Session, keys string) {
	for _, k := range keys {
		s.Press(Key(k))
		for {
			if _, ok := s.Pending(); !ok {
				break
			}
			s.RunJob(io.Discard)
		}
	}
}
