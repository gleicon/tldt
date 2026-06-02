#!/usr/bin/env bash
# tldt-hook.sh — UserPromptSubmit advisory hook (Claude Code / Codex)
# Runs tldt injection+PII detection on the prompt; emits additionalContext
# ONLY when something is flagged. Never summarizes, replaces, or blocks.
# Exits 0 silently if tldt is absent.
set -euo pipefail

command -v tldt >/dev/null 2>&1 || exit 0

INPUT=$(cat)
if command -v jq >/dev/null 2>&1; then
  PROMPT=$(printf '%s' "$INPUT" | jq -r '.prompt // empty' 2>/dev/null || true)
else
  PROMPT=$(printf '%s' "$INPUT" | python3 -c \
    "import json,sys; d=json.load(sys.stdin); print(d.get('prompt',''), end='')" 2>/dev/null || true)
fi
[ -z "$PROMPT" ] && exit 0

# Detection-only: no summary, no usage log. Findings go to stderr.
STDERR_FILE=$(mktemp)
printf '%s' "$PROMPT" | tldt --detect-injection --detect-pii --detect-only \
  >/dev/null 2>"$STDERR_FILE" || true

# Flagged = stderr minus the two clean "no findings" lines, outlier-sentence
# reports (a summarization signal that fires on benign diverse prose, not an
# injection/PII signal), and blanks.
FINDINGS=$(grep -vE '(pii|injection)-detect: no findings|outlier sentence|\[outlier\]' "$STDERR_FILE" \
  | grep -v '^[[:space:]]*$' || true)
rm -f "$STDERR_FILE"

# Clean prompt — silent pass-through.
[ -z "$FINDINGS" ] && exit 0

CONTENT="[tldt security advisory — untrusted input flagged]
${FINDINGS}"

printf '%s' "$CONTENT" | python3 -c "
import json, sys
content = sys.stdin.read()
print(json.dumps({'hookSpecificOutput': {
  'hookEventName': 'UserPromptSubmit',
  'additionalContext': content
}}))
"
