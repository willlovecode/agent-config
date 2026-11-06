package routing

import (
	"fmt"
	"strings"
)

const (
	metricsDate   = "2026-11-05"
	codexPricing  = "https://learn.chatgpt.com/docs/pricing"
	claudeUsage   = "https://support.claude.com/en/articles/14552983-models-usage-and-limits-in-claude-code"
	copilotPlans  = "https://docs.github.com/en/copilot/get-started/plans"
	googlePlans   = "https://antigravity.google/docs/plans"
	googlePricing = "https://cloud.google.com/model-garden/pricing"
)

type planFacts struct {
	price, allowance, source string
}

var publishedPlans = map[string]planFacts{
	"claude_pro":         {"", "Included usage is shared across Claude surfaces. Check Claude's usage screen for your limit.", claudeUsage},
	"claude_max":         {"$100/month", "5x Pro usage per five hours and per week.", "https://support.claude.com/en/articles/11049741-what-is-the-max-plan"},
	"claude_ultra":       {"$200/month", "Verified accounts get 10x Pro weekly usage without a five-hour limit.", "https://support.claude.com/en/articles/17728952-what-is-the-ultra-plan"},
	"claude_team":        {"$20 standard or $100 premium per seat/month annually, $25 or $125 monthly", "Premium seats get 5x the usage of standard seats.", "https://claude.com/pricing"},
	"claude_enterprise":  {"", "Allowance depends on the organization's contract and seat.", claudeUsage},
	"claude_api":         {"", "Token billing depends on the API or cloud provider and its region.", "https://platform.claude.com/docs/en/about-claude/pricing"},
	"chatgpt_free":       {"$0/month", "Desktop Luna access is subject to rollout.", codexPricing},
	"chatgpt_go":         {"$8/month", "Desktop Luna access is subject to rollout.", codexPricing},
	"chatgpt_plus":       {"$20/month", "Included usage varies by model. Local and cloud tasks share the allowance.", codexPricing},
	"chatgpt_pro":        {"$100, $200 or $500/month", "No five-hour limit for local messages. Usage still depends on the plan.", codexPricing},
	"chatgpt_business":   {"Standard $20/user/month billed annually or $25/user/month billed monthly", "Flexible pricing and credit-based workspaces have separate terms.", codexPricing},
	"chatgpt_edu":        {"", "Credit-based allowances or seat limits depend on the agreement.", codexPricing},
	"chatgpt_enterprise": {"", "Credit-based allowances or seat limits depend on the agreement.", codexPricing},
	"codex_api":          {"", "The API project pays for tokens. The project's rate limits apply.", "https://developers.openai.com/api/docs/pricing"},
	"copilot_free":       {"$0/month", "2,000 inline completions/month. Chat and agent use draw from the AI credit allowance.", copilotPlans},
	"copilot_student":    {"$0/month for verified students", "Model selection is Auto. Check GitHub for your AI credit allowance.", copilotPlans},
	"copilot_pro":        {"$10/month", "1,500 AI credits/month (1,000 base + 500 flex).", copilotPlans},
	"copilot_pro_plus":   {"$39/month", "7,000 AI credits/month (3,900 base + 3,100 flex).", copilotPlans},
	"copilot_max":        {"$100/month", "20,000 AI credits/month (10,000 base + 10,000 flex).", copilotPlans},
	"copilot_business":   {"$19/granted seat/month", "1,900 AI credits per seat/month in the organization's pool.", copilotPlans},
	"copilot_enterprise": {"$39/granted seat/month", "3,900 AI credits per seat/month in the organization's pool.", copilotPlans},
	"copilot_access":     {"$0 with GitHub Enterprise Cloud or $10/user/month standalone", "Metered from first use. No included AI credits.", copilotPlans},
}

// PlanMetrics describes published allowances independently of reference API prices.
func PlanMetrics(plan string) []string {
	f, ok := publishedPlans[plan]
	if !ok {
		if p, exists := PlanByID(plan); exists && p.Tool == ToolAntigravity {
			f = planFacts{"", "Check Antigravity for your current model quota and reset time.", googlePlans}
		} else if plan == "gemini_api" {
			f = planFacts{"", "Metered usage depends on the API, billing project and region.", googlePricing}
		} else {
			return []string{"Price and usage depend on the configured endpoint or agreement."}
		}
	}
	lines := []string{"Plan snapshot " + metricsDate}
	if f.price != "" {
		lines = append(lines, "Published USD price  "+f.price)
	}
	return append(lines, f.allowance, f.source)
}

type apiFacts struct {
	provider, limits, rate, note, source string
}

func openAIMetrics(id, rates string) apiFacts {
	return apiFacts{"OpenAI API", "1,050,000 context tokens / 128,000 max output",
		rates, "Above 272,000 input tokens, input and cache rates double and output is 1.5x.", "https://developers.openai.com/api/docs/models/" + id}
}

var apiModels = map[string]apiFacts{
	"gpt-6-astra":   openAIMetrics("gpt-6-astra", "$10 input / $1 cached / $12.50 cache write / $50 output"),
	Sol:             openAIMetrics(Sol, "$2 input / $0.10 cached / $2.50 cache write / $10 output"),
	"gpt-6-luna":    openAIMetrics("gpt-6-luna", "$0.10 input / $0.01 cached / $0.125 cache write / $0.50 output"),
	"gpt-6.1-terra": openAIMetrics("gpt-6.1-terra", "$1 input / $0.05 cached / $1.25 cache write (30 min) / $5 output"),
	Haiku: {"Claude API", "1,000,000 context tokens / 128,000 max output",
		"$0.10 input / $0.01 cached / $0.50 output up to 100,000 input tokens",
		"Above 100,000 input tokens: $0.50 input / $0.05 cached / $2.50 output per 1M.",
		"https://platform.claude.com/docs/en/models/haiku-5-5/overview"},
	Sonnet: {"Claude API", "1,000,000 context tokens / 128,000 max output",
		"$2 input / $0.10 cached / $2.50 cache write (5 min) / $10 output", "",
		"https://platform.claude.com/docs/en/models/sonnet-5-5/overview"},
	Opus: {"Claude API", "1,000,000 context tokens / 128,000 max output",
		"$4 input / $0.20 cached / $5 cache write (5 min) / $20 output", "",
		"https://platform.claude.com/docs/en/models/opus-5-5/overview"},
	"gemini-4-argon": {"Google Cloud API", "1,048,576 combined tokens / 65,536 default streaming output",
		"Global through 2026-11-30: $2 input / $0.10 cached / $10 output",
		"Global from 2026-12-01: $4 input / $0.20 cached / $20 output per 1M.",
		"https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/guides/gemini-4-argon"},
}

type benchmark struct {
	effort, firstAnswer, cost, slug string
	speed                           int
}

var measured = map[string][]benchmark{
	Haiku:            {{"medium", "11.39", "0.09", "claude-haiku-5-5-medium", 224}},
	Sonnet:           {{"high", "12.82", "1.64", "claude-sonnet-5-5-high", 143}},
	Opus:             {{"medium", "18.70", "2.45", "claude-opus-5-5-medium", 110}},
	Sol:              {{"medium", "6.31", "0.36", "gpt-6-1-sol-medium", 57}, {"xhigh", "69.80", "0.76", "gpt-6-1-sol-xhigh", 55}},
	"gpt-6-astra":    {{"medium", "6.49", "2.73", "gpt-6-astra-medium", 51}, {"xhigh", "103.63", "3.66", "gpt-6-astra-xhigh", 47}},
	"gpt-6.1-terra":  {{"medium", "4.43", "0.17", "gpt-6-1-terra-medium", 72}},
	"gemini-4-argon": {{"high", "40.61", "9.95", "gemini-4-argon", 74}},
}

var codexUsage = map[string]struct{ messages, credits string }{
	"gpt-6-astra":   {"5-45", "250 input / 25 cached / 1,250 output"},
	Sol:             {"15-160", "50 input / 2.5 cached / 250 output"},
	"gpt-6.1-terra": {"35-285", "25 input / 1.25 cached / 125 output"},
	"gpt-6-luna":    {"350-3,000", "2.5 input / 0.25 cached / 12.5 output"},
}

// ModelMetrics labels API references, plan usage and measured speed separately.
// An API benchmark is keyed to the effort that was measured.
func ModelMetrics(c Choice) []string {
	lines := []string{"Model data snapshot " + metricsDate}
	if c.Tool == ToolCodex && c.Model == Sol && strings.HasPrefix(c.Plan, "chatgpt_") {
		lines = append(lines, "Codex default context  500,000 tokens")
	}
	if usage, ok := codexUsage[c.Model]; ok && c.Tool == ToolCodex && strings.HasPrefix(c.Plan, "chatgpt_") {
		if c.Plan == "chatgpt_plus" || c.Plan == "chatgpt_business" {
			plan := "Plus"
			if c.Plan == "chatgpt_business" {
				plan = "Standard Business"
			}
			lines = append(lines, plan+" estimate  "+usage.messages+" local messages / 5 hours")
		}
		if c.Plan != "chatgpt_free" && c.Plan != "chatgpt_go" {
			lines = append(lines, "Standard Codex credits per 1M tokens  "+usage.credits, codexPricing)
		}
	}
	if f, ok := apiModels[c.Model]; ok {
		lines = append(lines, f.provider+" reference  "+f.limits, "Standard USD per 1M tokens  "+f.rate)
		if f.note != "" {
			lines = append(lines, f.note)
		}
		lines = append(lines, f.source)
		if c.Model == "gemini-4-argon" {
			lines = append(lines, googlePricing)
		}
	} else {
		lines = append(lines, "Token limits and API price  No public figure for this choice")
	}
	for _, b := range measured[c.Model] {
		if c.Effort == "" || c.Effort == b.effort {
			lines = append(lines,
				fmt.Sprintf("API benchmark (%s)  %d output tokens/s, %s s to first answer", b.effort, b.speed, b.firstAnswer),
				"Benchmark cost  $"+b.cost+" per Intelligence Index task",
				"https://artificialanalysis.ai/models/"+b.slug)
			return lines
		}
	}
	return append(lines, "Measured speed  No matching benchmark in this snapshot")
}
