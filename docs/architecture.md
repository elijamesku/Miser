# Miser Architecture

Miser is designed as an AI spend compiler, not just a cost dashboard.

## Core Loop

```text
Observe -> Cluster -> Diagnose -> Propose -> Verify -> Enforce -> Monitor
```

## Components

### Observer

Captures LLM traffic from SDK wrappers, gateways, exported logs, or providers.

Minimum call shape:

- workflow name
- provider/model
- prompt or prompt hash
- token counts
- cost
- latency
- optional quality signal

### Clusterer

Groups repeated expensive behavior. The first implementation uses prompt normalization and deterministic fingerprints. Later versions should add embeddings and workflow-aware similarity.

### Savings Compiler

Produces replacement candidates:

- exact cache
- semantic cache
- smaller cloud model
- local classifier
- deterministic parser
- prompt compression
- rule/template
- frontier fallback

### Verifier

Runs replay evals before an optimization is trusted. A route should not be applied without a quality threshold, rollback, and cost comparison.

### Enforcer

Turns recommendations into executable config or code:

```yaml
route:
  workflow: claim_denial_triage
  primary: local_classifier
  fallback: anthropic/claude-3-5-sonnet
  confidence_threshold: 0.84
  quality_guard: replay_eval >= 0.95
```

## Defensibility

The open-source core earns trust and distribution. The defensible asset is the verified savings loop:

- company-specific workflow memory
- replay datasets
- optimization receipts
- approved route history
- domain-specific recipes
- enterprise controls

The moat is not that nobody can copy the code. The moat is that Miser becomes the trusted savings memory layer for production AI traffic.
