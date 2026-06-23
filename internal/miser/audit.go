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
	}
}

func RenderAudit(report AuditReport, explain bool) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Miser AI Spend Audit")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Monthly spend analyzed: %s\n", dollars(report.MonthlySpendAnalyzed))
	fmt.Fprintf(&b, "Estimated avoidable spend: %s\n", dollars(report.EstimatedAvoidableSpend))
	fmt.Fprintf(&b, "Savings opportunity: %.1f%%\n", report.SavingsOpportunity*100)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Top waste:")
	if len(report.TopWaste) == 0 {
		fmt.Fprintln(&b, "No obvious waste patterns found.")
		return b.String()
	}
	for i, line := range report.TopWaste {
		fmt.Fprintf(&b, "%d. %s: %s\n", i+1, line.Label, dollars(line.EstimatedMonthlyWaste))
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
		for _, call := range group {
			sum += call.InputTokens
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
		Reason:                "The same workflow repeatedly sends large prompts. Miser assumes context can be compressed, cached, or split before model escalation.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func classificationWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
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
		Reason:                "Miser found repeated summary prompts after masking IDs and emails. These are strong candidates for exact or semantic caching.",
		Confidence:            "high",
		SampleCallIDs:         samples,
	}
}

func retryLoopWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		text := strings.ToLower(call.Workflow + " " + call.Prompt)
		if call.Metadata["retry"] != nil || call.Metadata["is_retry"] != nil || call.Metadata["attempt"] != nil || strings.Contains(text, "retry") || strings.Contains(text, "try again") || strings.Contains(text, "previous attempt failed") {
			total += call.CostUSD * 0.85
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Agent retry loops",
		EstimatedMonthlyWaste: total,
		Reason:                "Retry attempts are marked in metadata or prompt text. These usually need guardrails, tool-result caching, or deterministic failure handling.",
		Confidence:            "medium",
		SampleCallIDs:         samples,
	}
}

func oversizedPDFWaste(calls []LLMCall) WasteLine {
	total := 0.0
	var samples []string
	for _, call := range calls {
		text := strings.ToLower(call.Workflow + " " + call.Prompt)
		if strings.Contains(text, "pdf") && call.InputTokens >= 6000 {
			total += call.CostUSD * 0.60
			samples = appendSample(samples, call.ID)
		}
	}
	return WasteLine{
		Label:                 "Oversized PDF prompts",
		EstimatedMonthlyWaste: total,
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
