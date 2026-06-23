import unittest
from datetime import datetime, timezone

from miser.analyzer import AnalyzerConfig, analyze_calls
from miser.audit import audit_calls, render_audit
from miser.fingerprint import fingerprint_prompt, normalize_prompt
from miser.importers import import_ccusage
from miser.models import LLMCall
from miser.routes import render_route_config


class AnalyzerTest(unittest.TestCase):
    def test_prompt_fingerprint_masks_ids_and_emails(self) -> None:
        left = "Summarize ticket 123 for jane@example.com"
        right = "Summarize ticket 999 for sam@example.com"

        self.assertEqual(normalize_prompt(left), "summarize ticket <num> for <email>")
        self.assertEqual(fingerprint_prompt(left), fingerprint_prompt(right))

    def test_analyze_calls_emits_savings_receipt_for_repeated_pattern(self) -> None:
        calls = [
            LLMCall(
                id=f"call_{index}",
                timestamp=datetime(2026, 6, 1, index, tzinfo=timezone.utc),
                workflow="support_ticket_summary",
                provider="anthropic",
                model="claude-3-5-sonnet",
                prompt=f"Summarize support ticket {index} for jane{index}@example.com",
                input_tokens=2000,
                output_tokens=300,
                cost_usd=0.02,
                quality_score=0.98,
            )
            for index in range(10)
        ]

        receipts = analyze_calls(calls, AnalyzerConfig(min_cluster_size=3, min_monthly_savings_usd=1))

        self.assertEqual(len(receipts), 1)
        self.assertEqual(receipts[0].workflow, "support_ticket_summary")
        self.assertGreater(receipts[0].estimated_savings, 0)
        self.assertEqual(receipts[0].recommended_route, "semantic_cache -> smaller_model_fallback")

        route_config = render_route_config(receipts)
        self.assertIn("routes:", route_config)
        self.assertIn("workflow: support_ticket_summary", route_config)
        self.assertIn("approval_required: true", route_config)

    def test_audit_outputs_front_door_summary(self) -> None:
        calls = [
            LLMCall(
                id=f"call_{index}",
                timestamp=datetime(2026, 6, 1, index, tzinfo=timezone.utc),
                workflow="claim_denial_triage",
                provider="anthropic",
                model="claude-3-5-sonnet",
                prompt=f"Classify denial {index}. Decide appeal path.",
                input_tokens=1500,
                output_tokens=200,
                cost_usd=100,
                quality_score=0.96,
            )
            for index in range(3)
        ]

        report = audit_calls(calls)
        rendered = render_audit(report, explain=True)

        self.assertIn("Miser AI Spend Audit", rendered)
        self.assertIn("Why:", rendered)
        self.assertIn("Sample calls:", rendered)
        self.assertEqual(report.monthly_spend_analyzed, 300)
        self.assertGreater(report.estimated_avoidable_spend, 0)

    def test_import_ccusage_normalizes_usage_rows(self) -> None:
        rows = import_ccusage("examples/ccusage.json")

        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["workflow"], "coding_agent_usage")
        self.assertEqual(rows[0]["source"], "ccusage")
        self.assertGreater(rows[0]["input_tokens"], 0)

        calls = [LLMCall.from_dict(row) for row in rows]
        report = audit_calls(calls)
        rendered = render_audit(report, explain=True)

        self.assertIn("Coding-agent context reconstruction", rendered)
        self.assertIn("ccusage rows", rendered)


if __name__ == "__main__":
    unittest.main()
