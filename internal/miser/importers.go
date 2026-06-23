package miser

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func ImportCCUsage(path string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	var usageRows []map[string]interface{}
	findUsageRows(payload, &usageRows)

	out := make([]map[string]interface{}, 0, len(usageRows))
	for i, row := range usageRows {
		inputTokens := firstInt(row, "inputTokens", "input_tokens", "promptTokens", "prompt_tokens", "cacheCreationInputTokens", "cache_creation_input_tokens")
		outputTokens := firstInt(row, "outputTokens", "output_tokens", "completionTokens", "completion_tokens")
		cacheReadTokens := firstInt(row, "cacheReadInputTokens", "cache_read_input_tokens")
		totalTokens := firstInt(row, "totalTokens", "total_tokens", "tokens")
		if totalTokens > 0 && inputTokens == 0 && outputTokens == 0 {
			inputTokens = int(float64(totalTokens) * 0.8)
			outputTokens = totalTokens - inputTokens
		}
		project := firstString(row, "project", "projectName", "instance", "sessionId", "session_id")
		if project == "" {
			project = "unknown"
		}
		label := firstString(row, "date", "month", "week", "sessionId", "session_id")
		if label == "" {
			label = fmt.Sprintf("row_%d", i+1)
		}

		out = append(out, map[string]interface{}{
			"id":                fmt.Sprintf("ccusage_%04d", i+1),
			"timestamp":         timestamp(row),
			"workflow":          "coding_agent_usage",
			"provider":          defaultString(firstString(row, "source", "provider", "cli"), "ccusage"),
			"model":             defaultString(firstString(row, "model", "modelName", "model_name"), "unknown"),
			"prompt":            fmt.Sprintf("Coding agent usage aggregate for %s %s.", project, label),
			"input_tokens":      inputTokens + cacheReadTokens,
			"output_tokens":     outputTokens,
			"cost_usd":          firstFloat(row, "totalCost", "costUSD", "cost_usd", "cost", "total_cost"),
			"source":            "ccusage",
			"project":           project,
			"report_label":      label,
			"cache_read_tokens": cacheReadTokens,
			"raw":               row,
		})
	}
	return out, nil
}

func findUsageRows(value interface{}, rows *[]map[string]interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			findUsageRows(item, rows)
		}
	case map[string]interface{}:
		if looksLikeUsageRow(typed) {
			*rows = append(*rows, typed)
		}
		for _, child := range typed {
			findUsageRows(child, rows)
		}
	}
}

func looksLikeUsageRow(row map[string]interface{}) bool {
	tokenKeys := []string{"inputTokens", "input_tokens", "promptTokens", "prompt_tokens", "outputTokens", "output_tokens", "completionTokens", "completion_tokens", "totalTokens", "total_tokens", "tokens"}
	costKeys := []string{"totalCost", "costUSD", "cost_usd", "cost", "total_cost"}
	return hasAny(row, tokenKeys...) && hasAny(row, costKeys...)
}

func hasAny(row map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := row[key]; ok {
			return true
		}
	}
	return false
}

func firstInt(row map[string]interface{}, keys ...string) int {
	return int(firstFloat(row, keys...))
}

func firstFloat(row map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		switch value := row[key].(type) {
		case float64:
			return value
		case int:
			return float64(value)
		}
	}
	return 0
}

func firstString(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func timestamp(row map[string]interface{}) string {
	raw := firstString(row, "timestamp", "date", "month", "week", "startTime", "startedAt")
	if raw == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	if len(raw) == 7 {
		raw += "-01"
	}
	if ts, err := parseTime(raw); err == nil {
		return ts.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
