# Phase 8: Network Hardening + Hook Defense - Research

**Researched:** 2026-05-02
**Domain:** Go net/http SSRF mitigations, bash hook defense patterns, Go public API design
**Confidence:** HIGH

## Summary

Phase 8 is surgical: four requirements, four files, zero new packages (except `pkg/tldt/`). The SSRF work fits cleanly into the existing `internal/fetcher/fetcher.go` by adding a `blockPrivateIP` helper and a single combined `CheckRedirect` function on `http.Client`. The hook work is a structured expansion of an already-working bash script — the same `STATS_FILE` mktemp pattern is extended with `grep ^WARNING` splitting and an output guard pass. The `pkg/tldt/` library is a thin wrapper of already-public internal function signatures with no new logic.

The Go standard library provides all the building blocks needed for SSRF blocking. `net.IP.IsPrivate()` (RFC 1918 + RFC 4193), `net.IP.IsLoopback()`, and `net.IP.IsLinkLocalUnicast()` cover almost everything. The cloud metadata special cases (`169.254.169.254` and `fd00:ec2::254`) are caught by `IsLinkLocalUnicast()` plus a single exact-IP check for the IPv6 variant. No third-party libraries are needed.

The `CheckRedirect` function signature is `func(req *http.Request, via []*http.Request) error` — `len(via)` gives the number of redirects already followed. Rejecting at `len(via) >= 5` enforces the 5-hop cap (5 redirects allowed, the 6th call rejects). SSRF on redirects requires resolving the new hostname inside `CheckRedirect` and running the same block check. Both concerns fold into one combined function — auditable in one place per D-02.

**Primary recommendation:** Implement the combined `CheckRedirect` func with `blockPrivateIP` helper first (08-01), then expand the hook (08-02), then write the wrapper lib and docs. All test patterns are already established by fetcher_test.go.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**SSRF Block Architecture**
- D-01: SSRF blocking covers both the initial URL and every redirect hop. Initial hostname resolved via `net.LookupHost` before request. Each redirect hop resolved and checked inside `CheckRedirect`.
- D-02: Redirect cap and SSRF IP check share a single combined `CheckRedirect` function. One function: increment hop counter (reject at 6th hop = >5 redirects), resolve hostname, check IPs against block list.
- D-03: `Fetch()` returns typed sentinel errors. `var ErrSSRFBlocked = errors.New("SSRF blocked")` and `var ErrRedirectLimit = errors.New("redirect limit exceeded")`. Wrapped with `fmt.Errorf("...: %w", ErrSSRFBlocked)`.

**Hook Stderr Splitting**
- D-04: WARNING lines separated via `grep ^WARNING`. Token stats via `grep -v ^WARNING`. Zero changes to tldt binary required.
- D-05: When `--detect-injection` finds no issues, the hook stays silent — no "no injection detected" line added. Clean runs produce no noise.

**Output Guard Mechanism**
- D-06: Output guard: `echo "$SUMMARY" | tldt --detect-injection --sentences 999`. Stdout discarded; only stderr WARNING lines matter.
- D-07: If output guard finds injection patterns in summary, hook warns and still emits — advisory-only, consistent with SEC-07.

**additionalContext Structure**
- D-08: Labeled sections rendered conditionally. Structure:
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
- D-09: When no warnings (clean input and clean summary), additionalContext contains only `[Token savings]` and `[Summary]`. Warning sections omitted entirely.

**Security Documentation**
- D-10: Create `docs/security.md` — standalone technical reference covering OWASP LLM Top 10 2025: LLM01, LLM02, LLM05, LLM10 — one section per category with threat description, mitigation, CLI example.
- D-11: Update `docs/index.html` — add "Security" section or callout block listing OWASP categories addressed, linking to `docs/security.md`. Consistent with existing page style.

**Embeddable Go Library**
- D-12: Create `pkg/tldt/` as a new public Go package. Module path: `github.com/gleicon/tldt/pkg/tldt`. Exports:
  - `Summarize(text string, opts SummarizeOptions) (Result, error)`
  - `Detect(text string, opts DetectOptions) (DetectResult, error)`
  - `Sanitize(text string) (string, SanitizeReport, error)`
  - `Fetch(url string, opts FetchOptions) (string, error)`
  - `Pipeline(text string, opts PipelineOptions) (PipelineResult, error)`
  - Options structs: researcher to recommend idiomatic Go pattern
  - No global state; each call stateless
  - Unit tests in `pkg/tldt/tldt_test.go`

### Claude's Discretion

- Options struct pattern for `pkg/tldt/` (plain structs vs functional options)
- Exact field names in exported types beyond what CONTEXT.md specifies

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEC-11 | `--url` fetcher resolves hostname and blocks RFC 1918 (10.x, 172.16-31.x, 192.168.x), loopback (127.x, ::1), and cloud metadata (169.254.169.254, fd00:ec2::254) — exits non-zero with error to stderr | `net.IP.IsPrivate()`, `net.IP.IsLoopback()`, `net.IP.IsLinkLocalUnicast()` cover all cases; cloud metadata IPv6 needs explicit check |
| SEC-12 | `--url` fetcher limits redirect chain to ≤5 hops; exceeding limit exits non-zero with error to stderr | `http.Client.CheckRedirect` with `len(via) >= 5` enforces cap; combined with SEC-11 check in single function |
| SEC-13 | Auto-trigger hook invokes `tldt --sanitize --detect-injection --verbose` by default; any WARNING lines from stderr are appended to `additionalContext` | `grep ^WARNING` extracts WARNING lines from stderr capture; existing mktemp pattern is anchor for expansion |
| SEC-16 | Hook output guard re-runs `--detect-injection` on the summary text before emitting; any WARNING findings appended to context note | `echo "$SUMMARY" \| tldt --detect-injection --sentences 999`; only stderr captured; stdout discarded |
</phase_requirements>

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| SSRF IP blocking | API/Backend (fetcher) | — | Network policy enforced at fetch layer before response is processed |
| Redirect cap enforcement | API/Backend (fetcher) | — | Part of HTTP client configuration; `CheckRedirect` is a client-level hook |
| Hook injection detection | Client (bash hook) | — | Hook runs before prompt enters Claude context; detection is advisory pre-filter |
| Hook output guard | Client (bash hook) | — | Guard runs on hook output before additionalContext is assembled |
| Public library API | Library (pkg/tldt/) | — | Thin wrapper over internal/; no new logic, just exported surface |
| Security documentation | Static (docs/) | — | Markdown + HTML; no runtime component |

---

## Standard Stack

### Core (all stdlib — no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net` (stdlib) | Go 1.26.2 | IP parsing, hostname resolution, CIDR matching | `LookupHost`, `ParseIP`, `IP.IsPrivate()`, `IP.IsLoopback()`, `IP.IsLinkLocalUnicast()`, `IPNet.Contains()` — all present |
| `net/http` (stdlib) | Go 1.26.2 | `http.Client.CheckRedirect` hook | `CheckRedirect func(req *Request, via []*Request) error` — exact interface needed |
| `errors` (stdlib) | Go 1.26.2 | Typed sentinel errors with `errors.Is()` | `errors.New` + `fmt.Errorf("...: %w", ErrX)` pattern |
| `fmt` (stdlib) | Go 1.26.2 | Error message formatting | Already in use |

### Supporting (internal packages being wrapped)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/summarizer` | local | LexRank/TextRank/ensemble/graph | Exposed via `Summarize()` and `Pipeline()` in pkg/tldt/ |
| `internal/detector` | local | `Analyze()`, `DetectPatterns()`, `DetectOutliers()` | Exposed via `Detect()` in pkg/tldt/ |
| `internal/sanitizer` | local | `SanitizeAll()`, `ReportInvisibles()` | Exposed via `Sanitize()` in pkg/tldt/ |
| `internal/fetcher` | local | `Fetch()` (with Phase 8 hardening) | Exposed via `Fetch()` in pkg/tldt/ |

**No new external dependencies required for Phase 8.** [VERIFIED: Go stdlib 1.26.2]

---

## Architecture Patterns

### SSRF Block Flow

```
Fetch(rawURL) {
  1. Parse URL → validate scheme (existing)
  2. Resolve initial hostname → net.LookupHost(u.Hostname())
  3. blockPrivateIP(addrs) → error if any addr matches blocked range
  4. Build http.Client with CheckRedirect = combinedCheckRedirect
  5. client.Do(req)  →  CheckRedirect called on each hop
}

combinedCheckRedirect(req, via) {
  if len(via) >= 5 → return fmt.Errorf("...: %w", ErrRedirectLimit)
  resolve req.URL.Hostname() → net.LookupHost
  blockPrivateIP(addrs) → return fmt.Errorf("...: %w", ErrSSRFBlocked) if blocked
  return nil  (allow redirect)
}

blockPrivateIP(addrs []string) error {
  for each addr:
    ip = net.ParseIP(addr)
    if ip.IsLoopback() → blocked
    if ip.IsPrivate()  → blocked  (covers RFC 1918 + RFC 4193)
    if ip.IsLinkLocalUnicast() → blocked  (covers 169.254.x.x AWS metadata + IPv6 link-local)
    if ip == "fd00:ec2::254" → blocked  (EC2 IPv6 metadata, explicit check)
  return nil
}
```

[VERIFIED: Go stdlib net package docs — `IsPrivate()`, `IsLoopback()`, `IsLinkLocalUnicast()` confirmed in Go 1.26.2]

### IP Range Coverage

| Block | Go Method | Covers |
|-------|-----------|--------|
| RFC 1918 (10.x, 172.16-31.x, 192.168.x) | `ip.IsPrivate()` | All three private IPv4 ranges |
| RFC 4193 (fc00::/7) | `ip.IsPrivate()` | IPv6 ULA including fd00::/8 |
| Loopback (127.x, ::1) | `ip.IsLoopback()` | IPv4 + IPv6 loopback |
| Link-local (169.254.x.x) | `ip.IsLinkLocalUnicast()` | AWS/Azure/GCP metadata (IPv4) |
| EC2 IPv6 metadata (fd00:ec2::254) | `ip.IsPrivate()` is enough (fd00::/8 is ULA) | BUT explicit exact-IP check adds clarity and defense-in-depth |

[VERIFIED: Go net package source — `IsPrivate()` covers `fc00::/7` which includes `fd00::/8`. `IsLinkLocalUnicast()` covers `169.254.0.0/16`.]

**Note on fd00:ec2::254:** `ip.IsPrivate()` already blocks this because `fd00::/8` is within `fc00::/7`. Adding an explicit check is belt-and-suspenders and documents intent clearly.

### CheckRedirect Exact Signature

```go
// Source: Go stdlib net/http package documentation
client := &http.Client{
    Timeout: timeout,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        // len(via) == number of redirects already followed
        // len(via) == 0 means this is the first redirect attempt
        if len(via) >= 5 {
            return fmt.Errorf("redirect to %q after %d hops: %w", req.URL, len(via), ErrRedirectLimit)
        }
        addrs, err := net.LookupHost(req.URL.Hostname())
        if err != nil {
            return fmt.Errorf("resolving redirect host %q: %w", req.URL.Hostname(), err)
        }
        return blockPrivateIP(req.URL.Hostname(), addrs)
    },
}
```

[VERIFIED: Go stdlib CheckRedirect docs — `via` is oldest-first slice of previous requests; `len(via) >= 5` rejects on 6th redirect attempt]

### Hook Expansion Architecture

```bash
# Current (Phase 6 pattern):
STATS_FILE=$(mktemp)
SUMMARY=$(printf '%s' "$PROMPT" | tldt --verbose 2>"$STATS_FILE" || true)
SAVINGS=$(cat "$STATS_FILE")
rm -f "$STATS_FILE"

# Phase 8 expansion:
STDERR_FILE=$(mktemp)
SUMMARY=$(printf '%s' "$PROMPT" | tldt --sanitize --detect-injection --verbose 2>"$STDERR_FILE" || true)
WARNINGS=$(grep '^WARNING' "$STDERR_FILE" || true)         # injection warning lines
SAVINGS=$(grep -v '^WARNING' "$STDERR_FILE" || true)       # token stats (everything else)
rm -f "$STDERR_FILE"

# Output guard:
GUARD_FILE=$(mktemp)
printf '%s' "$SUMMARY" | tldt --detect-injection --sentences 999 2>"$GUARD_FILE" >/dev/null || true
SUMMARY_WARNINGS=$(grep '^WARNING' "$GUARD_FILE" || true)
rm -f "$GUARD_FILE"

# Conditional section builder:
REPLACEMENT="[Token savings]
${SAVINGS}"

if [ -n "$WARNINGS" ]; then
REPLACEMENT="${REPLACEMENT}

[Security warnings - input]
${WARNINGS}"
fi

if [ -n "$SUMMARY_WARNINGS" ]; then
REPLACEMENT="${REPLACEMENT}

[Security warnings - summary]
${SUMMARY_WARNINGS}"
fi

REPLACEMENT="${REPLACEMENT}

[Summary]
${SUMMARY}"
```

[VERIFIED: Existing hook at internal/installer/hooks/tldt-hook.sh lines 46-73 — STATS_FILE pattern confirmed]

### WARNING Line Format (from current codebase)

From `cmd/tldt/main.go` line 157:
```
injection-detect: WARNING — input flagged as suspicious
```

From detector pattern output (main.go lines 152-154):
```
injection-detect: N finding(s), max confidence X.XX
  [category] pattern (score=X.XX): excerpt
```

**Important:** The `grep ^WARNING` pattern in CONTEXT.md D-04 only catches the `WARNING —` line, NOT the multi-line finding detail. The D-04 design captures the Suspicious summary line only — this is intentional (single-line signal for additionalContext, not the full finding dump). [VERIFIED: main.go source read above]

### pkg/tldt/ API Design

**Plain struct option pattern** (recommended for this codebase): The existing codebase uses plain structs everywhere (config.Config, detector.Report, etc.). Functional options would be an inconsistency. For a public API with 5 exported functions, plain structs are idiomatic and lower complexity.

```go
// pkg/tldt/tldt.go

package tldt

import (
    "github.com/gleicon/tldt/internal/detector"
    "github.com/gleicon/tldt/internal/fetcher"
    "github.com/gleicon/tldt/internal/sanitizer"
    "github.com/gleicon/tldt/internal/summarizer"
    "time"
)

// SummarizeOptions controls summarization behavior.
type SummarizeOptions struct {
    Algorithm string // "lexrank"|"textrank"|"graph"|"ensemble" (default: "lexrank")
    Sentences int    // number of output sentences (default: 5)
    Format    string // "text"|"json"|"markdown" (default: "text")
}

// Result is the output of Summarize.
type Result struct {
    Summary   string
    TokensIn  int
    TokensOut int
    Reduction int // percentage
}

// DetectOptions controls detection behavior.
type DetectOptions struct {
    OutlierThreshold float64 // default: detector.DefaultOutlierThreshold
}

// DetectResult is the output of Detect.
type DetectResult struct {
    Report    detector.Report
    Warnings  []string // human-readable WARNING lines (same format as CLI stderr)
}

// SanitizeReport is the output metadata from Sanitize.
type SanitizeReport struct {
    RemovedCount int
    Invisibles   []sanitizer.InvisibleReport
}

// FetchOptions controls URL fetching behavior.
type FetchOptions struct {
    Timeout  time.Duration // default: 30s
    MaxBytes int64         // default: 5MB
}

// PipelineOptions combines all pipeline stages.
type PipelineOptions struct {
    Summarize SummarizeOptions
    Detect    DetectOptions
    Sanitize  bool // run sanitizer before detection/summarization
}

// PipelineResult is the output of Pipeline.
type PipelineResult struct {
    Summary     string
    TokensIn    int
    TokensOut   int
    Reduction   int
    Warnings    []string
    Redactions  int
}

// Sentinel errors re-exported from internal/fetcher for external error checking.
var (
    ErrSSRFBlocked   = fetcher.ErrSSRFBlocked
    ErrRedirectLimit = fetcher.ErrRedirectLimit
)
```

[ASSUMED: Plain struct vs functional options pattern choice — based on codebase analysis showing consistent plain struct usage, but could be debated for larger public APIs]

### Project Structure (new and modified files)

```
internal/fetcher/
├── fetcher.go          # MODIFIED: add blockPrivateIP, CheckRedirect, sentinel errors
└── fetcher_test.go     # MODIFIED: add TestFetch_SSRFBlock, TestFetch_RedirectLimit tests

internal/installer/hooks/
└── tldt-hook.sh        # MODIFIED: expand to D-04/D-05/D-06/D-07/D-08/D-09 pattern

pkg/tldt/
├── tldt.go             # NEW: exported API wrapper
└── tldt_test.go        # NEW: integration-style tests through internal/

docs/
├── security.md         # NEW: OWASP LLM Top 10 2025 technical reference
└── index.html          # MODIFIED: add security callout section + nav link
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| RFC 1918 IP range check | Manual CIDR arithmetic or string prefix matching | `net.IP.IsPrivate()` | Go stdlib handles all three RFC 1918 ranges + RFC 4193 IPv6 ULA in one call |
| Loopback detection | `ip == "127.0.0.1"` string comparison | `net.IP.IsLoopback()` | Covers all 127.x.x.x and ::1; string comparison misses 127.0.0.2 etc. |
| Link-local detection (169.254.x.x) | Hardcoded CIDR | `net.IP.IsLinkLocalUnicast()` | Covers full 169.254.0.0/16 in one call; string prefix fails for non-standard addresses |
| JSON encoding in bash | String interpolation | `python3 -c "import json..."` | Already established pattern in hook — control chars, quotes, backslashes all break naive bash interpolation |
| Redirect counting | Global counter variable | `len(via)` in CheckRedirect | `via` is provided by net/http; counter would require closure state and could be wrong |

**Key insight:** The Go stdlib's `net` package has everything needed for SSRF IP blocking. Rolling manual CIDR checks is error-prone (byte order, IPv4-in-IPv6 representation, IPv6 normalization) — stdlib handles all these edge cases.

---

## Common Pitfalls

### Pitfall 1: IPv4-in-IPv6 representation
**What goes wrong:** `net.ParseIP("127.0.0.1")` returns a 16-byte slice in IPv4-in-IPv6 form (`::ffff:127.0.0.1`). String comparison `ip.String() == "127.0.0.1"` may fail in some edge cases.
**Why it happens:** `ParseIP` always returns 16 bytes for IPv4 (IPv4-mapped IPv6).
**How to avoid:** Always use `net.IP` methods (`IsLoopback()`, `IsPrivate()`) rather than string comparison. These methods handle the IPv4-mapped form correctly.
**Warning signs:** Tests pass for `127.0.0.1` but fail for `::ffff:7f00:1`.

### Pitfall 2: LookupHost vs URL hostname extraction
**What goes wrong:** Calling `net.LookupHost(rawURL)` instead of `net.LookupHost(u.Hostname())`. `rawURL` includes scheme/path and `LookupHost` will fail.
**Why it happens:** Forgetting to parse the URL and extract only the hostname.
**How to avoid:** Always call `url.Parse(rawURL)` first, then `u.Hostname()` (strips brackets from IPv6 addresses automatically).
**Warning signs:** `LookupHost` returns errors for valid URLs.

### Pitfall 3: CheckRedirect error wrapping loses typed sentinel
**What goes wrong:** `return fmt.Errorf("redirect failed: %v", ErrSSRFBlocked)` — uses `%v` not `%w`. Caller's `errors.Is(err, fetcher.ErrSSRFBlocked)` returns false.
**Why it happens:** Mixing `%v` (string formatting) with `%w` (error wrapping).
**How to avoid:** Always use `fmt.Errorf("...: %w", ErrSSRFBlocked)` for typed sentinels. Net/http wraps CheckRedirect errors in `*url.Error` — `errors.Is()` still unwraps through it correctly.
**Warning signs:** `errors.Is(err, fetcher.ErrSSRFBlocked)` returns false even when block triggered.

### Pitfall 4: grep ^WARNING matches too broadly or too narrowly
**What goes wrong:** `grep '^injection-detect: WARNING'` — too specific and breaks if stderr format changes. Or `grep 'WARNING'` — too broad and catches non-injection warnings.
**Why it happens:** Checking the grep pattern against actual stderr output format.
**How to avoid:** The actual WARNING line format from main.go line 157 is `injection-detect: WARNING — input flagged as suspicious`. The `grep '^WARNING'` in D-04 will NOT match this because the line starts with `injection-detect:`, not `WARNING`. Need to verify with actual stderr output.

**CRITICAL FINDING:** The existing `--detect-injection` stderr output does NOT start with `WARNING`. From main.go inspection:
- Line 156: `if report.Suspicious {`
- Line 157: `fmt.Fprintln(os.Stderr, "injection-detect: WARNING — input flagged as suspicious")`

This line starts with `injection-detect:`, not `WARNING`. The D-04 decision `grep ^WARNING` assumes WARNING is the line prefix. Either:
(a) The grep pattern needs to be `grep 'WARNING'` (not anchored to start), or
(b) The tldt binary needs `--verbose` or a future flag that changes WARNING line format, or
(c) D-04 is correct and the implementation in main.go needs to change the prefix to match.

**Recommendation for planner:** The hook script grep must use `grep 'WARNING'` (unanchored) to match the current `injection-detect: WARNING —` format, OR the tldt binary's warning output format must be changed to start with `WARNING:`. This is a gap between D-04 and the current implementation. The planner should decide which to change — changing the hook grep is simpler.

**Warning signs:** No WARNING lines ever appear in additionalContext even when injection is detected.

### Pitfall 5: pkg/tldt/ creating import cycles
**What goes wrong:** `pkg/tldt/` imports `cmd/tldt/main.go` functions or reuses `main` package code.
**Why it happens:** Trying to reuse CLI flag parsing logic.
**How to avoid:** `pkg/tldt/` imports ONLY from `internal/` packages. Never from `cmd/`. All business logic in `internal/` — `pkg/tldt/` is a pure delegation layer.
**Warning signs:** `go build ./pkg/...` reports import cycle.

### Pitfall 6: --sentences 999 output guard causes re-summarization
**What goes wrong:** The output guard `echo "$SUMMARY" | tldt --detect-injection --sentences 999` actually summarizes the summary (which is already short), changing the text slightly and losing the original summary.
**Why it happens:** Misunderstanding that `--detect-injection` still runs through summarization before detection.
**How to avoid:** `--sentences 999` effectively means "return all sentences" for a short summary. The guard's stdout is discarded — only stderr WARNING lines are captured. The original `$SUMMARY` is what gets emitted. This is correct per D-06.
**Warning signs:** Output contains different text than what was originally summarized.

---

## Code Examples

### blockPrivateIP helper

```go
// Source: Go stdlib net package — verified in Go 1.26.2
var (
    ErrSSRFBlocked   = errors.New("SSRF blocked: private or reserved IP address")
    ErrRedirectLimit = errors.New("redirect limit exceeded")

    // Cloud metadata IPv6 endpoint — explicitly blocked for clarity
    cloudMetadataIPv6 = net.ParseIP("fd00:ec2::254")
)

// blockPrivateIP resolves host and returns ErrSSRFBlocked if any resolved IP
// is in a private, loopback, or link-local range.
func blockPrivateIP(host string, addrs []string) error {
    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if ip == nil {
            continue
        }
        if ip.IsLoopback() {
            return fmt.Errorf("host %q resolves to loopback %s: %w", host, addr, ErrSSRFBlocked)
        }
        if ip.IsPrivate() {
            return fmt.Errorf("host %q resolves to private IP %s: %w", host, addr, ErrSSRFBlocked)
        }
        if ip.IsLinkLocalUnicast() {
            return fmt.Errorf("host %q resolves to link-local IP %s: %w", host, addr, ErrSSRFBlocked)
        }
        if ip.Equal(cloudMetadataIPv6) {
            return fmt.Errorf("host %q resolves to cloud metadata IP %s: %w", host, addr, ErrSSRFBlocked)
        }
    }
    return nil
}
```

### Combined CheckRedirect

```go
// Source: Go stdlib net/http.Client.CheckRedirect docs — verified
combinedCheckRedirect := func(req *http.Request, via []*http.Request) error {
    // len(via) == number of redirects already followed
    // Reject on the 6th redirect attempt (5 hops max, inclusive)
    if len(via) >= 5 {
        return fmt.Errorf("too many redirects (%d) fetching %q: %w", len(via), req.URL, ErrRedirectLimit)
    }
    addrs, err := net.LookupHost(req.URL.Hostname())
    if err != nil {
        return fmt.Errorf("resolving redirect host %q: %w", req.URL.Hostname(), err)
    }
    return blockPrivateIP(req.URL.Hostname(), addrs)
}

client := &http.Client{
    Timeout:       timeout,
    CheckRedirect: combinedCheckRedirect,
}
```

### SSRF test pattern using httptest.NewServer

```go
// Source: established pattern from fetcher_test.go — adapated for SSRF
func TestFetch_SSRFBlockPrivateIP(t *testing.T) {
    // Server that redirects to a private IP address — simulates SSRF-by-redirect
    // We can't bind to 192.168.x.x in tests, but we can test direct private IP URLs
    _, err := Fetch("http://192.168.1.1/admin", testTimeout, testMaxBytes)
    if err == nil {
        t.Fatal("expected SSRF block error, got nil")
    }
    if !errors.Is(err, ErrSSRFBlocked) {
        t.Errorf("expected ErrSSRFBlocked, got: %v", err)
    }
}

func TestFetch_RedirectLimit(t *testing.T) {
    count := 0
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        count++
        http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently) // redirect to self
    }))
    defer ts.Close()

    _, err := Fetch(ts.URL, testTimeout, testMaxBytes)
    if err == nil {
        t.Fatal("expected redirect limit error, got nil")
    }
    if !errors.Is(err, ErrRedirectLimit) {
        t.Errorf("expected ErrRedirectLimit, got: %v", err)
    }
}
```

**Note on SSRF-by-redirect httptest:** httptest servers bind to 127.0.0.1 (loopback). A test that redirects from `ts.URL` to `http://127.0.0.1:PORT` will be blocked by `IsLoopback()`. This correctly simulates redirect-to-private-IP. The test server itself will serve the first response (redirect) before `CheckRedirect` fires on the loopback target.

### docs/index.html security callout placement

The nav currently has: `features | injection defense | algorithms | [GitHub]`

Adding `security` nav link and a security section between `#algorithms` and the pipe-safety section (before the `<div class="divider"></div>` at line 798):

```html
<!-- Nav addition (line 295, after algorithms link): -->
<a href="#security" class="nav-link">security</a>

<!-- New section (before line 800): -->
<div class="divider"></div>
<section class="section" id="security">
  <div class="container">
    <div class="reveal" style="margin-bottom:40px">
      <div class="section-label" style="color:var(--warning)">security</div>
      <h2 class="section-title">OWASP LLM Top 10 2025.<br><span class="acc">Addressed by design.</span></h2>
    </div>
    <!-- OWASP categories table + link to docs/security.md -->
  </div>
</section>
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No SSRF protection | `net.IP.IsPrivate()` + `CheckRedirect` | Phase 8 | Blocks SSRF-by-redirect attack vector |
| `http.Client` default 10-hop redirect | Custom `CheckRedirect` 5-hop cap | Phase 8 | Tighter limit per security best practices |
| Hook emits raw summary | Hook emits labeled sections with warnings | Phase 8 | Claude context is structured and security-aware |
| `tldt` is CLI-only | `pkg/tldt/` provides embeddable Go API | Phase 8 | External programs can embed pipeline without shelling out |

**Deprecated/outdated:**
- `http.Client` with no `CheckRedirect` is still valid for non-security-sensitive fetching, but for public-facing tools that accept user-provided URLs it is insufficient.

---

## Open Questions

1. **grep pattern mismatch (CRITICAL)**
   - What we know: D-04 specifies `grep ^WARNING` but tldt's actual injection detection output starts with `injection-detect: WARNING —` (not `WARNING`)
   - What's unclear: Should the hook use `grep 'WARNING'` (unanchored match), or should `main.go` be updated to emit a `WARNING: ...` prefixed line in addition to or instead of the current format?
   - Recommendation: Simpler fix is `grep 'WARNING'` in the hook. But if the intent is to have a machine-parseable prefix, adding `fmt.Fprintln(os.Stderr, "WARNING: injection detected")` as an additional line when `report.Suspicious` is true would make `^WARNING` work without changing any grep in the hook. The planner should decide which approach to take.

2. **pkg/tldt/ options: plain struct vs functional options**
   - What we know: Codebase uses plain structs everywhere. D-12 says "researcher to recommend."
   - What's unclear: Whether the public API is expected to evolve with backward-compat additions.
   - Recommendation: Plain structs. Go zero-values provide sensible defaults. If a field is added later, callers using `tldt.SummarizeOptions{Sentences: 3}` automatically get zero-values (which map to defaults) for new fields without breaking. Functional options add boilerplate for marginal benefit at this scale.

3. **docs/security.md Phase 9 content**
   - What we know: D-10 says to document Phase 9 items (PII/--detect-pii) in security.md even though Phase 9 hasn't shipped.
   - What's unclear: Should Phase 9 content be marked "coming soon" or described as if shipped?
   - Recommendation: Document Phase 9 CLI examples as planned behavior with a clear "(Phase 9)" marker. Security docs should be aspirational but accurate — callers need to know what's coming.

---

## Environment Availability

Step 2.6: External dependency audit for Phase 8.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All packages | Yes | 1.26.2 | — |
| go test ./... | Test suite | Yes | 302 tests passing | — |
| python3 | Hook JSON encoding | Yes (darwin default) | system | jq fallback already present |
| bash | Hook script | Yes | darwin /bin/bash | — |
| net (stdlib) | SSRF block | Yes | Go 1.26.2 | — |

[VERIFIED: `go version go1.26.2 darwin/arm64`; `go test ./...` shows 302 passed]

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib |
| Config file | none (go test ./...) |
| Quick run command | `go test ./internal/fetcher/... -run TestFetch -v` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-11 | SSRF block on direct private IP | unit | `go test ./internal/fetcher/... -run TestFetch_SSRF` | No — Wave 0 |
| SEC-11 | SSRF block on cloud metadata IP (169.254.169.254) | unit | `go test ./internal/fetcher/... -run TestFetch_SSRFMeta` | No — Wave 0 |
| SEC-11 | SSRF block via redirect to private IP (loopback redirect) | unit | `go test ./internal/fetcher/... -run TestFetch_SSRFRedirect` | No — Wave 0 |
| SEC-12 | Redirect limit: >5 hops rejected | unit | `go test ./internal/fetcher/... -run TestFetch_RedirectLimit` | No — Wave 0 |
| SEC-12 | Redirect limit: exactly 5 hops succeeds | unit | `go test ./internal/fetcher/... -run TestFetch_FiveHopsOK` | No — Wave 0 |
| SEC-13 | Hook invokes with --sanitize --detect-injection | manual | inspect hook script | No — Wave 0 |
| SEC-16 | Output guard re-runs detection on summary | manual | inspect hook script | No — Wave 0 |
| D-12 | pkg/tldt.Summarize wraps internal correctly | integration | `go test ./pkg/tldt/... -run TestSummarize` | No — Wave 0 |
| D-12 | pkg/tldt.Pipeline returns correct PipelineResult | integration | `go test ./pkg/tldt/... -run TestPipeline` | No — Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/fetcher/... -v`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green (currently 302 tests) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] New test functions in `internal/fetcher/fetcher_test.go` — covers SEC-11, SEC-12
- [ ] `pkg/tldt/tldt_test.go` — covers D-12 integration tests
- [ ] `pkg/tldt/tldt.go` — package must exist before tests compile

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | `net.LookupHost` + IP method checks for SSRF; `grep 'WARNING'` for hook input validation |
| V6 Cryptography | no | — |
| V10 Malicious Code | yes | SSRF prevention in fetcher; injection detection in hook |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SSRF via user-supplied URL | Tampering / Info Disclosure | Block private IPs before fetch; check after each redirect |
| SSRF via open redirect (redirect-to-private) | Tampering | `CheckRedirect` resolves + blocks each hop |
| Prompt injection in fetched content | Tampering | `--detect-injection` run before content enters Claude context (hook SEC-13) |
| Injection in summarization output | Tampering (LLM05) | Output guard re-checks summary (SEC-16) |
| Import of internal package by external program | Elevation of Privilege | `pkg/tldt/` wraps `internal/` — Go `internal/` package protection enforced by compiler |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `fd00:ec2::254` is already covered by `ip.IsPrivate()` (fd00::/8 is within fc00::/7 ULA range) | Architecture Patterns / IP Range Coverage | If wrong, explicit exact-IP check in `blockPrivateIP` is still required (already recommended as belt-and-suspenders) |
| A2 | Plain struct options pattern is preferred for `pkg/tldt/` | Code Examples / pkg/tldt API | If project style evolves to functional options, API will need revision (non-breaking if done before release) |

---

## Sources

### Primary (HIGH confidence)

- Go stdlib `net` package — `LookupHost`, `ParseIP`, `IP.IsPrivate()`, `IP.IsLoopback()`, `IP.IsLinkLocalUnicast()`, `ParseCIDR`, `IPNet.Contains` — verified via `go doc` in Go 1.26.2
- Go stdlib `net/http` package — `Client.CheckRedirect` signature and semantics — verified via `go doc`
- `internal/fetcher/fetcher.go` — current Fetch() implementation read in full
- `internal/fetcher/fetcher_test.go` — existing test pattern with httptest.NewServer read in full
- `internal/installer/hooks/tldt-hook.sh` — current hook implementation read in full (74 lines)
- `cmd/tldt/main.go` — pipeline order and WARNING line format verified by source read
- `internal/summarizer/summarizer.go` — Summarizer interface + New() factory verified
- `internal/detector/detector.go` — function signatures verified (`Analyze`, `DetectPatterns`, `DetectOutliers`)
- `internal/sanitizer/sanitizer.go` — function signatures verified (`SanitizeAll`, `ReportInvisibles`)
- `internal/config/config.go` — Config struct verified
- `.planning/phases/08-network-hardening/08-CONTEXT.md` — locked decisions D-01 through D-12

### Secondary (MEDIUM confidence)

- `docs/index.html` — page structure, nav links, section ids, CSS variables — read lines 1-905

### Tertiary (LOW confidence)

None.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib; no external deps; verified via go doc
- Architecture: HIGH — source code read directly; CheckRedirect signature confirmed
- Pitfalls: HIGH — derived from direct code inspection; Pitfall 4 (WARNING prefix mismatch) verified against main.go source
- pkg/tldt/ design: MEDIUM — plain struct recommendation is [ASSUMED] but consistent with codebase patterns

**Research date:** 2026-05-02
**Valid until:** 2026-06-02 (stable stdlib APIs; Go version pinned at 1.26.2)
