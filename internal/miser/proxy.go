package miser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ProxyOptions struct {
	Addr         string
	Provider     string
	Upstream     string
	APIKey       string
	LogPath      string
	CachePath    string
	AccountID    string
	Integration  string
	StorePrompts bool
}

type ProxyServer struct {
	opts     ProxyOptions
	upstream *url.URL
	client   *http.Client
	cache    *responseCache
	logger   *jsonlAppender
}

func NewProxyServer(opts ProxyOptions) (*ProxyServer, error) {
	if opts.Provider == "" {
		opts.Provider = "openai"
	}
	if opts.Upstream == "" {
		switch opts.Provider {
		case "openai":
			opts.Upstream = "https://api.openai.com"
		case "anthropic", "claude":
			opts.Provider = "anthropic"
			opts.Upstream = "https://api.anthropic.com"
		default:
			return nil, fmt.Errorf("unknown provider %q; use openai or anthropic", opts.Provider)
		}
	}
	if opts.LogPath == "" {
		opts.LogPath = ".miser/proxy-logs.jsonl"
	}
	parsed, err := url.Parse(opts.Upstream)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid upstream %q", opts.Upstream)
	}
	cache, err := loadResponseCache(opts.CachePath)
	if err != nil {
		return nil, err
	}
	logger, err := newJSONLAppender(opts.LogPath)
	if err != nil {
		return nil, err
	}
	return &ProxyServer{
		opts:     opts,
		upstream: parsed,
		client:   &http.Client{},
		cache:    cache,
		logger:   logger,
	}, nil
}

func (s *ProxyServer) Close() error {
	if s.logger == nil {
		return nil
	}
	return s.logger.Close()
}

func (s *ProxyServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/miser/api/requests", s.handleConsoleRequests)
	mux.HandleFunc("/", s.handle)
	return mux
}

func ServeProxy(opts ProxyOptions) error {
	server, err := NewProxyServer(opts)
	if err != nil {
		return err
	}
	defer server.Close()

	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:8788"
	}
	fmt.Printf("Miser proxy listening: http://%s\n", addr)
	fmt.Printf("Upstream: %s\n", server.upstream.String())
	fmt.Printf("Logs: %s\n", server.opts.LogPath)
	if server.cache != nil {
		fmt.Printf("Exact cache: %s\n", server.opts.CachePath)
	}
	return http.ListenAndServe(addr, server.Handler())
}

func (s *ProxyServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/miser") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, RenderConsoleHTML(ConsoleConfig{
			Provider:    s.opts.Provider,
			AccountID:   s.opts.AccountID,
			Integration: s.opts.Integration,
			LogPath:     s.opts.LogPath,
			CachePath:   s.opts.CachePath,
		}))
		return
	}
	start := time.Now()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	info := requestInfoFromBody(body)
	cacheable := isCacheableRequest(r, info)
	cacheKey := proxyCacheKey(r.Method, r.URL.RequestURI(), body)
	if cacheable && s.cache != nil {
		if cached, ok := s.cache.Get(cacheKey); ok {
			for key, values := range cached.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Header().Set("X-Miser-Cache", "HIT")
			w.WriteHeader(cached.StatusCode)
			_, _ = w.Write(cached.Body)
			s.logCall(r, info, cached.StatusCode, cached.Body, start, "hit", true)
			return
		}
	}

	upstreamReq, err := s.newUpstreamRequest(r, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		s.logProxyError(r, info, start, err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		s.logProxyError(r, info, start, err)
		return
	}

	copyResponseHeaders(w.Header(), resp.Header)
	if cacheable && s.cache != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.cache.Set(cacheKey, cachedResponse{StatusCode: resp.StatusCode, Header: responseHeaders(resp.Header), Body: respBody})
	}
	if cacheable {
		w.Header().Set("X-Miser-Cache", "MISS")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	s.logCall(r, info, resp.StatusCode, respBody, start, "miss", false)
}

func (s *ProxyServer) handleConsoleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := loadProxyLogRows(s.opts.LogPath, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(rows)
}

func (s *ProxyServer) newUpstreamRequest(original *http.Request, body []byte) (*http.Request, error) {
	target := *s.upstream
	target.Path = singleJoiningSlash(s.upstream.Path, original.URL.Path)
	target.RawQuery = original.URL.RawQuery
	req, err := http.NewRequestWithContext(original.Context(), original.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = original.Header.Clone()
	req.Host = s.upstream.Host
	if s.opts.APIKey != "" && req.Header.Get("Authorization") == "" {
		if s.opts.Provider == "anthropic" {
			req.Header.Set("x-api-key", s.opts.APIKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+s.opts.APIKey)
		}
	}
	req.Header.Set("X-Miser-Proxy", "1")
	return req, nil
}

func (s *ProxyServer) logCall(r *http.Request, info proxyRequestInfo, status int, responseBody []byte, start time.Time, cacheStatus string, cacheHit bool) {
	latency := int(time.Since(start).Milliseconds())
	usage := usageFromResponse(responseBody)
	inputTokens := usage.InputTokens
	outputTokens := usage.OutputTokens
	if inputTokens == 0 {
		inputTokens = estimateTokens(info.Prompt)
	}
	model := firstNonEmpty(usage.Model, info.Model, "unknown")
	cost := 0.0
	cacheSaved := 0.0
	costBasis := "unpriced_proxy_usage"
	if priced, pricing, ok := PriceTokenUsage(s.opts.Provider, model, inputTokens, outputTokens, usage.CachedInputTokens); ok {
		if cacheHit {
			cacheSaved = priced
		} else {
			cost = priced
			costBasis = "published_token_price"
		}
		_ = pricing
	}
	if cacheHit {
		costBasis = "miser_exact_cache"
	}

	prompt := fmt.Sprintf("%s proxy request path=%s model=%s fingerprint=%s", s.opts.Provider, r.URL.Path, model, info.Fingerprint)
	if s.opts.StorePrompts {
		prompt = info.Prompt
	}
	row := map[string]interface{}{
		"id":                   "miser_proxy_" + time.Now().UTC().Format("20060102T150405.000000000"),
		"timestamp":            time.Now().UTC().Format(time.RFC3339),
		"workflow":             proxyWorkflow(r.URL.Path),
		"provider":             s.opts.Provider,
		"model":                model,
		"prompt":               prompt,
		"input_tokens":         inputTokens,
		"output_tokens":        outputTokens,
		"cost_usd":             cost,
		"account_id":           s.opts.AccountID,
		"integration":          s.opts.Integration,
		"cost_basis":           costBasis,
		"latency_ms":           latency,
		"source":               "miser_proxy",
		"http_method":          r.Method,
		"http_path":            r.URL.Path,
		"http_status":          status,
		"cache_status":         cacheStatus,
		"cache_saved_usd":      cacheSaved,
		"request_fingerprint":  info.Fingerprint,
		"input_cached_tokens":  usage.CachedInputTokens,
		"prompt_chars":         len(info.Prompt),
		"miser_intercepted":    true,
		"miser_cache_eligible": isCacheableRequest(r, info),
	}
	_ = s.logger.Append(row)
}

func (s *ProxyServer) logProxyError(r *http.Request, info proxyRequestInfo, start time.Time, err error) {
	row := map[string]interface{}{
		"id":                  "miser_proxy_error_" + time.Now().UTC().Format("20060102T150405.000000000"),
		"timestamp":           time.Now().UTC().Format(time.RFC3339),
		"workflow":            proxyWorkflow(r.URL.Path),
		"provider":            s.opts.Provider,
		"model":               firstNonEmpty(info.Model, "unknown"),
		"prompt":              fmt.Sprintf("%s proxy error path=%s fingerprint=%s", s.opts.Provider, r.URL.Path, info.Fingerprint),
		"input_tokens":        estimateTokens(info.Prompt),
		"output_tokens":       0,
		"cost_usd":            0,
		"account_id":          s.opts.AccountID,
		"integration":         s.opts.Integration,
		"cost_basis":          "proxy_error",
		"latency_ms":          int(time.Since(start).Milliseconds()),
		"source":              "miser_proxy",
		"http_method":         r.Method,
		"http_path":           r.URL.Path,
		"request_fingerprint": info.Fingerprint,
		"error":               err.Error(),
	}
	_ = s.logger.Append(row)
}

type proxyRequestInfo struct {
	Model       string
	Prompt      string
	Stream      bool
	Fingerprint string
}

func requestInfoFromBody(body []byte) proxyRequestInfo {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)
	model, _ := payload["model"].(string)
	stream, _ := payload["stream"].(bool)
	prompt := extractPromptText(payload)
	fingerprint := FingerprintPrompt(prompt)
	if prompt == "" {
		fingerprint = shortSHA(body)
	}
	return proxyRequestInfo{Model: model, Prompt: prompt, Stream: stream, Fingerprint: fingerprint}
}

func extractPromptText(value interface{}) string {
	var parts []string
	collectText(value, &parts)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func collectText(value interface{}, parts *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			*parts = append(*parts, typed)
		}
	case []interface{}:
		for _, item := range typed {
			collectText(item, parts)
		}
	case map[string]interface{}:
		for _, key := range []string{"instructions", "input", "messages", "content", "text", "prompt"} {
			if child, ok := typed[key]; ok {
				collectText(child, parts)
			}
		}
	}
}

type proxyUsage struct {
	Model             string
	InputTokens       int
	OutputTokens      int
	CachedInputTokens int
}

func usageFromResponse(body []byte) proxyUsage {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return proxyUsage{}
	}
	usage, _ := payload["usage"].(map[string]interface{})
	model, _ := payload["model"].(string)
	input := firstJSONInt(usage, "prompt_tokens", "input_tokens")
	output := firstJSONInt(usage, "completion_tokens", "output_tokens")
	cached := nestedJSONInt(usage, "prompt_tokens_details", "cached_tokens")
	if cached == 0 {
		cached = nestedJSONInt(usage, "input_tokens_details", "cached_tokens")
	}
	return proxyUsage{Model: model, InputTokens: input, OutputTokens: output, CachedInputTokens: cached}
}

func firstJSONInt(values map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value := intFromAny(values[key]); value > 0 {
			return value
		}
	}
	return 0
}

func nestedJSONInt(values map[string]interface{}, objectKey, valueKey string) int {
	child, _ := values[objectKey].(map[string]interface{})
	return intFromAny(child[valueKey])
}

func isCacheableRequest(r *http.Request, info proxyRequestInfo) bool {
	if r.Method != http.MethodPost || info.Stream {
		return false
	}
	path := r.URL.Path
	return strings.HasSuffix(path, "/chat/completions") || strings.HasSuffix(path, "/responses")
}

func proxyWorkflow(path string) string {
	switch {
	case strings.Contains(path, "chat/completions"):
		return "proxy_chat_completion"
	case strings.Contains(path, "responses"):
		return "proxy_response"
	default:
		return "proxy_llm_request"
	}
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return len([]rune(text))/4 + 1
}

type cachedResponse struct {
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
}

type responseCache struct {
	path string
	mu   sync.Mutex
	Data map[string]cachedResponse `json:"data"`
}

func loadResponseCache(path string) (*responseCache, error) {
	if path == "" {
		return nil, nil
	}
	cache := &responseCache{path: path, Data: map[string]cachedResponse{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(raw, cache); err != nil {
		return nil, err
	}
	if cache.Data == nil {
		cache.Data = map[string]cachedResponse{}
	}
	return cache, nil
}

func (c *responseCache) Get(key string) (cachedResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, ok := c.Data[key]
	return resp, ok
}

func (c *responseCache) Set(key string, resp cachedResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Data[key] = resp
	_ = c.saveLocked()
}

func (c *responseCache) saveLocked() error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, raw, 0o644)
}

type jsonlAppender struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

func newJSONLAppender(path string) (*jsonlAppender, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	return &jsonlAppender{file: file, enc: enc}, nil
}

func (a *jsonlAppender) Append(row map[string]interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enc.Encode(row)
}

func (a *jsonlAppender) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}

func proxyCacheKey(method, uri string, body []byte) string {
	return shortSHA([]byte(method + "\n" + uri + "\n" + string(body)))
}

func shortSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func responseHeaders(header http.Header) http.Header {
	out := http.Header{}
	for key, values := range header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			out.Add(key, value)
		}
	}
	return out
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
