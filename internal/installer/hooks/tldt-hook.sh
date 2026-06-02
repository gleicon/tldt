#!/usr/bin/env bash
# tldt-hook.sh — UserPromptSubmit advisory hook (Claude Code / Codex).
# Delegates entirely to `tldt --hook-output`, which reads the {prompt} stdin
# envelope, runs injection+PII detection, and emits a metadata-only advisory
# envelope ONLY when something is flagged. It never summarizes, replaces, or
# blocks, and fails safe (no output) on bad input. Exits 0 silently if tldt is
# absent, so the agent is never blocked.
command -v tldt >/dev/null 2>&1 || exit 0
exec tldt --hook-output
