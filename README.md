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

This repo currently ships a local CLI that:

- ingests JSONL LLM call logs
- normalizes prompts into repeatable call fingerprints
- clusters repeated expensive calls
- estimates monthly waste
- emits a savings receipt
- suggests a cheaper route: cache, smaller model, local model, or deterministic workflow

## Quickstart

Run a free AI spend audit against the example log:

```bash
python3 -m miser audit examples/llm_calls.jsonl
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
python3 -m miser analyze examples/llm_calls.jsonl --min-cluster-size 2
```

Output:

```text
Miser found 3 savings candidates

support_ticket_summary
Current monthly cost: $3,219.86
Estimated monthly cost: $482.98
Estimated savings: $2,736.88
Recommended route: semantic_cache -> claude-3-haiku
Quality guard: replay_eval >= 0.95
```

You can also emit JSON:

```bash
python3 -m miser analyze examples/llm_calls.jsonl --json
```

Or generate a reviewable route config:

```bash
python3 -m miser analyze examples/llm_calls.jsonl --routes work/routes.yaml
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
