# Phase 4: URL Input - Research

**Researched:** 2026-05-02
**Domain:** Go HTTP fetching, HTML-to-text extraction, CLI flag wiring
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INP-01 | User can run `tldt --url <url>` to fetch a webpage, strip boilerplate HTML, and receive an extractive summary on stdout | `net/http` Client for fetching; `github.com/go-shiori/go-readability` for boilerplate stripping (`Article.TextContent`); wire via new `--url` flag in `resolveInputBytes()` |
| INP-02 | URL fetcher handles HTTP redirects; returns non-zero exit code with error to stderr on fetch failure | `net/http.Client` follows redirects by default (up to 10); non-2xx status checked via `resp.StatusCode`; error printed to stderr; `os.Exit(1)` |
</phase_requirements>

---

## Summary

Phase 4 adds a single new input path to tldt: the `--url` flag. The architecture is clean because `resolveInputBytes()` already handles input dispatch — `--url` is a new highest-priority branch that fetches a URL, strips HTML boilerplate to plain text, then returns that text as `[]byte`. Everything downstream (validation, sentence cap, summarizer, formatter) is entirely unchanged.

The two interesting sub-problems are (1) HTML boilerplate removal and (2) test isolation. For boilerplate removal, `github.com/go-shiori/go-readability` (Readability.js port) is the right choice: it strips nav, ads, headers, footers and returns `Article.TextContent` as clean prose. For testing, `net/http/httptest.NewServer` (stdlib) creates an in-process HTTP server so tests never make real network calls.

Security surface is narrow but real: a CLI tool with arbitrary URL input needs a timeout (DoS via slow server), a response size cap (memory DoS), and a scheme allowlist (prevent `file://`, `ftp://`). SSRF is not a significant concern for a local CLI tool (there is no server side), but the allowlist still guards against accidental misuse.

**Primary recommendation:** New package `internal/fetcher/fetcher.go` wraps fetch + readability extraction with a configurable timeout and size cap. `cmd/tldt/main.go` adds `--url` flag; `resolveInputBytes()` gains a new top branch. Tests use `httptest.NewServer`.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP fetch + redirect follow | `internal/fetcher/` | — | Isolates network I/O; keeps main.go thin |
| HTML boilerplate stripping | `internal/fetcher/` via go-readability | — | Content extraction is a fetcher concern |
| Content-type validation | `internal/fetcher/` | — | Gate on `Content-Type: text/html` before extraction |
| Timeout + size cap | `internal/fetcher/` | — | Resource limits belong near the I/O source |
| URL scheme validation | `internal/fetcher/` | — | Reject non-http(s) before any network call |
| `--url` flag wiring | `cmd/tldt/main.go` | — | CLI concerns stay in main |
| Input precedence | `resolveInputBytes()` in `main.go` | — | Existing dispatch function; add `--url` as new top branch |
| Error to stderr / exit 1 | `cmd/tldt/main.go` | — | Consistent with existing error-handling pattern |
| Test isolation (mock server) | `internal/fetcher/fetcher_test.go` | — | `httptest.NewServer` replaces real network |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` | stdlib | HTTP GET, redirect following, timeout via `http.Client` | Zero deps; follows redirects by default (10 max); supports `context.WithTimeout` |
| `net/http/httptest` | stdlib | Mock HTTP server for tests | Standard Go testing pattern; no real network calls |
| `context` | stdlib | Request timeout via `context.WithTimeout` | Required for `http.NewRequestWithContext` |
| `io` | stdlib | `io.LimitReader` for max response size cap | Prevents memory exhaustion from huge pages |
| `net/url` | stdlib | URL parsing and scheme validation | Rejects `file://`, `ftp://` before any request |
| `github.com/go-shiori/go-readability` | `v0.0.0-20251205110129-5db1dc9836f0` | Readability.js port — strips nav/ads/boilerplate, returns `Article.TextContent` | Most complete Go Readability implementation; correct for article extraction |

**Version verified:** `go list -m github.com/go-shiori/go-readability@latest` returns `v0.0.0-20251205110129-5db1dc9836f0` [VERIFIED: Go module proxy]

**Note on go-readability deprecation:** The `github.com/go-shiori/go-readability` repo was archived read-only on 2025-12-30. The recommended successor is `codeberg.org/readeck/go-readability/v2`. For Phase 4, the archived version is still safe to use — it is fully functional, MIT-licensed, and the deprecation only means no new features. The planner should note this as a future migration point. [CITED: github.com/go-shiori/go-readability README]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `jaytaylor.com/html2text` | `v0.0.0-20260303211410-1a4bdc82ecec` | HTML-to-text with link/table formatting | Use only if go-readability produces empty TextContent (e.g., non-article pages like homepages) — fallback, not primary |

**Version verified:** `go list -m jaytaylor.com/html2text@latest` returns `v0.0.0-20260303211410-1a4bdc82ecec` [VERIFIED: Go module proxy]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-readability | `golang.org/x/net/html` + hand-rolled DOM walker | Much more code (200+ lines) to replicate what Readability.js does; misses score-based content detection |
| go-readability | `github.com/PuerkitoBio/goquery` | goquery is a selector/scraping tool, not a content extractor; still requires custom boilerplate-removal logic |
| go-readability | `jaytaylor/html2text` | html2text converts all HTML to text including nav/footer noise; no boilerplate removal |
| `httptest.NewServer` | Real network calls in tests | Flaky, slow, blocked in CI; never use real URLs in unit tests |
| `io.LimitReader` | Unlimited response read | `io.ReadAll` on an adversarial 1GB page will OOM the process |

**Installation:**
```bash
go get github.com/go-shiori/go-readability@v0.0.0-20251205110129-5db1dc9836f0
```

---

## Architecture Patterns

### System Architecture Diagram

```
cmd/tldt/main.go
  --url flag
      |
      v
resolveInputBytes()   <-- new top branch: if urlFlag != "" → call fetcher
      |
      v
internal/fetcher/Fetch(url, timeout, maxBytes)
      |
      +-- net/url.Parse()         <-- scheme check (http/https only)
      |
      +-- net/http.Client{Timeout} + context.WithTimeout
      |       |
      |       v
      |   HTTP GET → follows redirects automatically (max 10)
      |       |
      |       v
      |   resp.StatusCode check   <-- non-2xx → error(statusCode, url)
      |       |
      |       v
      |   resp.Header.Get("Content-Type") check  <-- must contain "text/html"
      |       |
      |       v
      |   io.LimitReader(resp.Body, maxBytes)    <-- cap at 5MB
      |       |
      |       v
      |   readability.FromReader(body, pageURL)  <-- strips boilerplate
      |       |
      |       v
      |   article.TextContent                    <-- clean prose string
      |
      v
[]byte(article.TextContent)   <-- returned to resolveInputBytes
      |
      v
validateInput()    (existing — unchanged)
      |
      v
applySentenceCap() (existing — unchanged)
      |
      v
summarizer.New(algo).Summarize(text, n)  (existing — unchanged)
      |
      v
formatter dispatch (existing — unchanged)
      |
      v
stdout: summary text only
stderr: token stats (TTY gate — unchanged)
```

### Recommended Project Structure

```
cmd/tldt/
  main.go             # add --url flag; new top branch in resolveInputBytes()
  main_test.go        # add TestMain_URLFlag_* tests via httptest.NewServer
internal/
  fetcher/
    fetcher.go        # NEW: Fetch() function wrapping net/http + go-readability
    fetcher_test.go   # NEW: unit tests using httptest.NewServer
  formatter/          # UNCHANGED
  summarizer/         # UNCHANGED
```

### Pattern 1: New `--url` flag + resolveInputBytes branch

**What:** Add `urlFlag` as a new string flag. In `resolveInputBytes()`, check it first (highest priority). If set, call `fetcher.Fetch()` and return the result.
**Precedence decision:** `--url` takes top priority because it is the most explicit. Existing precedence (stdin > -f > positional) is unchanged when `--url` is absent.

```go
// Source: adapted from existing resolveInputBytes pattern in cmd/tldt/main.go
urlFlag := flag.String("url", "", "URL of a webpage to fetch and summarize")

func resolveInputBytes(args []string, filePath string, urlStr string) ([]byte, error) {
    // NEW: --url branch (highest priority — most explicit input)
    if urlStr != "" {
        text, err := fetcher.Fetch(urlStr, 30*time.Second, 5<<20) // 5MB cap
        if err != nil {
            return nil, err
        }
        return []byte(text), nil
    }
    // existing: stdin pipe > -f > positional args (unchanged)
    ...
}
```

[VERIFIED: resolveInputBytes signature from codebase — adding urlStr parameter extends existing pattern without breaking callers]

### Pattern 2: fetcher.Fetch()

**What:** Single exported function that validates URL, fetches with timeout, checks status and content-type, caps response size, extracts text via go-readability.

```go
// Source: net/http stdlib + github.com/go-shiori/go-readability
package fetcher

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "time"

    readability "github.com/go-shiori/go-readability"
)

// Fetch fetches a URL and returns the main article text content.
// timeout applies to the entire HTTP round-trip.
// maxBytes caps the response body read to prevent memory exhaustion.
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error) {
    // 1. Validate scheme — block file://, ftp://, etc.
    u, err := url.Parse(rawURL)
    if err != nil {
        return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return "", fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
    }

    // 2. Build context-scoped request (supports timeout)
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
    if err != nil {
        return "", fmt.Errorf("building request for %q: %w", rawURL, err)
    }
    req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

    // 3. Execute — net/http.Client follows redirects automatically (up to 10)
    client := &http.Client{Timeout: timeout}
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("fetching %q: %w", rawURL, err)
    }
    defer resp.Body.Close()

    // 4. Check HTTP status (non-2xx is an error)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("HTTP %d fetching %q", resp.StatusCode, rawURL)
    }

    // 5. Validate Content-Type — only proceed for HTML
    ct := resp.Header.Get("Content-Type")
    if !strings.Contains(ct, "text/html") {
        return "", fmt.Errorf("unsupported content type %q at %q (expected text/html)", ct, rawURL)
    }

    // 6. Cap response body to prevent memory exhaustion
    limited := io.LimitReader(resp.Body, maxBytes)

    // 7. Extract readable article text (strips nav, ads, footers)
    article, err := readability.FromReader(limited, u)
    if err != nil {
        return "", fmt.Errorf("extracting content from %q: %w", rawURL, err)
    }

    text := strings.TrimSpace(article.TextContent)
    if text == "" {
        return "", fmt.Errorf("no readable text content found at %q", rawURL)
    }

    return text, nil
}
```

[CITED: pkg.go.dev/github.com/go-shiori/go-readability — FromReader(io.Reader, *url.URL) returns Article with TextContent string]
[VERIFIED: net/http, context, io, net/url — all stdlib, available in Go 1.26.2]

### Pattern 3: httptest.NewServer for tests

**What:** Create an in-process HTTP server that serves controlled HTML. Pass its URL to Fetch(). No real network calls.
**When to use:** All unit and integration tests involving URL fetching.

```go
// Source: stdlib net/http/httptest [VERIFIED: go doc net/http/httptest NewServer]
func TestFetch_OK(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        fmt.Fprint(w, `<html><body>
            <nav>Navigation junk</nav>
            <article>
              <p>Alice discovered that the method worked well on long documents.
              She tested it against many articles and found consistent results.
              The algorithm proved reliable across domains.</p>
            </article>
            <footer>Footer noise</footer>
        </body></html>`)
    }))
    defer ts.Close()

    text, err := fetcher.Fetch(ts.URL, 5*time.Second, 1<<20)
    if err != nil {
        t.Fatalf("Fetch: %v", err)
    }
    if strings.TrimSpace(text) == "" {
        t.Error("expected non-empty text content")
    }
    if strings.Contains(text, "Navigation junk") {
        t.Error("nav junk leaked into text content")
    }
}
```

[VERIFIED: httptest.NewServer is stdlib — go doc net/http/httptest confirms]

### Pattern 4: Error cases for tests

```go
func TestFetch_404(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.NotFound(w, r)
    }))
    defer ts.Close()
    _, err := fetcher.Fetch(ts.URL, 5*time.Second, 1<<20)
    if err == nil {
        t.Error("expected error for 404, got nil")
    }
    if !strings.Contains(err.Error(), "404") {
        t.Errorf("expected 404 in error, got %q", err.Error())
    }
}

func TestFetch_Redirect(t *testing.T) {
    // httptest server that redirects /old → /new
    mux := http.NewServeMux()
    mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "/new", http.StatusMovedPermanently)
    })
    mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprint(w, `<html><body><article><p>Redirected content successfully.</p></article></body></html>`)
    })
    ts := httptest.NewServer(mux)
    defer ts.Close()

    text, err := fetcher.Fetch(ts.URL+"/old", 5*time.Second, 1<<20)
    if err != nil {
        t.Fatalf("redirect: unexpected error: %v", err)
    }
    if !strings.Contains(text, "Redirected content") {
        t.Errorf("redirect: expected content after redirect, got %q", text)
    }
}

func TestFetch_InvalidScheme(t *testing.T) {
    _, err := fetcher.Fetch("file:///etc/passwd", 5*time.Second, 1<<20)
    if err == nil {
        t.Error("expected error for file:// scheme")
    }
}

func TestFetch_NonHTMLContentType(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/pdf")
        w.Write([]byte("%PDF-1.4"))
    }))
    defer ts.Close()
    _, err := fetcher.Fetch(ts.URL, 5*time.Second, 1<<20)
    if err == nil {
        t.Error("expected error for PDF content-type")
    }
}
```

### Pattern 5: main.go integration test via subprocess (existing pattern)

```go
// Source: existing cmd/tldt/main_test.go pattern using run() helper
func TestMain_URLFlag_ServesHTML(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprint(w, `<html><body><article>
            <p>The fox is clever and quick in the forest.</p>
            <p>Dogs are loyal and brave companions.</p>
            <p>Scientists study animals carefully for years.</p>
        </article></body></html>`)
    }))
    defer ts.Close()

    stdout, _, ok := run(t, "", "--url", ts.URL, "--sentences", "2")
    if !ok {
        t.Fatal("--url: binary exited non-zero")
    }
    if strings.TrimSpace(stdout) == "" {
        t.Error("--url: expected non-empty summary")
    }
}

func TestMain_URLFlag_404(t *testing.T) {
    ts := httptest.NewServer(http.NotFoundHandler())
    defer ts.Close()
    _, stderr, ok := run(t, "", "--url", ts.URL)
    if ok {
        t.Error("--url 404: expected non-zero exit")
    }
    if !strings.Contains(stderr, "404") {
        t.Errorf("--url 404: stderr %q does not mention 404", stderr)
    }
}
```

### Anti-Patterns to Avoid

- **Real network calls in tests:** Never call external URLs (httpstat.us, example.com) in tests. Use httptest.NewServer. Real URLs are flaky, rate-limited, and fail in CI.
- **Using html2text as primary extractor:** html2text converts all HTML including nav/footer/ads. It produces noisy output for typical webpages. go-readability uses scoring to identify article body.
- **Reading full response body before size check:** Always wrap with `io.LimitReader` before `io.ReadAll` or passing to readability.
- **Setting Timeout on http.Client only:** `http.Client.Timeout` covers the full round-trip but not streaming reads after headers arrive. Use `io.LimitReader` as a belt-and-suspenders cap for response size.
- **Forgetting Content-Type check:** Attempting readability extraction on a PDF, image, or JSON response will produce garbage or a panic. Check Content-Type before reading body.
- **Modifying resolveInputBytes to accept the URL directly from flag.Parse():** Instead, pass the flag value as a parameter. This keeps the function testable without a full binary subprocess.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTML boilerplate removal | DOM walker that skips `<nav>`, `<footer>`, `<header>` | `github.com/go-shiori/go-readability` | Readability uses per-node scoring (link density, text density, class names) — a tag-name blocklist misses 50% of real-world boilerplate |
| HTML-to-text conversion | String-replacing `<` and `>` | go-readability's `TextContent` field | Entities, nested tags, script/style blocks need proper HTML parsing |
| Redirect following | Manual 301/302 Location header chasing | `net/http.Client` (built-in, up to 10 hops) | Infinite-loop guard, relative URL resolution, HTTPS upgrade — stdlib handles all of it |
| Response timeout | Manual goroutine + time.After | `context.WithTimeout` + `http.NewRequestWithContext` | Context cancellation propagates cleanly; goroutine approach leaks on error |

**Key insight:** The entire fetch-and-extract pipeline is ~50 lines using stdlib + go-readability. Any custom HTML parsing code will be 5-10x longer and still miss edge cases the Readability algorithm handles.

---

## Common Pitfalls

### Pitfall 1: go-readability returns empty TextContent on non-article pages
**What goes wrong:** `article.TextContent` is empty for pages like homepages, login pages, search results — pages that are heavy navigation with no article body.
**Why it happens:** Readability's scoring algorithm finds no content block exceeding the threshold.
**How to avoid:** After extracting `TextContent`, check `strings.TrimSpace(text) == ""` and return a descriptive error: `"no readable text content found at <url>"`. Let the caller (main.go) print this to stderr and exit non-zero.
**Warning signs:** `tldt --url https://google.com` hangs or exits 0 with no output.

### Pitfall 2: Content-Type with charset suffix
**What goes wrong:** `resp.Header.Get("Content-Type")` returns `"text/html; charset=utf-8"` — a string equality check against `"text/html"` fails.
**How to avoid:** Use `strings.Contains(ct, "text/html")` not `ct == "text/html"`.
**Warning signs:** Valid HTML pages rejected with "unsupported content type" error.

### Pitfall 3: Double timeout (context + Client.Timeout)
**What goes wrong:** Setting both `context.WithTimeout` and `http.Client{Timeout}` to the same value is redundant but harmless. Setting them to different values causes confusion about which fires.
**How to avoid:** Pick one: use `http.Client{Timeout: timeout}` for simplicity. If you need per-request timeout variation, use context instead and set `Client.Timeout` to zero.
**Recommendation for Phase 4:** Use `http.Client{Timeout: timeout}` — simpler, sufficient for a CLI tool.

### Pitfall 4: go-readability.FromURL does its own HTTP fetch
**What goes wrong:** `readability.FromURL(rawURL, timeout)` does the HTTP fetch internally, bypassing our size cap and Content-Type check. It also provides no way to inject a custom http.Client.
**How to avoid:** Use `readability.FromReader(body, pageURL)` instead — fetch manually with our controlled client, then pass the body to FromReader.
**Warning signs:** Large pages (> 10MB) cause OOM; PDFs cause parse errors.

### Pitfall 5: Passing `rawURL` string to readability.FromReader instead of `*url.URL`
**What goes wrong:** `readability.FromReader` takes `(io.Reader, *url.URL)` — the second argument is a parsed URL (for resolving relative links), not a raw string.
**How to avoid:** Parse with `url.Parse(rawURL)` before calling Fetch; pass `u` (the `*url.URL`) to `readability.FromReader(body, u)`.
**Warning signs:** Compile error: `cannot use string as *url.URL`.

### Pitfall 6: resolveInputBytes signature change breaks existing unit tests
**What goes wrong:** Adding a `urlStr string` parameter to `resolveInputBytes` changes the function signature — existing direct unit tests in `main_test.go` that call `resolveInputBytes(args, filePath)` fail to compile.
**How to avoid:** Update all existing call sites in `main_test.go` to pass the new parameter (empty string `""` for all non-URL tests). There are 5 existing test calls — update them all.
**Warning signs:** `go test ./cmd/tldt/ ...` fails with "too few arguments in call to resolveInputBytes".

### Pitfall 7: No User-Agent header causes bot detection blocks
**What goes wrong:** The default Go HTTP client sends `Go-http-client/2.0` as User-Agent. Many sites return 403 or redirect to a CAPTCHA.
**How to avoid:** Set a descriptive User-Agent: `req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")`.
**Warning signs:** `tldt --url https://medium.com/...` returns 403 or empty content.

---

## Code Examples

### Full fetcher.Fetch() implementation
```go
// Source: stdlib + github.com/go-shiori/go-readability (see Pattern 2 above)
// Key imports: net/http, context, io, net/url, strings, time, readability
```
(Full implementation shown in Pattern 2 above.)

### resolveInputBytes updated signature
```go
// Source: existing cmd/tldt/main.go resolveInputBytes + new --url branch
func resolveInputBytes(args []string, filePath string, urlStr string) ([]byte, error) {
    if urlStr != "" {
        text, err := fetcher.Fetch(urlStr, 30*time.Second, 5<<20)
        if err != nil {
            return nil, fmt.Errorf("fetching URL: %w", err)
        }
        return []byte(text), nil
    }
    // ... existing stdin/file/positional logic unchanged ...
}
```

### main.go flag addition
```go
// Add alongside existing flags
urlFlag := flag.String("url", "", "URL of a webpage to fetch and summarize")
// ...
rawBytes, err := resolveInputBytes(flag.Args(), *filePath, *urlFlag)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fetch raw HTML + strip tags manually | `readability.FromReader` → `Article.TextContent` | go-readability (2019+) | Article-quality extraction; removes nav/ads/footers via scoring |
| `golang.org/x/net/html` for text extraction | go-readability wraps it internally | — | go-readability is the high-level API; x/net/html is the low-level engine |
| `go-shiori/go-readability` (now archived) | `codeberg.org/readeck/go-readability/v2` | 2025-12-30 | v2 tracks Readability.js v0.6; migration is future work |

**Deprecated/outdated:**
- `readability.FromURL()`: Still works but bypasses our custom HTTP client — use `FromReader()` instead (see Pitfall 4)
- `github.com/go-shiori/go-readability`: Archived 2025-12-30; migration to `codeberg.org/readeck/go-readability/v2` is recommended as future work but not required for Phase 4

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `--url` takes highest precedence over stdin/file/positional args | Pattern 1 / Architecture | Low risk: if stdin is piped AND --url is set, one must win; --url being most explicit is natural but could surprise users piping content |
| A2 | 30 seconds is an appropriate fetch timeout for a CLI tool | Pattern 2 | Low: too short for slow sites, too long for interactive use — could be configurable later |
| A3 | 5MB (5<<20) is an appropriate response size cap | Pattern 2 | Low: most articles are <500KB; 5MB is generous; could be configurable later |
| A4 | `article.TextContent` from go-readability is sufficient for extractive summarization | Standard Stack | Medium: on non-article pages, TextContent may be empty or very short; the fallback error message handles this |
| A5 | `github.com/go-shiori/go-readability` archived version is acceptable for Phase 4 | Standard Stack | Low for Phase 4: library is complete and functional; risk increases over time as Go version advances |

---

## Open Questions (RESOLVED)

1. **go-readability deprecation — use archived or migrate to v2?**
   - What we know: `github.com/go-shiori/go-readability` archived 2025-12-30; `codeberg.org/readeck/go-readability/v2` is the successor
   - What's unclear: Does v2 have the same `FromReader(io.Reader, *url.URL)` API? Package name change?
   - Recommendation: Use the archived version for Phase 4 (it works); add a TODO comment noting the future migration. Do not let migration scope-creep Phase 4.

2. **What happens when a URL serves a redirect to a non-HTML page?**
   - What we know: `net/http.Client` follows redirects automatically; Content-Type check fires after the final redirect
   - What's unclear: no gap — the Content-Type check on the final response handles this correctly
   - Recommendation: No action needed; document the behavior in fetcher comments.

3. **Should `--url` error if stdin is also piped?**
   - What we know: current precedence puts stdin first in resolveInputBytes
   - What's unclear: whether silent precedence (--url wins) or explicit error (conflict) is better UX
   - Recommendation: Silent precedence (--url wins); consistent with `-f` already winning over positional args in the current code [ASSUMED]

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | Yes | 1.26.2 darwin/arm64 | — |
| `net/http` | HTTP fetch | Yes (stdlib) | — | — |
| `net/http/httptest` | Tests | Yes (stdlib) | — | — |
| `context` | Request timeout | Yes (stdlib) | — | — |
| `io.LimitReader` | Response size cap | Yes (stdlib) | — | — |
| `github.com/go-shiori/go-readability` | HTML extraction | Needs `go get` | v0.0.0-20251205110129 | — |
| Internet access (for real-URL smoke tests) | Manual testing only | Yes | — | httptest.NewServer for automated tests |

[VERIFIED: Go version via `go version`; stdlib packages via `go doc`; module versions via `go list -m ... @latest`]

**Missing dependencies with no fallback:** None — `go get github.com/go-shiori/go-readability@...` resolves the only non-stdlib dependency.

---

## Validation Architecture

> `workflow.nyquist_validation` is `false` in `.planning/config.json` — this section is skipped per config.

---

## Security Domain

Phase 4 opens a network fetch path — a new attack surface compared to v1.0's purely local input. SSRF is primarily a server-side concern, but URL validation is still warranted for a CLI tool.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `net/url.Parse()` + scheme allowlist (http/https only) |
| V2 Authentication | no | No auth, local CLI |
| V3 Session Management | no | Stateless, no sessions |
| V4 Access Control | no | No ACL |
| V6 Cryptography | no | TLS handled by `net/http` stdlib |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| `file:///etc/passwd` via `--url` | Information disclosure | Scheme allowlist: reject non-http(s) in `fetcher.Fetch()` before any read |
| Slow-read server (tarpit) | DoS | `http.Client{Timeout: 30s}` terminates stalled connections |
| Huge response body (memory exhaustion) | DoS | `io.LimitReader(resp.Body, 5<<20)` caps at 5MB |
| Non-HTML content (PDF, binary) | Unexpected behavior | `Content-Type` check; reject non-text/html before passing to readability |
| Redirect to internal/private IP | SSRF (low risk for local CLI) | Mitigated by OS network stack for local CLI; not a significant threat without a server |

**Note on SSRF:** Classical SSRF is a server-side concern (attacker forces a server to fetch internal resources). For a local CLI tool running on the user's machine, `--url file:///etc/passwd` is the meaningful risk — blocked by the scheme allowlist. IP-range blocking (blocking 169.254.x.x, 10.x, etc.) is NOT required for a local CLI. [ASSUMED: if tldt is ever deployed as a service/API, SSRF protection would need a deny-list or dialer-level check]

---

## Sources

### Primary (HIGH confidence)
- Codebase: `cmd/tldt/main.go` — verified current `resolveInputBytes()` signature, existing flag patterns, test helpers [VERIFIED]
- Codebase: `cmd/tldt/main_test.go` — verified `run()` helper pattern and existing test structure [VERIFIED]
- Go stdlib: `net/http`, `net/http/httptest`, `context`, `io`, `net/url` — all verified via `go doc` [VERIFIED]
- Go module proxy: `go list -m github.com/go-shiori/go-readability@latest` → `v0.0.0-20251205110129-5db1dc9836f0` [VERIFIED]
- Go module proxy: `go list -m jaytaylor.com/html2text@latest` → `v0.0.0-20260303211410-1a4bdc82ecec` [VERIFIED]

### Secondary (MEDIUM confidence)
- [pkg.go.dev/github.com/go-shiori/go-readability](https://pkg.go.dev/github.com/go-shiori/go-readability) — `FromURL`, `FromReader`, `Article.TextContent` API confirmed [CITED]
- [pkg.go.dev/jaytaylor.com/html2text](https://pkg.go.dev/jaytaylor.com/html2text) — `FromString`, `FromReader` API confirmed [CITED]
- [go-shiori/go-readability GitHub](https://github.com/go-shiori/go-readability) — archived 2025-12-30; successor is `codeberg.org/readeck/go-readability/v2` [CITED]
- [pkg.go.dev/net/http/httptest](https://pkg.go.dev/net/http/httptest) — `httptest.NewServer` pattern confirmed [CITED]

### Tertiary (LOW confidence)
- WebSearch results on SSRF prevention — general guidance, not verified against specific Go version; used only for non-mandatory CLI-context considerations
- WebSearch results on html2text vs goquery comparison — cross-referenced with pkg.go.dev

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions verified via Go module proxy; APIs confirmed via go doc and pkg.go.dev
- Architecture: HIGH — based on direct codebase inspection; pattern follows existing resolveInputBytes design
- Pitfalls: HIGH — derived from API-level understanding of readability.FromReader vs FromURL; Content-Type parsing behavior is documented stdlib behavior

**Research date:** 2026-05-02
**Valid until:** 2026-08-02 (go-readability is archived so no new API changes; stdlib is stable)
