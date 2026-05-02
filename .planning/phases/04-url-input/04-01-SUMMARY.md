---
plan: 04-01
phase: 04-url-input
status: complete
completed: "2026-05-02"
---

# Plan 04-01: internal/fetcher Package

## What Was Built

Created `internal/fetcher` package providing a single exported function `Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error)` that fetches a URL and extracts the main article text.

Added `github.com/go-shiori/go-readability` dependency to `go.mod`.

## Key Files

### key-files.created
- `internal/fetcher/fetcher.go` — Fetch function implementation
- `internal/fetcher/fetcher_test.go` — 5 httptest-based unit tests

### key-files.modified
- `go.mod` — added go-readability dependency
- `go.sum` — updated checksums

## Implementation Notes

- Uses `readability.FromReader(limited, u)` NOT `FromURL` — preserves size cap and custom http client
- `io.LimitReader(resp.Body, maxBytes)` caps at 5MB before any read
- Scheme allowlist: rejects non-http(s) before any network call (SSRF mitigation)
- `strings.Contains(ct, "text/html")` handles `text/html; charset=utf-8` correctly
- TODO comment added for future migration to `codeberg.org/readeck/go-readability/v2`

## Tests

All 5 tests pass with zero real network calls (all use `httptest.NewServer`):
- `TestFetch_OK` — valid HTML returns non-empty cleaned text, nav junk stripped
- `TestFetch_404` — 404 response returns error containing "404"
- `TestFetch_Redirect` — 301 redirect followed, content from destination returned
- `TestFetch_InvalidScheme` — `file://` returns "unsupported URL scheme" error
- `TestFetch_NonHTMLContentType` — `application/pdf` returns "unsupported content type" error

## Self-Check: PASSED

- `go test ./internal/fetcher/... -v -count=1` → 5/5 PASS
- `go build ./...` → exit 0
- `go vet ./internal/fetcher/...` → exit 0
- `grep "readability.FromReader" internal/fetcher/fetcher.go` → match
- `grep "io.LimitReader" internal/fetcher/fetcher.go` → match
- `grep "u.Scheme" internal/fetcher/fetcher.go` → match
