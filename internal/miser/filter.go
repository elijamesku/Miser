//miser filter - line up docs

package miser

type FilterConfig struct {
	AccountID   string
	Integration string
}

func FilterCalls(calls []LLMCall, config FilterConfig) []LLMCall {
	if config.AccountID == "" && config.Integration == "" {
		return calls
	}
	filtered := make([]LLMCall, 0, len(calls))
	for _, call := range calls {
		if config.AccountID != "" && call.AccountID != config.AccountID {
			continue
		}
		if config.Integration != "" && call.Integration != config.Integration {
			continue
		}
		filtered = append(filtered, call)
	}
	return filtered
}
