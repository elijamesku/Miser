package miser

import "fmt"

func ReconcileToActualSpend(calls []LLMCall, actualSpend float64) ([]map[string]interface{}, error) {
	if actualSpend < 0 {
		return nil, fmt.Errorf("actual spend must be zero or greater")
	}
	totalWeight := 0.0
	allocationBasis := "reported_cost_share"
	for _, call := range calls {
		if call.CostUSD > 0 {
			totalWeight += call.CostUSD
		}
	}
	if totalWeight == 0 {
		allocationBasis = "token_share"
		for _, call := range calls {
			totalWeight += float64(call.TotalTokens())
		}
	}
	if totalWeight == 0 {
		return nil, fmt.Errorf("cannot reconcile without usage cost or tokens")
	}

	rows := make([]map[string]interface{}, 0, len(calls))
	for _, call := range calls {
		weight := call.CostUSD
		if allocationBasis == "token_share" {
			weight = float64(call.TotalTokens())
		}
		allocatedCost := 0.0
		if weight > 0 {
			allocatedCost = actualSpend * (weight / totalWeight)
		}
		rows = append(rows, reconciledRow(call, allocatedCost, actualSpend, allocationBasis))
	}
	return rows, nil
}

func reconciledRow(call LLMCall, allocatedCost, actualSpend float64, allocationBasis string) map[string]interface{} {
	row := map[string]interface{}{}
	for key, value := range call.Metadata {
		row[key] = value
	}
	row["id"] = call.ID
	row["timestamp"] = call.Timestamp.Format("2006-01-02T15:04:05Z")
	row["workflow"] = call.Workflow
	row["provider"] = call.Provider
	row["model"] = call.Model
	row["prompt"] = call.Prompt
	row["input_tokens"] = call.InputTokens
	row["output_tokens"] = call.OutputTokens
	row["cost_usd"] = allocatedCost
	row["account_id"] = call.AccountID
	row["integration"] = call.Integration
	row["cost_basis"] = "actual_invoice_allocated"
	row["original_cost_usd"] = call.CostUSD
	row["original_cost_basis"] = call.CostBasis
	row["actual_invoice_total_usd"] = actualSpend
	row["allocation_basis"] = allocationBasis
	if call.LatencyMS != nil {
		row["latency_ms"] = *call.LatencyMS
	}
	if call.QualityScore != nil {
		row["quality_score"] = *call.QualityScore
	}
	return row
}
