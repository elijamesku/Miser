package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/elijamesku/Miser/internal/miser"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "audit":
		err = runAudit(os.Args[2:])
	case "analyze":
		err = runAnalyze(os.Args[2:])
	case "plan":
		err = runPlan(os.Args[2:])
	case "rules":
		err = runRules(os.Args[2:])
	case "apply":
		err = runApply(os.Args[2:])
	case "preview":
		err = runPreview(os.Args[2:])
	case "proxy":
		err = runProxy(os.Args[2:])
	case "reconcile":
		err = runReconcile(os.Args[2:])
	case "import":
		err = runImport(os.Args[2:])
	case "pull":
		err = runPull(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "miser:", err)
		os.Exit(1)
	}
}

func runAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	explain := fs.Bool("explain", false, "show evidence and reasoning for each waste bucket")
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	account := fs.String("account", "", "only audit one account_id")
	integration := fs.String("integration", "", "only audit one integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser audit [--explain] [--json] [--account id] [--integration name] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	report := miser.Audit(calls)
	if *jsonOut {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}
	fmt.Print(miser.RenderAudit(report, *explain))
	return nil
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	minClusterSize := fs.Int("min-cluster-size", 3, "minimum repeated calls per cluster")
	minMonthlySavings := fs.Float64("min-monthly-savings", 1, "minimum estimated monthly savings")
	minQualityScore := fs.Float64("min-quality-score", 0.95, "minimum replay eval quality guard")
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	routesPath := fs.String("routes", "", "write reviewable route config")
	account := fs.String("account", "", "only analyze one account_id")
	integration := fs.String("integration", "", "only analyze one integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser analyze [options] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	receipts := miser.Analyze(calls, miser.AnalyzerConfig{
		MinClusterSize:       *minClusterSize,
		MinMonthlySavingsUSD: *minMonthlySavings,
		MinQualityScore:      *minQualityScore,
	})
	if *jsonOut {
		encoded, err := json.MarshalIndent(receipts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
	} else {
		fmt.Print(miser.RenderReceipts(receipts))
	}
	if *routesPath != "" {
		return os.WriteFile(*routesPath, []byte(miser.RenderRoutes(receipts)), 0o644)
	}
	return nil
}

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	outPath := fs.String("out", "", "write executable savings plan YAML")
	account := fs.String("account", "", "only plan for one account_id")
	integration := fs.String("integration", "", "only plan for one integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser plan [--json] [--out miser-plan.yaml] [--account id] [--integration name] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	plan := miser.Plan(calls)
	if *jsonOut {
		encoded, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(miser.RenderPlanYAML(plan)), 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote savings plan to %s\n", *outPath)
		return nil
	}
	fmt.Print(miser.RenderPlan(plan))
	return nil
}

func runRules(args []string) error {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	outPath := fs.String("out", "", "write agent rule pack YAML")
	instructionsPath := fs.String("instructions", "", "write human-readable agent instructions")
	target := fs.String("target", "generic", "agent target: generic, codex, or claude")
	account := fs.String("account", "", "only generate rules for one account_id")
	integration := fs.String("integration", "", "only generate rules for one integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser rules [--json] [--out rules.yaml] [--instructions AGENT_RULES.md] [--target generic|codex|claude] [--account id] [--integration name] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	pack := miser.Rules(calls, *target)
	if *jsonOut {
		encoded, err := json.MarshalIndent(pack, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}
	if *outPath != "" {
		if err := writeTextFile(*outPath, miser.RenderRulesYAML(pack)); err != nil {
			return err
		}
		fmt.Printf("Wrote agent rule pack to %s\n", *outPath)
	}
	if *instructionsPath != "" {
		if err := writeTextFile(*instructionsPath, miser.RenderAgentInstructions(pack)); err != nil {
			return err
		}
		fmt.Printf("Wrote agent instructions to %s\n", *instructionsPath)
	}
	if *outPath == "" && *instructionsPath == "" {
		fmt.Print(miser.RenderRulesYAML(pack))
	}
	return nil
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	target := fs.String("target", "", "agent target: generic, codex, or claude")
	outDir := fs.String("out-dir", "", "directory for generated integration files")
	jsonOut := fs.Bool("json", false, "emit generated file list as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser apply [--target generic|codex|claude] [--out-dir .miser] rules.yaml")
	}

	rulesPath := fs.Arg(0)
	raw, err := os.ReadFile(rulesPath)
	if err != nil {
		return err
	}
	outputDir := *outDir
	if outputDir == "" {
		outputDir = filepath.Dir(rulesPath)
		if outputDir == "." || outputDir == "" {
			outputDir = ".miser"
		}
	}
	files := miser.ApplyRules(raw, *target, outputDir)
	if *jsonOut {
		encoded, err := json.MarshalIndent(files, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}
	for _, file := range files {
		if err := writeTextFile(file.Path, file.Content); err != nil {
			return err
		}
	}
	fmt.Print(miser.RenderApplySummary(files))
	return nil
}

func runPreview(args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	account := fs.String("account", "", "only preview one account_id")
	integration := fs.String("integration", "", "only preview one integration")
	addr := fs.String("addr", "127.0.0.1:8787", "address for the local web preview")
	outPath := fs.String("out", "", "write static HTML instead of serving")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser preview [--addr 127.0.0.1:8787] [--out preview.html] [--account id] [--integration name] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	html := miser.RenderPreviewHTML(calls, *account, *integration)
	if *outPath != "" {
		if err := writeTextFile(*outPath, html); err != nil {
			return err
		}
		fmt.Printf("Wrote web preview to %s\n", *outPath)
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	fmt.Printf("Miser web preview: http://%s\n", *addr)
	return http.ListenAndServe(*addr, mux)
}

func runProxy(args []string) error {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	provider := fs.String("provider", "openai", "provider to proxy: openai or anthropic")
	addr := fs.String("addr", "127.0.0.1:8788", "address for the local interception proxy")
	upstream := fs.String("upstream", "", "provider upstream URL")
	apiKeyEnv := fs.String("api-key-env", "OPENAI_API_KEY", "environment variable containing provider API key")
	logPath := fs.String("log", ".miser/proxy-logs.jsonl", "append intercepted calls to this JSONL file")
	cachePath := fs.String("cache", ".miser/exact-cache.json", "persistent exact response cache; empty disables cache")
	account := fs.String("account", "", "account_id to attach to intercepted rows")
	integration := fs.String("integration", "", "integration name to attach to intercepted rows")
	storePrompts := fs.Bool("store-prompts", false, "store full prompt text in proxy logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: miser proxy [--provider openai|anthropic] [--addr 127.0.0.1:8788] [--upstream url] [--api-key-env OPENAI_API_KEY] [--log .miser/proxy-logs.jsonl] [--cache .miser/exact-cache.json] [--account id] [--integration name] [--store-prompts]")
	}
	apiKey := os.Getenv(*apiKeyEnv)
	if apiKey == "" {
		return fmt.Errorf("missing API key in %s", *apiKeyEnv)
	}
	return miser.ServeProxy(miser.ProxyOptions{
		Addr:         *addr,
		Provider:     *provider,
		Upstream:     *upstream,
		APIKey:       apiKey,
		LogPath:      *logPath,
		CachePath:    *cachePath,
		AccountID:    *account,
		Integration:  *integration,
		StorePrompts: *storePrompts,
	})
}

func runReconcile(args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	actualSpend := fs.Float64("actual-spend", 0, "actual invoice spend to allocate across usage rows")
	invoiceCSV := fs.String("invoice-csv", "", "invoice CSV to sum and allocate across usage rows")
	out := fs.String("out", "", "write reconciled usage rows to JSONL")
	account := fs.String("account", "", "only reconcile one account_id")
	integration := fs.String("integration", "", "only reconcile one integration")
	args = reorderReconcileArgs(args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *out == "" {
		return fmt.Errorf("usage: miser reconcile [--actual-spend usd | --invoice-csv invoice.csv] --out actual_usage.jsonl [--account id] [--integration name] usage.jsonl")
	}
	if *actualSpend > 0 && *invoiceCSV != "" {
		return fmt.Errorf("use either --actual-spend or --invoice-csv, not both")
	}

	total := *actualSpend
	var err error
	if *invoiceCSV != "" {
		total, err = invoiceCSVTotal(*invoiceCSV, *account, *integration)
		if err != nil {
			return err
		}
	}
	if total <= 0 {
		return fmt.Errorf("actual invoice spend is required; pass --actual-spend or --invoice-csv")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
	calls = miser.FilterCalls(calls, miser.FilterConfig{AccountID: *account, Integration: *integration})
	rows, err := miser.ReconcileToActualSpend(calls, total)
	if err != nil {
		return err
	}
	if err := miser.WriteJSONL(rows, *out); err != nil {
		return err
	}
	fmt.Printf("Reconciled %d usage rows to actual invoice spend $%.2f into %s\n", len(rows), total, *out)
	return nil
}

func reorderReconcileArgs(args []string) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) > 0 && arg[0] == '-' {
			flags = append(flags, arg)
			if !hasInlineFlagValue(arg) && i+1 < len(args) && len(args[i+1]) > 0 && args[i+1][0] != '-' {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, arg)
	}
	return append(flags, positional...)
}

func hasInlineFlagValue(arg string) bool {
	for _, ch := range arg {
		if ch == '=' {
			return true
		}
	}
	return false
}

func runImport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: miser import <ccusage|invoice-csv> input --out logs.jsonl [--account id] [--integration name]")
	}
	kind := args[0]
	path, out, opts, err := parseImportArgs(args[1:])
	if err != nil {
		return err
	}
	if path == "" || out == "" {
		return fmt.Errorf("usage: miser import <ccusage|invoice-csv> input --out logs.jsonl [--account id] [--integration name]")
	}

	var rows []map[string]interface{}
	switch kind {
	case "ccusage":
		rows, err = miser.ImportCCUsage(path, opts)
	case "invoice-csv":
		rows, err = miser.ImportInvoiceCSV(path, opts)
	default:
		return fmt.Errorf("unknown import %q; expected ccusage or invoice-csv", kind)
	}
	if err != nil {
		return err
	}
	if err := miser.WriteJSONL(rows, out); err != nil {
		return err
	}
	fmt.Printf("Imported %d %s rows into %s\n", len(rows), kind, out)
	return nil
}

func runPull(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: miser pull <openai|openai-usage|claude> --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration name]")
	}
	provider := args[0]
	fs := flag.NewFlagSet("pull "+provider, flag.ContinueOnError)
	fromRaw := fs.String("from", "", "start date, YYYY-MM-DD")
	toRaw := fs.String("to", "", "end date, YYYY-MM-DD")
	out := fs.String("out", "", "write pulled billing rows to JSONL")
	account := fs.String("account", "", "account_id to attach to imported rows")
	integration := fs.String("integration", "", "integration name to attach to imported rows")
	apiKeyEnv := fs.String("api-key-env", "OPENAI_ADMIN_KEY", "environment variable containing provider admin API key")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *fromRaw == "" || *toRaw == "" || *out == "" {
		return fmt.Errorf("usage: miser pull <openai|openai-usage|claude> --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration name]")
	}
	from, err := parseDate(*fromRaw)
	if err != nil {
		return err
	}
	to, err := parseDate(*toRaw)
	if err != nil {
		return err
	}

	var rows []map[string]interface{}
	switch provider {
	case "openai":
		rows, err = miser.PullOpenAICosts(miser.PullOptions{
			AccountID:   *account,
			Integration: defaultString(*integration, "codex"),
			From:        from,
			To:          to,
			APIKey:      os.Getenv(*apiKeyEnv),
		})
	case "openai-usage":
		rows, err = miser.PullOpenAIUsage(miser.PullOptions{
			AccountID:   *account,
			Integration: defaultString(*integration, "codex"),
			From:        from,
			To:          to,
			APIKey:      os.Getenv(*apiKeyEnv),
		})
	case "claude":
		return fmt.Errorf("claude billing pull is not available yet; import a Claude invoice/billing CSV with `miser import invoice-csv ... --integration claude`")
	default:
		return fmt.Errorf("unknown pull provider %q; expected openai, openai-usage, or claude", provider)
	}
	if err != nil {
		return err
	}
	if err := miser.WriteJSONL(rows, *out); err != nil {
		return err
	}
	fmt.Printf("Pulled %d %s rows into %s\n", len(rows), provider, *out)
	return nil
}

func invoiceCSVTotal(path, account, integration string) (float64, error) {
	rows, err := miser.ImportInvoiceCSV(path, miser.ImportOptions{})
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, row := range rows {
		if account != "" && row["account_id"] != account {
			continue
		}
		if integration != "" && row["integration"] != integration {
			continue
		}
		cost, ok := row["cost_usd"].(float64)
		if ok {
			total += cost
		}
	}
	if total == 0 {
		return 0, fmt.Errorf("invoice CSV had no matching spend")
	}
	return total, nil
}

func parseDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q; use YYYY-MM-DD", value)
	}
	return parsed.UTC(), nil
}

func parseImportArgs(args []string) (string, string, miser.ImportOptions, error) {
	var path string
	var out string
	var opts miser.ImportOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return "", "", opts, fmt.Errorf("--out requires a path")
			}
			out = args[i+1]
			i++
		case "--account":
			if i+1 >= len(args) {
				return "", "", opts, fmt.Errorf("--account requires an id")
			}
			opts.AccountID = args[i+1]
			i++
		case "--integration":
			if i+1 >= len(args) {
				return "", "", opts, fmt.Errorf("--integration requires a name")
			}
			opts.Integration = args[i+1]
			i++
		default:
			if path != "" {
				return "", "", opts, fmt.Errorf("unexpected argument %q", args[i])
			}
			path = args[i]
		}
	}
	return path, out, opts, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  miser audit [--explain] [--json] [--account id] [--integration name] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser analyze [--json] [--routes routes.yaml] [--account id] [--integration name] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser plan [--json] [--out miser-plan.yaml] [--account id] [--integration name] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser rules [--json] [--out rules.yaml] [--instructions AGENT_RULES.md] [--target generic|codex|claude] [--account id] [--integration name] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser apply [--target generic|codex|claude] [--out-dir .miser] rules.yaml")
	fmt.Fprintln(os.Stderr, "  miser preview [--addr 127.0.0.1:8787] [--out preview.html] [--account id] [--integration name] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser proxy [--provider openai|anthropic] [--addr 127.0.0.1:8788] [--log .miser/proxy-logs.jsonl] [--cache .miser/exact-cache.json] [--account id] [--integration name]")
	fmt.Fprintln(os.Stderr, "  miser reconcile [--actual-spend usd | --invoice-csv invoice.csv] --out actual_usage.jsonl [--account id] [--integration name] usage.jsonl")
	fmt.Fprintln(os.Stderr, "  miser import ccusage ccusage.json --out logs.jsonl [--account id] [--integration claude|codex]")
	fmt.Fprintln(os.Stderr, "  miser import invoice-csv invoice.csv --out logs.jsonl [--account id] [--integration claude|codex]")
	fmt.Fprintln(os.Stderr, "  miser pull openai --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration codex]")
	fmt.Fprintln(os.Stderr, "  miser pull openai-usage --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration codex]")
}

func writeTextFile(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
