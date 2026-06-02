# Plan: AI-Agent Integration

Source: `.project/SPEC.md`. Ordered for independent shipping — pure-Go core first (fully testable), then agent artifacts, then installer. Spikes gate the two agent-specific installer targets.

## Now

**State** — Core CLI section complete (4 commits on `feat/agent-integration`: FR-5 removal `229fbc3`, usage logging `e3483c5`, `[stats]` opt-out + `--detect-only` `558b517`, `tldt stats` subcommand `3fb4f55`). New `internal/usage` package (`Append`/`Read`/`Reset`); `ts` is RFC3339 (spec updated to match). All tests pass `go test -race ./...`.

**Next** — First Agent-artifacts task: rewrite `internal/installer/hooks/tldt-hook.sh` into the advisory hook — `tldt --detect-injection --detect-pii --detect-only`, emit `additionalContext` only when flagged, never summarize/block, exit 0 silently when tldt absent (FR-1/2/3/4, NFR-5). Crux: confirm tldt's stderr signal for clean vs flagged (`reportInjection`/`reportPII` in `main.go`) so the hook can distinguish them. Also update `installer_test.go` (still pins the old `--sanitize --detect-injection --verbose` invocation).

**Open questions** — OQ-1 (Codex `UserPromptSubmit` prompt field) and OQ-2 (OpenCode user event) are live-environment spikes gating the Codex/OpenCode installer targets — defer until those tasks. OQ-4: `tldt stats --daily` deferred (not in first cut, per decision). `.project/EXPLORE.md` is untracked scratch — keep or delete at will.

## Roadmap

### Core CLI (Go, no agent config required)
- [x] `--print-threshold` flag and `[hook] threshold` config removed; nothing references them (FR-5)
- [x] Each summary-producing run appends a counts-only `{ts,in,out,saved}` line to `~/.tldt/usage.jsonl`; a log-write failure never alters stdout, exit code, or the summarization (FR-9, FR-11, NFR-2, NFR-4)
- [x] Usage logging honors `[stats] enabled=false` opt-out; the detection-only path writes no log line (FR-10, FR-12)
- [x] `tldt stats` reports count / total in / total out / saved / percent; `--json` emits the same machine-readably; `--reset` clears the log; missing-or-empty log reports zeros and exits 0 (FR-13, FR-14, FR-15, FR-16)
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
