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
