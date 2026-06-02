# Plan: AI-Agent Integration

Source: `.project/SPEC.md`. Ordered for independent shipping — pure-Go core first (fully testable), then agent artifacts, then installer. Spikes gate the two agent-specific installer targets.

## Now

**State** — Design fully resolved (`/ds-explore` → `/ds-grill-me` → `/ds-spec`). Spec at `.project/SPEC.md`, roadmap below. Branch `feat/agent-integration` (2 doc commits: spec + roadmap). No implementation started — all roadmap tasks `[ ]`.

**Next** — First Core task: remove `--print-threshold` flag and `[hook] threshold` config (FR-5), confirming nothing references them.

**Open questions** — OQ-1 (Codex `UserPromptSubmit` prompt field) and OQ-2 (OpenCode user event) are live-environment spikes gating the Codex/OpenCode installer targets — defer until those tasks. OQ-4: decide whether `tldt stats --daily` is in the first cut. `.project/EXPLORE.md` is untracked scratch — keep or delete at will.

## Roadmap

### Core CLI (Go, no agent config required)
- [ ] `--print-threshold` flag and `[hook] threshold` config removed; nothing references them (FR-5)
- [ ] Each summary-producing run appends a counts-only `{ts,in,out,saved}` line to `~/.tldt/usage.jsonl`; a log-write failure never alters stdout, exit code, or the summarization (FR-9, FR-11, NFR-2, NFR-4)
- [ ] Usage logging honors `[stats] enabled=false` opt-out; the detection-only path writes no log line (FR-10, FR-12)
- [ ] `tldt stats` reports count / total in / total out / saved / percent; `--json` emits the same machine-readably; `--reset` clears the log; missing-or-empty log reports zeros and exits 0 (FR-13, FR-14, FR-15, FR-16)
- [ ] `tldt stats --daily` per-day breakdown — optional, confirm inclusion (FR-15.a, OQ-4)

### Agent artifacts (content)
- [ ] Advisory hook runs `tldt --detect-injection --detect-pii` on every prompt, emits `additionalContext` only when flagged, never summarizes or blocks, and exits 0 silently when tldt is absent (FR-1, FR-2, FR-3, FR-4, NFR-5)
- [ ] Reader skill accepts url / file / text → `tldt --url` / `-f` / pipe, returns summary + savings line; description steers "long prose for context, not verbatim/code/edit" (FR-6, FR-7, FR-8)

### Spikes (gate the agent-specific installer targets)
- [ ] Codex `UserPromptSubmit` stdin prompt-field name confirmed against a live build; hook extractor branches if it differs from Claude's `.prompt` (OQ-1)
- [ ] OpenCode user-message event for the advisory plugin confirmed against a live build (OQ-2)

### Installer
- [ ] Config-dir resolves with precedence `--config-dir` > `CLAUDE_CONFIG_DIR`/`CODEX_HOME` > platform default (FR-22)
- [ ] `--project` writes repo-local artifacts; hook registered in `.claude/settings.local.json` via `$CLAUDE_PROJECT_DIR`; no machine-specific path written to a committed file (FR-23, FR-24)
- [ ] Re-running the installer overwrites skill + hook files, replaces an old summarizing hook with the advisory one, and leaves exactly one tldt hook registration (FR-25, FR-26)
- [ ] Default install reaches Claude / Codex / OpenCode / Cursor skill dirs; Cursor stays skill-only (FR-17, FR-21)
- [ ] Codex target installs skill + advisory shell hook in Codex hooks config — after OQ-1 (FR-18, FR-19)
- [ ] OpenCode target installs skill + JS/TS advisory plugin — after OQ-2 (FR-20)
