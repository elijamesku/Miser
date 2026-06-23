from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass
from datetime import timedelta
from statistics import mean

from miser.fingerprint import fingerprint_prompt
from miser.models import LLMCall, SavingsReceipt


@dataclass(frozen=True)
class AnalyzerConfig:
    min_cluster_size: int = 3
    min_monthly_savings_usd: float = 1.0
    min_quality_score: float = 0.95


def analyze_calls(calls: list[LLMCall], config: AnalyzerConfig | None = None) -> list[SavingsReceipt]:
    config = config or AnalyzerConfig()
    clusters = _cluster_calls(calls)
    receipts: list[SavingsReceipt] = []

    for cluster_id, cluster in clusters.items():
        if len(cluster) < config.min_cluster_size:
            continue

        receipt = _build_receipt(cluster_id, cluster, config)
        if receipt.estimated_savings >= config.min_monthly_savings_usd:
            receipts.append(receipt)

    return sorted(receipts, key=lambda receipt: receipt.estimated_savings, reverse=True)


def _cluster_calls(calls: list[LLMCall]) -> dict[str, list[LLMCall]]:
    clusters: dict[str, list[LLMCall]] = defaultdict(list)
    for call in calls:
        fingerprint = fingerprint_prompt(call.prompt)
        clusters[f"{call.workflow}:{fingerprint}"].append(call)
    return clusters


def _build_receipt(cluster_id: str, calls: list[LLMCall], config: AnalyzerConfig) -> SavingsReceipt:
    calls = sorted(calls, key=lambda call: call.timestamp)
    observed_cost = sum(call.cost_usd for call in calls)
    monthly_multiplier = _monthly_multiplier(calls)
    current_monthly_cost = observed_cost * monthly_multiplier
    monthly_calls = round(len(calls) * monthly_multiplier)

    quality_scores = [call.quality_score for call in calls if call.quality_score is not None]
    avg_quality = mean(quality_scores) if quality_scores else None
    avg_tokens = mean(call.total_tokens for call in calls)

    recommended_route, estimated_cost_factor, reason = _recommend_route(calls, avg_tokens, avg_quality, config)
    estimated_monthly_cost = current_monthly_cost * estimated_cost_factor
    estimated_savings = max(0.0, current_monthly_cost - estimated_monthly_cost)
    savings_rate = estimated_savings / current_monthly_cost if current_monthly_cost else 0.0
    representative = calls[0]

    return SavingsReceipt(
        cluster_id=cluster_id,
        workflow=representative.workflow,
        current_route=representative.route,
        monthly_calls=monthly_calls,
        current_monthly_cost=current_monthly_cost,
        estimated_monthly_cost=estimated_monthly_cost,
        estimated_savings=estimated_savings,
        savings_rate=savings_rate,
        recommended_route=recommended_route,
        quality_guard=f"replay_eval >= {config.min_quality_score:.2f}",
        rollback="enabled",
        reason=reason,
        sample_call_ids=[call.id for call in calls[:5] if call.id],
    )


def _monthly_multiplier(calls: list[LLMCall]) -> float:
    if len(calls) < 2:
        return 1.0

    first = min(call.timestamp for call in calls)
    last = max(call.timestamp for call in calls)
    window = max(last - first, timedelta(days=1))
    return timedelta(days=30).total_seconds() / window.total_seconds()


def _recommend_route(
    calls: list[LLMCall],
    avg_tokens: float,
    avg_quality: float | None,
    config: AnalyzerConfig,
) -> tuple[str, float, str]:
    repeated_exact = len(calls) >= 10
    high_quality = avg_quality is None or avg_quality >= config.min_quality_score

    if repeated_exact and high_quality:
        return (
            "semantic_cache -> smaller_model_fallback",
            0.15,
            "Repeated high-confidence prompt pattern. Cache common outputs and use a cheaper fallback.",
        )

    if avg_tokens > 3500:
        return (
            "prompt_compression -> smaller_model_fallback",
            0.45,
            "Large repeated context. Compress or extract deterministic fields before model escalation.",
        )

    if "classif" in calls[0].workflow or "triage" in calls[0].workflow:
        return (
            "local_classifier -> frontier_fallback",
            0.25,
            "Classification-like workflow. Route confident cases locally and escalate uncertain cases.",
        )

    return (
        "rules_or_cache -> frontier_fallback",
        0.35,
        "Repeated call pattern. Replace the common path with cache/rules and keep frontier fallback.",
    )
