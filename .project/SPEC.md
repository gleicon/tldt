# Specification: AI-Agent Integration (skill + advisory hook + savings tracker)

## Problem
tldt ships a Claude Code integration today — a `/tldt` skill and a `UserPromptSubmit` hook that summarizes long prompts — but the hook's premise is broken: Claude Code's `UserPromptSubmit` injects `additionalContext` *alongside* the prompt and cannot replace prompt text, so summarizing a prompt only *adds* tokens (original prompt + summary + warnings). The maintainer wants integration that (a) saves tokens honestly, (b) works across Claude Code, Codex, and OpenCode, (c) installs globally / per-project / into a chosen config dir, and (d) reports cumulative token savings the way `rtk gain` does. The reframe: real savings happen only when tldt is the *ingestion path* (its summary is the only thing that enters context), so token reduction must be agent-invoked, while the prompt hook is demoted to a security advisory.

## Scope
**In scope**
- Repurpose the `UserPromptSubmit` hook from summarization to **injection/PII advisory only**.
- Rework the skill into a **full reader** (`/tldt <url|file|text>`) whose description steers the model to use tldt before ingesting long prose-for-context.
- Add a **`tldt stats`** subcommand backed by a new counts-only usage log (`~/.tldt/usage.jsonl`).
- Add per-summarization usage logging (default-on, opt-out).
- Extend the installer: **Codex** target (skill + shell hook), **OpenCode** JS/TS plugin (auto-advisory), `--config-dir`, `CLAUDE_CONFIG_DIR`/`CODEX_HOME` auto-honor, and `--project` (gitignored, `$CLAUDE_PROJECT_DIR`-relative).
- Retire the now-dead summarization-trigger machinery: `--print-threshold` flag and `[hook] threshold` config.

**Out of scope**
- Any PreToolUse hook that forces/blocks tldt on tool reads (rejected — lossy on verbatim-needed content).
- A Claude Code marketplace plugin (`.claude-plugin/plugin.json`).
- An MCP server.
- Changing the extractive summarization algorithms or the `pkg/tldt` core API semantics.
- Cursor parity beyond the existing skill-only install.
- Logging or transmitting any prompt/document **content** (only token counts).

## Users
- **Maintainer / primary user (Claude Code with a non-default config dir, `~/.claude-personal`):** wants tldt to install into their actual config dir, fire a security advisory automatically, be reachable as a reader skill, and show real cumulative token savings.
- **Coding agents (Claude Code, Codex, OpenCode):** need a skill they can invoke when long prose should be summarized before entering context, and (Claude/Codex/OpenCode) an automatic advisory on untrusted prompt input.
- **Library consumers of `pkg/tldt`:** must see zero change to existing public API semantics.

## Functional Requirements

### Advisory hook (replaces prompt summarization)
- **FR-1:** The `UserPromptSubmit` hook SHALL run `tldt --detect-injection --detect-pii` against the submitted prompt on every prompt.
- **FR-2:** The hook SHALL emit `additionalContext` **only when** injection or PII patterns are detected; on a clean prompt it SHALL produce no output and exit 0 (silent pass-through).
- **FR-3:** The hook SHALL NOT summarize, replace, or block the prompt.
- **FR-4:** The hook SHALL exit 0 silently when the `tldt` binary is absent from PATH.
- **FR-5:** The system SHALL remove the `--print-threshold` CLI flag and the `[hook] threshold` config key, and the hook SHALL NOT depend on any token threshold.

### Reader skill
- **FR-6:** The installed skill SHALL accept a URL, a file path, or inline text and route to `tldt --url`, `tldt -f`, or piped stdin respectively.
- **FR-7:** The skill SHALL return the extractive summary together with the token-savings line.
- **FR-8:** The skill's `description` metadata SHALL instruct the model to invoke it before ingesting long prose needed only for context, and SHALL warn against using it for code, files to be edited, or content needed verbatim.

### Savings tracker
- **FR-9:** Each tldt invocation that produces a summary SHALL append one JSON line `{ts, in, out, saved}` (token counts only, no content) to the usage log at `~/.tldt/usage.jsonl`.
- **FR-10:** Usage logging SHALL be enabled by default and SHALL be disabled when `[stats] enabled = false` is set in config.
- **FR-11:** A failed usage-log write SHALL NOT alter stdout, SHALL NOT change the process exit code, and SHALL NOT fail the summarization.
- **FR-12:** The advisory/detection-only path SHALL NOT write a usage-log entry (it produces no summary).
- **FR-13:** `tldt stats` SHALL read the usage log and print aggregate totals: invocation count, total input tokens, total output tokens, tokens saved, and percent reduction.
- **FR-14:** `tldt stats --json` SHALL emit the same aggregates as machine-readable JSON on stdout.
- **FR-15:** `tldt stats --reset` SHALL clear the usage log.
- **FR-15.a:** `tldt stats --daily` MAY emit a per-day breakdown (optional).
- **FR-16:** When the usage log is absent or empty, `tldt stats` SHALL report zeroed totals and exit 0 (not an error).

### Installer — targets & ergonomics
- **FR-17:** `tldt --install-skill` SHALL install the reader skill to each detected target: Claude Code, Codex, OpenCode, and Cursor.
- **FR-18:** For Claude Code, the installer SHALL install the skill and register the advisory `UserPromptSubmit` shell hook.
- **FR-19:** For Codex, the installer SHALL install the skill and register the advisory `UserPromptSubmit` shell hook in Codex's hook config (`~/.codex/hooks.json` or `config.toml`; `.codex/` for project scope).
- **FR-20:** For OpenCode, the installer SHALL install the skill and a JS/TS plugin that runs the advisory by shelling to `tldt` on the user-message event.
- **FR-21:** For Cursor, the installer SHALL install the skill only (no hook), unchanged from current behavior.
- **FR-22:** The installer SHALL resolve the target config directory in this precedence: explicit `--config-dir <path>` > `CLAUDE_CONFIG_DIR`/`CODEX_HOME` env var (per target) > the platform default (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.cursor`).
- **FR-23:** `tldt --install-skill --project` SHALL write into the current repository (`./.claude/`, `./.codex/`) instead of the user config dir.
- **FR-24:** A `--project` install SHALL register hooks in the gitignored local settings file (`.claude/settings.local.json`) and SHALL express the hook command path via `$CLAUDE_PROJECT_DIR` so the path is portable and not committed.
- **FR-25:** The installer SHALL remain idempotent: re-running SHALL overwrite the skill and hook script files and SHALL NOT create duplicate hook registrations in settings.
- **FR-26:** Re-running the installer SHALL overwrite a previously installed summarizing hook script with the new advisory hook script (upgrade path).

## Non-Functional Requirements
- **NFR-1:** Latency — the advisory hook SHALL add no more than one `tldt` subprocess invocation per prompt; no network calls (detection is local).
- **NFR-2:** Concurrency — concurrent appends to `usage.jsonl` from parallel tldt processes SHALL rely on POSIX `O_APPEND` atomicity for single-line writes; the writer SHALL emit one `write` of a newline-terminated record.
- **NFR-3:** Data retention — the usage log persists until `tldt stats --reset`; it contains token counts and timestamps only, never source/prompt content.
- **NFR-4:** Pipe safety — only summary output goes to stdout on the summarization path; stats/log/advisory output never contaminates a piped stdout (existing project invariant preserved).
- **NFR-5:** Availability — every integration artifact SHALL degrade silently (exit 0, no output) when `tldt` is not on PATH.

## Interfaces
**CLI**
- `cat <file> | tldt` / `tldt --url <url>` / `tldt -f <file>` — unchanged summarization entry points (now also append a usage-log record).
- `tldt stats [--json] [--reset] [--daily]` — new subcommand; aggregate savings report.
- `tldt --install-skill [--target claude|codex|cursor|opencode|all] [--config-dir <path>] [--project] [--skill-dir <path>]` — extended installer.
- **Removed:** `tldt --print-threshold`.

**Files / data formats at boundaries**
- `~/.tldt/usage.jsonl` — append-only JSON Lines, one object per summary: `{"ts": <unix-seconds int>, "in": <int>, "out": <int>, "saved": <int>}`.
- `~/.tldt.toml` — gains `[stats] enabled = <bool>`; loses `[hook] threshold`.
- Claude/Codex `settings.json`/`hooks.json` — advisory hook registration (command type, JSON-on-stdin contract).
- OpenCode plugin module (JS/TS) under the OpenCode plugins dir.

**External systems**
- Host coding agents: Claude Code, Codex CLI, OpenCode, Cursor (filesystem config dirs only; no network integration).

## Constraints
- **Language/runtime:** Go 1.22+ (project targets Go 1.26.x); installer assets embedded via `embed.go`. OpenCode plugin is a JS/TS artifact (the only non-Go deliverable).
- **Extractive-only:** no abstractive/generative summarization; `tldt stats` "saved" is derived from the existing chars/4 token heuristic, consistent with current `--verbose` accounting.
- **CLI-depends-on-library:** `cmd/tldt` routes core operations through `pkg/tldt`; usage logging is wired at the CLI/command layer, not buried in `internal/summarizer`.
- **Security at the boundary:** the reader skill's URL path uses the existing hardened fetcher (SSRF blocklist, byte cap, redirect cap); the advisory hook treats prompt text as untrusted input.
- **Surgical/simple:** keep the flag-based core untouched; add only `tldt stats` as a subcommand; no rtk-style full subcommand restructure; no ASCII graph.
- **Forbidden approaches:** PreToolUse force/block hook; logging prompt/document content; marketplace plugin; MCP server; committing machine-specific hook paths.

## Acceptance Criteria
- **AC-1:** Given a prompt containing an injection pattern, when the `UserPromptSubmit` hook runs, then it emits `additionalContext` naming the detected injection/PII and does not summarize or block the prompt. *(FR-1, FR-2, FR-3)*
- **AC-2:** Given a clean prompt, when the hook runs, then it produces no stdout and exits 0. *(FR-2)*
- **AC-3:** Given `tldt` is not on PATH, when any installed hook/plugin fires, then it exits 0 with no output. *(FR-4, NFR-5)*
- **AC-4:** Given the built binary, when `tldt --print-threshold` is run, then it is rejected as an unknown flag; and `[hook] threshold` in config is ignored with no error. *(FR-5)*
- **AC-5:** Given a URL, a file, and a text argument, when each is passed to the installed skill, then tldt summarizes via `--url`, `-f`, and stdin respectively and returns summary + savings line. *(FR-6, FR-7)*
- **AC-6:** Given the installed skill metadata, when inspected, then its description names the "long prose for context" use and the "not for verbatim/code/edit" exclusion. *(FR-8)*
- **AC-7:** Given default config, when a summary is produced, then exactly one `{ts,in,out,saved}` line is appended to `~/.tldt/usage.jsonl` with no content fields. *(FR-9)*
- **AC-8:** Given `[stats] enabled = false`, when a summary is produced, then no usage-log line is written. *(FR-10)*
- **AC-9:** Given a read-only/unwritable usage-log path, when a summary is produced, then stdout is the summary only, exit code is success, and the summarization succeeds. *(FR-11, NFR-4)*
- **AC-10:** Given a prompt processed by the advisory (detection-only) path, when it completes, then no usage-log line is written. *(FR-12)*
- **AC-11:** Given a usage log with known records, when `tldt stats` runs, then it prints count, total in, total out, tokens saved, and percent reduction matching the records; and `--json` emits the same values as JSON. *(FR-13, FR-14)*
- **AC-12:** Given a populated usage log, when `tldt stats --reset` runs, then the log is cleared and a subsequent `tldt stats` reports zeros. *(FR-15, FR-16)*
- **AC-13:** Given no usage log file, when `tldt stats` runs, then it reports zeroed totals and exits 0. *(FR-16)*
- **AC-14:** Given a default install run, when `tldt --install-skill` completes, then the reader skill exists in each detected target's skills dir (Claude, Codex, OpenCode, Cursor). *(FR-17, FR-21)*
- **AC-15:** Given a Claude and a Codex target, when install completes, then each has the advisory `UserPromptSubmit` hook registered in its respective settings/hooks config with a `command`-type, JSON-stdin entry. *(FR-18, FR-19)*
- **AC-16:** Given an OpenCode target, when install completes, then the skill and a JS/TS advisory plugin are present in the OpenCode config dirs. *(FR-20)*
- **AC-17:** Given `CLAUDE_CONFIG_DIR=/tmp/cc` set and no `--config-dir`, when installing for Claude, then artifacts are written under `/tmp/cc`, not `~/.claude`. *(FR-22)*
- **AC-18:** Given `--config-dir /x` and `CLAUDE_CONFIG_DIR=/y` both set, when installing, then `/x` wins. *(FR-22)*
- **AC-19:** Given `--project` in a repo, when install completes, then the hook is registered in `.claude/settings.local.json`, the hook command references `$CLAUDE_PROJECT_DIR`, and no absolute machine path is written to a committed file. *(FR-23, FR-24)*
- **AC-20:** Given a target already containing a prior tldt install, when the installer re-runs, then skill and hook script files are overwritten, the new advisory hook script replaces the old summarizing one, and settings contain exactly one tldt hook registration. *(FR-25, FR-26)*

**Coverage**
- FR-1 → AC-1 · FR-2 → AC-1, AC-2 · FR-3 → AC-1 · FR-4 → AC-3 · FR-5 → AC-4
- FR-6 → AC-5 · FR-7 → AC-5 · FR-8 → AC-6
- FR-9 → AC-7 · FR-10 → AC-8 · FR-11 → AC-9 · FR-12 → AC-10 · FR-13 → AC-11 · FR-14 → AC-11 · FR-15 → AC-12 · FR-15.a → (optional; AC-11 pattern) · FR-16 → AC-12, AC-13
- FR-17 → AC-14 · FR-18 → AC-15 · FR-19 → AC-15 · FR-20 → AC-16 · FR-21 → AC-14 · FR-22 → AC-17, AC-18 · FR-23 → AC-19 · FR-24 → AC-19 · FR-25 → AC-20 · FR-26 → AC-20

## Technical Profile
- **Primary language:** Go (module `github.com/gleicon/tldt`); secondary JS/TS for the OpenCode plugin only.
- **Runtime target:** darwin/arm64 and other Go-supported platforms; CLI binary.
- **Build toolchain:** `make build`/`make install`; embedded assets via `internal/installer/embed.go`; release via `.goreleaser.yaml`.
- **Testing framework:** `go test -race ./...`; table-driven subtests; `httptest` for fetch; subprocess tests in `cmd/tldt/main_test.go`; no real network/filesystem in unit tests (use temp dirs for usage-log tests).

## Open Questions
- **OQ-1 (impl):** Codex's `UserPromptSubmit` stdin JSON field name for the prompt — confirm whether it is `.prompt` (as Claude) or differs; the shared hook script's extractor may need a per-agent branch.
- **OQ-2 (impl):** OpenCode's exact event for "user submitted a prompt" — likely `message.updated` filtered to the user role; confirm against a live OpenCode build before finalizing the plugin.
- **OQ-3:** Should `~/.tldt/usage.jsonl` honor `XDG_STATE_HOME` (e.g. `~/.local/state/tldt/`) instead of a home-dir dotdir? Current decision: `~/.tldt/`; revisit only if it conflicts with packaging expectations.
- **OQ-4:** Is `tldt stats --daily` in the first cut (FR-15.a) or deferred? Default: deferred unless cheap.
- **OQ-5:** Does the reader skill cap input size before fetching very large URLs, or rely solely on the fetcher's existing 5MB byte cap? Default: rely on the fetcher cap.
