package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	fmt.Fprintln(os.Stderr, "  miser import ccusage ccusage.json --out logs.jsonl [--account id] [--integration claude|codex]")
	fmt.Fprintln(os.Stderr, "  miser import invoice-csv invoice.csv --out logs.jsonl [--account id] [--integration claude|codex]")
	fmt.Fprintln(os.Stderr, "  miser pull openai --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration codex]")
	fmt.Fprintln(os.Stderr, "  miser pull openai-usage --from YYYY-MM-DD --to YYYY-MM-DD --out logs.jsonl [--account id] [--integration codex]")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
