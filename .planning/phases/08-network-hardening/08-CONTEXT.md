# Phase 8: Network Hardening + Hook Defense - Context

**Gathered:** 2026-05-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 8 delivers two security layers:

1. **URL fetcher hardening** (`internal/fetcher/fetcher.go`): SSRF protection via hostname resolution + private IP blocking on every hop, plus a 5-hop redirect cap enforced via a custom `http.Client.CheckRedirect` function.
2. **Hook defense** (`internal/installer/hooks/tldt-hook.sh`): The embedded hook template is updated to invoke `tldt --sanitize --detect-injection --verbose` by default, split WARNING lines from token stats via grep, run an output guard that re-checks the summary before emitting, and compose everything into a labeled `additionalContext` structure.

No new Go packages. No new CLI flags on `tldt` binary. Changes confined to `internal/fetcher/fetcher.go` and `internal/installer/hooks/tldt-hook.sh` (the embedded source).

</domain>

<decisions>
## Implementation Decisions

### SSRF Block Architecture

- **D-01:** SSRF blocking covers **both the initial URL and every redirect hop**. The initial hostname is resolved via `net.LookupHost` before the request is made. Each redirect hop is also resolved and checked inside `CheckRedirect`. This prevents SSRF-by-redirect attacks where a public URL redirects to a private IP.
- **D-02:** Redirect cap and SSRF IP check share a **single combined `CheckRedirect` function**. One function: increment hop counter (reject at 6th hop = >5 redirects), resolve hostname, check IPs against block list. One place to audit.
- **D-03:** Fetch() returns **typed sentinel errors** for SSRF and redirect limit violations. Callers can use `errors.Is()` to distinguish them. Example: `var ErrSSRFBlocked = errors.New("SSRF blocked")` and `var ErrRedirectLimit = errors.New("redirect limit exceeded")`. Wraps with `fmt.Errorf("...: %w", ErrSSRFBlocked)` to include descriptive detail.

### Hook Stderr Splitting

- **D-04:** WARNING lines are separated from token stats using **grep on the WARNING prefix**. `tldt` already prefixes all detection warnings with `WARNING:`. The hook greps for `^WARNING` to extract warnings and `grep -v ^WARNING` for stats. Zero changes to the tldt binary required.
- **D-05:** When `--detect-injection` finds **no issues, the hook stays silent** — no "no injection detected" line is added to additionalContext. Clean runs produce no noise.

### Output Guard Mechanism

- **D-06:** The output guard re-runs detection with `echo "$SUMMARY" | tldt --detect-injection --sentences 999`. Using `--sentences 999` ensures all summary sentences pass through without re-summarization. Stdout is discarded; only stderr WARNING lines matter.
- **D-07:** If the output guard finds injection patterns in the summary, the hook **warns and still emits** — WARNING lines appended to additionalContext, summary still included. Advisory-only contract, consistent with SEC-07 / --detect-injection behavior.

### additionalContext Structure

- **D-08:** additionalContext uses **labeled sections** but only renders non-empty sections. Structure:
  ```
  [Token savings]
  ~X -> ~Y tokens (Z% reduction)

  [Security warnings - input]
  WARNING: ...

  [Security warnings - summary]
  WARNING: ...

  [Summary]
  ...
  ```
- **D-09:** When there are **no warnings** (clean input and clean summary), additionalContext contains only `[Token savings]` and `[Summary]` sections. Warning sections are omitted entirely. No noise on clean runs.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` §SEC-11, SEC-12, SEC-13, SEC-16 — 4 requirements for this phase
- `.planning/ROADMAP.md` §Phase 8 — Goal, success criteria, cross-cutting constraints, wave breakdown

### Fetcher (target file for 08-01)
- `internal/fetcher/fetcher.go` — Current Fetch() implementation; SSRF and redirect changes go here
- `internal/fetcher/fetcher_test.go` — Existing tests using httptest.NewServer; new SSRF + redirect tests follow same pattern

### Hook (target file for 08-02)
- `internal/installer/hooks/tldt-hook.sh` — Embedded hook template source; this is what gets changed (NOT a deployed copy)
- `internal/installer/embed.go` — go:embed wiring that packages the hook template into the binary

### Cross-cutting constraints (from ROADMAP.md — MUST follow)
- SSRF block must resolve hostname after each redirect, not just initial URL
- Cloud metadata ranges: `169.254.169.254/32` (IPv4) and `fd00:ec2::254/128` (IPv6)
- Redirect cap: 5 hops inclusive (5 allowed, 6th rejected) via `http.Client.CheckRedirect`
- All new fetcher tests use `httptest.NewServer` — no live network calls (memory feedback)
- Hook changes target the embedded template source, not any deployed copy

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/fetcher/fetcher.go` `Fetch()`: Existing scheme validation and `io.LimitReader` patterns — SSRF block fits naturally after scheme check (step 1b) and as a `CheckRedirect` on the `http.Client` (step 2).
- `internal/installer/hooks/tldt-hook.sh` `STATS_FILE` pattern: Existing mktemp+capture pattern is the anchor for the expanded stderr-split logic.
- Existing `grep` pattern in hook: hook already uses shell text processing; adding `grep ^WARNING` is consistent.

### Established Patterns
- **No live network in tests**: All URL tests use `httptest.NewServer`. New SSRF tests must use `httptest.NewServer` that redirects to a private IP address to simulate SSRF-by-redirect.
- **Typed errors in Go stdlib style**: `fetcher.go` currently uses `fmt.Errorf` strings. D-03 introduces typed sentinels — follow the `errors.New` + `fmt.Errorf("...: %w", ErrX)` pattern standard in Go.
- **Hook: advisory-only stderr**: `tldt` already separates stdout (summary) from stderr (stats/warnings). The hook must never let detection output bleed into stdout.
- **Embedded template, not deployed copy**: `internal/installer/hooks/tldt-hook.sh` is the source; `go:embed` in `embed.go` packages it. Changes here require `go generate` / rebuild to take effect.

### Integration Points
- `internal/fetcher/fetcher.go`: Add `blockPrivateIP(host string) error` helper + typed sentinels. Wire initial check after scheme validation. Pass combined `CheckRedirect` func to `http.Client`.
- `internal/installer/hooks/tldt-hook.sh`: Replace current `tldt --verbose` invocation with `tldt --sanitize --detect-injection --verbose`. Add stderr-split (grep WARNING / grep -v WARNING). Add output guard section. Replace flat REPLACEMENT string with labeled-section builder.

</code_context>

<specifics>
## Specific Ideas

- The `--sentences 999` trick for output guard is intentional: it makes the guard a pure detection pass without re-summarization side effects. If `--sentences` ever gets a dedicated "no-summarize" mode in a future phase, the hook can be simplified then.
- Labeled sections in additionalContext are rendered conditionally — the bash script should only emit a section header if its content is non-empty. Python3 JSON encoding (already used in the hook) handles the final output safely.
- `ErrSSRFBlocked` and `ErrRedirectLimit` as package-level vars in `internal/fetcher` make test assertions clean: `errors.Is(err, fetcher.ErrSSRFBlocked)`.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 8-Network Hardening + Hook Defense*
*Context gathered: 2026-05-02*
