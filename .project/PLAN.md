# Plan: AI-Agent Integration

Source: `.project/SPEC.md`. Ordered for independent shipping — pure-Go core first (fully testable), then agent artifacts, then installer. Spikes gate the two agent-specific installer targets.

## Now

**State** — Spikes resolved (Codex 0.133.0, OpenCode 1.15.10) and both agent-specific installer targets shipped: Codex plugin+marketplace (`90a2a09`, FR-18/19) and OpenCode `chat.message` advisory plugin (`15625d1`, FR-20). All earlier work intact (advisory hook, reader skill, config-dir/`--project`/idempotency FR-22–26). `go build` / `go vet` / `go test -race ./...` clean. Branch `feat/agent-integration`.

**Next** — Implement FR-17/21: the default (no-`--target`) run + `--target all` should reach every detected app (Claude, Codex, OpenCode, Cursor) in one pass — Codex is wired via `codexTargeted`, OpenCode via the optional-target path; add a default-run e2e asserting all four land, and refresh `--install-skill` help/usage text.

**Open questions** — Two live-only verifications (ask the user, can't auto-test): (a) Codex advisory fires in a real interactive session; (b) OpenCode toast (`client.tui.showToast`) fires in a real session with a working model. One doc-vs-observation to confirm if (b) fails: docs say OpenCode plugins load from `~/.config/opencode/plugins/` (plural, used here) but an earlier probe loaded from `plugin/` (singular). OQ-4: `tldt stats --daily` deferred. `.project/EXPLORE.md` is untracked scratch.

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
- [x] Codex `UserPromptSubmit` field confirmed `.prompt` (Claude-identical I/O) against 0.133.0; hooks are plugin-scoped, not standalone `hooks.json` (OQ-1)
- [x] OpenCode advisory hook confirmed `chat.message` (text from `parts`) against 1.15.10 / `@opencode-ai/plugin` 1.14.40 (OQ-2)

### Installer
- [x] Config-dir resolves with precedence `--config-dir` > `CLAUDE_CONFIG_DIR` > platform default — Claude target only; `CODEX_HOME` lands with Codex target (FR-22) — `eb3e736`
- [x] `--project` writes repo-local artifacts; hook registered in `.claude/settings.local.json` via `$CLAUDE_PROJECT_DIR`; no machine-specific path written to a committed file (FR-23, FR-24) — `336f73c`
- [x] Re-running the installer overwrites skill + hook files, replaces an old summarizing hook with the advisory one, and leaves exactly one tldt hook registration (FR-25, FR-26) — `8eb9b61`
- [ ] Default install reaches Claude / Codex / OpenCode / Cursor skill dirs; Cursor stays skill-only (FR-17, FR-21)
- [x] Codex target installs a `plugin.json` plugin (skill + advisory hook) under `<codexBase>/tldt-marketplace/`, registered via `codex plugin marketplace add` + `plugin add` (falls back to printing the commands); hook script shared with Claude (FR-18, FR-19) — `90a2a09`. Verified e2e into an isolated `CODEX_HOME` (installs + enables, idempotent). Live-TUI firing not auto-verifiable — Codex gates first dir-open behind a trust prompt and `codex exec` doesn't fire `UserPromptSubmit`.
- [x] OpenCode target installs skill + JS `chat.message` advisory plugin at `<opencode>/plugins/tldt-advisory.js` (per OpenCode docs); reads prompt text from `output.parts`, shells to `tldt`, surfaces a TUI toast via `client.tui.showToast` when flagged (FR-20) — `15625d1`. Verified e2e: files land at documented paths; plugin loads in opencode (probe). Live firing needs a real session with a model — ask the user to confirm.
