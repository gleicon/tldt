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

	// cloudMetadataIPv6 is the EC2 IPv6 metadata endpoint.
	// ip.IsPrivate() already covers fd00::/8 (ULA), but explicit check documents intent.
	cloudMetadataIPv6 = net.ParseIP("fd00:ec2::254")

	// lookupHost is a package-level variable for DNS resolution, enabling test injection.
	lookupHost = net.LookupHost
)

// cgnBlock is the IANA Shared Address Space (RFC 6598, 100.64.0.0/10).
// Reachable inside many cloud-provider VPC fabrics; not covered by IsPrivate().
var cgnBlock = &net.IPNet{
	IP:   net.ParseIP("100.64.0.0"),
	Mask: net.CIDRMask(10, 32),
}

// blockPrivateIP returns ErrSSRFBlocked if any addr in addrs resolves to a
// loopback, private, link-local, unspecified, CGN, or cloud metadata IP.
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
		if ip.IsUnspecified() {
			return fmt.Errorf("host %q resolves to unspecified IP %s: %w", host, addr, ErrSSRFBlocked)
		}
		if cgnBlock.Contains(ip) {
			return fmt.Errorf("host %q resolves to shared-address-space IP %s: %w", host, addr, ErrSSRFBlocked)
		}
		if ip.Equal(cloudMetadataIPv6) {
			return fmt.Errorf("host %q resolves to cloud metadata IP %s: %w", host, addr, ErrSSRFBlocked)
		}
	}
	return nil
}

// Fetch fetches rawURL and returns the main article text content.
// timeout applies to the entire HTTP round-trip (http.Client level).
// maxBytes caps the response body read to prevent memory exhaustion.
//
// Only http and https schemes are accepted. Non-2xx status codes and
// non-HTML content types are returned as errors. HTTP redirects are
// followed with SSRF + 5-hop guard.
func Fetch(rawURL string, timeout time.Duration, maxBytes int64) (string, error) {
	// 1. Validate scheme — block file://, ftp://, etc.
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}

	// 1b. Resolve initial hostname and block private IPs (SSRF pre-check per D-01).
	addrs, err := lookupHost(u.Hostname())
	if err != nil {
		return "", fmt.Errorf("resolving host %q: %w", u.Hostname(), err)
	}
	if err := blockPrivateIP(u.Hostname(), addrs); err != nil {
		return "", err
	}

	// 2. HTTP client with combined redirect guard (5-hop cap + SSRF check per hop, per D-02).
	combinedCheckRedirect := func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects (%d) fetching %q: %w", len(via), req.URL, ErrRedirectLimit)
		}
		hopAddrs, err := lookupHost(req.URL.Hostname())
		if err != nil {
			return fmt.Errorf("resolving redirect host %q: %w", req.URL.Hostname(), err)
		}
		return blockPrivateIP(req.URL.Hostname(), hopAddrs)
	}
	client := &http.Client{
		Timeout:       timeout,
		CheckRedirect: combinedCheckRedirect,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

	// 3. Execute — net/http.Client follows redirects with SSRF + 5-hop guard.
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
