# Miser

Miser is a CLI for finding wasted LLM spend.

It reads LLM call logs, groups repeated patterns, and points out places where expensive model calls could probably be replaced with cache, smaller models, local models, rules, or normal code.

This is early. The goal is to make the audit useful first, then make the fixes executable.

## Why

AI bills get messy fast, especially with agents.

The first version is focused on answering:

- where is the spend going?
- what looks repeated?
- what looks too expensive for the task?
- what could be cached, routed, or replaced?
- which workflows might save 80-90% if optimized?

I do not think Miser should claim 90% savings across an entire company bill. That sounds fake.

A more realistic target:

- 25-50% total bill reduction for messy/high-volume AI usage
- 80-90% savings on specific repetitive workflows

## What It Does Now

- reads JSONL LLM logs
- imports `ccusage` JSON
- imports invoice/billing CSV rows for actual spend
- separates `actual_invoice` from `estimated_token_cost`
- filters by account and integration
- fingerprints similar prompts
- finds obvious waste buckets
- prints an audit summary
- explains why each bucket was flagged
- generates route suggestions for repeated call clusters

## Install

Build the CLI:

```bash
go build -o bin/miser ./cmd/miser
```

Run tests:

```bash
go test ./cmd/... ./internal/...
```

## Audit

```bash
bin/miser audit examples/llm_calls.jsonl
```

Example output:

```text
Miser AI Spend Audit

Monthly spend analyzed: $0.44
Cost basis: reported log cost
Estimated avoidable spend: $0.36
Savings opportunity: 80.6%

Top waste:
1. Duplicate summaries: $0.16 (80% workflow savings potential)
2. Repeated long-context calls: $0.07 (55% workflow savings potential)
3. Oversized PDF prompts: $0.06 (60% workflow savings potential)
4. Expensive model used for classification: $0.04 (70% workflow savings potential)
5. Agent retry loops: $0.04 (85% workflow savings potential)
```

To see the reasoning:

```bash
bin/miser audit --explain examples/llm_calls.jsonl
```

Example:

```text
1. Duplicate summaries: $0.16 (80% workflow savings potential)
   Why: Miser found repeated summary prompts after masking IDs and emails. These are strong candidates for exact or semantic caching.
   Confidence: high
   Sample calls: call_001, call_002, call_003, call_004, call_005
```

## Analyze

`audit` is the quick summary. `analyze` gives more detailed savings receipts for repeated call clusters.

```bash
bin/miser analyze --min-cluster-size 2 examples/llm_calls.jsonl
```

Example:

```text
support_ticket_summary
Current monthly cost: $3.28
Estimated monthly cost: $0.49
Estimated savings: $2.79
Recommended route: semantic_cache -> smaller_model_fallback
Quality guard: replay_eval >= 0.95
```

JSON:

```bash
bin/miser analyze --json examples/llm_calls.jsonl
```

Route config:

```bash
bin/miser analyze --routes work/routes.yaml examples/llm_calls.jsonl
```

## Accounts And Integrations

Miser needs to know what it is measuring.

Use `--account` when you have multiple Claude/Codex accounts:

```bash
bin/miser audit --account claude-work logs.jsonl
bin/miser audit --account claude-personal logs.jsonl
```

Use `--integration` when you want to split tools:

```bash
bin/miser audit --integration claude logs.jsonl
bin/miser audit --integration codex logs.jsonl
```

The important field is `cost_basis`:

- `actual_invoice`: actual money from a billing export or invoice
- `reported_log_cost`: cost reported by request logs
- `estimated_token_cost`: estimated token/API value, not your actual invoice

## Import ccusage

Miser can use `ccusage` output as input. This is useful for Claude Code, Codex, and other coding-agent usage.

Important: `ccusage` is treated as `estimated_token_cost`. It is not proof of what you paid.

```bash
npx ccusage@latest daily --json > ccusage.json
bin/miser import ccusage ccusage.json --out logs.jsonl --account codex-local --integration codex
bin/miser audit --explain --account codex-local --integration codex logs.jsonl
```

Example finding:

```text
1. Coding-agent context reconstruction: $20.05 (35% workflow savings potential)
   Why: ccusage rows show large coding-agent input/cache-read token volume. This often means the agent is re-reading project context instead of using session handoffs, code indexes, or narrower task scopes.
   Confidence: medium
   Sample calls: ccusage_0001, ccusage_0002
```

## Import Actual Spend

To know exactly how much money you spent on a specific Claude account, import a billing export or invoice CSV.

Minimum columns:

```csv
date,provider,account,description,cost_usd
2026-06-01,anthropic,claude-work,Claude Team subscription,200.00
```

Import it:

```bash
bin/miser import invoice-csv examples/invoice.csv --out work/invoice_logs.jsonl --integration claude
bin/miser audit --account claude-work --integration claude work/invoice_logs.jsonl
```

That output uses:

```text
Cost basis: actual invoice/billing export
```

That is the number to use when you care about exact account spend.

## Pull OpenAI/Codex Billing

Miser can pull OpenAI organization costs through the OpenAI costs API.

Set an admin API key:

```bash
export OPENAI_ADMIN_KEY=...
```

Pull costs:

```bash
bin/miser pull openai --from 2026-06-01 --to 2026-07-01 --out work/openai_bill.jsonl --account codex-work --integration codex
bin/miser audit --account codex-work --integration codex work/openai_bill.jsonl
```

That output uses:

```text
Cost basis: provider billing API
```

This is the path to test real OpenAI/Codex API spend.

Claude billing pull is not automatic yet. Use `invoice-csv` for Claude until there is a supported Claude billing API for your account type.

## Log Format

Miser expects newline-delimited JSON:

```json
{
  "id": "call_001",
  "timestamp": "2026-06-01T12:00:00Z",
  "workflow": "support_ticket_summary",
  "provider": "anthropic",
  "model": "claude-3-5-sonnet",
  "prompt": "Summarize this support ticket...",
  "input_tokens": 2100,
  "output_tokens": 340,
  "cost_usd": 0.0124,
  "account_id": "claude-work",
  "integration": "claude",
  "cost_basis": "reported_log_cost",
  "latency_ms": 4810,
  "quality_score": 0.98
}
```

## Savings Model

The big savings come from finding repeated AI work that should not be expensive AI anymore.

Examples:

- repeated summaries -> cache
- simple classification -> smaller/local model
- giant PDF prompts -> chunk/parser/template first
- retry loops -> better guardrails or deterministic failure handling
- stable extraction -> code instead of LLM call

The highest-savings workflows are usually boring and repetitive. That is the point.

## Roadmap

- better importers
- HTML report
- replay evals
- semantic cache adapter
- route config generation
- local model fallback
- generated code/config patches
- realized savings tracking
- Anthropic/OpenAI billing importers once stable exports/APIs are available
