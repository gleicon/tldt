---
phase: 08-network-hardening
fixed_at: 2026-05-02T00:00:00Z
review_path: .planning/phases/08-network-hardening/08-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 08: Code Review Fix Report

**Fixed at:** 2026-05-02T00:00:00Z
**Source review:** .planning/phases/08-network-hardening/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (2 Critical, 4 Warning)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: SSRF filter misses IPv6 link-local (`fe80::/10`), CGN (`100.64.0.0/10`), and unspecified address

**Files modified:** `internal/fetcher/fetcher.go`
**Commit:** a576d1d
**Applied fix:** Added `ip.IsUnspecified()` check and a `cgnBlock` (`100.64.0.0/10`, RFC 6598) CIDR check to `blockPrivateIP`. Added `cgnBlock` as a package-level `*net.IPNet` var. Error messages follow existing pattern. Also added test cases for `0.0.0.0` (unspecified) and `100.64.1.1` (CGN) to `TestBlockPrivateIP`.

---

### CR-02: `pkg/tldt.Detect` silently ignores `DetectOptions.OutlierThreshold`

**Files modified:** `pkg/tldt/tldt.go`
**Commit:** 586faec
**Applied fix:** Updated the `OutlierThreshold` struct doc to explain that statistical outlier detection is not available through `Detect`/`Pipeline` (requires a precomputed LexRank similarity matrix). Both `Detect` and `Pipeline` now return an explicit `fmt.Errorf` when `OutlierThreshold != 0`, preventing callers from silently receiving the default behavior instead of their configured value. This matches the reviewer's recommendation to "return an error when it is non-zero to prevent silent misconfiguration." Marked as `"fixed: requires human verification"` — the error message wording should be reviewed to ensure it is clear for API consumers.

---

### WR-01: Temp files in hook leak on unexpected exit (no `trap` cleanup)

**Files modified:** `internal/installer/hooks/tldt-hook.sh`
**Commit:** b83b700
**Applied fix:** Moved both `mktemp` calls to before the `trap` registration and added `trap 'rm -f "$STDERR_FILE" "$GUARD_FILE"' EXIT` immediately after. Removed the two manual `rm -f` lines (lines 50 and 62 in original) that were unreachable on early `set -e` exit. The trap now handles cleanup unconditionally on any exit path including signals and unexpected errors.

---

### WR-02: Redirect SSRF check resolves the redirect target hostname but not the final resolved IP after `http.Client` connects

**Files modified:** `internal/fetcher/fetcher.go`, `internal/fetcher/fetcher_test.go`
**Commit:** a6f8706
**Applied fix:** Replaced the standalone `lookupHost` pre-check with a custom `http.Transport.DialContext` that intercepts DNS resolution at TCP connection time. Added `dialTCP` as an injectable package-level function variable (parallel to `lookupHost`) so tests can redirect connections to httptest servers without bypassing the SSRF filter logic. The `CheckRedirect` guard is kept as belt-and-suspenders for redirect-hop hostname checks. Updated `fetcher_test.go`: replaced `withLookup(publicLookup, ...)` pattern for httptest-based tests with a new `withServer(ts, ...)` helper that overrides both `lookupHost` (returns public IP to pass SSRF filter) and `dialTCP` (connects to the real test server address). Added `CGN` and `unspecified` test cases to `TestBlockPrivateIP`. Marked as `"fixed: requires human verification"` — the TOCTOU elimination logic should be reviewed to confirm the resolved address is correctly pinned between SSRF check and TCP dial.

---

### WR-03: `--sentences 999` in output guard is a fragile magic sentinel

**Files modified:** `internal/installer/hooks/tldt-hook.sh`
**Commit:** 004f4b1
**Applied fix:** Expanded the single-line comment above the `--sentences 999` invocation into a four-line comment explaining: (a) that 999 exceeds any realistic sentence count, (b) the purpose is to prevent re-summarization, (c) only stderr WARNING detection matters here, and (d) a note to replace with `--detect-only` if that flag is ever added to `tldt`.

---

### WR-04: `TestResolveInputBytes_Stdin` mutates `os.Stdin` without test isolation guarantee

**Files modified:** `cmd/tldt/main.go`, `cmd/tldt/main_test.go`
**Commit:** 00cd3c6
**Applied fix:** Added `stdinReader io.Reader` as the fourth parameter to `resolveInputBytes`. When non-nil, the function reads directly from it (skipping the `os.Stdin.Stat()` pipe detection). When nil, the existing `os.Stdin` pipe-detection path is used unchanged. Updated the production call site to pass `nil`. Updated all test call sites to pass `nil`. Rewrote `TestResolveInputBytes_Stdin` to inject `strings.NewReader("piped content here")` directly — no `os.Pipe`, no `os.Stdin` mutation, safe for `t.Parallel()`.

---

## Skipped Issues

None — all in-scope findings were fixed.

---

_Fixed: 2026-05-02T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
