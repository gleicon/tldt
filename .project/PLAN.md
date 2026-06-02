# Plan: AI-Agent Integration

Source: `.project/SPEC.md`. Ordered for independent shipping — pure-Go core first (fully testable), then agent artifacts, then installer. Spikes gate the two agent-specific installer targets.

## Now

**State** — All spike-independent work done. 6 more commits on `feat/agent-integration`: advisory hook `63eda3e`, reader skill `0974df1`, config-dir resolution `eb3e736`, `--project` `336f73c`, robust upgrade dedup `8eb9b61` (Core CLI commits from prior session unchanged). Agent-artifacts section complete; installer config-dir / `--project` / re-run-idempotency (FR-22/23/24/25/26) complete. `go test -race ./...` + `go vet` clean.

**Next** — Run the two live-environment spikes (OQ-1, OQ-2); they gate everything left. Until then nothing further can be built from the repo alone. After OQ-1: Codex installer target (skill + advisory shell hook in Codex hooks config, FR-18/19). After OQ-2: OpenCode JS/TS advisory plugin (FR-20). Then default multi-target reaching Codex/OpenCode (FR-17/21).

**Open questions** — OQ-1: Codex `UserPromptSubmit` stdin prompt-field name (does it match Claude's `.prompt`?) — confirm against a live Codex build; hook extractor branches if it differs. OQ-2: OpenCode user-message event name for the advisory plugin — confirm against a live OpenCode build. Both need a running build to probe stdin/event contracts. OQ-4: `tldt stats --daily` deferred (not in first cut). `.project/EXPLORE.md` is untracked scratch — keep or delete at will.

## Roadmap

### Core CLI (Go, no agent config required)
- [x] `--print-threshold` flag and `[hook] threshold` config removed; nothing references them (FR-5)
- [x] Each summary-producing run appends a counts-only `{ts,in,out,saved}` line to `~/.tldt/usage.jsonl`; a log-write failure never alters stdout, exit code, or the summarization (FR-9, FR-11, NFR-2, NFR-4)
- [x] Usage logging honors `[stats] enabled=false` opt-out; the detection-only path writes no log line (FR-10, FR-12)
- [x] `tldt stats` reports count / total in / total out / saved / percent; `--json` emits the same machine-readably; `--reset` clears the log; missing-or-empty log reports zeros and exits 0 (FR-13, FR-14, FR-15, FR-16)
- [ ] `tldt stats --daily` per-day breakdown — optional, confirm inclusion (FR-15.a, OQ-4)

### Agent artifacts (content)
- [x] Advisory hook runs `tldt --detect-injection --detect-pii --detect-only` on every prompt, emits `additionalContext` only when flagged, never summarizes or blocks, and exits 0 silently when tldt is absent (FR-1, FR-2, FR-3, FR-4, NFR-5) — `63eda3e`
- [x] Reader skill accepts url / file / text → `tldt --url` / `-f` / pipe, returns summary + savings line; description steers "long prose for context, not verbatim/code/edit" (FR-6, FR-7, FR-8) — `0974df1`

### Spikes (gate the agent-specific installer targets)
- [ ] Codex `UserPromptSubmit` stdin prompt-field name confirmed against a live build; hook extractor branches if it differs from Claude's `.prompt` (OQ-1)
- [ ] OpenCode user-message event for the advisory plugin confirmed against a live build (OQ-2)

### Installer
- [x] Config-dir resolves with precedence `--config-dir` > `CLAUDE_CONFIG_DIR` > platform default — Claude target only; `CODEX_HOME` lands with Codex target (FR-22) — `eb3e736`
- [x] `--project` writes repo-local artifacts; hook registered in `.claude/settings.local.json` via `$CLAUDE_PROJECT_DIR`; no machine-specific path written to a committed file (FR-23, FR-24) — `336f73c`
- [x] Re-running the installer overwrites skill + hook files, replaces an old summarizing hook with the advisory one, and leaves exactly one tldt hook registration (FR-25, FR-26) — `8eb9b61`
- [ ] Default install reaches Claude / Codex / OpenCode / Cursor skill dirs; Cursor stays skill-only (FR-17, FR-21)
- [ ] Codex target installs skill + advisory shell hook in Codex hooks config — after OQ-1 (FR-18, FR-19)
- [ ] OpenCode target installs skill + JS/TS advisory plugin — after OQ-2 (FR-20)
