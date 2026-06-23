package miser

import "strings"

type ModelPricing struct {
	Provider               string
	Source                 string
	InputPerMillionTokens  float64
	CachedPerMillionTokens float64
	OutputPerMillionTokens float64
}

func PriceTokenUsage(provider, model string, inputTokens, outputTokens, cachedInputTokens int) (float64, ModelPricing, bool) {
	pricing, ok := PricingForModel(provider, model)
	if !ok {
		return 0, ModelPricing{}, false
	}
	uncachedInputTokens := inputTokens - cachedInputTokens
	if uncachedInputTokens < 0 {
		uncachedInputTokens = 0
	}
	cost := (float64(uncachedInputTokens)/1_000_000)*pricing.InputPerMillionTokens +
		(float64(cachedInputTokens)/1_000_000)*pricing.CachedPerMillionTokens +
		(float64(outputTokens)/1_000_000)*pricing.OutputPerMillionTokens
	return cost, pricing, true
}

func PricingForModel(provider, model string) (ModelPricing, bool) {
	provider = normalizeProvider(provider, model)
	name := strings.ToLower(model)
	switch provider {
	case "openai":
		return openAIModelPricing(name)
	case "anthropic":
		return anthropicModelPricing(name)
	default:
		return ModelPricing{}, false
	}
}

func normalizeProvider(provider, model string) string {
	value := strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(model)
	switch {
	case value == "openai" || strings.Contains(model, "gpt-") || strings.Contains(model, "o1") || strings.Contains(model, "o3") || strings.Contains(model, "o4"):
		return "openai"
	case value == "anthropic" || value == "claude" || strings.Contains(model, "claude"):
		return "anthropic"
	default:
		return value
	}
}

func openAIModelPricing(model string) (ModelPricing, bool) {
	switch {
	case strings.Contains(model, "gpt-5.5"):
		return publishedPricing("openai", 5.00, 0.50, 30.00), true
	case strings.Contains(model, "gpt-5.4-mini"):
		return publishedPricing("openai", 0.75, 0.075, 4.50), true
	case strings.Contains(model, "gpt-5.4"):
		return publishedPricing("openai", 2.50, 0.25, 15.00), true
	case strings.Contains(model, "gpt-4.1-mini"):
		return publishedPricing("openai", 0.40, 0.10, 1.60), true
	case strings.Contains(model, "gpt-4.1-nano"):
		return publishedPricing("openai", 0.10, 0.025, 0.40), true
	case strings.Contains(model, "gpt-4.1"):
		return publishedPricing("openai", 2.00, 0.50, 8.00), true
	case strings.Contains(model, "gpt-4o-mini"):
		return publishedPricing("openai", 0.15, 0.075, 0.60), true
	case strings.Contains(model, "gpt-4o"):
		return publishedPricing("openai", 2.50, 1.25, 10.00), true
	default:
		return ModelPricing{}, false
	}
}

func anthropicModelPricing(model string) (ModelPricing, bool) {
	switch {
	case strings.Contains(model, "opus-4.1") || strings.Contains(model, "opus 4.1") ||
		strings.Contains(model, "opus-4") || strings.Contains(model, "opus 4") ||
		strings.Contains(model, "claude-3-opus"):
		return anthropicPricing(15.00, 75.00), true
	case strings.Contains(model, "sonnet-4.5") || strings.Contains(model, "sonnet 4.5") ||
		strings.Contains(model, "sonnet-4") || strings.Contains(model, "sonnet 4") ||
		strings.Contains(model, "claude-3-7-sonnet") ||
		strings.Contains(model, "claude-3.7-sonnet") ||
		strings.Contains(model, "claude-3-5-sonnet") ||
		strings.Contains(model, "claude-3.5-sonnet"):
		return anthropicPricing(3.00, 15.00), true
	case strings.Contains(model, "haiku-4.5") || strings.Contains(model, "haiku 4.5"):
		return anthropicPricing(1.00, 5.00), true
	case strings.Contains(model, "haiku-3.5") || strings.Contains(model, "haiku 3.5") ||
		strings.Contains(model, "claude-3-5-haiku") ||
		strings.Contains(model, "claude-3.5-haiku"):
		return anthropicPricing(0.80, 4.00), true
	case strings.Contains(model, "claude-3-haiku"):
		return anthropicPricing(0.25, 1.25), true
	default:
		return ModelPricing{}, false
	}
}

func publishedPricing(provider string, input, cached, output float64) ModelPricing {
	return ModelPricing{
		Provider:               provider,
		Source:                 provider + "_public_pricing",
		InputPerMillionTokens:  input,
		CachedPerMillionTokens: cached,
		OutputPerMillionTokens: output,
	}
}

func anthropicPricing(input, output float64) ModelPricing {
	return ModelPricing{
		Provider:               "anthropic",
		Source:                 "anthropic_public_pricing",
		InputPerMillionTokens:  input,
		CachedPerMillionTokens: input * 0.10,
		OutputPerMillionTokens: output,
	}
}
