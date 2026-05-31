// Package fetcher fetches a URL and extracts the main article text content
// using the readability algorithm to strip boilerplate HTML.
package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
)

var (
	// ErrSSRFBlocked is returned when a URL resolves to a private or reserved IP address.
	ErrSSRFBlocked = errors.New("SSRF blocked: private or reserved IP address")
	// ErrRedirectLimit is returned when the redirect chain exceeds the 5-hop cap.
	ErrRedirectLimit = errors.New("redirect limit exceeded")
	// ErrHTTPError is returned when the server responds with a non-2xx status.
	ErrHTTPError = errors.New("non-2xx HTTP status")
	// ErrNonHTML is returned when the response Content-Type is not text/html.
	ErrNonHTML = errors.New("unsupported content type")

	// cloudMetadataIPv6 is the EC2 IPv6 metadata endpoint.
	// ip.IsPrivate() already covers fd00::/8 (ULA), but explicit check documents intent.
	cloudMetadataIPv6 = net.ParseIP("fd00:ec2::254")

	// lookupHost is a package-level variable for DNS resolution, enabling test injection.
	lookupHost = net.LookupHost

	// blockIP validates resolved IPs and is a package-level variable for test
	// injection. Production uses blockPrivateIP.
	blockIP = blockPrivateIP
)

// safeDialContext resolves the address's host, validates every resolved IP, and
// dials a validated IP literal. Because the IP that passed the SSRF check is the
// exact IP connected to, a DNS-rebinding response (public on the validating
// lookup, private on the connecting lookup) cannot reach an internal target.
// It is wired as http.Transport.DialContext so it runs for the initial request
// and every redirect hop. TLS verification still uses the URL hostname (the
// Transport sets ServerName independently of the dialed address).
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	addrs, err := lookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("resolving host %q: %w", host, err)
	}
	if err := blockIP(host, addrs); err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, ip := range addrs {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// blockPrivateIP returns ErrSSRFBlocked if any addr in addrs resolves to a
// loopback, private, link-local, or cloud metadata IP.
// host is included in the error message for debuggability.
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

// Result carries the extracted text alongside the response metadata observed
// while fetching, so callers can inspect the outcome without re-fetching.
type Result struct {
	Text        string // extracted main article text
	StatusCode  int    // HTTP status code of the final response
	ContentType string // response Content-Type header
	FinalURL    string // final URL after any redirects
}

// Fetch fetches rawURL and returns the main article text plus response metadata.
// timeout applies to the entire HTTP round-trip (http.Client level).
// maxBytes caps the response body read to prevent memory exhaustion.
//
// Only http and https schemes are accepted. Non-2xx status codes (ErrHTTPError)
// and non-HTML content types (ErrNonHTML) are returned as errors. HTTP redirects
// are followed with SSRF + 5-hop guard.
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (Result, error) {
	// 1. Validate scheme — block file://, ftp://, etc.
	u, err := url.Parse(rawURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Result{}, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}

	// 2. HTTP client. SSRF validation happens at dial time via safeDialContext
	// (runs for the initial request and every redirect hop, closing the
	// DNS-rebinding TOCTOU). CheckRedirect only enforces the 5-hop cap.
	transport := &http.Transport{DialContext: safeDialContext}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects (%d) fetching %q: %w", len(via), req.URL, ErrRedirectLimit)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("building request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

	// 3. Execute — net/http.Client follows redirects with SSRF + 5-hop guard.
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fetching %q: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 4. Non-2xx status is always an error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("HTTP %d fetching %q: %w", resp.StatusCode, rawURL, ErrHTTPError)
	}

	// 5. Content-Type guard — use Contains because real headers are
	// "text/html; charset=utf-8".
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return Result{}, fmt.Errorf("unsupported content type %q at %q (expected text/html): %w", ct, rawURL, ErrNonHTML)
	}

	// 6. Cap response body to prevent memory exhaustion (DoS mitigation).
	// io.LimitReader is belt-and-suspenders on top of the client timeout.
	limited := io.LimitReader(resp.Body, maxBytes)

	// 7. Extract article text — strips nav/ads/footers via Readability scoring.
	// Use FromReader, NOT FromURL: FromURL bypasses our size cap and client.
	// Second arg is *url.URL for relative-link resolution, not a raw string.
	article, err := readability.FromReader(limited, u)
	if err != nil {
		return Result{}, fmt.Errorf("extracting content from %q: %w", rawURL, err)
	}

	text := strings.TrimSpace(article.TextContent)
	if text == "" {
		return Result{}, fmt.Errorf("no readable text content found at %q", rawURL)
	}
	return Result{
		Text:        text,
		StatusCode:  resp.StatusCode,
		ContentType: ct,
		FinalURL:    resp.Request.URL.String(),
	}, nil
}
