package install

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func geminiMD(home string) string { return filepath.Join(home, ".gemini", "GEMINI.md") }

func TestGeminiMDOnlyWhenChosen(t *testing.T) {
	for _, tools := range []string{"claude_code, codex", ""} {
		o, _ := setup(t, "A", tools)
		if err := Apply(o); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(o.Home, ".gemini")); !os.IsNotExist(err) {
			t.Errorf("tools [%s]: ~/.gemini must not exist unless gemini is chosen (%v)", tools, err)
		}
	}

	o, _ := setup(t, "A", "claude_code, codex, gemini")
	o.Profile.Answers.Routing, o.Profile.Answers.Plans = "balanced", allPlans
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	got := read(t, geminiMD(o.Home))
	if !strings.HasPrefix(got, generatedNotice+"\n\n# Working agreement") {
		t.Errorf("GEMINI.md must start with the generated notice and the base text:\n%.200s", got)
	}
	if strings.HasPrefix(got, utf8BOM) || strings.Contains(got, "\r") {
		t.Error("GEMINI.md must be UTF-8 without BOM and with LF line endings")
	}
	if strings.Contains(got, "Model routing") || strings.Contains(got, "agtk:model-routing") || strings.Contains(got, "| Task | Use | Fallback |") {
		t.Errorf("GEMINI.md must not carry the routing table:\n%s", got)
	}
	// Same text as AGENTS.md up to the handoff paragraph.
	if agents := read(t, filepath.Join(o.Home, ".codex", "AGENTS.md")); got != agents+"\n"+geminiHandoff {
		t.Errorf("GEMINI.md differs from AGENTS.md + handoff:\n%s", got)
	}
	if !strings.Contains(got, "Read .agent-handoff/latest.md, then run git status and git diff, then continue with Next step.") {
		t.Error("GEMINI.md lacks the handoff one-liner")
	}
	if entries, _ := os.ReadDir(filepath.Join(o.Home, ".gemini")); len(entries) != 1 {
		t.Errorf("nothing but GEMINI.md may be written under ~/.gemini: %v", entries)
	}
}

func TestGeminiOnlyWritesNoOtherToolFiles(t *testing.T) {
	o, _ := setup(t, "A", "gemini")
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(geminiMD(o.Home)); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".claude/CLAUDE.md", ".codex/AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(o.Home, p)); !os.IsNotExist(err) {
			t.Errorf("%s must not be written for a gemini-only profile (%v)", p, err)
		}
	}
}

func TestGeminiMDIsIdempotentBackedUpAndUndoable(t *testing.T) {
	o, _ := setup(t, "A", "gemini")
	p := geminiMD(o.Home)
	mine := "# mine\r\n\xef\xbb\xbfodd bytes\r\n"
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(mine), 0o644)
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	first := read(t, p)
	if first == mine {
		t.Fatal("GEMINI.md was not replaced")
	}

	var out strings.Builder
	o.Out = &out
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != first || !strings.Contains(out.String(), "Nothing changed; no backup kept.") {
		t.Errorf("a second apply must change nothing:\n%s", out.String())
	}

	if err := Undo(StateDir(o.Home), io.Discard); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != mine {
		t.Error("undo must restore the previous GEMINI.md byte for byte")
	}
}

func TestHandoffSkillInstalled(t *testing.T) {
	o, _ := setup(t, "A", "claude_code, codex")
	o.Profile.Items = append(o.Profile.Items, "handoff")
	var out strings.Builder
	o.Out = &out
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".agents/skills/handoff/SKILL.md", ".claude/skills/handoff/SKILL.md"} {
		if b := read(t, filepath.Join(o.Home, p)); !strings.Contains(b, "name: handoff") || strings.Contains(b, "disable-model-invocation") {
			t.Errorf("%s: not an implicit-invocation handoff skill:\n%s", p, b)
		}
	}
	if pin := read(t, filepath.Join(o.Home, ".agents/skills/handoff/.agent-config-pin")); !strings.HasPrefix(pin, "agtk explicit_only=False") {
		t.Errorf("pin %q", pin)
	}
	if !strings.Contains(out.String(), "install  handoff  (authored)") {
		t.Errorf("apply output:\n%s", out.String())
	}
}

// checks/handoff.mjs sizes GEMINI.md for the Node budget check; it must hold the same text.
func TestNodePortWritesTheSameHandoff(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	mjs, err := filepath.Abs(filepath.Join("..", "..", "checks", "handoff.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	const script = `import { pathToFileURL } from "node:url";
const m = await import(pathToFileURL(process.argv[1]).href);
process.stdout.write(m.GEMINI_HANDOFF);`
	out, err := exec.Command(node, "--input-type=module", "-e", script, mjs).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != geminiHandoff {
		t.Errorf("checks/handoff.mjs differs from geminiHandoff\n--- node\n%s\n--- go\n%s", out, geminiHandoff)
	}
}
