# Phase 6: AI Integration - Context

**Gathered:** 2026-05-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 6 delivers tldt as an installable Claude Code skill + auto-trigger hook.

- A `/tldt` Claude Code skill file: user invokes it on selected text; tldt summarizes via stdin pipe; summary + token savings line appear inline in conversation.
- An auto-trigger hook (bash): fires on `UserPromptSubmit` when prompt token count exceeds a configurable threshold; replaces the prompt with the tldt summary + savings line; passes through silently if tldt is not in PATH.
- `tldt --install-skill` CLI flag (self-contained): writes skill + hook files to correct Claude Code (and optionally other coding app) directories; wired to `make install-skill` in Makefile.
- `tldt --print-threshold` CLI flag: prints the configured hook token threshold to stdout (used by the hook to read threshold from `~/.tldt.toml`).
- Skill and hook template files live in `skills/` and `hooks/` at repo root; embedded into the binary at build time via `go:embed`.

</domain>

<decisions>
## Implementation Decisions

### Skill File Format
- **D-01:** Skill invokes tldt via **stdin pipe** — `echo "$text" | tldt`. Uses existing pipe-safe design; no new input modes.
- **D-02:** User triggers via **`/tldt` slash command** inside Claude Code. Single trigger mechanism; no keybinding.
- **D-03:** Skill passes **no explicit flags** to tldt — reads user's `~/.tldt.toml` for algorithm/sentences/format defaults.
- **D-04:** Skill **captures stderr and shows it** in conversation — token savings line (`~X -> ~Y tokens (Z% reduction)`) is the key value prop and must appear before the summary.

### Hook Trigger Architecture
- **D-05:** Hook script written in **bash** — zero dependencies, runs anywhere Go is installed.
- **D-06:** When threshold exceeded, hook **replaces prompt with summary + savings line** — original text discarded. No pass-through of original.
- **D-07:** Hook receives prompt text via **stdin** — consistent with tldt's pipe-safe design.
- **D-08:** If `tldt` not found in PATH, hook **exits 0 silently** (no-op) — user session never breaks.

### Threshold Configuration
- **D-09:** Token threshold lives in **`~/.tldt.toml` under a `[hook]` section** — e.g., `threshold = 2000`. Consistent with Phase 5 config design.
- **D-10:** Hook reads threshold by calling **`tldt --print-threshold`** — clean abstraction; hook doesn't parse TOML directly.
- **D-11:** Default threshold when no config exists: **2000 tokens** (~8KB text). Matches ROADMAP.md success criteria.
- **D-12:** `internal/config` Config struct gains a `[hook]` section with a `Threshold int` field (default 2000). `--print-threshold` prints it to stdout.

### Install UX
- **D-13:** Primary install mechanism: **`tldt --install-skill` CLI flag** — self-contained binary writes skill + hook files to target directories. Wired to `make install-skill` in Makefile (`make install-skill` calls `go run ./cmd/tldt --install-skill` or the installed binary).
- **D-14:** Skill + hook templates embedded in binary via **`go:embed`** — install works from any PATH location, not just the repo clone.
- **D-15:** Source files for inspect/audit at **`skills/` and `hooks/` at repo root**.
- **D-16:** Default install targets: `~/.claude/skills/tldt/SKILL.md` + hook registered in `~/.claude/settings.json` hooks array.
- **D-17:** Also accept **`--skill-dir <path>` flag** for non-default install targets.
- **D-18:** **Researcher must investigate other coding assistant install targets** (OpenCode, Cursor, Copilot, Zed, etc.). GSD uses a multi-target approach — review how GSD skill installation handles multiple coding apps and adapt the same pattern. The `--install-skill` command should support a `--target <app>` option or install to all detected apps.

### Claude's Discretion
- Hook bash token counting: use `wc -c` and divide by 4 (chars/4 heuristic) — same heuristic as tldt's `TokenizeSentences`. Claude can implement this inline in the hook script.
- Skill file SKILL.md structure: follow standard Claude Code skill format (YAML frontmatter + bash blocks). Claude can determine the exact schema from Claude Code docs.
- JSON structure for hook registration in `~/.claude/settings.json`: Claude should research the exact format.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` §AI — AI-01, AI-02, AI-03, AI-04 (4 requirements for this phase)
- `.planning/ROADMAP.md` §Phase 6 — Success criteria, phase goal, depends-on chain

### Existing Config System (Phase 5)
- `internal/config/config.go` — Config struct, TOML loading, defaults; extend with `[hook]` section
- `cmd/tldt/main.go` — Flag wiring pattern; add `--print-threshold` and `--install-skill` here

### Codebase Patterns
- `cmd/tldt/main.go` — resolveInputBytes() precedence; stdin pipe path; token stats → stderr only
- `internal/summarizer/` — TokenizeSentences() for token counting (chars/4 heuristic)

### Research Required (no files yet)
- Claude Code skill file format and SKILL.md schema — researcher must fetch from official docs
- Claude Code `settings.json` hooks array format — researcher must verify exact JSON structure
- OpenCode / Cursor / Copilot / Zed skill/hook install paths — researcher must investigate; reference how GSD handles multi-target skill installation

No external specs exist yet for the skill/hook format — researcher must gather these before planning.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go`: Config struct + TOML loader — extend with `Hook struct { Threshold int }` under `[hook]` TOML section.
- `cmd/tldt/main.go` flag pattern: all flags declared at top of `main()`, config loaded early, `flag.Visit` override detection — follow same pattern for `--print-threshold` and `--install-skill`.
- `internal/summarizer/TokenizeSentences()` — token estimate used in hook threshold check (bash reimplements chars/4, Go binary exposes via `--print-threshold`).

### Established Patterns
- **stdout = summary only; stderr = stats/errors** — skill must redirect stderr to capture savings line; hook must not pollute stdout.
- **Pipe-safe by default** — skill and hook both use stdin pipe as the canonical input path.
- **go:embed** not yet used in this repo — Phase 6 introduces it for skill/hook templates.
- **Makefile targets** (`make install`, `make test`, `make build`) — add `make install-skill` consistent with existing targets.

### Integration Points
- `cmd/tldt/main.go` `main()`: add `--install-skill` and `--print-threshold` flags; dispatch before normal summarization flow.
- `internal/config/config.go` `Config` struct + `Load()`: add `Hook` sub-struct; ensure `--print-threshold` reads from loaded config.
- `skills/` and `hooks/` at repo root: new directories; skill + hook template files go here; `//go:embed skills/* hooks/*` in a new `embed.go` file.

</code_context>

<specifics>
## Specific Ideas

- The auto-trigger hook mirrors tldt's core value prop: show token savings before/after. The savings line format `~X -> ~Y tokens (Z% reduction)` already exists in main.go's stderr output — the hook captures this and prepends it to the replacement prompt.
- `make install-skill` should be simple: `$(INSTALL_BIN) --install-skill` (where `INSTALL_BIN` is the built binary). Researcher should check if other Makefiles in the ecosystem do it differently.
- Multi-app install: researcher should look at GSD's skill install mechanism as a reference for detecting which coding apps are installed and routing the skill/hook files accordingly.
- `tldt --install-skill --target opencode` or `tldt --install-skill` (detects all) — exact CLI design left to researcher + planner.

</specifics>

<deferred>
## Deferred Ideas

- MCP server mode — deferred to v3+ (already in STATE.md deferred list)
- Clipboard auto-read (`pbpaste`/`xclip`) — deferred to v3+ (already in STATE.md deferred list)
- `--url` authentication headers / cookie support — deferred to v3+
- TOML validation/lint command (`tldt --check-config`) — deferred to v3+

None of the above came up in this discussion — all were pre-existing deferred items.

</deferred>

---

*Phase: 6-AI Integration*
*Context gathered: 2026-05-02*
