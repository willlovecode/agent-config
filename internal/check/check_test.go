package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/profile"
)

// loadProfile writes a profile with the given answers and model-settings file to a folder.
func loadProfile(t *testing.T, answers, modelSettings string) *profile.Profile {
	t.Helper()
	dir := t.TempDir()
	yaml := "description: test\nanswers:\n" + answers + "items: []\n"
	if modelSettings != "" {
		yaml += "claude_model_settings: model-settings.json\n"
		if err := os.WriteFile(filepath.Join(dir, "model-settings.json"), []byte(modelSettings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "profile.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := profile.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestModelSettingsMustAgreeWithThePreset(t *testing.T) {
	agree := `{"claude-opus-5-5": {"effortLevel": "medium"}, "claude-sonnet-5-5": {"effortLevel": "high"}}`
	differ := `{"claude-opus-5-5": {"effortLevel": "medium"}, "claude-sonnet-5-5": {"effortLevel": "xhigh"}}`
	routed := "  tools: [claude_code]\n  plans: [claude_pro]\n  routing: balanced\n"
	for _, c := range []struct {
		name, answers, settings string
		wantFail                bool
	}{
		{"agrees", routed, agree, false},
		{"differs", routed, differ, true},
		{"no model-settings file", routed, "", false},
		{"routing none", "  tools: [claude_code]\n  routing: none\n", differ, false},
		{"no routing", "  tools: [claude_code]\n", differ, false},
		{"routing without Claude Code is skipped", "  tools: [codex]\n  plans: [claude_pro]\n  routing: balanced\n", differ, false},
	} {
		var out strings.Builder
		ch := &checker{w: &out}
		ch.modelSettings(loadProfile(t, c.answers, c.settings), "p")
		if c.wantFail != (ch.failures == 1) || ch.failures > 1 {
			t.Errorf("%s: %d failures:\n%s", c.name, ch.failures, out.String())
		}
		if c.wantFail {
			want := "FAIL profile p: model-settings.json sets claude-sonnet-5-5 to xhigh, routing preset balanced says high"
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: want %q in:\n%s", c.name, want, out.String())
			}
		}
	}

	var out strings.Builder
	ch := &checker{w: &out}
	ch.modelSettings(loadProfile(t, routed, "{"), "p")
	if ch.failures != 1 {
		t.Errorf("broken JSON must fail:\n%s", out.String())
	}
}

// The repository's own profiles must pass.
func TestRepoProfilesAgreeWithTheirPresets(t *testing.T) {
	dirs, err := filepath.Glob("../../profiles/*/profile.yaml")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no profiles: %v", err)
	}
	for _, d := range dirs {
		p, err := profile.Load(filepath.Dir(d))
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		ch := &checker{w: &out}
		ch.modelSettings(p, filepath.Base(filepath.Dir(d)))
		if ch.failures != 0 {
			t.Errorf("%s", out.String())
		}
	}
}
