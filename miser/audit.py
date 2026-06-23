from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass

from miser.fingerprint import fingerprint_prompt
from miser.models import LLMCall


@dataclass(frozen=True)
class WasteLine:
    label: str
    estimated_monthly_waste: float


@dataclass(frozen=True)
class AuditReport:
    monthly_spend_analyzed: float
    estimated_avoidable_spend: float
    savings_opportunity: float
    top_waste: list[WasteLine]

    def to_dict(self) -> dict[str, object]:
        return {
            "monthly_spend_analyzed": round(self.monthly_spend_analyzed, 4),
            "estimated_avoidable_spend": round(self.estimated_avoidable_spend, 4),
            "savings_opportunity": round(self.savings_opportunity, 4),
            "top_waste": [
                {
                    "label": line.label,
                    "estimated_monthly_waste": round(line.estimated_monthly_waste, 4),
                }
                for line in self.top_waste
            ],
        }


def audit_calls(calls: list[LLMCall]) -> AuditReport:
    monthly_spend = sum(call.cost_usd for call in calls)
    waste = [
        WasteLine("Repeated long-context calls", _repeated_long_context_waste(calls)),
        WasteLine("Expensive model used for classification", _classification_waste(calls)),
        WasteLine("Duplicate summaries", _duplicate_summary_waste(calls)),
        WasteLine("Agent retry loops", _retry_loop_waste(calls)),
        WasteLine("Oversized PDF prompts", _oversized_pdf_waste(calls)),
    ]
    waste = sorted((line for line in waste if line.estimated_monthly_waste > 0), key=lambda line: line.estimated_monthly_waste, reverse=True)
    avoidable = min(monthly_spend, sum(line.estimated_monthly_waste for line in waste))
    savings_opportunity = avoidable / monthly_spend if monthly_spend else 0.0

    return AuditReport(
        monthly_spend_analyzed=monthly_spend,
        estimated_avoidable_spend=avoidable,
        savings_opportunity=savings_opportunity,
        top_waste=waste[:5],
    )


def render_audit(report: AuditReport) -> str:
    lines = [
        "Miser AI Spend Audit",
        "",
        f"Monthly spend analyzed: ${report.monthly_spend_analyzed:,.2f}",
        f"Estimated avoidable spend: ${report.estimated_avoidable_spend:,.2f}",
        f"Savings opportunity: {report.savings_opportunity:.1%}",
        "",
        "Top waste:",
    ]

    if not report.top_waste:
        lines.append("No obvious waste patterns found.")
    else:
        for index, line in enumerate(report.top_waste, start=1):
            lines.append(f"{index}. {line.label}: ${line.estimated_monthly_waste:,.2f}")

    return "\n".join(lines)


def _repeated_long_context_waste(calls: list[LLMCall]) -> float:
    groups = _groups(calls)
    total = 0.0
    for grouped_calls in groups.values():
        if len(grouped_calls) < 3:
            continue
        avg_input_tokens = sum(call.input_tokens for call in grouped_calls) / len(grouped_calls)
        if avg_input_tokens >= 3000:
            total += sum(call.cost_usd for call in grouped_calls) * 0.55
    return total


def _classification_waste(calls: list[LLMCall]) -> float:
    total = 0.0
    for call in calls:
        workflow = call.workflow.lower()
        prompt = call.prompt.lower()
        is_classification = "classif" in workflow or "triage" in workflow or "classify" in prompt
        is_expensive = any(name in call.model.lower() for name in ["sonnet", "opus", "gpt-4", "gemini-1.5-pro"])
        if is_classification and is_expensive:
            total += call.cost_usd * 0.70
    return total


def _duplicate_summary_waste(calls: list[LLMCall]) -> float:
    groups = _groups(calls)
    total = 0.0
    for grouped_calls in groups.values():
        sample = grouped_calls[0]
        is_summary = "summar" in sample.workflow.lower() or "summar" in sample.prompt.lower()
        if is_summary and len(grouped_calls) >= 3:
            total += sum(call.cost_usd for call in grouped_calls[1:]) * 0.80
    return total


def _retry_loop_waste(calls: list[LLMCall]) -> float:
    retries = [call for call in calls if _is_retry(call)]
    return sum(call.cost_usd for call in retries) * 0.85


def _oversized_pdf_waste(calls: list[LLMCall]) -> float:
    total = 0.0
    for call in calls:
        text = f"{call.workflow} {call.prompt}".lower()
        if "pdf" in text and call.input_tokens >= 6000:
            total += call.cost_usd * 0.60
    return total


def _groups(calls: list[LLMCall]) -> dict[str, list[LLMCall]]:
    grouped: dict[str, list[LLMCall]] = defaultdict(list)
    for call in calls:
        grouped[f"{call.workflow}:{fingerprint_prompt(call.prompt)}"].append(call)
    return grouped


def _is_retry(call: LLMCall) -> bool:
    retryish = call.metadata.get("retry") or call.metadata.get("is_retry") or call.metadata.get("attempt")
    if retryish:
        return True

    text = f"{call.workflow} {call.prompt}".lower()
    return "retry" in text or "try again" in text or "previous attempt failed" in text
