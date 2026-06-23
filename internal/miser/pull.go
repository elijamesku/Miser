package miser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
				"id":           fmt.Sprintf("openai_cost_%04d", rowNumber),
				"timestamp":    timestamp,
				"workflow":     "provider_billing",
				"provider":     "openai",
				"model":        defaultString(result.LineItem, "n/a"),
				"prompt":       label,
				"input_tokens": 0,
				"output_tokens": 0,
				"cost_usd":     result.Amount.Value,
				"account_id":   accountID,
				"integration":  integration,
				"cost_basis":   "provider_billing_api",
				"currency":     defaultString(result.Amount.Currency, "usd"),
				"line_item":    result.LineItem,
				"project_id":   result.ProjectID,
				"api_key_id":   result.APIKeyID,
				"source":       "openai_costs_api",
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
