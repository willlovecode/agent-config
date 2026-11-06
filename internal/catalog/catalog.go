// Package catalog loads the vetted items in catalog/*.yaml and decides which of them to
// recommend for a machine and a set of answers.
package catalog

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Kinds an item can have; apply knows how to install each.
const (
	KindInstructions = "instructions" // text assembled into AGENTS.md
	KindHook         = "hook"         // a hook program run as `agtk hook <id>`
	KindRules        = "rules"        // Codex execpolicy rules
	KindAgents       = "agents"       // Claude Code subagent definitions
	KindSkill        = "skill"
	KindMCP          = "mcp"
	KindPlugin       = "plugin" // Claude Code plugin, optionally with npm packages it needs
)

var kinds = map[string]bool{KindInstructions: true, KindHook: true, KindRules: true, KindAgents: true,
	KindSkill: true, KindMCP: true, KindPlugin: true}

type Item struct {
	ID         string `yaml:"id"`
	Kind       string `yaml:"kind"`
	Summary    string `yaml:"summary"` // one line for the wizard list
	Task       string `yaml:"task"`
	Purpose    string `yaml:"purpose"`
	Website    string `yaml:"website"`
	Popularity *struct {
		Label   string `yaml:"label"`
		URL     string `yaml:"url"`
		Checked string `yaml:"checked"`
	} `yaml:"popularity"`
	Access string   `yaml:"access"` // local, command, or network
	Needs  string   `yaml:"needs"`
	Tools  []string `yaml:"tools"` // claude_code, codex, gemini
	OS     []string `yaml:"os"`    // windows, linux
	Scope  string   `yaml:"scope"` // user (default) or project
	Pack   string   `yaml:"pack"`  // the pack the PowerShell installer knows it by
	Source struct {
		Repo    string `yaml:"repo"`
		Path    string `yaml:"path"`
		Commit  string `yaml:"commit"`
		License string `yaml:"license"`
	} `yaml:"source"`
	Vendored      string `yaml:"vendored"` // path in the repo (and the binary)
	Authored      bool   `yaml:"authored"` // a skill written in this repo: no upstream source, not in baseline.json
	Audit         string `yaml:"audit"`
	ExplicitOnly  bool   `yaml:"explicit_only"`
	Modified      bool   `yaml:"modified"`
	RecommendWhen *Rule  `yaml:"recommend_when"`
	NeverWhen     *Rule  `yaml:"never_when"`
	Why           struct {
		Recommended    string `yaml:"recommended"`
		NotRecommended string `yaml:"not_recommended"`
		Never          string `yaml:"never"`
	} `yaml:"why"`
	MCP *struct {
		URL               string   `yaml:"url"`
		CodexEnabledTools []string `yaml:"codex_enabled_tools"`
	} `yaml:"mcp"`
	Plugin *struct {
		ID          string `yaml:"id"`
		Marketplace string `yaml:"marketplace"`
	} `yaml:"plugin"`
	NPM []string `yaml:"npm"` // global npm packages, name@version
}

func (it *Item) Supports(tool string) bool { return contains(it.Tools, tool) }

func isHTTPS(link string) bool {
	u, err := url.Parse(link)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

func (it *Item) SourceURL() string {
	if it.Source.Repo == "" {
		return ""
	}
	link := "https://github.com/" + it.Source.Repo + "/tree/" + it.Source.Commit
	if it.Source.Path != "" {
		link += "/" + it.Source.Path
	}
	return link
}

type Catalog struct {
	Items []*Item
	byID  map[string]*Item
}

func (c *Catalog) Get(id string) (*Item, bool) { it, ok := c.byID[id]; return it, ok }

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Load reads catalog/*.yaml from fsys and checks each item against the vendored files.
func Load(fsys fs.FS) (*Catalog, error) {
	files, err := fs.Glob(fsys, "catalog/*.yaml")
	if err != nil {
		return nil, err
	}
	c := &Catalog{byID: map[string]*Item{}}
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		it := &Item{}
		if err := dec.Decode(it); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if err := validate(fsys, it, f); err != nil {
			return nil, err
		}
		if c.byID[it.ID] != nil {
			return nil, fmt.Errorf("%s: duplicate id %s", f, it.ID)
		}
		c.byID[it.ID] = it
		c.Items = append(c.Items, it)
	}
	sort.Slice(c.Items, func(i, j int) bool { return c.Items[i].ID < c.Items[j].ID })
	return c, nil
}

func validate(fsys fs.FS, it *Item, file string) error {
	bad := func(format string, a ...any) error { return fmt.Errorf("%s: "+format, append([]any{file}, a...)...) }
	if it.ID == "" || it.ID+".yaml" != path.Base(file) {
		return bad("id %q must match the file name", it.ID)
	}
	if !kinds[it.Kind] {
		return bad("unknown kind %q", it.Kind)
	}
	if it.Summary == "" || len(it.Tools) == 0 {
		return bad("summary and tools are required")
	}
	for _, t := range it.Tools {
		if t != "claude_code" && t != "codex" && t != "gemini" {
			return bad("unknown tool %q", t)
		}
	}
	if it.Scope == "" {
		it.Scope = "user"
	}
	if it.Source.Repo != "" && !shaRe.MatchString(it.Source.Commit) {
		return bad("source.commit must be a full commit SHA")
	}
	if it.Website != "" && !isHTTPS(it.Website) {
		return bad("website must be a public HTTPS URL")
	}
	if p := it.Popularity; p != nil {
		if it.Kind != KindSkill || p.Label == "" || !isHTTPS(p.URL) {
			return bad("popularity requires a skill, a label and a public HTTPS URL")
		}
		if _, err := time.Parse(time.DateOnly, p.Checked); err != nil {
			return bad("popularity.checked must be a date in YYYY-MM-DD format")
		}
	}
	switch it.Access {
	case "", "local", "command", "network":
	default:
		return bad("unknown access %q", it.Access)
	}
	if it.Vendored != "" {
		if _, err := fs.Stat(fsys, it.Vendored); err != nil {
			return bad("vendored path %s missing", it.Vendored)
		}
	}
	switch it.Kind {
	case KindSkill:
		if _, err := fs.Stat(fsys, path.Join(it.Vendored, "SKILL.md")); err != nil {
			return bad("no SKILL.md in %s", it.Vendored)
		}
		if it.Authored && it.Source.Repo != "" {
			return bad("an authored skill has no source")
		}
		if !it.Authored && it.Source.Repo == "" {
			return bad("a skill needs source.repo and a pinned source.commit, or authored: true")
		}
	case KindInstructions, KindAgents, KindRules:
		if it.Vendored == "" {
			return bad("vendored is required for %s", it.Kind)
		}
	case KindMCP:
		if it.MCP == nil || it.MCP.URL == "" {
			return bad("mcp.url is required")
		}
	case KindPlugin:
		if it.Plugin == nil || !strings.Contains(it.Plugin.ID, "@") {
			return bad("plugin.id must be name@marketplace")
		}
	}
	for _, pkg := range it.NPM {
		if strings.LastIndex(pkg, "@") <= 0 {
			return bad("npm package %q must be pinned as name@version", pkg)
		}
	}
	return nil
}

// Decision is the recommendation for one item.
type Decision struct {
	Item        *Item
	Recommended bool
	Reason      string
}

// Recommend decides every item for the given facts. Items for tools the user does not
// use, or for another OS, are left out. No tools chosen means both, as in apply.
func (c *Catalog) Recommend(f Facts) []Decision {
	tools, _ := f["tools"].([]string)
	if len(tools) == 0 {
		tools = []string{"claude_code", "codex"}
	}
	osName, _ := f["os"].(string)
	var out []Decision
	for _, it := range c.Items {
		if !overlaps(it.Tools, tools) || (len(it.OS) > 0 && !contains(it.OS, osName)) {
			continue
		}
		d := Decision{Item: it}
		switch {
		case it.NeverWhen != nil && it.NeverWhen.Eval(f):
			d.Reason = it.Why.Never
		case it.RecommendWhen != nil && it.RecommendWhen.Eval(f):
			d.Recommended, d.Reason = true, it.Why.Recommended
		default:
			d.Reason = it.Why.NotRecommended
		}
		out = append(out, d)
	}
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

func overlaps(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}
