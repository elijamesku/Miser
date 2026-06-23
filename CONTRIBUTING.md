# Contributing

Miser is early. Keep changes small and easy to review.

## Workflow

Use branches for changes:

```bash
git checkout -b feature/name
go test ./cmd/... ./internal/...
git push -u origin feature/name
```

Open a pull request into `main`.

Right now Eli is the only maintainer, so there is no required second approver. CI still needs to pass before merge.

## What To Preserve

- Do not call estimated token cost actual spend.
- Keep `account_id`, `integration`, and `cost_basis` on imported rows when possible.
- Prefer boring, inspectable savings logic over magic.
- Add tests when changing importers, audit math, or route recommendations.

## Cost Basis

Miser uses these cost basis values:

- `actual_invoice`: billing export or invoice data. This is actual money spent.
- `reported_log_cost`: request logs with provider-reported cost.
- `estimated_token_cost`: estimated token/API value. This is not an invoice.
