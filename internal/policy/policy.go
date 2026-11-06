// Package policy gates changes made by agtk. The agent clients enforce their own
// runtime policies, including cloud-managed rules that a local installer cannot see.
package policy

import (
	"bytes"
	"fmt"
	"io"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
)

type Policy struct {
	Items  []string        `yaml:"allow_items"`
	Models []routing.Model `yaml:"allow_models"`
	Source string          `yaml:"-"`
}

func System(cat *catalog.Catalog) (Policy, error) {
	data, source, err := readSystemPolicy()
	if err != nil {
		return Policy{}, err
	}
	if source == "" {
		return Policy{}, nil
	}
	return parse(data, source, cat)
}

func parse(data []byte, source string, cat *catalog.Catalog) (Policy, error) {
	p := Policy{Source: source}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&p); err != nil {
		// A malformed document can contain sensitive values. Report its location only.
		return Policy{}, fmt.Errorf("invalid company policy at %s", source)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return Policy{}, fmt.Errorf("expected one company policy document at %s", source)
	}
	for _, id := range p.Items {
		if _, ok := cat.Get(id); !ok {
			return Policy{}, fmt.Errorf("company policy at %s contains an unknown item", source)
		}
	}
	seen := map[string]bool{}
	for _, m := range p.Models {
		plan, _ := routing.PlanByID(m.Plan)
		if routing.ValidateModel(m) != nil || !plan.Company || seen[m.Key()] {
			return Policy{}, fmt.Errorf("company policy at %s contains an invalid or repeated company model", source)
		}
		seen[m.Key()] = true
	}
	return p, nil
}

func (p Policy) Company(a profile.Answers) bool { return p.Source != "" || a.Company() }

func (p Policy) ItemBlock(a profile.Answers, it *catalog.Item) string {
	if !p.Company(a) || slices.Contains(p.Items, it.ID) {
		return ""
	}
	switch {
	case it.Kind == catalog.KindMCP:
		return "Company policy requires IT approval for MCP servers."
	case it.Kind == catalog.KindPlugin || len(it.NPM) > 0:
		return "Company policy requires IT approval for plugins and package downloads."
	case it.Access == "network":
		return "Company policy requires IT approval for skills that contact external services."
	case it.Access == "command":
		return "Company policy requires IT approval for this command-based item."
	case it.Kind == catalog.KindHook && it.ID != "git-guard":
		return "Company policy requires IT approval for this hook."
	case it.Kind == catalog.KindSkill && it.Access != "local":
		return "Company policy requires IT approval for skills with unknown access."
	}
	return ""
}

func (p Policy) Check(pr *profile.Profile, cat *catalog.Catalog, items []*catalog.Item) error {
	a := pr.Answers
	if a.Use != "" && a.Use != "personal" && !a.Company() {
		return fmt.Errorf("invalid use value; choose personal or company")
	}
	if !p.Company(a) {
		return nil
	}
	if pr.ClaudeModelSettings != "" || pr.CodexDefaults != "" || len(pr.CodexProfiles) > 0 {
		return fmt.Errorf("company setup uses approved routing instead of profile-supplied model and client settings")
	}
	if err := p.CheckRouting(a); err != nil {
		return err
	}
	for _, it := range items {
		if reason := p.ItemBlock(a, it); reason != "" {
			return fmt.Errorf("%s: %s", it.ID, reason)
		}
	}
	for _, project := range pr.ProjectItems {
		for _, id := range project.Items {
			it, ok := cat.Get(id)
			if ok {
				if reason := p.ItemBlock(a, it); reason != "" {
					return fmt.Errorf("%s: %s", id, reason)
				}
			}
		}
	}
	return nil
}

func (p Policy) CheckRouting(a profile.Answers) error {
	if !p.Company(a) || routing.Active(a.Routing) == "" {
		return nil
	}
	if len(a.Models) == 0 {
		return fmt.Errorf("company routing needs models approved by IT; keep current models or choose approved access")
	}
	for _, m := range a.Models {
		allowed := slices.ContainsFunc(p.Models, func(approved routing.Model) bool {
			return approved.Key() == m.Key() && approved.Class == m.Class && slices.Equal(approved.Efforts, m.Efforts)
		})
		if !allowed {
			return fmt.Errorf("a confirmed model or its settings are outside company policy")
		}
	}
	for _, id := range a.Plans {
		if !slices.ContainsFunc(p.Models, func(m routing.Model) bool { return m.Plan == id }) {
			return fmt.Errorf("a selected AI plan is outside company policy")
		}
	}
	return nil
}
