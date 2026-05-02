---
phase: 04-url-input
verified: 2026-05-02T00:00:00Z
status: passed
score: 12/12 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run `tldt --url https://en.wikipedia.org/wiki/Extractive_summarization --sentences 3` against a live URL"
    expected: "3 sentences printed to stdout, no HTML markup visible, stderr may contain stats"
    why_human: "All automated tests use httptest.NewServer; live network behavior (TLS, real DNS, real redirects) cannot be verified without an outbound connection"
  - test: "Run `tldt --url https://httpstat.us/301 --sentences 3` to verify redirect following on a real host"
    expected: "Summary printed to stdout, exit 0 — redirect transparently followed"
    why_human: "httpstat.us/301 is a real redirect service; test requires live network"
---

# Phase 4: URL Input Verification Report

**Phase Goal:** Enable users to feed a URL as input (`tldt --url https://...`) so the CLI fetches the page, strips HTML boilerplate, and summarizes the article text — no manual copy-paste required.
**Verified:** 2026-05-02T00:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | fetcher.Fetch(url, timeout, maxBytes) returns cleaned article text for a valid HTML URL | VERIFIED | `TestFetch_OK` passes; `fetcher.go:74` calls `readability.FromReader` then `strings.TrimSpace(article.TextContent)` |
| 2 | fetcher.Fetch returns a descriptive error for non-2xx HTTP status codes | VERIFIED | `fetcher.go:56` checks `resp.StatusCode < 200 \|\| resp.StatusCode >= 300`; `TestFetch_404` passes |
| 3 | fetcher.Fetch follows HTTP redirects transparently and returns content from the final destination | VERIFIED | `http.Client` follows redirects by default; `TestFetch_Redirect` passes |
| 4 | fetcher.Fetch rejects non-http(s) URL schemes before making any network call | VERIFIED | `fetcher.go:33-35` scheme allowlist; `TestFetch_InvalidScheme` passes; error text "unsupported URL scheme" |
| 5 | fetcher.Fetch rejects responses with Content-Type that does not contain text/html | VERIFIED | `fetcher.go:62-64` `strings.Contains(ct, "text/html")` guard; `TestFetch_NonHTMLContentType` passes |
| 6 | Response body is capped at maxBytes via io.LimitReader before any read | VERIFIED | `fetcher.go:68` `limited := io.LimitReader(resp.Body, maxBytes)` — cap applied before `readability.FromReader` |
| 7 | go test ./internal/fetcher/... passes with zero real network calls | VERIFIED | 5/5 tests pass; all use `httptest.NewServer`; no external URLs in test file |
| 8 | tldt --url <url> fetches the page and prints an extractive summary to stdout | VERIFIED | `TestMain_URLFlag_ServesHTML` passes; `resolveInputBytes` URL branch wired at `main.go:204-209` |
| 9 | tldt --url <url> \| wc -l produces only summary lines on stdout — no HTML, no headers | VERIFIED | `resolveInputBytes` returns `[]byte(text)` (plain text from readability); no HTML returned to stdout path |
| 10 | tldt --url <404-url> exits non-zero and prints a descriptive error containing the status code to stderr | VERIFIED | `TestMain_URLFlag_404` passes; error propagates via `fmt.Fprintln(os.Stderr, err)` then `os.Exit(1)` |
| 11 | tldt --url <redirect-url> follows the redirect and still produces a summary | VERIFIED | `TestFetch_Redirect` verifies `http.Client` redirect following; wired through same `fetcher.Fetch` path |
| 12 | go test ./... passes including all pre-existing tests (no regressions) | VERIFIED | `go test ./... -count=1` → 199/199 PASS across 4 packages |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/fetcher/fetcher.go` | `Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error)` | VERIFIED | Exists, 85 lines, substantive implementation; imported and called from `main.go` |
| `internal/fetcher/fetcher_test.go` | Unit tests using httptest.NewServer; contains TestFetch_OK | VERIFIED | 5 tests, all pass, all use `httptest.NewServer`, no real network calls |
| `go.mod` | go-readability dependency declared | VERIFIED | `github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0` present as direct dependency |
| `cmd/tldt/main.go` | --url flag wired into resolveInputBytes as highest-priority branch; contains `urlFlag` | VERIFIED | `urlFlag` declared line 21; called line 38 as 3rd arg; URL branch at lines 204-209 runs before stdin/file/positional |
| `cmd/tldt/main_test.go` | Integration tests for --url flag; contains TestMain_URLFlag_ServesHTML | VERIFIED | Both `TestMain_URLFlag_ServesHTML` and `TestMain_URLFlag_404` present and pass; all 5 pre-existing `resolveInputBytes` call sites updated to 3-argument form |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/fetcher/fetcher.go` | `github.com/go-shiori/go-readability` | `readability.FromReader(limited, u)` | VERIFIED | Line 74: `article, err := readability.FromReader(limited, u)` — uses `FromReader` not `FromURL`, preserving size cap |
| `internal/fetcher/fetcher.go` | `resp.Body` | `io.LimitReader(resp.Body, maxBytes)` | VERIFIED | Line 68: `limited := io.LimitReader(resp.Body, maxBytes)` — cap applied before any read |
| `cmd/tldt/main.go` | `internal/fetcher.Fetch` | `resolveInputBytes` urlStr branch | VERIFIED | Line 205: `text, err := fetcher.Fetch(urlStr, 30*time.Second, 5<<20)` — wired as highest-priority branch |
| `cmd/tldt/main_test.go` | `httptest.NewServer` | `TestMain_URLFlag_*` tests | VERIFIED | Lines 586 and 607: both new URL tests use `httptest.NewServer` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `cmd/tldt/main.go` | `rawBytes` (URL branch) | `fetcher.Fetch(urlStr, ...)` → `readability.FromReader` → `article.TextContent` | Yes — real HTML parsing from HTTP response | FLOWING |
| `internal/fetcher/fetcher.go` | `article.TextContent` | `readability.FromReader(limited, u)` from live HTTP body | Yes — readability extracts article text from HTML DOM | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Fetcher unit tests pass | `go test ./internal/fetcher/... -count=1` | 5 passed | PASS |
| Full test suite (no regressions) | `go test ./... -count=1` | 199 passed in 4 packages | PASS |
| Build succeeds | `go build ./...` | exit 0 | PASS |
| URL flag integration tests | `go test ./cmd/tldt/... -run 'TestMain_URLFlag' -count=1` | 2 passed | PASS |
| resolveInputBytes signature fix | `go test ./cmd/tldt/... -run 'TestResolveInputBytes' -count=1` | 5 passed | PASS |
| Live network (Wikipedia URL) | requires network — skipped | n/a | SKIP (human needed) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| INP-01 | 04-01, 04-02 | User can run `tldt --url <url>` to fetch a webpage, strip boilerplate HTML, and receive an extractive summary on stdout | SATISFIED | `--url` flag declared in `main.go:21`; `fetcher.Fetch` strips HTML via go-readability; `TestMain_URLFlag_ServesHTML` validates end-to-end |
| INP-02 | 04-01, 04-02 | URL fetcher handles HTTP redirects; returns non-zero exit code with error to stderr on fetch failure | SATISFIED | `http.Client` follows redirects automatically; `TestFetch_Redirect` and `TestMain_URLFlag_404` validate both behaviors |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/fetcher/fetcher.go` | 2-4 | TODO comment about migrating to `codeberg.org/readeck/go-readability/v2` | Info | Intentional — per plan spec; archived library is functional for Phase 4; migration deferred to future phase |

No blockers or warnings found. The TODO is explicitly required by the plan spec (acceptance criterion: "grep TODO.*codeberg.org/readeck returns a match").

### Human Verification Required

#### 1. Live URL Summarization (INP-01 full path)

**Test:** `go build -o /tmp/tldt ./cmd/tldt && /tmp/tldt --url https://en.wikipedia.org/wiki/Extractive_summarization --sentences 3`
**Expected:** 3 plain-text sentences on stdout, no HTML markup, no boilerplate navigation text; exit 0
**Why human:** All automated tests use `httptest.NewServer`. Real outbound TLS connections, DNS resolution, Wikipedia's actual HTML structure, and potential bot-detection headers cannot be verified programmatically without live network access in this environment.

#### 2. Live Redirect Following (INP-02 redirect path)

**Test:** `go build -o /tmp/tldt ./cmd/tldt && /tmp/tldt --url https://httpstat.us/301 --sentences 3`
**Expected:** Summary text on stdout, exit 0 — redirect transparently followed to destination
**Why human:** `TestFetch_Redirect` validates the redirect logic via `httptest.NewServer`, but confirming the behavior against a real external redirect service requires live network.

### Gaps Summary

No gaps found. All 12 observable truths are verified at all four levels (existence, substantive implementation, wiring, and data-flow). Both INP-01 and INP-02 requirements are satisfied. The 199-test suite (including 7 new tests added in this phase) passes with zero regressions.

The two human verification items are not blockers — they test the same code paths already validated by `TestMain_URLFlag_ServesHTML`, `TestMain_URLFlag_404`, `TestFetch_Redirect`, and `TestFetch_404` via local httptest servers. Human testing is requested to confirm behavior against live external hosts as a final smoke-test.

---

_Verified: 2026-05-02T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
