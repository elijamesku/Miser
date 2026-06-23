package miser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PullOptions struct {
	AccountID   string
	Integration string
	From        time.Time
	To          time.Time
	APIKey      string
}

func PullOpenAICosts(opts PullOptions) ([]map[string]interface{}, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("missing OpenAI admin API key")
	}
	if opts.From.IsZero() || opts.To.IsZero() {
		return nil, fmt.Errorf("from and to dates are required")
	}
	endpoint, err := openAICostsURL(opts.From, opts.To)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+opts.APIKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("openai costs request failed: %s: %s", response.Status, string(body))
	}

	var payload openAICostsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	integration := defaultString(opts.Integration, "codex")
	accountID := defaultString(opts.AccountID, "openai")
	var rows []map[string]interface{}
	rowNumber := 1
	for _, bucket := range payload.Data {
		timestamp := time.Unix(bucket.StartTime, 0).UTC().Format(time.RFC3339)
		for _, result := range bucket.Results {
			if result.Amount == nil || result.Amount.Value == 0 {
				continue
			}
			label := defaultString(result.LineItem, "OpenAI organization costs")
			if result.ProjectID != "" {
				label += " project=" + result.ProjectID
			}
			if result.APIKeyID != "" {
				label += " api_key=" + result.APIKeyID
			}
			rows = append(rows, map[string]interface{}{
				"id":            fmt.Sprintf("openai_cost_%04d", rowNumber),
				"timestamp":     timestamp,
				"workflow":      "provider_billing",
				"provider":      "openai",
				"model":         defaultString(result.LineItem, "n/a"),
				"prompt":        label,
				"input_tokens":  0,
				"output_tokens": 0,
				"cost_usd":      result.Amount.Value,
				"account_id":    accountID,
				"integration":   integration,
				"cost_basis":    "provider_billing_api",
				"currency":      defaultString(result.Amount.Currency, "usd"),
				"line_item":     result.LineItem,
				"project_id":    result.ProjectID,
				"api_key_id":    result.APIKeyID,
				"source":        "openai_costs_api",
			})
			rowNumber++
		}
	}
	return rows, nil
}

func PullOpenAIUsage(opts PullOptions) ([]map[string]interface{}, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("missing OpenAI admin API key")
	}
	if opts.From.IsZero() || opts.To.IsZero() {
		return nil, fmt.Errorf("from and to dates are required")
	}
	endpoint, err := openAIUsageURL(opts.From, opts.To)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+opts.APIKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("openai usage request failed: %s: %s", response.Status, string(body))
	}

	var payload openAIUsageResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	integration := defaultString(opts.Integration, "codex")
	accountID := defaultString(opts.AccountID, "openai")
	var rows []map[string]interface{}
	rowNumber := 1
	for _, bucket := range payload.Data {
		timestamp := time.Unix(bucket.StartTime, 0).UTC().Format(time.RFC3339)
		for _, result := range bucket.Results {
			if result.InputTokens == 0 && result.OutputTokens == 0 && result.NumModelRequests == 0 {
				continue
			}
			model := defaultString(result.Model, "unknown")
			costUSD, priced := estimateOpenAICostUSD(model, result.InputTokens, result.OutputTokens, result.InputCachedTokens)
			costBasis := "unpriced_token_usage"
			pricingSource := "none"
			if priced {
				costBasis = "published_token_price"
				pricingSource = "openai_public_pricing"
			}
			rows = append(rows, map[string]interface{}{
				"id":                  fmt.Sprintf("openai_usage_%04d", rowNumber),
				"timestamp":           timestamp,
				"workflow":            "openai_api_usage",
				"provider":            "openai",
				"model":               model,
				"prompt":              usagePrompt(model, result),
				"input_tokens":        result.InputTokens,
				"output_tokens":       result.OutputTokens,
				"cost_usd":            costUSD,
				"account_id":          accountID,
				"integration":         integration,
				"cost_basis":          costBasis,
				"pricing_source":      pricingSource,
				"source":              "openai_usage_api",
				"project_id":          result.ProjectID,
				"user_id":             result.UserID,
				"api_key_id":          result.APIKeyID,
				"batch":               result.Batch,
				"num_model_requests":  result.NumModelRequests,
				"input_cached_tokens": result.InputCachedTokens,
			})
			rowNumber++
		}
	}
	return rows, nil
}

func openAICostsURL(from, to time.Time) (string, error) {
	if !from.Before(to) {
		return "", fmt.Errorf("from date must be before to date")
	}
	values := url.Values{}
	values.Set("start_time", strconv.FormatInt(from.Unix(), 10))
	values.Set("end_time", strconv.FormatInt(to.Unix(), 10))
	values.Set("bucket_width", "1d")
	values.Add("group_by", "line_item")
	values.Add("group_by", "project_id")
	return "https://api.openai.com/v1/organization/costs?" + values.Encode(), nil
}

func openAIUsageURL(from, to time.Time) (string, error) {
	if !from.Before(to) {
		return "", fmt.Errorf("from date must be before to date")
	}
	values := url.Values{}
	values.Set("start_time", strconv.FormatInt(from.Unix(), 10))
	values.Set("end_time", strconv.FormatInt(to.Unix(), 10))
	values.Set("bucket_width", "1d")
	values.Add("group_by", "model")
	values.Add("group_by", "project_id")
	values.Add("group_by", "api_key_id")
	return "https://api.openai.com/v1/organization/usage/completions?" + values.Encode(), nil
}

func usagePrompt(model string, result openAIUsageResult) string {
	parts := []string{"OpenAI API usage", "model=" + model}
	if result.ProjectID != "" {
		parts = append(parts, "project="+result.ProjectID)
	}
	if result.APIKeyID != "" {
		parts = append(parts, "api_key="+result.APIKeyID)
	}
	if result.NumModelRequests > 0 {
		parts = append(parts, fmt.Sprintf("requests=%d", result.NumModelRequests))
	}
	return strings.Join(parts, " ")
}

func estimateOpenAICostUSD(model string, inputTokens, outputTokens, cachedInputTokens int) (float64, bool) {
	inputPerMillion, cachedPerMillion, outputPerMillion, ok := openAIModelRates(model)
	if !ok {
		return 0, false
	}
	uncachedInputTokens := inputTokens - cachedInputTokens
	if uncachedInputTokens < 0 {
		uncachedInputTokens = 0
	}
	return (float64(uncachedInputTokens)/1_000_000)*inputPerMillion +
		(float64(cachedInputTokens)/1_000_000)*cachedPerMillion +
		(float64(outputTokens)/1_000_000)*outputPerMillion, true
}

func openAIModelRates(model string) (float64, float64, float64, bool) {
	name := strings.ToLower(model)
	switch {
	case strings.Contains(name, "gpt-5.5"):
		return 5.00, 0.50, 30.00, true
	case strings.Contains(name, "gpt-5.4-mini"):
		return 0.75, 0.075, 4.50, true
	case strings.Contains(name, "gpt-5.4"):
		return 2.50, 0.25, 15.00, true
	case strings.Contains(name, "gpt-4.1-mini"):
		return 0.40, 0.10, 1.60, true
	case strings.Contains(name, "gpt-4.1-nano"):
		return 0.10, 0.025, 0.40, true
	case strings.Contains(name, "gpt-4.1"):
		return 2.00, 0.50, 8.00, true
	case strings.Contains(name, "gpt-4o-mini"):
		return 0.15, 0.075, 0.60, true
	case strings.Contains(name, "gpt-4o"):
		return 2.50, 1.25, 10.00, true
	default:
		return 0, 0, 0, false
	}
}

type openAICostsResponse struct {
	Data []openAICostBucket `json:"data"`
}

type openAICostBucket struct {
	StartTime int64               `json:"start_time"`
	EndTime   int64               `json:"end_time"`
	Results   []openAICostsResult `json:"results"`
}

type openAICostsResult struct {
	Amount    *openAIAmount `json:"amount"`
	LineItem  string        `json:"line_item"`
	ProjectID string        `json:"project_id"`
	APIKeyID  string        `json:"api_key_id"`
}

type openAIAmount struct {
	Currency string  `json:"currency"`
	Value    float64 `json:"value"`
}

type openAIUsageResponse struct {
	Data []openAIUsageBucket `json:"data"`
}

type openAIUsageBucket struct {
	StartTime int64               `json:"start_time"`
	EndTime   int64               `json:"end_time"`
	Results   []openAIUsageResult `json:"results"`
}

type openAIUsageResult struct {
	InputTokens       int    `json:"input_tokens"`
	OutputTokens      int    `json:"output_tokens"`
	InputCachedTokens int    `json:"input_cached_tokens"`
	NumModelRequests  int    `json:"num_model_requests"`
	Model             string `json:"model"`
	ProjectID         string `json:"project_id"`
	UserID            string `json:"user_id"`
	APIKeyID          string `json:"api_key_id"`
	Batch             string `json:"batch"`
}
