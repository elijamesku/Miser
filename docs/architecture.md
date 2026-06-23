# Architecture

Rough shape for Miser:

```text
logs -> audit -> explain -> recommend -> verify -> route/patch -> track savings
```

## 1. Logs

Input can come from:

- JSONL logs
- `ccusage`
- gateways like LiteLLM
- provider exports
- app-specific logs

Minimum useful fields:

- workflow
- provider/model
- prompt or prompt hash
- token counts
- cost
- timestamp

## 2. Audit

The audit finds obvious waste:

- duplicate summaries
- long repeated context
- expensive model used for classification
- oversized PDF prompts
- retry loops
- coding-agent context reconstruction

This should stay explainable. Every finding needs sample call IDs and a reason.

## 3. Recommendations

Miser should recommend the cheapest safe replacement:

- cache
- prompt compression
- smaller model
- local model
- rule/template
- deterministic code
- expensive model fallback

## 4. Verification

This part matters most.

Before Miser applies anything automatically, it should replay old examples and check that quality stays above a threshold.

No proof, no auto-route.

## 5. Routes And Patches

Eventually Miser should output real changes:

```yaml
route:
  workflow: claim_denial_triage
  primary: local_classifier
  fallback: anthropic/claude-3-5-sonnet
  confidence_threshold: 0.84
  quality_guard: replay_eval >= 0.95
```

Or a code patch when the LLM call can become normal software.

## Savings Targets

Keep two numbers separate:

- total bill savings
- per-workflow savings

I think the believable target is:

- 25-50% off total AI spend for messy/high-volume orgs
- 80-90% off specific repetitive workflows

The 80-90% number comes from replacing expensive calls with stacked cheaper paths:

1. compress context
2. cache repeated work
3. use smaller models for easy cases
4. use local models when good enough
5. replace stable workflows with code
6. keep frontier fallback for hard/ambiguous cases
