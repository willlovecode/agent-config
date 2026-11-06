package wizard

import (
	"slices"
	"sort"
	"strings"

	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/routing"
)

var questions = []*Question{
	{ID: "use", Stage: StageProfile, Prompt: "Who is this setup for?", Options: []Option{
		{Value: "personal", Label: "Personal", Detail: "Configure your own tools and accounts."},
		{Value: "company", Label: "Company", Detail: "Apply company restrictions to items and AI access."},
	}, def: fixed("personal")},
	{ID: "work", Stage: StageWork, Prompt: "What kind of work?", Multi: true, Options: []Option{
		{Value: "greenfield", Label: "Features", Group: "Build and maintain", Detail: "Build features, services and APIs."},
		{Value: "testing", Label: "Testing", Group: "Quality", Detail: "Write tests, improve coverage and catch regressions."},
		{Value: "debugging", Label: "Debugging", Group: "Quality", Detail: "Reproduce failures and investigate performance."},
		{Value: "legacy_migration", Label: "Migration", Group: "Build and maintain", Detail: "Modernize, upgrade or replace existing code."},
		{Value: "review", Label: "Code review", Group: "Quality", Detail: "Review diffs for correctness, risks and missing tests."},
		{Value: "security", Label: "Security", Group: "Quality", Detail: "Audit dependencies, permissions and sensitive data flows."},
		{Value: "ui", Label: "UI design", Group: "Design and writing", Detail: "Build and review interfaces and interactions."},
		{Value: "accessibility", Label: "Accessibility", Group: "Design and writing", Detail: "Check semantics, contrast and keyboard access."},
		{Value: "docs", Label: "Documentation", Group: "Design and writing", Detail: "Write or revise technical guides and references."},
		{Value: "research", Label: "Research", Group: "Data and research", Detail: "Find evidence and compare sources."},
		{Value: "data", Label: "Data analysis", Group: "Data and research", Detail: "Explore datasets, queries, schemas and migrations."},
		{Value: "ml", Label: "Machine learning", Group: "Data and research", Detail: "Build models, run experiments and work in notebooks."},
		{Value: "ops", Label: "Infrastructure", Group: "Operations", Detail: "Maintain infrastructure, automation and CI."},
	}, def: fixed("greenfield")},
	{ID: "stacks", Stage: StageWork, Prompt: "Which languages and runtimes?", Multi: true, Options: []Option{
		{Value: "python", Label: "Python", Group: "Application", Detail: "Services, scripts, testing and data workflows."},
		{Value: "javascript", Label: "JavaScript / TypeScript", Group: "Web", Detail: "Browser applications, Node.js services and web tooling."},
		{Value: "java", Label: "Java", Group: "Application", Detail: "JVM applications and services, including Spring."},
		{Value: "kotlin", Label: "Kotlin", Group: "Application", Detail: "JVM applications, Android and multiplatform projects."},
		{Value: "dotnet", Label: "C# / .NET", Group: "Application", Detail: ".NET applications, APIs and tests."},
		{Value: "go", Label: "Go", Group: "Systems", Detail: "Network services, command-line tools and infrastructure."},
		{Value: "rust", Label: "Rust", Group: "Systems", Detail: "Native tools, services and systems programming."},
		{Value: "cpp", Label: "C / C++", Group: "Systems", Detail: "Native libraries, systems and embedded software."},
		{Value: "sql", Label: "SQL", Group: "Data", Detail: "Queries, schemas, reporting and data migrations."},
		{Value: "bash", Label: "Bash", Group: "Shell", Detail: "Shell automation on Linux, macOS and WSL."},
		{Value: "powershell", Label: "PowerShell", Group: "Shell", Detail: "Windows administration and cross-platform scripting."},
		{Value: "ruby", Label: "Ruby", Group: "Web", Detail: "Rails applications, scripts and tooling."},
		{Value: "php", Label: "PHP", Group: "Web", Detail: "Server-side sites, services and applications."},
		{Value: "swift", Label: "Swift", Group: "Mobile", Detail: "Apple platform apps and Swift packages."},
		{Value: "dart", Label: "Dart", Group: "Mobile", Detail: "Flutter applications and Dart packages."},
		{Value: "r", Label: "R", Group: "Data", Detail: "Statistical analysis, visualization and reporting."},
	}, def: func(w *Wizard) []string {
		var out []string
		for fact, stack := range map[string]string{"python": "python", "jdk": "java", "go": "go", "node": "javascript"} {
			if w.Scan.Find(fact) != nil {
				out = append(out, stack)
			}
		}
		sort.Strings(out)
		return out
	}},
	{ID: "frameworks", Stage: StageWork, Prompt: "Which frameworks and platforms?", Multi: true, Options: []Option{
		{Value: "react", Label: "React", Group: "Frontend", Detail: "Component-based web interfaces."},
		{Value: "nextjs", Label: "Next.js", Group: "Frontend", Detail: "React apps with server rendering and routing."},
		{Value: "vue", Label: "Vue / Nuxt", Group: "Frontend", Detail: "Reactive web interfaces and full-stack Vue apps."},
		{Value: "angular", Label: "Angular", Group: "Frontend", Detail: "Structured TypeScript web apps."},
		{Value: "svelte", Label: "Svelte / SvelteKit", Group: "Frontend", Detail: "Compiled UI components and web apps."},
		{Value: "react_native", Label: "React Native", Group: "Mobile", Detail: "Cross-platform mobile apps with React."},
		{Value: "nodejs", Label: "Node.js", Group: "Backend", Detail: "JavaScript and TypeScript services and tooling."},
		{Value: "fastapi", Label: "FastAPI", Group: "Backend", Detail: "Python APIs with type validation and OpenAPI."},
		{Value: "django", Label: "Django", Group: "Backend", Detail: "Python web apps, database models and administration."},
		{Value: "flask", Label: "Flask", Group: "Backend", Detail: "Lightweight Python web apps and APIs."},
		{Value: "spring", Label: "Spring Boot", Group: "Backend", Detail: "Java and Kotlin services and REST APIs."},
		{Value: "aspnet", Label: "ASP.NET Core", Group: "Backend", Detail: ".NET web apps, APIs and services."},
		{Value: "ef_core", Label: "Entity Framework Core", Group: "Data", Detail: ".NET data access and database migrations."},
		{Value: "postgres", Label: "PostgreSQL", Group: "Data", Detail: "Relational schemas, query performance and migrations."},
		{Value: "sqlite", Label: "SQLite", Group: "Data", Detail: "Embedded databases and local data storage."},
		{Value: "jupyter", Label: "Jupyter", Group: "Data", Detail: "Interactive notebooks and reproducible experiments."},
		{Value: "android", Label: "Android", Group: "Mobile", Detail: "Native Android apps, builds and tests."},
		{Value: "ios", Label: "iOS", Group: "Mobile", Detail: "Native iPhone and iPad apps."},
		{Value: "flutter", Label: "Flutter", Group: "Mobile", Detail: "Cross-platform apps with Dart."},
		{Value: "docker", Label: "Docker", Group: "Infrastructure and CI", Detail: "Container builds, images and Compose stacks."},
		{Value: "kubernetes", Label: "Kubernetes", Group: "Infrastructure and CI", Detail: "Deploy and operate containers in clusters."},
		{Value: "terraform", Label: "Terraform", Group: "Infrastructure and CI", Detail: "Provision and review infrastructure as code."},
		{Value: "github_actions", Label: "GitHub Actions", Group: "Infrastructure and CI", Detail: "Automate builds, tests and releases."},
	}, def: fixed()},
	{ID: "tools", Stage: StageModels, Prompt: "Which agent tools will you configure?", Multi: true,
		Options: []Option{
			{Value: "claude_code", Label: "Claude Code", Detail: "Prepare Claude Code's instructions, skills and settings.\nInstall guide: https://code.claude.com/docs/en/setup"},
			{Value: "codex", Label: "Codex", Detail: "Prepare Codex's instructions, skills and settings.\nInstall guide: https://learn.chatgpt.com/docs/quickstart"},
			{Value: "gemini", Label: "Gemini CLI / Antigravity", Detail: "Write ~/.gemini/GEMINI.md, which both read. No skills, settings or hooks.\nInstall guide: https://github.com/google-gemini/gemini-cli"},
		}, def: func(w *Wizard) []string {
			var out []string
			for _, tool := range []string{"claude_code", "codex", "gemini"} {
				if w.Scan.Find(tool) != nil {
					out = append(out, tool)
				}
			}
			return out
		}},
	{ID: "routing", Stage: StageModels, Prompt: "How should tasks be routed?", Options: routingOptions(), def: routingDefault},
	{ID: "plans", Stage: StageModels, Prompt: "Which AI plans or accounts do you have?", Multi: true, when: routingOn, def: fixed()},
	{ID: "models", Stage: StageModels, Prompt: "Which models can you use on those accounts?", Multi: true, when: routingOn, def: fixed()},
	{ID: "routes", Stage: StageModels, Prompt: "Preview routes for your confirmed models", ReadOnly: true, when: routingOn, def: fixed()},
}

func fixed(v ...string) func(*Wizard) []string { return func(*Wizard) []string { return v } }

func routingOptions() []Option {
	return []Option{
		{Value: routing.None, Label: "Keep current models", Detail: "Continue with your existing model settings."},
		{Value: "saver", Label: "Saver", Detail: "Prefer included usage. Use fast models for small jobs and general models for coding and review."},
		{Value: "balanced", Label: "Balanced", Detail: "Use general models for everyday coding and deeper reasoning for hard work and review."},
		{Value: "quality", Label: "Quality", Detail: "Prefer the strongest confirmed reasoning models for coding and review. Keep simple edits fast."},
	}
}

func routingDefault(w *Wizard) []string {
	if w.Company() {
		return []string{routing.None}
	}
	return []string{routing.Default}
}

func routingOn(w *Wizard) bool { return routing.Active(w.One("routing")) != "" }

func (w *Wizard) Company() bool { return w.Policy.Company(profile.Answers{Use: w.One("use")}) }

func (w *Wizard) planOptions() []Option {
	var out []Option
	for _, p := range routing.PlanOptions() {
		if w.Company() && !slices.ContainsFunc(w.Policy.Models, func(m routing.Model) bool { return m.Plan == p.ID }) {
			continue
		}
		if !w.Company() && p.ID == "company_gateway" {
			continue
		}
		out = append(out, Option{Value: p.ID, Label: p.Label, Group: planGroup(p.Tool), Detail: p.Detail})
	}
	return out
}

func planGroup(tool string) string {
	switch tool {
	case routing.ToolClaude:
		return "Anthropic"
	case routing.ToolCodex:
		return "OpenAI"
	case routing.ToolCopilot:
		return "GitHub Copilot"
	case routing.ToolAntigravity, routing.ToolGeminiCLI:
		return "Google"
	}
	return "Local and managed access"
}

func (w *Wizard) selectedPlans() []string {
	plans := w.values["plans"]
	if !w.Company() {
		return plans
	}
	var selected []string
	for _, id := range plans {
		if slices.ContainsFunc(w.Policy.Models, func(m routing.Model) bool { return m.Plan == id }) {
			selected = append(selected, id)
		}
	}
	return selected
}

func (w *Wizard) models() []routing.Model {
	plans := w.selectedPlans()
	var out []routing.Model
	if w.Company() {
		for _, m := range w.Policy.Models {
			if slices.Contains(plans, m.Plan) {
				out = append(out, m)
			}
		}
		return out
	}
	for _, m := range w.loadedModels {
		if slices.Contains(plans, m.Plan) {
			out = append(out, m)
		}
	}
	for _, m := range routing.Candidates(plans) {
		if !slices.ContainsFunc(out, func(o routing.Model) bool { return o.Key() == m.Key() }) {
			out = append(out, m)
		}
	}
	return out
}

func (w *Wizard) modelOptions() []Option {
	var out []Option
	for _, m := range w.models() {
		p, _ := routing.PlanByID(m.Plan)
		detail := p.Label
		if len(m.Efforts) > 0 {
			detail += " | Effort: " + strings.Join(m.Efforts, ", ")
		}
		if m.Plan == "chatgpt_free" || m.Plan == "chatgpt_go" {
			detail += " | Desktop, when available"
		}
		if m.ID != "auto" {
			detail += "\nModel ID: " + m.ID
		}
		out = append(out, Option{Value: m.Key(), Label: m.Choice("").String(), Note: p.Label, Group: p.Label, Detail: detail})
	}
	return out
}

func routeOptions(a profile.Answers) []Option {
	route, _, err := a.ResolveRouting()
	if err != nil || route == nil {
		return nil
	}
	var out []Option
	for _, r := range route.Rows() {
		detail := describeRoute(r.Use)
		if len(r.Fallbacks) > 0 {
			detail += " | Fallback: " + describeRoute(r.Fallbacks[0])
		} else {
			detail += " | No fallback"
		}
		out = append(out, Option{Label: r.Task.Label, Note: r.Use.String(), Detail: detail + "\n" + route.Reason(r.Task)})
	}
	return out
}

func describeRoute(c routing.Choice) string {
	if plan, ok := routing.PlanByID(c.Plan); ok {
		return c.String() + " (" + plan.Label + ")"
	}
	return c.String()
}
