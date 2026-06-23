from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def import_ccusage(path: str | Path) -> list[dict[str, Any]]:
    """Normalize ccusage --json output into Miser JSONL rows.

    ccusage supports several report shapes: daily, monthly, session, source-specific,
    and model breakdown reports. This importer walks the JSON and converts any object
    that looks like a usage row into a Miser-compatible call aggregate.
    """

    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    rows = _find_usage_rows(payload)
    normalized: list[dict[str, Any]] = []

    for index, row in enumerate(rows, start=1):
        input_tokens = _first_number(
            row,
            "inputTokens",
            "input_tokens",
            "promptTokens",
            "prompt_tokens",
            "cacheCreationInputTokens",
            "cache_creation_input_tokens",
        )
        output_tokens = _first_number(row, "outputTokens", "output_tokens", "completionTokens", "completion_tokens")
        cache_read_tokens = _first_number(row, "cacheReadInputTokens", "cache_read_input_tokens")
        total_tokens = _first_number(row, "totalTokens", "total_tokens", "tokens")

        if total_tokens and not input_tokens and not output_tokens:
            input_tokens = int(total_tokens * 0.8)
            output_tokens = total_tokens - input_tokens

        cost = _first_float(row, "totalCost", "costUSD", "cost_usd", "cost", "total_cost") or 0.0
        timestamp = _timestamp(row)
        model = _string_value(row, "model", "modelName", "model_name", default="unknown")
        source = _string_value(row, "source", "provider", "cli", default="ccusage")
        project = _string_value(row, "project", "projectName", "instance", "sessionId", "session_id", default="unknown")
        label = _string_value(row, "date", "month", "week", "sessionId", "session_id", default=f"row_{index}")

        normalized.append(
            {
                "id": f"ccusage_{index:04d}",
                "timestamp": timestamp,
                "workflow": "coding_agent_usage",
                "provider": source,
                "model": model,
                "prompt": f"Coding agent usage aggregate for {project} {label}.",
                "input_tokens": int(input_tokens + cache_read_tokens),
                "output_tokens": int(output_tokens),
                "cost_usd": float(cost),
                "source": "ccusage",
                "project": project,
                "report_label": label,
                "cache_read_tokens": int(cache_read_tokens),
                "raw": row,
            }
        )

    return normalized


def write_jsonl(rows: list[dict[str, Any]], path: str | Path) -> None:
    with Path(path).open("w", encoding="utf-8") as handle:
        for row in rows:
            handle.write(json.dumps(row, separators=(",", ":")) + "\n")


def _find_usage_rows(value: Any) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []

    if isinstance(value, list):
        for item in value:
            rows.extend(_find_usage_rows(item))
        return rows

    if not isinstance(value, dict):
        return rows

    if _looks_like_usage_row(value):
        rows.append(value)

    for child in value.values():
        if isinstance(child, (dict, list)):
            rows.extend(_find_usage_rows(child))

    return rows


def _looks_like_usage_row(row: dict[str, Any]) -> bool:
    token_keys = {
        "inputTokens",
        "input_tokens",
        "promptTokens",
        "prompt_tokens",
        "outputTokens",
        "output_tokens",
        "completionTokens",
        "completion_tokens",
        "totalTokens",
        "total_tokens",
        "tokens",
    }
    cost_keys = {"totalCost", "costUSD", "cost_usd", "cost", "total_cost"}
    return any(key in row for key in token_keys) and any(key in row for key in cost_keys)


def _first_number(row: dict[str, Any], *keys: str) -> int:
    for key in keys:
        if key in row and row[key] is not None:
            return int(float(row[key]))
    return 0


def _first_float(row: dict[str, Any], *keys: str) -> float | None:
    for key in keys:
        if key in row and row[key] is not None:
            return float(row[key])
    return None


def _string_value(row: dict[str, Any], *keys: str, default: str) -> str:
    for key in keys:
        if key in row and row[key] not in (None, ""):
            return str(row[key])
    return default


def _timestamp(row: dict[str, Any]) -> str:
    raw = _string_value(row, "timestamp", "date", "month", "week", "startTime", "startedAt", default="")
    if raw:
        if len(raw) == 7:
            raw = f"{raw}-01"
        try:
            parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            return parsed.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")
        except ValueError:
            pass

    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
