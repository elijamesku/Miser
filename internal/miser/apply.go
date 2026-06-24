package miser

import (
	"fmt"
	"path/filepath"
	"strings"
)

type AppliedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func ApplyRules(policyYAML []byte, target, outputDir string) []AppliedFile {
	if target == "" {
		target = yamlValue(string(policyYAML), "target")
	}
	if target == "" {
		target = "generic"
	}
	if outputDir == "" {
		outputDir = ".miser"
	}

	policy := string(policyYAML)
	files := []AppliedFile{
		{
			Path:    filepath.Join(outputDir, target+"-policy.md"),
			Content: renderTargetPolicy(policy, target),
		},
		{
			Path:    filepath.Join(outputDir, "session-handoff-template.md"),
			Content: renderSessionHandoffTemplate(target),
		},
		{
			Path:    filepath.Join(outputDir, "replay-eval-checklist.md"),
			Content: renderReplayEvalChecklist(target),
		},
	}
	if strings.Contains(policy, "miser.coding_agent_context_replay.v1") {
		files = append(files, AppliedFile{
			Path:    filepath.Join(outputDir, "context-replay-metrics.json"),
			Content: renderContextReplayMetrics(target),
		})
	}
	return files
}

func RenderApplySummary(files []AppliedFile) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Miser Apply")
	fmt.Fprintln(&b)
	if len(files) == 0 {
		fmt.Fprintln(&b, "No files generated.")
		return b.String()
	}
	fmt.Fprintln(&b, "Generated files:")
	for _, file := range files {
		fmt.Fprintf(&b, "- %s\n", file.Path)
	}
	return b.String()
}

func renderTargetPolicy(policy, target string) string {
	var b strings.Builder
	titleTarget := titleWord(target)
	fmt.Fprintf(&b, "# %s Miser Policy\n", titleTarget)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Generated from `.miser/agent-rules.yaml`.")
	fmt.Fprintln(&b)
	if strings.Contains(policy, "miser.coding_agent_context_replay.v1") {
		fmt.Fprintln(&b, "## Context Replay Control")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Trigger when cached input dominates the request and output is tiny compared with replayed context.")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Rules:")
		fmt.Fprintln(&b, "- Start from a short repo/session summary before reopening files.")
		fmt.Fprintln(&b, "- Keep active context to 12 directly relevant files or fewer.")
		fmt.Fprintln(&b, "- Keep replayed prompt context under 8,000 tokens unless the quality guard would fail.")
		fmt.Fprintln(&b, "- Fold command output over 200 lines into a summary plus exact file/command references.")
		fmt.Fprintln(&b, "- Write a session handoff before switching tasks or after 20 minutes.")
		fmt.Fprintln(&b, "- Preserve stable instruction prefixes to keep provider prompt caching effective.")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Quality guard: `replay_eval >= 0.95`.")
		fmt.Fprintln(&b, "Fallback: use the current model and full context when the task cannot be completed safely.")
	}
	if target == "codex" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "## Codex Notes")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "- Prefer `rg` results, file paths, and line references before pasting large files.")
		fmt.Fprintln(&b, "- Put long handoffs in `.miser/session-handoff-template.md` format.")
		fmt.Fprintln(&b, "- Track actual savings against `actual_invoice_allocated_cost_usd`.")
	}
	if target == "claude" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "## Claude Code Notes")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "- Keep `CLAUDE.md` short.")
		fmt.Fprintln(&b, "- Move long command outputs and file dumps into referenced artifacts.")
		fmt.Fprintln(&b, "- Track actual savings against `actual_invoice_allocated_cost_usd`.")
	}
	return b.String()
}

func titleWord(value string) string {
	if value == "" {
		return "Generic"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func renderSessionHandoffTemplate(target string) string {
	return fmt.Sprintf(`# Session Handoff

Target: %s

## Current Goal

## Files Touched

## Decisions Made

## Commands Run

## Open Questions

## Next Step

## Context Budget

- files in active context:
- replayed prompt tokens estimate:
- tool output lines kept:
`, target)
}

func renderReplayEvalChecklist(target string) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Replay Eval Checklist")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Target: %s\n", target)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Use this before enforcing a Miser rule.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- Same task result as the full-context baseline.")
	fmt.Fprintln(&b, "- No missing file, function, or requirement needed for correctness.")
	fmt.Fprintln(&b, "- No quality drop on sampled calls.")
	fmt.Fprintln(&b, "- `replay_eval >= 0.95`.")
	fmt.Fprintln(&b, "- Fallback route still works.")
	fmt.Fprintln(&b, "- Rollback is available behind a feature flag.")
	fmt.Fprintln(&b, "- Actual invoice allocated cost is tracked before and after.")
	return b.String()
}

func renderContextReplayMetrics(target string) string {
	return fmt.Sprintf(`{
  "target": %q,
  "rule_id": "miser.coding_agent_context_replay.v1",
  "metrics": [
    "input_tokens",
    "cached_input_tokens",
    "output_tokens",
    "context_files_count",
    "tool_output_lines",
    "replay_eval_score",
    "actual_invoice_allocated_cost_usd"
  ],
  "limits": {
    "max_context_files": 12,
    "max_replayed_prompt_tokens": 8000,
    "max_tool_output_lines": 200,
    "session_handoff_after": "20m or before switching tasks"
  }
}
`, target)
}

func yamlValue(raw, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		value = strings.Trim(value, "\"")
		return value
	}
	return ""
}
