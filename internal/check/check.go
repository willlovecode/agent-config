// Package check holds the repo checks CI runs through `agtk check`: catalog and skill
// validation, agreement with the PowerShell installer's files, and the context budget.
package check

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/profile"
)

// Budget is what every session may load before the task starts (docs/portable-toolkit-design.md).
var Budget = struct{ Instructions, SkillCount, SkillTokens, AgentTokens int }{1200, 15, 1500, 400}

type checker struct {
	w        io.Writer
	failures int
}

func (c *checker) fail(format string, a ...any) {
	c.failures++
	fmt.Fprintf(c.w, "FAIL "+format+"\n", a...)
}

// Run checks the repo at root and returns the number of failures.
func Run(root string, w io.Writer) int {
	c := &checker{w: w}
	fsys := os.DirFS(root)
	cat, err := catalog.Load(fsys)
	if err != nil {
		c.fail("catalog: %v", err)
		return c.failures
	}
	fmt.Fprintf(w, "catalog: %d items\n", len(cat.Items))
	c.skills(fsys, cat)
	c.baseline(fsys, cat)
	dirs, _ := filepath.Glob(filepath.Join(root, "profiles", "*", "profile.yaml"))
	for _, d := range dirs {
		c.profile(fsys, cat, filepath.Dir(d))
	}
	return c.failures
}

func (c *checker) skills(fsys fs.FS, cat *catalog.Catalog) {
	for _, it := range cat.Items {
		if it.Kind != catalog.KindSkill {
			continue
		}
		b, _ := fs.ReadFile(fsys, path.Join(it.Vendored, "SKILL.md"))
		name, desc := catalog.Frontmatter(b)
		if name != it.ID {
			c.fail("%s: frontmatter name is %q", it.ID, name)
		}
		if desc == "" || len(desc) > 1024 {
			c.fail("%s: description must be 1-1024 chars (has %d)", it.ID, len(desc))
		}
	}
}

type baselineSkill struct {
	Name         string   `json:"name"`
	Pack         string   `json:"pack"`
	Scope        string   `json:"scope"`
	Repo         string   `json:"repo"`
	Path         string   `json:"path"`
	Commit       string   `json:"commit"`
	Tools        []string `json:"tools"`
	ExplicitOnly bool     `json:"explicit_only"`
}

func readBaseline(fsys fs.FS) ([]baselineSkill, error) {
	b, err := fs.ReadFile(fsys, "skills/baseline.json")
	if err != nil {
		return nil, err
	}
	var bl struct{ Skills []baselineSkill }
	return bl.Skills, json.Unmarshal(b, &bl)
}

// baseline checks that skills/baseline.json (read by the PowerShell installer) and the
// catalog describe the same skills at the same pins.
func (c *checker) baseline(fsys fs.FS, cat *catalog.Catalog) {
	skills, err := readBaseline(fsys)
	if err != nil {
		c.fail("skills/baseline.json: %v", err)
		return
	}
	seen := map[string]bool{}
	for _, s := range skills {
		seen[s.Name] = true
		it, ok := cat.Get(s.Name)
		if !ok || it.Kind != catalog.KindSkill {
			c.fail("baseline skill %s has no catalog item", s.Name)
			continue
		}
		scope := s.Scope
		if scope == "" {
			scope = "user"
		}
		var tools []string
		for _, t := range s.Tools {
			tools = append(tools, strings.Replace(t, "claude", "claude_code", 1))
		}
		want := fmt.Sprintf("%s %s %s %s %s %v %v", s.Pack, scope, s.Repo, s.Path, s.Commit, s.ExplicitOnly, sorted(tools))
		got := fmt.Sprintf("%s %s %s %s %s %v %v", it.Pack, it.Scope, it.Source.Repo, it.Source.Path, it.Source.Commit, it.ExplicitOnly, sorted(it.Tools))
		if want != got {
			c.fail("%s: baseline.json says %q, catalog says %q", s.Name, want, got)
		}
	}
	for _, it := range cat.Items {
		if it.Kind == catalog.KindSkill && !seen[it.ID] {
			c.fail("catalog skill %s is missing from skills/baseline.json", it.ID)
		}
	}
	fmt.Fprintf(c.w, "baseline.json: %d skills agree with the catalog\n", len(skills))
}

type legacyProfile struct {
	Packs         []string `json:"packs"`
	ProjectSkills []struct {
		Project string   `json:"project"`
		Skills  []string `json:"skills"`
	} `json:"projectSkills"`
	MCP []struct {
		Name string `json:"name"`
	} `json:"mcp"`
	NPMGlobal     []string `json:"npmGlobal"`
	ClaudePlugins []struct {
		ID string `json:"id"`
	} `json:"claudePlugins"`
}

func (c *checker) profile(fsys fs.FS, cat *catalog.Catalog, dir string) {
	name := filepath.Base(dir)
	p, err := profile.Load(dir)
	if err != nil {
		c.fail("profile %s: %v", name, err)
		return
	}
	items, err := p.Resolve(cat)
	if err != nil {
		c.fail("profile %s: %v", name, err)
		return
	}
	c.legacy(cat, p, items, name)
	c.budget(fsys, p, items, name)
}

// legacy compares profile.yaml with the profile.json the PowerShell installer reads.
func (c *checker) legacy(cat *catalog.Catalog, p *profile.Profile, items []*catalog.Item, name string) {
	b, err := p.File("profile.json")
	if err != nil {
		return // no fallback for this profile
	}
	var lp legacyProfile
	if err := json.Unmarshal(b, &lp); err != nil {
		c.fail("profile %s: profile.json: %v", name, err)
		return
	}
	var want, got []string
	for _, it := range cat.Items {
		if (it.Kind == catalog.KindSkill || it.Kind == catalog.KindAgents) && it.Scope == "user" && contains(lp.Packs, it.Pack) {
			want = append(want, it.ID)
		}
	}
	for _, m := range lp.MCP {
		want = append(want, m.Name)
	}
	for _, pl := range lp.ClaudePlugins {
		for _, it := range cat.Items {
			if it.Plugin != nil && it.Plugin.ID == pl.ID {
				want = append(want, it.ID)
			}
		}
	}
	var npm []string
	for _, it := range items {
		switch it.Kind {
		case catalog.KindSkill, catalog.KindAgents, catalog.KindMCP, catalog.KindPlugin:
			got = append(got, it.ID)
		}
		npm = append(npm, it.NPM...)
	}
	if a, b := sorted(want), sorted(got); fmt.Sprint(a) != fmt.Sprint(b) {
		c.fail("profile %s: profile.json means %v, profile.yaml has %v", name, a, b)
	}
	if a, b := sorted(lp.NPMGlobal), sorted(npm); fmt.Sprint(a) != fmt.Sprint(b) {
		c.fail("profile %s: npm packages differ: profile.json %v, catalog %v", name, a, b)
	}
	var wantProj, gotProj []string
	for _, ps := range lp.ProjectSkills {
		for _, s := range ps.Skills {
			wantProj = append(wantProj, ps.Project+":"+s)
		}
	}
	for _, pi := range p.ProjectItems {
		for _, s := range pi.Items {
			gotProj = append(gotProj, pi.Project+":"+s)
		}
	}
	if a, b := sorted(wantProj), sorted(gotProj); fmt.Sprint(a) != fmt.Sprint(b) {
		c.fail("profile %s: project skills differ: profile.json %v, profile.yaml %v", name, a, b)
	}
}

func (c *checker) budget(fsys fs.FS, p *profile.Profile, items []*catalog.Item, name string) {
	skillCount, skillTokens, agentTokens := 0, 0, 0
	var texts []string // joined before counting, as AGENTS.md is
	for _, it := range items {
		cost := catalog.Cost(fsys, it)
		switch it.Kind {
		case catalog.KindInstructions:
			b, _ := fs.ReadFile(fsys, it.Vendored)
			texts = append(texts, string(b))
		case catalog.KindSkill:
			if !it.ExplicitOnly {
				skillCount++
				skillTokens += cost
			}
		case catalog.KindAgents:
			agentTokens += cost
		}
	}
	if p.Instructions != "" {
		b, _ := p.File(p.Instructions)
		texts = append(texts, string(b))
	}
	instructions := catalog.Tokens(strings.Join(texts, "\n\n"))
	fmt.Fprintf(c.w, "\n%s\n", name)
	rows := []struct {
		label    string
		val, max int
	}{
		{"instructions (AGENTS.md)", instructions, Budget.Instructions},
		{"listed skills (count)", skillCount, Budget.SkillCount},
		{"listed skill descriptions", skillTokens, Budget.SkillTokens},
		{"subagent descriptions", agentTokens, Budget.AgentTokens},
	}
	for _, r := range rows {
		mark := "ok  "
		if r.val > r.max {
			mark = "OVER"
			c.failures++
		}
		fmt.Fprintf(c.w, "  %s %-28s %5d / %d\n", mark, r.label, r.val, r.max)
	}
	fmt.Fprintf(c.w, "  total always-loaded ~%d tokens\n", instructions+skillTokens+agentTokens)
}

func sorted(l []string) []string {
	out := append([]string(nil), l...)
	sort.Strings(out)
	return out
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
