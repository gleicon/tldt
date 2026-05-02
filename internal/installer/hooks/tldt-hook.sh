#!/usr/bin/env bash
# tldt-hook.sh — UserPromptSubmit hook for Claude Code
# Auto-summarizes prompts that exceed the configured token threshold.
# Exits 0 silently if tldt is not in PATH (D-08).
# Installed by: tldt --install-skill

set -euo pipefail

# Require tldt in PATH — exit 0 silently if absent (D-08)
if ! command -v tldt >/dev/null 2>&1; then
  exit 0
fi

# Read JSON from stdin (Claude Code sends event as JSON on stdin, D-07)
INPUT=$(cat)

# Extract prompt text — jq primary, python3 fallback (Pitfall 3: control chars in .prompt)
if command -v jq >/dev/null 2>&1; then
  PROMPT=$(printf '%s' "$INPUT" | jq -r '.prompt // empty' 2>/dev/null || true)
else
  PROMPT=$(printf '%s' "$INPUT" | python3 -c \
    "import json,sys; d=json.load(sys.stdin); print(d.get('prompt',''), end='')" 2>/dev/null || true)
fi

# Empty prompt — no-op
if [ -z "$PROMPT" ]; then
  exit 0
fi

# Token estimate: chars / 4 heuristic — same as tldt's TokenizeSentences (D-10)
# wc -c counts bytes; slight over-estimate for multi-byte UTF-8 — acceptable (conservative)
CHAR_COUNT=$(printf '%s' "$PROMPT" | wc -c | tr -d ' ')
TOKEN_ESTIMATE=$(( CHAR_COUNT / 4 ))

# Get threshold from tldt config — reads ~/.tldt.toml [hook] threshold (D-10)
# Falls back to 2000 if tldt --print-threshold fails (D-11)
THRESHOLD=$(tldt --print-threshold 2>/dev/null || echo "2000")

# Below threshold — pass through silently (no output = Claude proceeds normally)
if [ "$TOKEN_ESTIMATE" -lt "$THRESHOLD" ]; then
  exit 0
fi

# Summarize — capture stderr (savings line) separately from stdout (summary)
# MUST use --verbose: tldt only prints token stats to stderr when --verbose is set (Pitfall 1)
STATS_FILE=$(mktemp)
SUMMARY=$(printf '%s' "$PROMPT" | tldt --verbose 2>"$STATS_FILE" || true)
SAVINGS=$(cat "$STATS_FILE")
rm -f "$STATS_FILE"

# If summarization failed or returned empty — pass through silently (D-08 spirit)
if [ -z "$SUMMARY" ]; then
  exit 0
fi

# Build replacement context: savings line first, then summary (D-04, D-06)
REPLACEMENT="${SAVINGS}

${SUMMARY}"

# Output hookSpecificOutput JSON for Claude Code to inject as additionalContext (D-06)
# Use python3 for JSON encoding — bash string interpolation breaks on special chars (Pitfall 2)
printf '%s' "$REPLACEMENT" | python3 -c "
import json, sys
content = sys.stdin.read()
output = {
  'hookSpecificOutput': {
    'hookEventName': 'UserPromptSubmit',
    'additionalContext': content
  }
}
print(json.dumps(output))
"
