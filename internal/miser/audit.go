//audit code + coding agent  flow
package miser

import (
	"fmt"
	"sort"
	"strings"
)

func Audit(calls []LLMCall) AuditReport {
	monthlySpend := 0.0
	for _, call := range calls {
		monthlySpend += call.CostUSD
	}

	waste := []WasteLine{
		openAIContextReplayWaste(calls),
		providerUsageWaste(calls),
		codingAgentContextWaste(calls),
		repeatedLongContextWaste(calls),
		classificationWaste(calls),
		duplicateSummaryWaste(calls),
		retryLoopWaste(calls),
		oversizedPDFWaste(calls),
	}
	filtered := make([]WasteLine, 0, len(waste))
	for _, line := range waste {
		if line.EstimatedMonthlyWaste > 0 {
			filtered = append(filtered, line)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].EstimatedMonthlyWaste > filtered[j].EstimatedMonthlyWaste
	})
	if len(filtered) > 5 {
		filtered = filtered[:5]
	}

	avoidable := 0.0
	for _, line := range filtered {
		avoidable += line.EstimatedMonthlyWaste
	}
	if avoidable > monthlySpend {
		avoidable = monthlySpend
	}
	savingsOpportunity := 0.0
	if monthlySpend > 0 {
		savingsOpportunity = avoidable / monthlySpend
	}

	return AuditReport{
		MonthlySpendAnalyzed:    monthlySpend,
		EstimatedAvoidableSpend: avoidable,
		SavingsOpportunity:      savingsOpportunity,
		TopWaste:                filtered,
		CostBasis:               costBasis(calls),
	}
}

func RenderAudit(report AuditReport, explain bool) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Miser AI Spend Audit")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Monthly spend analyzed: %s\n", dollars(report.MonthlySpendAnalyzed))
	if report.CostBasis != "" {
		fmt.Fprintf(&b, "Cost basis: %s\n", report.CostBasis)
	}
	fmt.Fprintf(&b, "Estimated avoidable spend: %s\n", dollars(report.EstimatedAvoidableSpend))
	fmt.Fprintf(&b, "Savings opportunity: %.1f%%\n", report.SavingsOpportunity*100)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Top waste:")
	if len(report.TopWaste) == 0 {
		fmt.Fprintln(&b, "No obvious waste patterns found.")
		return b.String()
	}
	for i, line := range report.TopWaste {
		fmt.Fprintf(&b, "%d. %s: %s", i+1, line.Label, dollars(line.EstimatedMonthlyWaste))
		if line.WorkflowSavingsRate > 0 {
			fmt.Fprintf(&b, " (%.0f%% workflow savings potential)", line.WorkflowSavingsRate*100)
		}
		fmt.Fprintln(&b)
		if explain {
			samples := "none"
			if len(line.SampleCallIDs) > 0 {
				samples = strings.Join(line.SampleCallIDs, ", ")
			}
			fmt.Fprintf(&b, "   Why: %s\n", line.Reason)
			fmt.Fprintf(&b, "   Confidence: %s\n", line.Confidence)
			fmt.Fprintf(&b, "   Sample calls: %s\n", samples)
		}
	}
	return b.String()
}

func openAIContextReplayWaste(calls []LLMCall) WasteLine {
	totalCost := 0.0
	inputTokens := 0
	cachedTokens := 0
	outputTokens := 0
	models := map[string]bool{}
	var samples []string

	for _, call := range calls {
		source, _ := call.Metadata["source"].(string)
		if source != "openai_usage_api" {
			continue
		}
		cacheRead := intFromAny(call.Metadata["input_cached_tokens"])
		if call.InputTokens < 25000 || cacheRead < 10000 {
			continue
		}
		cacheRatio := float64(cacheRead) / float64(call.InputTokens)
		if cacheRatio < 0.50 {
			continue
		}

		totalCost += call.CostUSD * 0.25
		inputTokens += call.InputTokens
		cachedTokens += cacheRead
		outputTokens += call.OutputTokens
		models[call.Model] = true
		samples = appendSample(samples, call.ID)
	}

	if totalCost == 0 {
		return WasteLine{}
	}

	cacheRatio := 0.0
	if inputTokens > 0 {
		cacheRatio = float64(cachedTokens) / float64(inputTokens)
	}
	outputRatio := 0.0
	if inputTokens+outputTokens > 0 {
		outputRatio = float64(outputTokens) / float64(inputTokens+outputTokens)
	}

	return WasteLine{
		Label:                 "Coding-agent context replay",
		EstimatedMonthlyWaste: totalCost,
		WorkflowSavingsRate:   0.25,
		Reason: fmt.Sprintf(
			"OpenAI usage rows show %s input tokens, %s cached input tokens (%.1f%% cached), and %s output tokens (%.1f%% of total tokens) on %s. Caching is working, but the agent is still replaying huge context. Likely fixes: narrower task scope, repo/session summaries, file indexes, and smaller context handoffs before model calls.",
			formatInt(inputTokens),
			formatInt(cachedTokens),
			cacheRatio*100,
			formatInt(outputTokens),
			outputRatio*100,
			modelList(models),
		),
		Confidence:    "medium",
		SampleCallIDs: samples,
	}
}

func providerUsageWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		source, _ := call.Metadata["source"].(string)
		if source != "openai_usage_api" {
			continue
		}
		cacheRead := intFromAny(call.Metadata["input_cached_tokens"])
		if call.InputTokens >= 25000 && cacheRead >= 10000 && float64(cacheRead)/float64(call.InputTokens) >= 0.50 {
			continue
		}
		requests := intFromAny(call.Metadata["num_model_requests"])
		if requests >= 10 || call.InputTokens >= 25000 || call.OutputTokens >= 10000 {
			total += call.CostUSD * 0.25
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "High-volume provider API usage",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.25,
		Reason:                "OpenAI usage API rows show high request or token volume. These aggregate rows are good first-pass candidates for prompt tightening, cache checks, cheaper model routing, and per-project/API-key drilldown.",
		Confidence:            "low",
		SampleCallIDs:         samples,
	}
}

func codingAgentContextWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		source, _ := call.Metadata["source"].(string)
		if source != "ccusage" && call.Workflow != "coding_agent_usage" {
			continue
		}
		cacheRead := intFromAny(call.Metadata["cache_read_tokens"])
		if call.InputTokens >= 50000 || cacheRead >= 25000 {
			total += call.CostUSD * 0.35
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Coding-agent context reconstruction",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.35,
		Reason:                "ccusage rows show large coding-agent input/cache-read token volume. This often means the agent is re-reading project context instead of using session handoffs, code indexes, or narrower task scopes.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func repeatedLongContextWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, group := range groups(calls) {
		if len(group) < 3 {
			continue
		}
		sum := 0
		skip := false
		for _, call := range group {
			if isCCUsageAggregate(call) {
				skip = true
				break
			}
			sum += call.InputTokens
		}
		if skip {
			continue
		}
		if float64(sum)/float64(len(group)) >= 3000 {
			for _, call := range group {
				total += call.CostUSD * 0.55
				samples = appendSample(samples, call.ID)
			}
		}
	}
	return WasteLine{
		Label:                 "Repeated long-context calls",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.55,
		Reason:                "The same workflow repeatedly sends large prompts. Miser assumes context can be compressed, cached, or split before model escalation.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func classificationWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		if isCCUsageAggregate(call) {
			continue
		}
		text := strings.ToLower(call.Workflow + " " + call.Prompt)
		model := strings.ToLower(call.Model)
		classification := strings.Contains(text, "classif") || strings.Contains(text, "triage")
		expensive := strings.Contains(model, "sonnet") || strings.Contains(model, "opus") || strings.Contains(model, "gpt-4") || strings.Contains(model, "gemini-1.5-pro")
		if classification && expensive {
			total += call.CostUSD * 0.70
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Expensive model used for classification",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.70,
		Reason:                "Classification and triage tasks often route well through local classifiers, smaller models, or rules with frontier fallback.",
		Confidence:            "high",
		SampleCallIDs:         samples,
	}
}

func duplicateSummaryWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, group := range groups(calls) {
		if len(group) < 3 {
			continue
		}
		if isCCUsageAggregate(group[0]) {
			continue
		}
		sample := strings.ToLower(group[0].Workflow + " " + group[0].Prompt)
		if !strings.Contains(sample, "summar") {
			continue
		}
		for i, call := range group {
			if i > 0 {
				total += call.CostUSD * 0.80
			}
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Duplicate summaries",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.80,
		Reason:                "Miser found repeated summary prompts after masking IDs and emails. These are strong candidates for exact or semantic caching.",
		Confidence:            "high",
		SampleCallIDs:         samples,
	}
}

func retryLoopWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		if isCCUsageAggregate(call) {
			continue
		}
		text := strings.ToLower(call.Workflow + " " + call.Prompt)
		if call.Metadata["retry"] != nil || call.Metadata["is_retry"] != nil || call.Metadata["attempt"] != nil || strings.Contains(text, "retry") || strings.Contains(text, "try again") || strings.Contains(text, "previous attempt failed") {
			total += call.CostUSD * 0.85
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Agent retry loops",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.85,
		Reason:                "Retry attempts are marked in metadata or prompt text. These usually need guardrails, tool-result caching, or deterministic failure handling.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func oversizedPDFWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		if isCCUsageAggregate(call) {
			continue
		}
		text := strings.ToLower(call.Workflow + " " + call.Prompt)
		if strings.Contains(text, "pdf") && call.InputTokens >= 6000 {
			total += call.CostUSD * 0.60
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Oversized PDF prompts",
		EstimatedMonthlyWaste: total,
		WorkflowSavingsRate:   0.60,
		Reason:                "PDF workflows are sending very large prompts. Extraction should often be chunked, templated, or handled with deterministic parsing before LLM review.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func groups(calls []LLMCall) map[string][]LLMCall {
	out := map[string][]LLMCall{}
	for _, call := range calls {
		key := call.Workflow + ":" + FingerprintPrompt(call.Prompt)
		out[key] = append(out[key], call)
	}
	return out
}

func appendSample(samples []string, id string) []string {
	if id == "" || len(samples) >= 5 {
		return samples
	}
	return append(samples, id)
}

func isCCUsageAggregate(call LLMCall) bool {
	return call.CostBasis == "estimated_token_cost" || call.Integration == "ccusage"
}

func costBasis(calls []LLMCall) string {
	if len(calls) == 0 {
		return ""
	}
	bases := map[string]bool{}
	for _, call := range calls {
		if call.CostBasis != "" {
			bases[call.CostBasis] = true
		}
	}
	if len(bases) == 1 {
		if bases["actual_invoice"] {
			return "actual invoice/billing export"
		}
		if bases["actual_invoice_allocated"] {
			return "actual invoice allocated to usage"
		}
		if bases["provider_billing_api"] {
			return "provider billing API"
		}
		if bases["estimated_token_cost"] {
			return "estimated token cost, not your actual invoice"
		}
		if bases["published_token_price"] {
			return "published token price, not your actual invoice"
		}
		if bases["unpriced_token_usage"] {
			return "unpriced token usage"
		}
		if bases["reported_log_cost"] {
			return "reported log cost"
		}
	}
	if len(bases) > 1 {
		return "mixed cost basis"
	}
	return "reported log cost"
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func dollars(value float64) string {
	return fmt.Sprintf("$%.2f", value)
}

func formatInt(value int) string {
	raw := fmt.Sprintf("%d", value)
	if len(raw) <= 3 {
		return raw
	}
	var b strings.Builder
	prefix := len(raw) % 3
	if prefix == 0 {
		prefix = 3
	}
	b.WriteString(raw[:prefix])
	for i := prefix; i < len(raw); i += 3 {
		b.WriteString(",")
		b.WriteString(raw[i : i+3])
	}
	return b.String()
}

func modelList(models map[string]bool) string {
	if len(models) == 0 {
		return "unknown models"
	}
	var names []string
	for model := range models {
		names = append(names, model)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
