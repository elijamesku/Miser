from __future__ import annotations

import argparse
import json
import sys

from miser.analyzer import AnalyzerConfig, analyze_calls
from miser.audit import audit_calls, render_audit
from miser.io import dumps_receipts, load_jsonl
from miser.routes import render_route_config


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(prog="miser", description="Find repeated expensive LLM calls and emit savings receipts.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    audit = subparsers.add_parser("audit", help="Run a free AI spend audit against JSONL LLM logs.")
    audit.add_argument("path", help="Path to a JSONL log file.")
    audit.add_argument("--json", action="store_true", help="Emit machine-readable JSON.")

    analyze = subparsers.add_parser("analyze", help="Analyze JSONL LLM call logs.")
    analyze.add_argument("path", help="Path to a JSONL log file.")
    analyze.add_argument("--min-cluster-size", type=int, default=3)
    analyze.add_argument("--min-monthly-savings", type=float, default=1.0)
    analyze.add_argument("--min-quality-score", type=float, default=0.95)
    analyze.add_argument("--json", action="store_true", help="Emit machine-readable JSON.")
    analyze.add_argument("--routes", help="Write a reviewable route config to this path.")

    args = parser.parse_args(argv)

    if args.command == "audit":
        calls = load_jsonl(args.path)
        report = audit_calls(calls)
        if args.json:
            print(json.dumps(report.to_dict(), indent=2))
        else:
            print(render_audit(report))

    elif args.command == "analyze":
        calls = load_jsonl(args.path)
        receipts = analyze_calls(
            calls,
            AnalyzerConfig(
                min_cluster_size=args.min_cluster_size,
                min_monthly_savings_usd=args.min_monthly_savings,
                min_quality_score=args.min_quality_score,
            ),
        )

        if args.json:
            print(dumps_receipts(receipts))
        else:
            _print_human(receipts)

        if args.routes:
            with open(args.routes, "w", encoding="utf-8") as handle:
                handle.write(render_route_config(receipts))


def _print_human(receipts: list[object]) -> None:
    if not receipts:
        print("Miser found no savings candidates.")
        return

    print(f"Miser found {len(receipts)} savings candidate{'s' if len(receipts) != 1 else ''}")
    for receipt in receipts:
        print()
        print(receipt.workflow)
        print(f"Cluster: {receipt.cluster_id}")
        print(f"Current route: {receipt.current_route}")
        print(f"Monthly calls: {receipt.monthly_calls:,}")
        print(f"Current monthly cost: ${receipt.current_monthly_cost:,.2f}")
        print(f"Estimated monthly cost: ${receipt.estimated_monthly_cost:,.2f}")
        print(f"Estimated savings: ${receipt.estimated_savings:,.2f} ({receipt.savings_rate:.0%})")
        print(f"Recommended route: {receipt.recommended_route}")
        print(f"Quality guard: {receipt.quality_guard}")
        print(f"Rollback: {receipt.rollback}")
        print(f"Why: {receipt.reason}")


if __name__ == "__main__":
    try:
        main()
    except BrokenPipeError:
        sys.exit(1)
