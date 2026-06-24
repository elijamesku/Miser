package miser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFingerprintMasksIDsAndEmails(t *testing.T) {
	left := "Summarize ticket 123 for jane@example.com"
	right := "Summarize ticket 999 for sam@example.com"

	if NormalizePrompt(left) != "summarize ticket <num> for <email>" {
		t.Fatalf("unexpected normalized prompt: %s", NormalizePrompt(left))
	}
	if FingerprintPrompt(left) != FingerprintPrompt(right) {
		t.Fatal("expected prompts to share fingerprint")
	}
}

func TestAuditExplain(t *testing.T) {
	calls := []LLMCall{
		testCall("call_1", "claim_denial_triage", "Classify denial 1.", 100),
		testCall("call_2", "claim_denial_triage", "Classify denial 2.", 100),
		testCall("call_3", "claim_denial_triage", "Classify denial 3.", 100),
	}
	report := Audit(calls)
	rendered := RenderAudit(report, true)

	if !strings.Contains(rendered, "Miser AI Spend Audit") {
		t.Fatal("missing audit header")
	}
	if !strings.Contains(rendered, "Why:") || !strings.Contains(rendered, "Sample calls:") {
		t.Fatal("missing explain output")
	}
	if report.MonthlySpendAnalyzed != 300 {
		t.Fatalf("unexpected spend: %f", report.MonthlySpendAnalyzed)
	}
}

func TestAnalyzeEmitsReceipt(t *testing.T) {
	var calls []LLMCall
	for i := 0; i < 10; i++ {
		calls = append(calls, testCall("call", "support_ticket_summary", "Summarize support ticket 123 for jane@example.com", 0.02))
	}
	receipts := Analyze(calls, AnalyzerConfig{MinClusterSize: 3, MinMonthlySavingsUSD: 1, MinQualityScore: 0.95})
	if len(receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(receipts))
	}
	if receipts[0].RecommendedRoute != "semantic_cache -> smaller_model_fallback" {
		t.Fatalf("unexpected route: %s", receipts[0].RecommendedRoute)
	}
}

func TestImportCCUsage(t *testing.T) {
	rows, err := ImportCCUsage("../../examples/ccusage.json", ImportOptions{AccountID: "codex-local", Integration: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if rows[0]["workflow"] != "coding_agent_usage" || rows[0]["source"] != "ccusage" {
		t.Fatalf("unexpected row: %#v", rows[0])
	}
	if rows[0]["account_id"] != "codex-local" || rows[0]["integration"] != "codex" {
		t.Fatalf("missing account/integration: %#v", rows[0])
	}
	if rows[0]["cost_basis"] != "estimated_token_cost" {
		t.Fatalf("unexpected cost basis: %#v", rows[0])
	}
}

func TestFilterCalls(t *testing.T) {
	calls := []LLMCall{
		{ID: "1", AccountID: "claude-work", Integration: "claude"},
		{ID: "2", AccountID: "codex-local", Integration: "codex"},
	}
	filtered := FilterCalls(calls, FilterConfig{AccountID: "claude-work", Integration: "claude"})
	if len(filtered) != 1 || filtered[0].ID != "1" {
		t.Fatalf("unexpected filtered calls: %#v", filtered)
	}
}

func TestCostBasisUsesActualInvoice(t *testing.T) {
	report := Audit([]LLMCall{
		{
			ID:          "invoice_1",
			Timestamp:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:    "billing_invoice",
			Provider:    "anthropic",
			CostUSD:     20,
			AccountID:   "claude-work",
			Integration: "claude",
			CostBasis:   "actual_invoice",
			Metadata:    map[string]interface{}{},
		},
	})
	if report.CostBasis != "actual invoice/billing export" {
		t.Fatalf("unexpected cost basis: %q", report.CostBasis)
	}
}

func TestCostBasisUsesActualInvoiceAllocated(t *testing.T) {
	report := Audit([]LLMCall{
		{
			ID:          "openai_usage_1",
			Timestamp:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:    "openai_api_usage",
			Provider:    "openai",
			Model:       "gpt-5.5",
			CostUSD:     10,
			AccountID:   "openai-personal",
			Integration: "codex",
			CostBasis:   "actual_invoice_allocated",
			Metadata:    map[string]interface{}{},
		},
	})
	if report.CostBasis != "actual invoice allocated to usage" {
		t.Fatalf("unexpected cost basis: %q", report.CostBasis)
	}
}

func TestCostBasisUsesProviderBillingAPI(t *testing.T) {
	report := Audit([]LLMCall{
		{
			ID:          "openai_cost_1",
			Timestamp:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:    "provider_billing",
			Provider:    "openai",
			CostUSD:     20,
			AccountID:   "codex-local",
			Integration: "codex",
			CostBasis:   "provider_billing_api",
			Metadata:    map[string]interface{}{},
		},
	})
	if report.CostBasis != "provider billing API" {
		t.Fatalf("unexpected cost basis: %q", report.CostBasis)
	}
}

func TestOpenAICostsURL(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	got, err := openAICostsURL(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "https://api.openai.com/v1/organization/costs?") {
		t.Fatalf("unexpected costs URL: %s", got)
	}
	if !strings.Contains(got, "bucket_width=1d") || !strings.Contains(got, "group_by=line_item") || !strings.Contains(got, "group_by=project_id") {
		t.Fatalf("missing expected query params: %s", got)
	}
}

func TestOpenAIUsageURL(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	got, err := openAIUsageURL(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "https://api.openai.com/v1/organization/usage/completions?") {
		t.Fatalf("unexpected usage URL: %s", got)
	}
	if !strings.Contains(got, "bucket_width=1d") || !strings.Contains(got, "group_by=model") || !strings.Contains(got, "group_by=project_id") {
		t.Fatalf("missing expected query params: %s", got)
	}
}

func TestOpenAIUsageRowsCanFlagContextReplay(t *testing.T) {
	report := Audit([]LLMCall{
		{
			ID:           "openai_usage_1",
			Timestamp:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:     "openai_api_usage",
			Provider:     "openai",
			Model:        "gpt-4o",
			Prompt:       "OpenAI API usage model=gpt-4o requests=25",
			InputTokens:  50000,
			OutputTokens: 12000,
			CostUSD:      0.25,
			AccountID:    "codex-work",
			Integration:  "codex",
			CostBasis:    "estimated_token_cost",
			Metadata: map[string]interface{}{
				"source":              "openai_usage_api",
				"num_model_requests":  float64(25),
				"input_cached_tokens": float64(48000),
			},
		},
	})
	if len(report.TopWaste) == 0 {
		t.Fatal("expected usage waste finding")
	}
	if report.TopWaste[0].Label != "Coding-agent context replay" {
		t.Fatalf("unexpected top waste: %#v", report.TopWaste[0])
	}
	if !strings.Contains(report.TopWaste[0].Reason, "96.0% cached") {
		t.Fatalf("missing cache ratio in reason: %s", report.TopWaste[0].Reason)
	}
}

func TestOpenAIModelRatesUsePublishedGPT55Pricing(t *testing.T) {
	cost, _, ok := PriceTokenUsage("openai", "gpt-5.5", 17794923, 50472, 17386752)
	if !ok {
		t.Fatal("expected gpt-5.5 to have published pricing")
	}
	if fmt.Sprintf("%.6f", cost) != "12.248391" {
		t.Fatalf("unexpected cost: %.6f", cost)
	}
}

func TestOpenAIModelRatesRejectUnknownModels(t *testing.T) {
	cost, _, ok := PriceTokenUsage("openai", "unknown-future-model", 1000000, 1000000, 0)
	if ok {
		t.Fatal("expected unknown model to be unpriced")
	}
	if cost != 0 {
		t.Fatalf("unexpected unknown model cost: %f", cost)
	}
}

func TestOpenAIUsageUnmarshalAppliesPublishedPricing(t *testing.T) {
	raw := []byte(`{"account_id":"openai-personal","cost_basis":"estimated_token_cost","cost_usd":9.303435,"id":"openai_usage_0001","input_cached_tokens":17386752,"input_tokens":17794923,"integration":"codex","model":"gpt-5.5","output_tokens":50472,"prompt":"OpenAI API usage model=gpt-5.5","provider":"openai","source":"openai_usage_api","timestamp":"2026-06-03T00:00:00Z","workflow":"openai_api_usage"}`)
	var call LLMCall
	if err := call.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if call.CostBasis != "published_token_price" {
		t.Fatalf("unexpected cost basis: %s", call.CostBasis)
	}
	if fmt.Sprintf("%.6f", call.CostUSD) != "12.248391" {
		t.Fatalf("unexpected cost: %.6f", call.CostUSD)
	}
}

func TestActualInvoiceAllocatedRowsAreNotRepriced(t *testing.T) {
	raw := []byte(`{"account_id":"openai-personal","actual_invoice_total_usd":10,"cost_basis":"actual_invoice_allocated","cost_usd":4.25,"id":"openai_usage_0001","input_cached_tokens":17386752,"input_tokens":17794923,"integration":"codex","model":"gpt-5.5","output_tokens":50472,"prompt":"OpenAI API usage model=gpt-5.5","provider":"openai","source":"openai_usage_api","timestamp":"2026-06-03T00:00:00Z","workflow":"openai_api_usage"}`)
	var call LLMCall
	if err := call.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if call.CostBasis != "actual_invoice_allocated" {
		t.Fatalf("unexpected cost basis: %s", call.CostBasis)
	}
	if fmt.Sprintf("%.2f", call.CostUSD) != "4.25" {
		t.Fatalf("unexpected cost: %.6f", call.CostUSD)
	}
}

func TestClaudeModelPricingUsesProviderCatalog(t *testing.T) {
	cost, pricing, ok := PriceTokenUsage("anthropic", "claude-sonnet-4-5", 1000000, 1000000, 0)
	if !ok {
		t.Fatal("expected Claude Sonnet to have published pricing")
	}
	if pricing.Provider != "anthropic" || pricing.Source != "anthropic_public_pricing" {
		t.Fatalf("unexpected pricing metadata: %#v", pricing)
	}
	if fmt.Sprintf("%.2f", cost) != "18.00" {
		t.Fatalf("unexpected Claude Sonnet cost: %.6f", cost)
	}
}

func TestClaudePricingInfersProviderFromModel(t *testing.T) {
	cost, pricing, ok := PriceTokenUsage("ccusage", "claude-3-5-sonnet-20241022", 1000000, 1000000, 1000000)
	if !ok {
		t.Fatal("expected Claude model to infer Anthropic pricing")
	}
	if pricing.Provider != "anthropic" {
		t.Fatalf("unexpected provider: %#v", pricing)
	}
	if fmt.Sprintf("%.2f", cost) != "15.30" {
		t.Fatalf("unexpected cached Claude Sonnet cost: %.6f", cost)
	}
}

func TestCCUsageUnmarshalAppliesClaudePricing(t *testing.T) {
	raw := []byte(`{"account_id":"claude-work","cache_read_tokens":1000000,"cost_basis":"estimated_token_cost","cost_usd":3.00,"id":"ccusage_0001","input_tokens":1000000,"integration":"claude","model":"claude-3-5-sonnet-20241022","output_tokens":100000,"prompt":"Coding agent usage aggregate.","provider":"ccusage","source":"ccusage","timestamp":"2026-06-03T00:00:00Z","workflow":"coding_agent_usage"}`)
	var call LLMCall
	if err := call.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if call.CostBasis != "published_token_price" {
		t.Fatalf("unexpected cost basis: %s", call.CostBasis)
	}
	if fmt.Sprintf("%.2f", call.CostUSD) != "1.80" {
		t.Fatalf("unexpected cost: %.6f", call.CostUSD)
	}
}

func TestPlanTurnsAuditFindingIntoExecutablePolicy(t *testing.T) {
	plan := Plan([]LLMCall{
		{
			ID:           "openai_usage_1",
			Timestamp:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:     "openai_api_usage",
			Provider:     "openai",
			Model:        "gpt-5.5",
			Prompt:       "OpenAI API usage model=gpt-5.5",
			InputTokens:  100000,
			OutputTokens: 100,
			CostUSD:      10,
			AccountID:    "openai-personal",
			Integration:  "codex",
			CostBasis:    "published_token_price",
			Metadata: map[string]interface{}{
				"source":              "openai_usage_api",
				"input_cached_tokens": float64(90000),
			},
		},
	})
	if len(plan.Items) != 1 {
		t.Fatalf("expected one plan item, got %d", len(plan.Items))
	}
	item := plan.Items[0]
	if item.Workflow != "coding_agent_context_replay" {
		t.Fatalf("unexpected workflow: %s", item.Workflow)
	}
	if fmt.Sprintf("%.2f", item.CurrentMonthlyCost) != "10.00" || fmt.Sprintf("%.2f", item.EstimatedSavings) != "2.50" {
		t.Fatalf("unexpected plan money: %#v", item)
	}
	rendered := RenderPlanYAML(plan)
	if !strings.Contains(rendered, "bounded_context") || !strings.Contains(rendered, "competitor_overlap") || !strings.Contains(rendered, "miser_difference") {
		t.Fatalf("missing executable plan details:\n%s", rendered)
	}
}

func TestRulesGenerateCodexContextReplayPolicy(t *testing.T) {
	pack := Rules([]LLMCall{
		{
			ID:           "openai_usage_1",
			Timestamp:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:     "openai_api_usage",
			Provider:     "openai",
			Model:        "gpt-5.5",
			Prompt:       "OpenAI API usage model=gpt-5.5",
			InputTokens:  100000,
			OutputTokens: 100,
			CostUSD:      10,
			AccountID:    "openai-personal",
			Integration:  "codex",
			CostBasis:    "actual_invoice_allocated",
			Metadata: map[string]interface{}{
				"source":              "openai_usage_api",
				"input_cached_tokens": float64(90000),
			},
		},
	}, "codex")
	if pack.Target != "codex" || len(pack.Rules) != 1 {
		t.Fatalf("unexpected rule pack: %#v", pack)
	}
	rule := pack.Rules[0]
	if rule.ID != "miser.coding_agent_context_replay.v1" {
		t.Fatalf("unexpected rule id: %s", rule.ID)
	}
	if rule.Limits["max_tool_output_lines"] != "200" || rule.Limits["max_replayed_prompt_tokens"] != "8000" {
		t.Fatalf("missing concrete limits: %#v", rule.Limits)
	}
	if !strings.Contains(strings.Join(rule.Enforcement, "\n"), "For Codex") {
		t.Fatalf("missing Codex-specific enforcement: %#v", rule.Enforcement)
	}
	rendered := RenderRulesYAML(pack)
	if !strings.Contains(rendered, "policy-as-code") || !strings.Contains(rendered, "cached_input_tokens/input_tokens") {
		t.Fatalf("unexpected rendered rules:\n%s", rendered)
	}
}

func TestRenderAgentInstructions(t *testing.T) {
	pack := AgentRulePack{
		Version:                 "miser.rules/v1",
		Target:                  "codex",
		MonthlySpendAnalyzed:    10,
		CostBasis:               "actual invoice allocated to usage",
		EstimatedAvoidableSpend: 2.5,
		Rules: []AgentRule{
			{
				ID:                     "miser.coding_agent_context_replay.v1",
				Finding:                "Coding-agent context replay",
				Trigger:                "cached_input_tokens/input_tokens >= 0.50",
				ExpectedMonthlySavings: 2.5,
				SavingsRate:            0.25,
				Limits: map[string]string{
					"max_context_files": "12",
				},
				Enforcement:  []string{"Start from a short repo/session summary."},
				QualityGuard: "replay_eval >= 0.95",
				Fallback:     "current_model",
				Rollback:     "keep current route behind a feature flag",
			},
		},
	}
	rendered := RenderAgentInstructions(pack)
	if !strings.Contains(rendered, "# Miser Agent Rules") || !strings.Contains(rendered, "Start from a short repo/session summary.") {
		t.Fatalf("unexpected instructions:\n%s", rendered)
	}
}

func TestApplyRulesGeneratesCodexArtifacts(t *testing.T) {
	raw := []byte(`# Generated by Miser.
version: "miser.rules/v1"
target: "codex"
rules:
  - id: "miser.coding_agent_context_replay.v1"
`)
	files := ApplyRules(raw, "codex", ".miser")
	if len(files) != 4 {
		t.Fatalf("expected four generated files, got %d: %#v", len(files), files)
	}
	var paths []string
	contents := map[string]string{}
	for _, file := range files {
		paths = append(paths, file.Path)
		contents[file.Path] = file.Content
	}
	joined := strings.Join(paths, "\n")
	for _, want := range []string{
		".miser/codex-policy.md",
		".miser/session-handoff-template.md",
		".miser/replay-eval-checklist.md",
		".miser/context-replay-metrics.json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing generated path %s in %v", want, paths)
		}
	}
	if !strings.Contains(contents[".miser/codex-policy.md"], "Context Replay Control") {
		t.Fatalf("missing context replay policy:\n%s", contents[".miser/codex-policy.md"])
	}
	if !strings.Contains(contents[".miser/context-replay-metrics.json"], "actual_invoice_allocated_cost_usd") {
		t.Fatalf("missing invoice metric:\n%s", contents[".miser/context-replay-metrics.json"])
	}
}

func TestReconcileToActualSpendScalesUsageRows(t *testing.T) {
	rows, err := ReconcileToActualSpend([]LLMCall{
		{
			ID:           "usage_1",
			Timestamp:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Workflow:     "openai_api_usage",
			Provider:     "openai",
			Model:        "gpt-5.5",
			InputTokens:  100,
			OutputTokens: 10,
			CostUSD:      20,
			AccountID:    "openai-personal",
			Integration:  "codex",
			CostBasis:    "published_token_price",
			Metadata: map[string]interface{}{
				"source": "openai_usage_api",
			},
		},
		{
			ID:           "usage_2",
			Timestamp:    time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			Workflow:     "openai_api_usage",
			Provider:     "openai",
			Model:        "gpt-5.5",
			InputTokens:  100,
			OutputTokens: 10,
			CostUSD:      5,
			AccountID:    "openai-personal",
			Integration:  "codex",
			CostBasis:    "published_token_price",
			Metadata:     map[string]interface{}{},
		},
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if fmt.Sprintf("%.2f", rows[0]["cost_usd"]) != "8.00" || fmt.Sprintf("%.2f", rows[1]["cost_usd"]) != "2.00" {
		t.Fatalf("unexpected allocated costs: %#v", rows)
	}
	if rows[0]["cost_basis"] != "actual_invoice_allocated" || rows[0]["actual_invoice_total_usd"] != float64(10) {
		t.Fatalf("missing actual invoice allocation metadata: %#v", rows[0])
	}
}

func testCall(id, workflow, prompt string, cost float64) LLMCall {
	return LLMCall{
		ID:           id,
		Timestamp:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Workflow:     workflow,
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet",
		Prompt:       prompt,
		InputTokens:  2000,
		OutputTokens: 300,
		CostUSD:      cost,
		Metadata:     map[string]interface{}{},
	}
}
