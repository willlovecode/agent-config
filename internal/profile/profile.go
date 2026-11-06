// Package profile loads a saved profile: the user's answers, the chosen catalog items and
// any personal files (instructions, model settings) kept next to it.
package profile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/routing"
)

type Answers struct {
	Use              string          `yaml:"use"` // personal, company (also accepts the old corporate values)
	Tools            []string        `yaml:"tools"`
	Stacks           []string        `yaml:"stacks,omitempty"`
	Work             []string        `yaml:"work,omitempty"`
	Autonomy         string          `yaml:"autonomy,omitempty"`
	Budget           string          `yaml:"budget,omitempty"`
	AllowExternalMCP bool            `yaml:"allow_external_mcp,omitempty"`
	Plans            []string        `yaml:"plans,omitempty"`   // selected accounts (legacy: gemini = gemini_pro)
	Models           []routing.Model `yaml:"models,omitempty"`  // explicitly confirmed client models
	Routing          string          `yaml:"routing,omitempty"` // saver, balanced, quality; empty or none leaves models alone
}

func (a Answers) Company() bool {
	switch a.Use {
	case "company", "corporate_gateway", "corporate_key":
		return true
	}
	return false
}

func (a Answers) ResolveRouting() (*routing.Setup, bool, error) {
	return routing.ResolveSetup(a.Routing, a.Plans, a.Models, a.Tools)
}

type ProjectItems struct {
	Project string   `yaml:"project"`
	Items   []string `yaml:"items"`
}

type Profile struct {
	Description         string         `yaml:"description"`
	Answers             Answers        `yaml:"answers"`
	Items               []string       `yaml:"items"`
	ProjectItems        []ProjectItems `yaml:"project_items,omitempty"`
	Instructions        string         `yaml:"instructions,omitempty"`          // personal AGENTS.md part, appended to the base
	ClaudeModelSettings string         `yaml:"claude_model_settings,omitempty"` // JSON merged into settings.json modelSettings
	CodexDefaults       string         `yaml:"codex_defaults,omitempty"`        // TOML whose model keys go into config.toml
	CodexProfiles       []string       `yaml:"codex_profiles,omitempty"`        // TOML files copied into ~/.codex

	Dir  string `yaml:"-"` // folder holding the profile; side files resolve against it
	Path string `yaml:"-"`
}

// Load reads profile.yaml from a folder, or the given YAML file.
func Load(p string) (*Profile, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		abs = filepath.Join(abs, "profile.yaml")
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	pr := &Profile{}
	if err := dec.Decode(pr); err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}
	pr.Path, pr.Dir = abs, filepath.Dir(abs)
	return pr, nil
}

// File reads a side file named in the profile.
func (p *Profile) File(name string) ([]byte, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(p.Dir, name)
	}
	return os.ReadFile(name)
}

// At returns a copy saved at path, keeping any side files anchored to their original location.
func (p Profile) At(path string) *Profile {
	dir := filepath.Dir(path)
	if p.Dir != "" && p.Dir != dir {
		rebase := func(name string) string {
			if name == "" || filepath.IsAbs(name) {
				return name
			}
			absolute := filepath.Join(p.Dir, name)
			if relative, err := filepath.Rel(dir, absolute); err == nil {
				return relative
			}
			return absolute
		}
		p.Instructions = rebase(p.Instructions)
		p.ClaudeModelSettings = rebase(p.ClaudeModelSettings)
		p.CodexDefaults = rebase(p.CodexDefaults)
		p.CodexProfiles = append([]string(nil), p.CodexProfiles...)
		for i, name := range p.CodexProfiles {
			p.CodexProfiles[i] = rebase(name)
		}
	}
	p.Dir, p.Path = dir, path
	return &p
}

// Resolve checks that every chosen item exists in the catalog and fits its scope.
func (p *Profile) Resolve(c *catalog.Catalog) (user []*catalog.Item, err error) {
	for _, id := range p.Items {
		it, ok := c.Get(id)
		if !ok {
			return nil, fmt.Errorf("profile item %q is not in the catalog", id)
		}
		if it.Scope == "project" {
			return nil, fmt.Errorf("profile item %q is project-scoped; list it under project_items", id)
		}
		user = append(user, it)
	}
	for _, pi := range p.ProjectItems {
		for _, id := range pi.Items {
			it, ok := c.Get(id)
			if !ok || it.Scope != "project" {
				return nil, fmt.Errorf("project item %q is not a project-scoped catalog item", id)
			}
		}
	}
	return user, nil
}

// Facts adds the answers to scan facts for the recommendation rules.
func (a Answers) Facts(f catalog.Facts) catalog.Facts {
	out := catalog.Facts{}
	for k, v := range f {
		out[k] = v
	}
	out["use"] = a.Use
	out["tools"] = a.Tools
	out["stacks"] = a.Stacks
	out["work"] = a.Work
	out["autonomy"] = a.Autonomy
	out["budget"] = a.Budget
	out["allow_external_mcp"] = a.AllowExternalMCP
	out["plans"] = a.Plans
	out["routing"] = a.Routing
	return out
}
