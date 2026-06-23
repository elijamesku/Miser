package miser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ImportOptions struct {
	AccountID   string
	Integration string
}

func ImportCCUsage(path string, opts ImportOptions) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	usageRows := ccusageRows(payload)

	out := make([]map[string]interface{}, 0, len(usageRows))
	for i, row := range usageRows {
		inputTokens := firstInt(row, "inputTokens", "input_tokens", "promptTokens", "prompt_tokens", "cacheCreationInputTokens", "cache_creation_input_tokens")
		outputTokens := firstInt(row, "outputTokens", "output_tokens", "completionTokens", "completion_tokens")
		cacheReadTokens := firstInt(row, "cacheReadTokens", "cacheReadInputTokens", "cache_read_tokens", "cache_read_input_tokens")
		totalTokens := firstInt(row, "totalTokens", "total_tokens", "tokens")
		if totalTokens > 0 && inputTokens == 0 && outputTokens == 0 {
			inputTokens = int(float64(totalTokens) * 0.8)
			outputTokens = totalTokens - inputTokens
		}
		project := firstString(row, "project", "projectName", "instance", "sessionId", "session_id")
		if project == "" {
			project = "unknown"
		}
		label := firstString(row, "period", "date", "month", "week", "sessionId", "session_id")
		if label == "" {
			label = fmt.Sprintf("row_%d", i+1)
		}

		integration := defaultString(opts.Integration, "ccusage")
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
			"account_id":        opts.AccountID,
			"integration":       integration,
			"cost_basis":        "estimated_token_cost",
			"source":            "ccusage",
			"project":           project,
			"report_label":      label,
			"cache_read_tokens": cacheReadTokens,
			"raw":               row,
		})
	}
	return out, nil
}

func ImportInvoiceCSV(path string, opts ImportOptions) ([]map[string]interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("invoice csv needs a header row and at least one data row")
	}

	header := map[string]int{}
	for i, name := range records[0] {
		header[normalizeColumn(name)] = i
	}

	out := make([]map[string]interface{}, 0, len(records)-1)
	for i, record := range records[1:] {
		cost, err := invoiceCost(record, header)
		if err != nil {
			return nil, fmt.Errorf("invoice row %d: %w", i+2, err)
		}
		date := invoiceString(record, header, "date", "timestamp", "period", "month")
		if date == "" {
			return nil, fmt.Errorf("invoice row %d: missing date/timestamp/period/month", i+2)
		}
		provider := defaultString(invoiceString(record, header, "provider", "vendor"), opts.Integration)
		if provider == "" {
			provider = "unknown"
		}
		description := defaultString(invoiceString(record, header, "description", "item", "memo"), "Billing export row")
		accountID := defaultString(opts.AccountID, invoiceString(record, header, "account", "account_id", "workspace", "organization"))
		integration := defaultString(opts.Integration, provider)
		out = append(out, map[string]interface{}{
			"id":            fmt.Sprintf("invoice_%04d", i+1),
			"timestamp":     normalizeTimestamp(date),
			"workflow":      "billing_invoice",
			"provider":      provider,
			"model":         defaultString(invoiceString(record, header, "model"), "n/a"),
			"prompt":        description,
			"input_tokens":  0,
			"output_tokens": 0,
			"cost_usd":      cost,
			"account_id":    accountID,
			"integration":   integration,
			"cost_basis":    "actual_invoice",
			"source":        "invoice_csv",
		})
	}
	return out, nil
}

func invoiceCost(record []string, header map[string]int) (float64, error) {
	raw := invoiceString(record, header, "cost_usd", "cost", "amount", "total", "charge")
	if raw == "" {
		return 0, fmt.Errorf("missing cost_usd/cost/amount/total/charge")
	}
	cleaned := strings.TrimSpace(strings.ReplaceAll(raw, "$", ""))
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func invoiceString(record []string, header map[string]int, names ...string) string {
	for _, name := range names {
		index, ok := header[normalizeColumn(name)]
		if ok && index < len(record) {
			return strings.TrimSpace(record[index])
		}
	}
	return ""
}

func normalizeColumn(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func normalizeTimestamp(value string) string {
	if len(value) == 7 {
		value += "-01"
	}
	if ts, err := parseTime(value); err == nil {
		return ts.UTC().Format(time.RFC3339)
	}
	return value
}

func ccusageRows(payload interface{}) []map[string]interface{} {
	root, ok := payload.(map[string]interface{})
	if !ok {
		var rows []map[string]interface{}
		findUsageRows(payload, &rows)
		return rows
	}
	for _, key := range []string{"daily", "monthly", "sessions"} {
		if rows := usageRowsFromArray(root[key]); len(rows) > 0 {
			return rows
		}
	}
	var rows []map[string]interface{}
	findUsageRows(payload, &rows)
	return rows
}

func usageRowsFromArray(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	rows := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if ok && looksLikeUsageRow(row) {
			rows = append(rows, row)
		}
	}
	return rows
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
	raw := firstString(row, "timestamp", "period", "date", "month", "week", "startTime", "startedAt")
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
