# Miser

Miser is an open-source AI spend compiler.

It observes expensive LLM calls, finds repeated waste, and turns those calls into cheaper executable workflows: cache hits, local models, smaller models, deterministic code, or rules with frontier fallback.

```text
LLM logs -> repeated call clusters -> savings receipts -> routing patches
```

## Why

AI agent spend is becoming the new cloud bill: opaque, dynamic, duplicated, and easy to let run wild. Most tools show where the money went. Miser is built to reduce the bill.

The invariant:

> Miser should only recommend an optimization when it can show lower cost with an explicit quality guard.

## First MVP

This repo currently ships a local Go CLI that:

- ingests JSONL LLM call logs
- imports ccusage JSON reports
- normalizes prompts into repeatable call fingerprints
- clusters repeated expensive calls
- estimates monthly waste
- emits a savings receipt
- suggests a cheaper route: cache, smaller model, local model, or deterministic workflow

## Quickstart

Build a local binary:

```bash
go build -o bin/miser ./cmd/miser
```

Run a free AI spend audit against the example log:

```bash
bin/miser audit examples/llm_calls.jsonl
```

Output:

```text
Miser AI Spend Audit

Monthly spend analyzed: $0.44
Estimated avoidable spend: $0.36
Savings opportunity: 80.6%

Top waste:
1. Duplicate summaries: $0.16
2. Repeated long-context calls: $0.07
3. Oversized PDF prompts: $0.06
4. Expensive model used for classification: $0.04
5. Agent retry loops: $0.04
```

For deeper receipts, run:

```bash
bin/miser analyze --min-cluster-size 2 examples/llm_calls.jsonl
```

Output:

```text
Miser found 4 savings candidates

support_ticket_summary
Current monthly cost: $3.28
Estimated monthly cost: $0.49
Estimated savings: $2.79
Recommended route: semantic_cache -> smaller_model_fallback
Quality guard: replay_eval >= 0.95
```

You can also emit JSON:

```bash
bin/miser analyze --json examples/llm_calls.jsonl
```

Or generate a reviewable route config:

```bash
bin/miser analyze --routes work/routes.yaml examples/llm_calls.jsonl
```

To see why Miser flagged each bucket:

```bash
bin/miser audit --explain examples/llm_calls.jsonl
```

Example:

```text
1. Duplicate summaries: $0.16
   Why: Miser found repeated summary prompts after masking IDs and emails. These are strong candidates for exact or semantic caching.
   Confidence: high
   Sample calls: call_017, call_018, call_019, call_001, call_002
```

## Import ccusage

Miser can use ccusage as the measurement layer for Claude Code, Codex, Gemini CLI, and other coding-agent usage reports.

```bash
npx ccusage@latest daily --json > ccusage.json
bin/miser import ccusage ccusage.json --out logs.jsonl
bin/miser audit --explain logs.jsonl
```

Example imported audit finding:

```text
1. Coding-agent context reconstruction: $20.05
   Why: ccusage rows show large coding-agent input/cache-read token volume. This often means the agent is re-reading project context instead of using session handoffs, code indexes, or narrower task scopes.
   Confidence: medium
   Sample calls: ccusage_0001, ccusage_0002
```

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
  "latency_ms": 4810,
  "quality_score": 0.98
}
```

## The Receipt

Every Miser recommendation is shaped as a receipt:

```text
Cluster: support_ticket_summary
Current route: anthropic/claude-3-5-sonnet
Monthly calls: 840000
Current monthly cost: $18,400
Recommended route: semantic_cache -> claude-3-haiku
Estimated monthly cost: $4,900
Estimated savings: $13,500
Quality guard: replay_eval >= 0.95
Rollback: enabled
```

Generated route config:

```yaml
routes:
  - workflow: claim_denial_triage
    current_route: anthropic/claude-3-5-sonnet
    primary: local_classifier
    fallback: frontier_fallback
    quality_guard: "replay_eval >= 0.95"
    rollback: enabled
    approval_required: true
```

That receipt is the product. Dashboards are copyable. Verified savings are harder.

## Open Source Strategy

Open-source core:

- capture LLM calls
- generate local savings reports
- run replay evals
- emit routing configs
- integrate with model providers and local models

Paid enterprise layer later:

- hosted control plane
- SSO/RBAC/audit logs
- approval workflows
- private/VPC deployment
- continuous monitoring
- cross-team savings intelligence
- workflow-specific optimization packs

## Roadmap

- Replay eval runner
- SDK wrappers for OpenAI, Anthropic, Gemini, LiteLLM
- Semantic cache adapter
- Route config generator
- GitHub PR patch generator for deterministic replacements
- Domain packs for support ops, PDF extraction, coding agents, legal review, and healthcare RCM
