from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any


@dataclass(frozen=True)
class LLMCall:
    id: str
    timestamp: datetime
    workflow: str
    provider: str
    model: str
    prompt: str
    input_tokens: int
    output_tokens: int
    cost_usd: float
    latency_ms: int | None = None
    quality_score: float | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, row: dict[str, Any]) -> "LLMCall":
        raw_timestamp = row.get("timestamp")
        if not raw_timestamp:
            raise ValueError("call is missing timestamp")

        timestamp = _parse_timestamp(raw_timestamp)
        known = {
            "id",
            "timestamp",
            "workflow",
            "provider",
            "model",
            "prompt",
            "input_tokens",
            "output_tokens",
            "cost_usd",
            "latency_ms",
            "quality_score",
        }

        return cls(
            id=str(row.get("id", "")),
            timestamp=timestamp,
            workflow=str(row.get("workflow", "unknown")),
            provider=str(row.get("provider", "unknown")),
            model=str(row.get("model", "unknown")),
            prompt=str(row.get("prompt", "")),
            input_tokens=int(row.get("input_tokens", 0)),
            output_tokens=int(row.get("output_tokens", 0)),
            cost_usd=float(row.get("cost_usd", 0)),
            latency_ms=_optional_int(row.get("latency_ms")),
            quality_score=_optional_float(row.get("quality_score")),
            metadata={key: value for key, value in row.items() if key not in known},
        )

    @property
    def total_tokens(self) -> int:
        return self.input_tokens + self.output_tokens

    @property
    def route(self) -> str:
        return f"{self.provider}/{self.model}"


@dataclass(frozen=True)
class SavingsReceipt:
    cluster_id: str
    workflow: str
    current_route: str
    monthly_calls: int
    current_monthly_cost: float
    estimated_monthly_cost: float
    estimated_savings: float
    savings_rate: float
    recommended_route: str
    quality_guard: str
    rollback: str
    reason: str
    sample_call_ids: list[str]

    def to_dict(self) -> dict[str, Any]:
        return {
            "cluster_id": self.cluster_id,
            "workflow": self.workflow,
            "current_route": self.current_route,
            "monthly_calls": self.monthly_calls,
            "current_monthly_cost": round(self.current_monthly_cost, 4),
            "estimated_monthly_cost": round(self.estimated_monthly_cost, 4),
            "estimated_savings": round(self.estimated_savings, 4),
            "savings_rate": round(self.savings_rate, 4),
            "recommended_route": self.recommended_route,
            "quality_guard": self.quality_guard,
            "rollback": self.rollback,
            "reason": self.reason,
            "sample_call_ids": self.sample_call_ids,
        }


def _parse_timestamp(value: str) -> datetime:
    normalized = value.replace("Z", "+00:00")
    parsed = datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def _optional_int(value: Any) -> int | None:
    if value is None:
        return None
    return int(value)


def _optional_float(value: Any) -> float | None:
    if value is None:
        return None
    return float(value)
