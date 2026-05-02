# Phase 4: URL Input - Pattern Map

**Mapped:** 2026-05-02
**Files analyzed:** 5
**Analogs found:** 4 / 5

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/fetcher/fetcher.go` | service | request-response | `internal/summarizer/lexrank.go` | role-match (both are pure internal packages with one exported function) |
| `internal/fetcher/fetcher_test.go` | test | request-response | `internal/summarizer/lexrank_test.go` | role-match |
| `cmd/tldt/main.go` | controller | request-response | `cmd/tldt/main.go` (self — extend) | exact |
| `cmd/tldt/main_test.go` | test | request-response | `cmd/tldt/main_test.go` (self — extend) | exact |
| `go.mod` / `go.sum` | config | — | `go.mod` (self — extend) | exact |

---

## Pattern Assignments

### `internal/fetcher/fetcher.go` (service, request-response)

**Analog:** `internal/summarizer/lexrank.go` — same role pattern: an internal package that exports exactly one entry-point function, uses only Go stdlib plus one external dependency, returns `(value, error)`, and has no package-level state.

**Package declaration pattern** (`internal/summarizer/lexrank.go` line 1):
```go
package summarizer
```
New file should open:
```go
package fetcher
```

**Import block pattern** (`internal/summarizer/lexrank.go` lines 1-6):
```go
package summarizer

import (
	"math"
	"sort"
)
```
For `fetcher.go` the import block follows the same stdlib-first, one-external-dep layout:
```go
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
```

**Exported function signature pattern** (`internal/summarizer/lexrank.go` lines 18-19):
```go
func (l *LexRank) Summarize(text string, n int) ([]string, error) {
```
`fetcher.Fetch` is a package-level function (no receiver needed — no state):
```go
// Fetch fetches a URL and returns the main article text content.
// timeout applies to the entire HTTP round-trip.
// maxBytes caps the response body read to prevent memory exhaustion.
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error) {
```

**Error handling pattern** (`internal/summarizer/lexrank.go` lines 18-26, `internal/summarizer/summarizer.go` lines 24-26):
Every internal package function returns a typed `error`; callers in `main.go` do:
```go
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```
`fetcher.go` wraps all errors with `fmt.Errorf("context: %w", err)` — matching the style used in `main.go`'s `resolveInputBytes`:
```go
return nil, fmt.Errorf("reading file %q: %w", filePath, err)
```

**Core function body pattern** — full `Fetch` implementation modeled on research Pattern 2:
```go
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error) {
	// 1. Scheme validation — block file://, ftp://, etc.
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}

	// 2. HTTP GET with Client-level timeout (covers full round-trip)
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// 3. Non-2xx → error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d fetching %q", resp.StatusCode, rawURL)
	}

	// 4. Content-Type guard (use Contains — real headers are "text/html; charset=utf-8")
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return "", fmt.Errorf("unsupported content type %q at %q (expected text/html)", ct, rawURL)
	}

	// 5. Cap response body to prevent memory exhaustion
	limited := io.LimitReader(resp.Body, maxBytes)

	// 6. Extract article text (strips nav/ads/footers via Readability scoring)
	// NOTE: Use FromReader, NOT FromURL — FromURL bypasses our size cap and client.
	// NOTE: Second arg is *url.URL (for relative-link resolution), not a raw string.
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

---

### `internal/fetcher/fetcher_test.go` (test, request-response)

**Analog:** `internal/summarizer/lexrank_test.go`

**Package declaration + import block** (`internal/summarizer/lexrank_test.go` lines 1-8):
```go
package summarizer

import (
	"math"
	"strings"
	"testing"
)
```
New test file opens:
```go
package fetcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)
```

**Test function naming convention** (throughout `lexrank_test.go`):
`TestTypeName_MethodName_Scenario` — e.g., `TestLexRank_Summarize_Basic`, `TestLexRank_Summarize_EmptyInput`.
Apply same pattern: `TestFetch_OK`, `TestFetch_404`, `TestFetch_Redirect`, `TestFetch_InvalidScheme`, `TestFetch_NonHTMLContentType`.

**Table-driven / assertion style** (`internal/summarizer/lexrank_test.go` lines 220-236):
```go
func TestLexRank_Summarize_Basic(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Summarize returned %d sentences, want 3", len(result))
	}
```
Use the same `if err != nil { t.Fatalf(...) }` then positive assertion style.

**httptest.NewServer pattern** for `fetcher_test.go` (from RESEARCH.md Pattern 3):
```go
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

	text, err := Fetch(ts.URL, 5*time.Second, 1<<20)
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

**Error-case test pattern** — verify both non-nil error AND message content (matches `lexrank_test.go` style):
```go
func TestFetch_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()
	_, err := Fetch(ts.URL, 5*time.Second, 1<<20)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got %q", err.Error())
	}
}
```

**Redirect test pattern** using `http.NewServeMux` (from RESEARCH.md Pattern 4):
```go
func TestFetch_Redirect(t *testing.T) {
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
	text, err := Fetch(ts.URL+"/old", 5*time.Second, 1<<20)
	if err != nil {
		t.Fatalf("redirect: unexpected error: %v", err)
	}
	if !strings.Contains(text, "Redirected") {
		t.Errorf("redirect: expected content after redirect, got %q", text)
	}
}
```

---

### `cmd/tldt/main.go` — modify (controller, request-response)

**Analog:** itself — extend the existing file.

**Existing flag declaration pattern** (`cmd/tldt/main.go` lines 18-32):
```go
filePath := flag.String("f", "", "input file path")
algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph|ensemble")
sentences := flag.Int("sentences", 5, "number of output sentences")
```
Add the new flag in the same block:
```go
urlFlag := flag.String("url", "", "URL of a webpage to fetch and summarize")
```

**Existing `flag.Usage` string** (`cmd/tldt/main.go` line 28) must be extended to include `--url`:
```go
fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [-url url] [-algorithm ...] ...")
```

**Existing `resolveInputBytes` call site** (`cmd/tldt/main.go` line 35):
```go
rawBytes, err := resolveInputBytes(flag.Args(), *filePath)
```
Becomes:
```go
rawBytes, err := resolveInputBytes(flag.Args(), *filePath, *urlFlag)
```

**Existing `resolveInputBytes` function signature** (`cmd/tldt/main.go` lines 199-218):
```go
func resolveInputBytes(args []string, filePath string) ([]byte, error) {
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		...
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		...
	}
	if len(args) > 0 {
		return []byte(strings.Join(args, " ")), nil
	}
	return nil, fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
}
```
Becomes — add `urlStr string` parameter and new top branch:
```go
func resolveInputBytes(args []string, filePath string, urlStr string) ([]byte, error) {
	// NEW: --url branch (highest priority — most explicit input)
	if urlStr != "" {
		text, err := fetcher.Fetch(urlStr, 30*time.Second, 5<<20) // 5MB cap
		if err != nil {
			return nil, fmt.Errorf("fetching URL: %w", err)
		}
		return []byte(text), nil
	}
	// existing stdin / -f / positional branches unchanged below
	...
}
```

**Import additions required** in `main.go`:
```go
"time"

"github.com/gleicon/tldt/internal/fetcher"
```
`time` may already be absent from imports — must add. `fetcher` is the new internal package.

---

### `cmd/tldt/main_test.go` — modify (test, request-response)

**Analog:** itself — extend the existing file.

**Existing `run` helper** (`cmd/tldt/main_test.go` lines 55-67) — reuse as-is:
```go
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, ok bool) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return outBuf.String(), errBuf.String(), err == nil
}
```

**CRITICAL: existing `resolveInputBytes` direct-call tests** (`main_test.go` lines 149-185) call the 2-argument form. All 5 call sites must be updated to pass a third argument `""`:
```go
// Before:
got, err := resolveInputBytes([]string{"hello", "world"}, "")
// After:
got, err := resolveInputBytes([]string{"hello", "world"}, "", "")
```
Affected tests: `TestResolveInputBytes_PositionalArgs`, `TestResolveInputBytes_FilePath`, `TestResolveInputBytes_FileNotFound`, `TestResolveInputBytes_NoInput`, `TestResolveInputBytes_Stdin`.

**New import additions** in `main_test.go`:
```go
"fmt"
"net/http"
"net/http/httptest"
```

**New integration test pattern** — matches existing `TestMain_*` subprocess style using `run()`:
```go
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

---

### `go.mod` / `go.sum` — modify (config)

**Analog:** `go.mod` itself.

**Existing `require` block** (`go.mod` lines 5-7):
```go
require github.com/didasy/tldr v0.7.0

require github.com/alixaxel/pagerank v0.0.0-20160306110729-14bfb4c1d88c // indirect
```
After `go get github.com/go-shiori/go-readability@v0.0.0-20251205110129-5db1dc9836f0` the direct require block becomes:
```go
require (
	github.com/didasy/tldr v0.7.0
	github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0
)
```
Do not hand-edit `go.mod` or `go.sum` — run `go get` and `go mod tidy` to let the toolchain manage both files.

---

## Shared Patterns

### Error wrapping with `fmt.Errorf` + `%w`
**Source:** `cmd/tldt/main.go` lines 204-213 (`resolveInputBytes`)
**Apply to:** `fetcher.Fetch` — all error return paths
```go
return nil, fmt.Errorf("reading file %q: %w", filePath, err)
// pattern: fmt.Errorf("short context describing what failed: %w", err)
```

### Error propagation in `main.go` — print to stderr + `os.Exit(1)`
**Source:** `cmd/tldt/main.go` lines 36-39
**Apply to:** The new `resolveInputBytes` URL branch error is propagated identically to existing branches — callers already handle it:
```go
rawBytes, err := resolveInputBytes(flag.Args(), *filePath, *urlFlag)
if err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```
No changes needed in the error-handling block after `resolveInputBytes`.

### Test assertion style — `t.Fatalf` for setup/blocking errors, `t.Errorf` for value checks
**Source:** `cmd/tldt/main_test.go` throughout (e.g., lines 348-354); `internal/summarizer/lexrank_test.go` lines 222-236
**Apply to:** All new tests in `fetcher_test.go` and new `TestMain_URL*` tests
```go
if err != nil {
    t.Fatalf("Fetch: %v", err)   // use Fatalf — further assertions meaningless without value
}
if strings.TrimSpace(text) == "" {
    t.Error("expected non-empty text content")   // use Errorf/Error for value checks
}
```

### `defer ts.Close()` immediately after `httptest.NewServer`
**Source:** RESEARCH.md Pattern 3 (stdlib convention)
**Apply to:** All tests in `fetcher_test.go` and `main_test.go` that create a test server
```go
ts := httptest.NewServer(handler)
defer ts.Close()
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/fetcher/fetcher.go` (network I/O portion) | service | request-response | No existing package in this codebase performs HTTP I/O. The analog is structural only (package shape, error idioms) — the `net/http` + `go-readability` implementation is entirely new. |

---

## Metadata

**Analog search scope:** `cmd/tldt/`, `internal/summarizer/`, `internal/formatter/`, `go.mod`
**Files read:** `cmd/tldt/main.go`, `cmd/tldt/main_test.go`, `internal/summarizer/lexrank.go`, `internal/summarizer/lexrank_test.go`, `internal/summarizer/summarizer.go`, `internal/formatter/formatter.go` (partial), `go.mod`
**Pattern extraction date:** 2026-05-02
