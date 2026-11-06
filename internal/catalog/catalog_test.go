package catalog

import (
	"os"
	"testing"
	"testing/fstest"
)

func TestRules(t *testing.T) {
	f := Facts{
		"use": "personal", "stacks": []string{"Java", "python"}, "jdk": "21.0.2",
		"node": "", "tools": []string{"claude_code"}, "ram_gb": 16.0,
	}
	cases := map[string]bool{
		"always":                     true,
		"never":                      false,
		"":                           false,
		"use == personal":            true,
		"use != personal":            false,
		"stacks contains java":       true,
		"stacks contains go":         false,
		"jdk >= 17":                  true,
		"jdk >= 21.1":                false,
		"ram_gb > 8":                 true,
		"node":                       false,
		"missing":                    false,
		"missing != x":               true,
		"not node and stacks":        true,
		"use == corporate or jdk":    true,
		"not (use == personal or x)": false,
		`use == "personal"`:          true,
		"stacks contains python and tools contains claude_code and jdk >= 17": true,
	}
	for src, want := range cases {
		r, err := ParseRule(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if got := r.Eval(f); got != want {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
	for _, bad := range []string{"use ==", "(use", "and x", "x == y z", "jdk >= )", `use == "abc`} {
		if _, err := ParseRule(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestCatalogLoadsAndRecommends(t *testing.T) {
	c, err := Load(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	decide := func(f Facts) map[string]Decision {
		m := map[string]Decision{}
		for _, d := range c.Recommend(f) {
			m[d.Item.ID] = d
		}
		return m
	}
	corp := decide(Facts{"use": "corporate_gateway", "tools": []string{"claude_code", "codex"}, "stacks": []string{"java"}})
	if corp["context7"].Recommended || corp["context7"].Reason == "" {
		t.Errorf("context7 must be off with a reason for corporate: %+v", corp["context7"])
	}
	if !corp["git-guard"].Recommended || !corp["tdd"].Recommended {
		t.Error("git-guard and tdd must be on")
	}
	codexOnly := decide(Facts{"use": "personal", "tools": []string{"codex"}})
	if _, ok := codexOnly["subagents"]; ok {
		t.Error("Claude-only items must be left out for a Codex-only user")
	}
	optIn := decide(Facts{"use": "corporate_key", "tools": []string{"codex"}, "stacks": []string{"go"}, "allow_external_mcp": true})
	if optIn["context7"].Recommended {
		t.Error("a legacy user preference cannot approve external MCP in Company mode")
	}
}

func TestAuthoredSkillsAndGeminiTool(t *testing.T) {
	const skill = "---\nname: mine\ndescription: x\n---\n"
	load := func(yaml string) error {
		_, err := Load(fstest.MapFS{
			"catalog/mine.yaml":               {Data: []byte(yaml)},
			"packs/core/skills/mine/SKILL.md": {Data: []byte(skill)},
		})
		return err
	}
	head := "id: mine\nkind: skill\nsummary: s\nvendored: packs/core/skills/mine\n"
	sha := "0123456789abcdef0123456789abcdef01234567"
	for _, c := range []struct {
		name, yaml string
		ok         bool
	}{
		{"authored without source", head + "tools: [claude_code, gemini]\nauthored: true\n", true},
		{"vendored with source", head + "tools: [codex]\nsource: {repo: a/b, commit: " + sha + "}\n", true},
		{"neither", head + "tools: [codex]\n", false},
		{"authored with source", head + "tools: [codex]\nauthored: true\nsource: {repo: a/b, commit: " + sha + "}\n", false},
		{"unknown tool", head + "tools: [vim]\nauthored: true\n", false},
	} {
		if err := load(c.yaml); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
}
