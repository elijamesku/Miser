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
- prices known OpenAI and Claude token usage from published model prices
- filters by account and integration
- fingerprints similar prompts
- finds obvious waste buckets
- prints an audit summary
- explains why each bucket was flagged
- writes executable savings plans
- writes agent rule packs and instructions
- reconciles token usage to an actual invoice total
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

## Plan

`plan` turns audit findings into an executable savings plan. This is the part that makes Miser more than a spend dashboard.

```bash
bin/miser plan --out miser-plan.yaml logs.jsonl
```

Example:

```yaml
workflow: "coding_agent_context_replay"
finding: "Coding-agent context replay"
current_monthly_cost: 20.84
estimated_savings: 5.21
recommended_fixes:
  - "bounded_context"
  - "repo_handoff"
  - "folded_tool_outputs"
  - "prefix_cache_discipline"
quality_guard: "replay_eval >= 0.95"
fallback: "current_model"
rollback: "keep current route behind a feature flag"
competitor_overlap:
  - "inferoa"
  - "headroom"
  - "codemap"
miser_difference: "bill-backed savings receipt plus executable policy"
```

The point is to show what to change, how much it should save, and how to protect quality before deploying it.

## Rules

`rules` turns the savings plan into policy a team can review and enforce.

```bash
bin/miser rules \
  --target codex \
  --out .miser/agent-rules.yaml \
  --instructions .miser/AGENT_RULES.md \
  logs.jsonl
```

For coding-agent context replay, Miser emits concrete controls like:

- max context files
- max replayed prompt tokens
- max tool output lines
- session handoff timing
- quality guard
- fallback and rollback
- metrics to track realized savings against actual invoice dollars

This is where Miser starts becoming policy-as-code instead of just a report.

Apply the rule pack to generate integration files:

```bash
bin/miser apply --target codex .miser/agent-rules.yaml
```

That writes:

- `.miser/codex-policy.md`
- `.miser/session-handoff-template.md`
- `.miser/replay-eval-checklist.md`
- `.miser/context-replay-metrics.json`

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
- `published_token_price`: token usage priced from a known model catalog
- `unpriced_token_usage`: token usage for a model Miser does not know yet
- `actual_invoice_allocated`: actual invoice dollars allocated across usage rows

Miser prices known GPT and Claude model rows dynamically from the provider/model name. Unknown models stay unpriced so the audit does not invent fake spend.

## Import ccusage

Miser can use `ccusage` output as input. This is useful for Claude Code, Codex, and other coding-agent usage.

Important: `ccusage` is treated as `estimated_token_cost`. It is not proof of what you paid.
If the row has a known Claude model, Miser will reprice it with published token pricing during audit.

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

If billing returns zero rows, pull usage instead. This can still show optimization room when spend was covered by credits, the billing period has not finalized, or you want token/request analysis instead of invoice math.

```bash
bin/miser pull openai-usage --from 2026-06-01 --to 2026-07-01 --out work/openai_usage.jsonl --account codex-work --integration codex
bin/miser audit --explain --account codex-work --integration codex work/openai_usage.jsonl
```

Usage rows use:

```text
Cost basis: estimated token cost, not your actual invoice
```

That means the audit is useful for finding waste, but it is not a receipt. To use your real invoice amount, reconcile usage to the actual charge first:

```bash
bin/miser reconcile work/openai_usage.jsonl --actual-spend 10.00 --out work/openai_actual_usage.jsonl --account openai-personal --integration codex
bin/miser audit --explain --account openai-personal --integration codex work/openai_actual_usage.jsonl
bin/miser plan --out miser-plan.yaml --account openai-personal --integration codex work/openai_actual_usage.jsonl
```

Or use an invoice CSV:

```bash
bin/miser reconcile work/openai_usage.jsonl --invoice-csv invoice.csv --out work/openai_actual_usage.jsonl --account openai-personal --integration codex
```

Reconciled rows use:

```text
Cost basis: actual invoice allocated to usage
```

Claude billing pull is not automatic yet. Use `invoice-csv` for Claude until there is a supported Claude billing API for your account type.

## Live Proxy

`audit` looks backward at logs. `proxy` sits in the request path.

Run an OpenAI-compatible proxy:

```bash
miser proxy \
  --provider openai \
  --addr 127.0.0.1:8788 \
  --account openai-personal \
  --integration codex \
  --log .miser/proxy-logs.jsonl \
  --cache .miser/exact-cache.json
```

Then point an OpenAI-compatible client at:

```text
http://127.0.0.1:8788/v1
```

The proxy forwards requests to OpenAI, logs each intercepted call, prices usage from response token counts when available, and exact-caches identical non-streaming chat/responses requests. Cache hits return before the provider call and are logged with `cost_basis: miser_exact_cache`.

For privacy, full prompt text is not stored by default. Use `--store-prompts` only when you are testing locally or have permission to keep prompt text in logs.

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
