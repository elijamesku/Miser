package miser

import (
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
	rows, err := ImportCCUsage("../../examples/ccusage.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if rows[0]["workflow"] != "coding_agent_usage" || rows[0]["source"] != "ccusage" {
		t.Fatalf("unexpected row: %#v", rows[0])
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
