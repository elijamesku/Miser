package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	case "import":
		err = runImport(os.Args[2:])
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: miser audit [--explain] [--json] logs.jsonl")
	}

	calls, err := miser.LoadJSONL(fs.Arg(0))
	if err != nil {
		return err
	}
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

func runImport(args []string) error {
	if len(args) < 1 || args[0] != "ccusage" {
		return fmt.Errorf("usage: miser import ccusage ccusage.json --out logs.jsonl")
	}
	path, out, err := parseImportArgs(args[1:])
	if err != nil {
		return err
	}
	if path == "" || out == "" {
		return fmt.Errorf("usage: miser import ccusage ccusage.json --out logs.jsonl")
	}

	rows, err := miser.ImportCCUsage(path)
	if err != nil {
		return err
	}
	if err := miser.WriteJSONL(rows, out); err != nil {
		return err
	}
	fmt.Printf("Imported %d ccusage rows into %s\n", len(rows), out)
	return nil
}

func parseImportArgs(args []string) (string, string, error) {
	var path string
	var out string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--out requires a path")
			}
			out = args[i+1]
			i++
		default:
			if path != "" {
				return "", "", fmt.Errorf("unexpected argument %q", args[i])
			}
			path = args[i]
		}
	}
	return path, out, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  miser audit [--explain] [--json] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser analyze [--json] [--routes routes.yaml] logs.jsonl")
	fmt.Fprintln(os.Stderr, "  miser import ccusage ccusage.json --out logs.jsonl")
}
