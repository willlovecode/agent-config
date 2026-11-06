package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
)

func TestCompanyRejectsBeforePlanOrApplyWrites(t *testing.T) {
	o, _ := setup(t, "A", "codex")
	for _, tc := range []struct {
		name, use, source string
	}{
		{"company", "company", ""},
		{"legacy company", "corporate_key", ""},
		{"personal on managed machine", "personal", "test company policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o.Home = t.TempDir()
			o.Policy = policy.Policy{Source: tc.source}
			o.Profile = &profile.Profile{
				Answers: profile.Answers{Use: tc.use, Tools: []string{"codex"}, AllowExternalMCP: true},
				Items:   []string{"instructions-base", "context7"},
			}
			if changes, err := Plan(o); err == nil || len(changes) > 0 || !strings.Contains(err.Error(), "IT approval") {
				t.Errorf("unexpected plan: %v, %v", changes, err)
			}
			if err := Apply(o); err == nil || !strings.Contains(err.Error(), "IT approval") {
				t.Errorf("unexpected apply result: %v", err)
			}
			if entries, err := os.ReadDir(o.Home); err != nil || len(entries) > 0 {
				t.Fatalf("policy denial wrote files: %v, %v", entries, err)
			}
		})
	}
}

func TestCompanyOnlyWritesApprovedModelAndIntegrations(t *testing.T) {
	o, _ := setup(t, "A", "codex")
	model := routing.Model{Plan: "company_gateway", Tool: routing.ToolCodex, ID: "approved-model", Class: "standard", Efforts: []string{"medium"}}
	o.Policy = policy.Policy{Source: "test company policy", Items: []string{"context7"}, Models: []routing.Model{model}}
	o.Profile = &profile.Profile{
		Answers: profile.Answers{Use: "company", Tools: []string{"codex"}, Routing: "balanced", Plans: []string{model.Plan}, Models: []routing.Model{model}},
		Items:   []string{"instructions-base", "context7", "terraform-style-guide"},
	}
	o.Profile.Answers.Models[0].Efforts = []string{"high"}
	if err := Apply(o); err == nil || !strings.Contains(err.Error(), "outside company policy") {
		t.Fatalf("unapproved effort: %v", err)
	}
	o.Profile.Answers.Models[0] = model
	o.Profile.CodexDefaults = "unapproved.toml"
	if _, err := Plan(o); err == nil || !strings.Contains(err.Error(), "profile-supplied") {
		t.Fatalf("unapproved sidecar: %v", err)
	}
	o.Profile.CodexDefaults = ""
	if err := Apply(o); err != nil {
		t.Fatal(err)
	}
	config := read(t, filepath.Join(o.Home, ".codex", "config.toml"))
	for _, want := range []string{`model = "approved-model"`, `model_reasoning_effort = "medium"`, "[mcp_servers.context7]"} {
		if !strings.Contains(config, want) {
			t.Errorf("approved settings should be written: missing %q in %s", want, config)
		}
	}
	guide := filepath.Join(o.Home, ".agents", "skills", "terraform-style-guide", "SKILL.md")
	if !strings.Contains(read(t, guide), "Terraform") {
		t.Fatal("approved local skill was not installed")
	}
}
