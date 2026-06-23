package miser

import (
	"encoding/json"
	"fmt"
	"time"
)

type LLMCall struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Workflow     string                 `json:"workflow"`
	Provider     string                 `json:"provider"`
	Model        string                 `json:"model"`
	Prompt       string                 `json:"prompt"`
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	CostUSD      float64                `json:"cost_usd"`
	LatencyMS    *int                   `json:"latency_ms,omitempty"`
	QualityScore *float64               `json:"quality_score,omitempty"`
	Metadata     map[string]interface{} `json:"-"`
}

func (c *LLMCall) UnmarshalJSON(data []byte) error {
	type rawCall struct {
		ID           string   `json:"id"`
		Timestamp    string   `json:"timestamp"`
		Workflow     string   `json:"workflow"`
		Provider     string   `json:"provider"`
		Model        string   `json:"model"`
		Prompt       string   `json:"prompt"`
		InputTokens  int      `json:"input_tokens"`
		OutputTokens int      `json:"output_tokens"`
		CostUSD      float64  `json:"cost_usd"`
		LatencyMS    *int     `json:"latency_ms"`
		QualityScore *float64 `json:"quality_score"`
	}
	var raw rawCall
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Timestamp == "" {
		return fmt.Errorf("call is missing timestamp")
	}
	ts, err := parseTime(raw.Timestamp)
	if err != nil {
		return err
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	for _, key := range []string{"id", "timestamp", "workflow", "provider", "model", "prompt", "input_tokens", "output_tokens", "cost_usd", "latency_ms", "quality_score"} {
		delete(meta, key)
	}

	*c = LLMCall{
		ID:           raw.ID,
		Timestamp:    ts,
		Workflow:     defaultString(raw.Workflow, "unknown"),
		Provider:     defaultString(raw.Provider, "unknown"),
		Model:        defaultString(raw.Model, "unknown"),
		Prompt:       raw.Prompt,
		InputTokens:  raw.InputTokens,
		OutputTokens: raw.OutputTokens,
		CostUSD:      raw.CostUSD,
		LatencyMS:    raw.LatencyMS,
		QualityScore: raw.QualityScore,
		Metadata:     meta,
	}
	return nil
}

func (c LLMCall) TotalTokens() int {
	return c.InputTokens + c.OutputTokens
}

func (c LLMCall) Route() string {
	return c.Provider + "/" + c.Model
}

type WasteLine struct {
	Label                 string   `json:"label"`
	EstimatedMonthlyWaste float64  `json:"estimated_monthly_waste"`
	WorkflowSavingsRate   float64  `json:"workflow_savings_rate"`
	Reason                string   `json:"reason"`
	Confidence            string   `json:"confidence"`
	SampleCallIDs         []string `json:"sample_call_ids"`
}

type AuditReport struct {
	MonthlySpendAnalyzed    float64     `json:"monthly_spend_analyzed"`
	EstimatedAvoidableSpend float64     `json:"estimated_avoidable_spend"`
	SavingsOpportunity      float64     `json:"savings_opportunity"`
	TopWaste                []WasteLine `json:"top_waste"`
}

type SavingsReceipt struct {
	ClusterID            string   `json:"cluster_id"`
	Workflow             string   `json:"workflow"`
	CurrentRoute         string   `json:"current_route"`
	MonthlyCalls         int      `json:"monthly_calls"`
	CurrentMonthlyCost   float64  `json:"current_monthly_cost"`
	EstimatedMonthlyCost float64  `json:"estimated_monthly_cost"`
	EstimatedSavings     float64  `json:"estimated_savings"`
	SavingsRate          float64  `json:"savings_rate"`
	RecommendedRoute     string   `json:"recommended_route"`
	QualityGuard         string   `json:"quality_guard"`
	Rollback             string   `json:"rollback"`
	Reason               string   `json:"reason"`
	SampleCallIDs        []string `json:"sample_call_ids"`
}

func parseTime(value string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts.UTC(), nil
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts.UTC(), nil
	}
	if ts, err := time.Parse("2006-01", value); err == nil {
		return ts.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
