package miser

import (
	"fmt"
	"html"
	"strings"
)

func RenderPreviewHTML(calls []LLMCall, account, integration string) string {
	report := Audit(calls)
	plan := Plan(calls)
	rules := Rules(calls, defaultString(integration, "generic"))

	var b strings.Builder
	fmt.Fprintln(&b, "<!doctype html>")
	fmt.Fprintln(&b, `<html lang="en">`)
	fmt.Fprintln(&b, "<head>")
	fmt.Fprintln(&b, `<meta charset="utf-8">`)
	fmt.Fprintln(&b, `<meta name="viewport" content="width=device-width, initial-scale=1">`)
	fmt.Fprintln(&b, "<title>Miser Preview</title>")
	fmt.Fprintln(&b, "<style>")
	fmt.Fprintln(&b, previewCSS())
	fmt.Fprintln(&b, "</style>")
	fmt.Fprintln(&b, "</head>")
	fmt.Fprintln(&b, "<body>")
	fmt.Fprintln(&b, `<main class="shell">`)
	fmt.Fprintln(&b, `<section class="hero">`)
	fmt.Fprintln(&b, `<div>`)
	fmt.Fprintln(&b, `<p class="eyebrow">Miser Preview</p>`)
	fmt.Fprintln(&b, `<h1>AI spend audit</h1>`)
	if account != "" || integration != "" {
		fmt.Fprintf(&b, `<p class="subtle">%s%s</p>`, labelIfSet("Account", account), labelIfSet("Integration", integration))
	}
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintf(&b, `<div class="badge">%s</div>`, html.EscapeString(defaultString(report.CostBasis, "reported log cost")))
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<section class="metrics">`)
	metricCard(&b, "Monthly spend analyzed", dollars(report.MonthlySpendAnalyzed))
	metricCard(&b, "Estimated avoidable spend", dollars(report.EstimatedAvoidableSpend))
	metricCard(&b, "Savings opportunity", fmt.Sprintf("%.1f%%", report.SavingsOpportunity*100))
	metricCard(&b, "Rules generated", fmt.Sprintf("%d", len(rules.Rules)))
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<section class="grid">`)
	fmt.Fprintln(&b, `<div class="panel">`)
	fmt.Fprintln(&b, `<h2>Top waste</h2>`)
	if len(report.TopWaste) == 0 {
		fmt.Fprintln(&b, `<p class="empty">No obvious waste patterns found.</p>`)
	} else {
		for _, line := range report.TopWaste {
			fmt.Fprintln(&b, `<article class="finding">`)
			fmt.Fprintf(&b, `<div><h3>%s</h3><p>%s</p></div>`, html.EscapeString(line.Label), html.EscapeString(line.Reason))
			fmt.Fprintf(&b, `<strong>%s</strong>`, dollars(line.EstimatedMonthlyWaste))
			fmt.Fprintln(&b, `</article>`)
		}
	}
	fmt.Fprintln(&b, `</div>`)

	fmt.Fprintln(&b, `<div class="panel">`)
	fmt.Fprintln(&b, `<h2>Executable plan</h2>`)
	if len(plan.Items) == 0 {
		fmt.Fprintln(&b, `<p class="empty">No plan items yet.</p>`)
	} else {
		for _, item := range plan.Items {
			fmt.Fprintln(&b, `<article class="plan">`)
			fmt.Fprintf(&b, `<h3>%s</h3>`, html.EscapeString(item.Workflow))
			fmt.Fprintf(&b, `<p>Expected savings: <strong>%s</strong></p>`, dollars(item.EstimatedSavings))
			fmt.Fprintln(&b, `<ul>`)
			for _, fix := range item.RecommendedFixes {
				fmt.Fprintf(&b, `<li>%s</li>`, html.EscapeString(strings.ReplaceAll(fix, "_", " ")))
			}
			fmt.Fprintln(&b, `</ul>`)
			fmt.Fprintln(&b, `</article>`)
		}
	}
	fmt.Fprintln(&b, `</div>`)
	fmt.Fprintln(&b, `</section>`)

	fmt.Fprintln(&b, `<section class="panel">`)
	fmt.Fprintln(&b, `<h2>Runtime rule preview</h2>`)
	if len(rules.Rules) == 0 {
		fmt.Fprintln(&b, `<p class="empty">No rules generated yet.</p>`)
	} else {
		for _, rule := range rules.Rules {
			fmt.Fprintln(&b, `<article class="rule">`)
			fmt.Fprintf(&b, `<h3>%s</h3>`, html.EscapeString(rule.ID))
			fmt.Fprintf(&b, `<p><strong>Trigger:</strong> %s</p>`, html.EscapeString(rule.Trigger))
			fmt.Fprintln(&b, `<div class="limits">`)
			for _, key := range sortedRuleLimitKeys(rule.Limits) {
				fmt.Fprintf(&b, `<span>%s: %s</span>`, html.EscapeString(key), html.EscapeString(rule.Limits[key]))
			}
			fmt.Fprintln(&b, `</div>`)
			fmt.Fprintln(&b, `</article>`)
		}
	}
	fmt.Fprintln(&b, `</section>`)
	fmt.Fprintln(&b, `</main>`)
	fmt.Fprintln(&b, "</body>")
	fmt.Fprintln(&b, "</html>")
	return b.String()
}

func metricCard(b *strings.Builder, label, value string) {
	fmt.Fprintln(b, `<article class="metric">`)
	fmt.Fprintf(b, `<span>%s</span>`, html.EscapeString(label))
	fmt.Fprintf(b, `<strong>%s</strong>`, html.EscapeString(value))
	fmt.Fprintln(b, `</article>`)
}

func labelIfSet(label, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s ", label, value)
}

func previewCSS() string {
	return `
:root {
  color-scheme: light;
  --ink: #15181d;
  --muted: #687181;
  --line: #d8dde7;
  --panel: #ffffff;
  --bg: #f5f7fb;
  --accent: #0f766e;
  --accent-soft: #d9f4ef;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font: 15px/1.5 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.shell {
  width: min(1120px, calc(100% - 32px));
  margin: 0 auto;
  padding: 32px 0 48px;
}
.hero {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  margin-bottom: 20px;
}
.eyebrow {
  margin: 0 0 4px;
  color: var(--accent);
  font-weight: 700;
  text-transform: uppercase;
  font-size: 12px;
  letter-spacing: .08em;
}
h1, h2, h3, p { margin-top: 0; }
h1 { margin-bottom: 6px; font-size: 38px; line-height: 1.05; }
h2 { font-size: 20px; margin-bottom: 16px; }
h3 { font-size: 15px; margin-bottom: 6px; }
.subtle, .empty, .finding p, .plan p, .rule p { color: var(--muted); }
.badge {
  border: 1px solid var(--line);
  background: var(--panel);
  border-radius: 999px;
  padding: 8px 12px;
  color: var(--muted);
  white-space: nowrap;
}
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.metric, .panel {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
}
.metric { padding: 16px; }
.metric span {
  display: block;
  color: var(--muted);
  margin-bottom: 8px;
}
.metric strong {
  display: block;
  font-size: 26px;
  line-height: 1.1;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 12px;
}
.panel { padding: 18px; }
.finding, .plan, .rule {
  border-top: 1px solid var(--line);
  padding: 14px 0;
}
.finding:first-of-type, .plan:first-of-type, .rule:first-of-type { border-top: 0; padding-top: 0; }
.finding {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 16px;
}
.finding strong {
  color: var(--accent);
}
ul { margin: 8px 0 0; padding-left: 18px; }
.limits {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.limits span {
  background: var(--accent-soft);
  color: #115e59;
  border-radius: 999px;
  padding: 6px 10px;
}
@media (max-width: 820px) {
  .hero, .finding { display: block; }
  .metrics, .grid { grid-template-columns: 1fr; }
  h1 { font-size: 31px; }
  .badge { display: inline-block; margin-top: 10px; white-space: normal; }
}
`
}
