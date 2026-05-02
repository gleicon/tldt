// Package fetcher fetches a URL and extracts the main article text content
// using the readability algorithm to strip boilerplate HTML.
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

// Fetch fetches rawURL and returns the main article text content.
// timeout applies to the entire HTTP round-trip (http.Client level).
// maxBytes caps the response body read to prevent memory exhaustion.
//
// Only http and https schemes are accepted. Non-2xx status codes and
// non-HTML content types are returned as errors. HTTP redirects are
// followed automatically (up to 10 hops via net/http default).
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error) {
	// 1. Validate scheme — block file://, ftp://, etc.
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}

	// 2. HTTP GET with Client-level timeout (covers full round-trip).
	// Use http.Client.Timeout rather than context.WithTimeout to avoid
	// double-timeout confusion (Pitfall 3 in research).
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

	// 3. Execute — net/http.Client follows redirects automatically (max 10 hops).
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// 4. Non-2xx status is always an error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d fetching %q", resp.StatusCode, rawURL)
	}

	// 5. Content-Type guard — use Contains because real headers are
	// "text/html; charset=utf-8" (Pitfall 2 in research).
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return "", fmt.Errorf("unsupported content type %q at %q (expected text/html)", ct, rawURL)
	}

	// 6. Cap response body to prevent memory exhaustion (DoS mitigation).
	// io.LimitReader is belt-and-suspenders on top of the client timeout.
	limited := io.LimitReader(resp.Body, maxBytes)

	// 7. Extract article text — strips nav/ads/footers via Readability scoring.
	// Use FromReader, NOT FromURL: FromURL bypasses our size cap and client
	// (Pitfall 4 in research). Second arg is *url.URL for relative-link
	// resolution, not a raw string (Pitfall 5 in research).
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
