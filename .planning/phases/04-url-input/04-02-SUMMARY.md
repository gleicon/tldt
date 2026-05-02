---
plan: 04-02
phase: 04-url-input
status: complete
completed: "2026-05-02"
---

# Plan 04-02: --url Flag Wiring in main.go

## What Was Built

Wired `--url` flag into `cmd/tldt/main.go`. Extended `resolveInputBytes` signature to accept a `urlStr string` third parameter. The URL branch runs first (highest priority), delegating to `fetcher.Fetch(urlStr, 30*time.Second, 5<<20)`.

Fixed 5 existing `resolveInputBytes` call sites in `main_test.go` (added `""` as third arg). Added 2 new integration tests.

## Key Files

### key-files.modified
- `cmd/tldt/main.go` — added `--url` flag, fetcher import, time import, updated signature and call site
- `cmd/tldt/main_test.go` — fixed 5 call sites, added 2 URL integration tests

## Implementation Notes

- URL branch is highest priority in resolveInputBytes (URL > stdin > file > positional)
- Passes fixed constants to Fetch: 30s timeout, 5MB cap — as specified in research
- All security controls enforced inside fetcher.Fetch (scheme allowlist, content-type, size cap)
- Error from fetcher.Fetch propagates to stderr via existing `fmt.Fprintln(os.Stderr, err)` pattern

## Tests

All 199 tests pass including:
- `TestMain_URLFlag_ServesHTML` — valid HTML httptest server → non-empty summary on stdout, exit 0
- `TestMain_URLFlag_404` — 404 httptest server → non-zero exit, "404" in stderr
- All 5 pre-existing `TestResolveInputBytes_*` tests compile with new 3-argument signature

## Self-Check: PASSED

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go test ./... -count=1` → 199/199 PASS
- `grep "urlFlag" cmd/tldt/main.go` → 3 matches (declaration, call site, parameter)
- `grep "fetcher.Fetch" cmd/tldt/main.go` → match inside resolveInputBytes
