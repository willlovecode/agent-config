package routing

import "testing"

func TestConfirmedModelsDetermineRoutes(t *testing.T) {
	models := []Model{
		{Plan: "chatgpt_plus", Tool: ToolCodex, ID: "small", Class: "fast"},
		{Plan: "chatgpt_plus", Tool: ToolCodex, ID: "daily", Class: "standard", Efforts: []string{"high", "medium"}},
		{Plan: "codex_api", Tool: ToolCodex, ID: "deep", Class: "deep", Efforts: []string{"medium", "xhigh"}},
	}
	for _, tc := range []struct{ preset, routine, review, effort string }{
		{"saver", "daily", "daily", "medium"},
		{"balanced", "daily", "deep", "xhigh"},
		{"quality", "deep", "deep", "xhigh"},
	} {
		t.Run(tc.preset, func(t *testing.T) {
			setup, _, err := ResolveSetup(tc.preset, []string{"chatgpt_plus", "codex_api"}, models, []string{"codex"})
			if err != nil {
				t.Fatal(err)
			}
			if def := setup.Default(ToolCodex); def.Model != tc.routine || setup.Default(ToolClaude).Model != "" {
				t.Errorf("defaults: %+v", def)
			}
			for _, row := range setup.Rows() {
				switch row.Task.ID {
				case "review":
					if row.Use.Model != tc.review || row.Use.Effort != tc.effort || len(row.Fallbacks) != 1 {
						t.Errorf("review: %+v", row)
					}
				case "explore", "mechanical":
					if row.Use.Model != "small" {
						t.Errorf("quick task: %+v", row)
					}
				}
			}
		})
	}

	only := models[1:2]
	setup, _, err := ResolveSetup("quality", []string{"chatgpt_plus", "codex_api"}, only, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range setup.Rows() {
		if row.Use.Model != "daily" || len(row.Fallbacks) != 0 {
			t.Errorf("unconfirmed choices must not appear: %+v", row)
		}
	}
}

func TestConfirmedDesktopAccessDoesNotSetCLIDefault(t *testing.T) {
	for _, plan := range []string{"chatgpt_free", "chatgpt_go"} {
		models := Candidates([]string{plan})
		if len(models) != 1 {
			t.Fatalf("%s candidates: %+v", plan, models)
		}
		setup, _, err := ResolveSetup("balanced", []string{plan}, models, []string{"codex"})
		if err != nil || setup.Default(ToolCodex).Model != "" || setup.Rows()[0].Use.Plan != plan {
			t.Errorf("%s access should be advisory: %+v, %v", plan, setup, err)
		}
	}
}
