package routing

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

// checks/routing.mjs is a port of Section for the Node budget check; it must write the same
// text, or checks/budget.mjs sizes a different CLAUDE.md than agtk writes.
func TestNodePortWritesTheSameSection(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	mjs, err := filepath.Abs(filepath.Join("..", "..", "checks", "routing.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	planSets := [][]string{
		nil,
		{"claude_pro"},
		{"claude_max", "copilot"},
		{"claude_pro", "chatgpt"},
		{"claude_pro", "gemini_plus"},
		{"claude_pro", "gemini_pro", "chatgpt", "copilot"},
		{"claude_pro", "gemini", "gemini_pro"},
	}
	const script = `import { pathToFileURL } from "node:url";
const m = await import(pathToFileURL(process.argv[1]).href);
const [preset, plans] = [process.argv[2], JSON.parse(process.argv[3])];
process.stdout.write(m.section(preset, plans));`
	for _, p := range Presets() {
		for _, plans := range planSets {
			arg, _ := json.Marshal(plans)
			out, err := exec.Command(node, "--input-type=module", "-e", script, mjs, p.ID, string(arg)).Output()
			if err != nil {
				t.Fatalf("%s %v: %v", p.ID, plans, err)
			}
			if want := p.Section(plans); string(out) != want {
				t.Errorf("%s %v: checks/routing.mjs differs from Section\n--- node\n%s\n--- go\n%s", p.ID, plans, out, want)
			}
		}
	}
}
