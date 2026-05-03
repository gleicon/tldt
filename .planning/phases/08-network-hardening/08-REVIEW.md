---
phase: 08-network-hardening
reviewed: 2026-05-02T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - cmd/tldt/main_test.go
  - docs/index.html
  - docs/security.md
  - internal/fetcher/fetcher.go
  - internal/fetcher/fetcher_test.go
  - internal/installer/hooks/tldt-hook.sh
  - pkg/tldt/tldt.go
  - pkg/tldt/tldt_test.go
findings:
  critical: 2
  warning: 4
  info: 2
  total: 8
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-05-02T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Phase 08 delivers SSRF hardening for `--url` fetching, an embeddable `pkg/tldt` API, an updated Claude Code hook, and updated docs. The core SSRF pre-check and redirect guard logic in `internal/fetcher/fetcher.go` are structurally sound. The main loop (scheme validation → DNS pre-check → redirect-aware client → body cap → readability extraction) is correct. Tests are well-structured and avoid live network calls.

Two critical defects were found: the SSRF filter is incomplete for IPv6 link-local addresses (`fe80::/10`) and certain IPv4 edge ranges (CGN `100.64.0.0/10`, unspecified `0.0.0.0`), and the `pkg/tldt.Detect` function silently ignores its `OutlierThreshold` option, making the field a no-op that will mislead callers. Four warnings cover temp-file leaks in the hook on unexpected exits, the missing DNS re-check after redirect resolution, a race condition in the test stdin override, and the `--sentences 999` guard in the hook being a fragile sentinel value.

---

## Critical Issues

### CR-01: SSRF filter misses IPv6 link-local (`fe80::/10`), CGN (`100.64.0.0/10`), and unspecified address

**File:** `internal/fetcher/fetcher.go:36-56`

**Issue:** `blockPrivateIP` checks `IsLoopback`, `IsPrivate`, `IsLinkLocalUnicast`, and the single EC2 IPv6 metadata literal. `net.IP.IsLinkLocalUnicast` in Go returns `true` for `169.254.0.0/16` and `fe80::/10` — so IPv6 link-local is covered. However the following are **not** blocked:

- `100.64.0.0/10` — IANA Shared Address Space (CGN / carrier-grade NAT). RFC 6598. Reachable inside many cloud provider VPC internal fabrics.
- `0.0.0.0` / `::` — `net.IP.IsUnspecified()` returns `true` for these; binding behavior is OS-dependent but permitting them as fetch targets is unambiguous SSRF risk.
- IPv6 multicast (`ff00::/8`) — not a typical SSRF vector but semantically should be blocked.

An attacker who controls DNS and can return `100.64.x.x` (common in shared-tenancy cloud environments) bypasses the filter entirely.

**Fix:**
```go
func blockPrivateIP(host string, addrs []string) error {
    var (
        cgn = &net.IPNet{
            IP:   net.ParseIP("100.64.0.0"),
            Mask: net.CIDRMask(10, 32),
        }
    )
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
        if ip.IsUnspecified() {
            return fmt.Errorf("host %q resolves to unspecified IP %s: %w", host, addr, ErrSSRFBlocked)
        }
        if cgn.Contains(ip) {
            return fmt.Errorf("host %q resolves to shared-address-space IP %s: %w", host, addr, ErrSSRFBlocked)
        }
        if ip.Equal(cloudMetadataIPv6) {
            return fmt.Errorf("host %q resolves to cloud metadata IP %s: %w", host, addr, ErrSSRFBlocked)
        }
    }
    return nil
}
```

---

### CR-02: `pkg/tldt.Detect` silently ignores `DetectOptions.OutlierThreshold`

**File:** `pkg/tldt/tldt.go:129-136`

**Issue:** The exported `Detect` function accepts a `DetectOptions` struct that documents an `OutlierThreshold` field (line 32: `// default: 0.85`). The function body calls `detector.Analyze(text)` without forwarding the threshold. `detector.Analyze` is hardcoded to `DefaultOutlierThreshold`. Any caller that configures a custom threshold (e.g., stricter `0.95` for high-security pipelines) silently gets the default `0.85` instead. This is a correctness defect in the public API contract.

The same defect exists in `Pipeline` (line 175: `detector.Analyze(text)` — ignores `opts.Detect.OutlierThreshold`).

**Fix:**
```go
// detector.Analyze must accept a threshold, or a separate function must exist.
// If detector exposes AnalyzeWithThreshold(text string, threshold float64) Report:

func Detect(text string, opts DetectOptions) (DetectResult, error) {
    threshold := opts.OutlierThreshold
    if threshold == 0 {
        threshold = detector.DefaultOutlierThreshold
    }
    report := detector.AnalyzeWithThreshold(text, threshold)
    // ... rest unchanged
}
```

If `detector.Analyze` cannot be changed, the threshold must at minimum be validated and an error returned when it is non-zero (to prevent silent misconfiguration), or the field must be removed from the public struct with a doc comment explaining why.

---

## Warnings

### WR-01: Temp files in hook leak on unexpected exit (no `trap` cleanup)

**File:** `internal/installer/hooks/tldt-hook.sh:46-62`

**Issue:** The hook creates two temp files via `mktemp` (lines 46 and 59) and removes them inline with `rm -f` (lines 50 and 62). The script has `set -euo pipefail`. If any command between `mktemp` and `rm -f` exits non-zero for an unexpected reason (e.g., signal, out-of-memory kill of a subprocess), the `set -e` will abort the script before the `rm -f` runs, leaving the temp file on disk. The files contain the summarized content of the user's prompt, which may include sensitive data.

**Fix:**
```bash
# Register cleanup once, before any mktemp calls
STDERR_FILE=$(mktemp)
GUARD_FILE=$(mktemp)
trap 'rm -f "$STDERR_FILE" "$GUARD_FILE"' EXIT

# Remove the explicit rm -f lines on 50 and 62 — trap handles them
```

---

### WR-02: Redirect SSRF check resolves the *redirect target hostname* but not the *final resolved IP* after `http.Client` connects

**File:** `internal/fetcher/fetcher.go:85-94`

**Issue:** `combinedCheckRedirect` calls `lookupHost(req.URL.Hostname())` inside the `CheckRedirect` callback. This is a DNS lookup performed before the connection is made, which is correct in principle. However, Go's `net/http.Client` does its own DNS resolution when it opens the TCP connection — separately from the one in `CheckRedirect`. Between the two resolutions there is a TOCTOU (time-of-check/time-of-use) window: a DNS entry could resolve to a public IP during the SSRF check and then flip to a private IP when the actual TCP SYN goes out (DNS rebinding). This applies to both the initial pre-check (lines 76-82) and the redirect check.

Full DNS rebinding prevention requires pinning the resolved IP (using a custom `net.Dialer` that validates the IP at dial time) rather than checking it at a separate earlier moment.

**Fix:** Replace the standalone `lookupHost` pre-check with a custom `net.Dialer.DialContext` that intercepts the final resolved address at connection time:

```go
dialer := &net.Dialer{}
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        host, port, _ := net.SplitHostPort(addr)
        addrs, err := net.DefaultResolver.LookupHost(ctx, host)
        if err != nil {
            return nil, err
        }
        if err := blockPrivateIP(host, addrs); err != nil {
            return nil, err
        }
        return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0], port))
    },
}
client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: redirectCapOnly}
```

This eliminates the TOCTOU window because the IP check happens at the exact moment the TCP connection is initiated.

---

### WR-03: `--sentences 999` in output guard is a fragile magic sentinel

**File:** `internal/installer/hooks/tldt-hook.sh:60`

**Issue:** The output guard passes `--sentences 999` to prevent re-summarization of the already-summarized output. This relies on the assumption that the summary will always have fewer than 999 sentences, which is a reasonable but undocumented assumption. More importantly, if `tldt` changes its maximum sentence cap behavior (e.g., imposes an upper bound lower than 999), the guard silently becomes a re-summarizer, discarding injection warnings embedded in the summary.

**Fix:** Use a dedicated `--detect-only` flag if one exists, or document this sentinel value prominently with a comment that ties it to the internal maximum sentence limit. At minimum add a comment:

```bash
# 999 exceeds any realistic sentence count — prevents re-summarization.
# If tldt ever adds a --detect-only flag, replace this with that flag.
printf '%s' "$SUMMARY" | tldt --detect-injection --sentences 999 2>"$GUARD_FILE" >/dev/null || true
```

---

### WR-04: `TestResolveInputBytes_Stdin` mutates `os.Stdin` without test isolation guarantee

**File:** `cmd/tldt/main_test.go:191-215`

**Issue:** The test replaces `os.Stdin` globally (line 198: `os.Stdin = r`) and restores it via `defer`. The comment on line 194 notes "Tests in package main run sequentially (no t.Parallel), so global mutation is safe." This is a fragile assumption: if any future test in this package is annotated with `t.Parallel()`, the test will race against this global mutation without any compile-time or runtime warning. `t.Parallel()` is easy to add and the failure mode is non-deterministic.

**Fix:** Use `t.Setenv` pattern or move the stdin-reading logic into a function that accepts an `io.Reader` parameter, enabling test injection without global mutation:
```go
// In production code:
func resolveInputBytes(args []string, filePath string, stdinReader io.Reader) ([]byte, error) { ... }

// In test:
got, err := resolveInputBytes([]string{}, "", strings.NewReader("piped content here"))
```

---

## Info

### IN-01: `DetectOptions.OutlierThreshold` zero-value is ambiguous — "not set" vs "zero threshold"

**File:** `pkg/tldt/tldt.go:31-33`

**Issue:** The `OutlierThreshold` field is a `float64` with zero-value `0.0`. A caller setting `DetectOptions{}` (zero value) currently gets the default `0.85` because the field is ignored (see CR-02). Once CR-02 is fixed, the zero value needs to be distinguished from an explicit `0.0` (which would flag every sentence as an outlier). The idiomatic Go solution is to add a sentinel or use a pointer, but the simplest fix is documentation.

**Fix:** Add to the struct comment:
```go
// OutlierThreshold is the outlier score above which a sentence is flagged suspicious.
// Zero value (0.0) uses the default: detector.DefaultOutlierThreshold (0.85).
// To flag everything, set to a value > 0, e.g., 0.001.
OutlierThreshold float64
```

---

### IN-02: `docs/index.html` loads Tailwind CSS from CDN without SRI hash

**File:** `docs/index.html:8`

**Issue:** `<script src="https://cdn.tailwindcss.com"></script>` loads an external JavaScript file without a `integrity=` Subresource Integrity hash. If the CDN is compromised or the URL is typosquatted, malicious JavaScript executes in visitors' browsers. The Google Fonts `<link>` tags (lines 9-11) have the same issue but CSS carries lower XSS risk.

This is a documentation/landing page, not application code, so severity is Info. However, for a security-focused tool whose landing page discusses SSRF and injection defense, the absence of SRI on the primary script import is an inconsistency worth noting.

**Fix:**
```html
<script src="https://cdn.tailwindcss.com"
        integrity="sha384-<hash-here>"
        crossorigin="anonymous"></script>
```
Or self-host the Tailwind build artifact and remove the CDN dependency entirely (preferred for a security-conscious project).

---

_Reviewed: 2026-05-02T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
