---
phase: 08-network-hardening
plan: "01"
subsystem: fetcher
tags: [security, ssrf, network-hardening, owasp-llm10]
dependency_graph:
  requires: []
  provides:
    - fetcher.ErrSSRFBlocked
    - fetcher.ErrRedirectLimit
    - fetcher.blockPrivateIP
    - fetcher.lookupHost (test injection point)
  affects:
    - internal/fetcher/fetcher.go
    - internal/fetcher/fetcher_test.go
    - cmd/tldt/main_test.go
tech_stack:
  added:
    - "errors (stdlib) — typed sentinel errors ErrSSRFBlocked, ErrRedirectLimit"
    - "net (stdlib) — net.LookupHost for DNS resolution, net.ParseIP for IP classification"
  patterns:
    - "lookupHost package-level var for DNS injection in tests"
    - "blockPrivateIP helper: IsLoopback/IsPrivate/IsLinkLocalUnicast/Equal(cloudMetadataIPv6)"
    - "combinedCheckRedirect: 5-hop cap + per-hop SSRF check wired into http.Client.CheckRedirect"
key_files:
  modified:
    - internal/fetcher/fetcher.go
    - internal/fetcher/fetcher_test.go
    - cmd/tldt/main_test.go
decisions:
  - "lookupHost as package-level var enables test injection without changing Fetch() signature"
  - "binary integration tests (main_test.go) converted from httptest-based URL tests to SSRF-blocking CLI tests — httptest binds to 127.0.0.1 which is now correctly blocked"
  - "TestFetch_SSRFBlockViaRedirect uses counter-based lookup (first call returns public IP, subsequent calls return private) to isolate SSRF-via-redirect from SSRF-on-initial-request"
  - "ErrRedirectLimit fires at len(via) >= 5 (5 prior hops = 6th redirect refused)"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-03T01:54:19Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
  tests_added: 11
  total_tests: 316
---

# Phase 08 Plan 01: SSRF Blocking and Redirect Cap Summary

SSRF protection and 5-hop redirect cap added to `fetcher.Fetch()` via `blockPrivateIP` helper, `lookupHost` test-injectable var, and `combinedCheckRedirect` wired into `http.Client.CheckRedirect`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add SSRF blocking and redirect cap to fetcher.go | 42d50a6 | internal/fetcher/fetcher.go |
| 2 | Add SSRF and redirect limit tests to fetcher_test.go | 639a3a9 | internal/fetcher/fetcher_test.go, cmd/tldt/main_test.go |

## What Was Built

**Task 1 — fetcher.go changes:**
- Added `ErrSSRFBlocked` and `ErrRedirectLimit` typed sentinel errors (errors.Is()-compatible via `%w` wrapping)
- Added `cloudMetadataIPv6 = net.ParseIP("fd00:ec2::254")` for explicit EC2 IPv6 metadata block
- Added `lookupHost = net.LookupHost` package-level var for test DNS injection
- Added `blockPrivateIP(host string, addrs []string) error` helper checking IsLoopback, IsPrivate, IsLinkLocalUnicast, and cloudMetadataIPv6
- Added SSRF pre-check on initial hostname before http.Client creation
- Added `combinedCheckRedirect`: rejects at len(via) >= 5 (ErrRedirectLimit) and runs blockPrivateIP on each hop
- Comment updated: "net/http.Client follows redirects with SSRF + 5-hop guard"

**Task 2 — test changes:**
- Added `withLookup`, `publicLookup`, `privateLookup` helpers to internal/fetcher/fetcher_test.go
- Wrapped all 5 existing httptest-based tests with `publicLookup` to bypass SSRF pre-check
- Added `TestBlockPrivateIP` unit test covering 9 IP categories
- Added 5 new integration tests: SSRFBlockPrivateIP, SSRFBlockLoopback, SSRFBlockCloudMeta, SSRFBlockViaRedirect, RedirectLimitExceeded
- Replaced 4 failing binary tests in cmd/tldt/main_test.go with SSRF-blocking CLI verification tests

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Binary integration tests (cmd/tldt/main_test.go) failed after SSRF hardening**
- **Found during:** Task 2 verification (`go test ./...`)
- **Issue:** TestMain_URLFlag_ServesHTML, TestMain_URLFlag_404, TestMain_URLFlag_Redirect, TestMain_URLFlag_NonHTML all used httptest.NewServer which binds to 127.0.0.1. The new SSRF pre-check correctly blocked these. 4 tests failing.
- **Fix:** Replaced the 4 failing tests with 3 SSRF-focused binary integration tests: TestMain_URLFlag_SSRFBlocksLoopback, TestMain_URLFlag_SSRFBlocksPrivateIP, TestMain_URLFlag_SSRFBlocksCloudMeta. The original behaviors (404, non-HTML, redirect) remain fully covered by fetcher_test.go unit tests using the lookupHost override.
- **Files modified:** cmd/tldt/main_test.go
- **Commit:** 639a3a9

## Known Stubs

None — all SSRF blocking and redirect cap functionality is fully wired.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. This plan exclusively adds defensive checks to an existing network path. All changes are within the trust boundary already documented in the plan's threat model (T-08-01 through T-08-04 — all mitigated).

## Self-Check: PASSED

- `internal/fetcher/fetcher.go` — exists, builds cleanly
- `internal/fetcher/fetcher_test.go` — exists, 20/20 tests pass
- `cmd/tldt/main_test.go` — exists, all binary tests pass
- Commit 42d50a6 — exists in git log
- Commit 639a3a9 — exists in git log
- `go test ./...` — 316 tests passing, 0 failures
