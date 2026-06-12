// Package fetcher fetches a URL and extracts the main article text content
// using the readability algorithm to strip boilerplate HTML.
package fetcher

import (
	"bytes"
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
	"golang.org/x/net/html"

	"github.com/gleicon/tldt/internal/surfaces"
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
	// ErrNoTextContent is returned when readability extracts no visible text (e.g. a JS SPA).
	// The returned Result still carries any HTML comments extracted from the raw body.
	ErrNoTextContent = errors.New("no readable text content found")

	// cloudMetadataIPv6 is the EC2 IPv6 metadata endpoint.
	// ip.IsPrivate() already covers fd00::/8 (ULA), but explicit check documents intent.
	cloudMetadataIPv6 = net.ParseIP("fd00:ec2::254")

	// reservedBlocks are ranges the net.IP helpers (IsPrivate/IsLoopback/
	// IsLinkLocalUnicast) do not cover but that must still be blocked to prevent
	// SSRF. Parsed once at init from constant literals.
	reservedBlocks = parseCIDRs(
		"100.64.0.0/10", // CGNAT shared address space (RFC 6598)
		"198.18.0.0/15", // benchmarking range (RFC 2544)
		"64:ff9b::/96",  // NAT64 well-known prefix (RFC 6052) — can embed private IPv4
	)

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
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses resolved for host %q", host)
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
		if ip.IsUnspecified() {
			return fmt.Errorf("host %q resolves to unspecified address %s: %w", host, addr, ErrSSRFBlocked)
		}
		if ip.Equal(cloudMetadataIPv6) {
			return fmt.Errorf("host %q resolves to cloud metadata IP %s: %w", host, addr, ErrSSRFBlocked)
		}
		for _, block := range reservedBlocks {
			if block.Contains(ip) {
				return fmt.Errorf("host %q resolves to reserved IP %s: %w", host, addr, ErrSSRFBlocked)
			}
		}
	}
	return nil
}

// parseCIDRs parses constant CIDR literals into networks. It panics on a
// malformed literal — a programmer error caught at package init, never at
// runtime (mirrors the regexp.MustCompile convention).
func parseCIDRs(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, len(cidrs))
	for i, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(fmt.Sprintf("fetcher: invalid reserved CIDR %q: %v", c, err))
		}
		nets[i] = n
	}
	return nets
}

// Result carries the extracted text alongside the response metadata observed
// while fetching, so callers can inspect the outcome without re-fetching.
type Result struct {
	Text           string                   // extracted main article text
	HiddenSurfaces []surfaces.HiddenSurface // non-visible HTML surfaces that may carry injection payloads
	StatusCode     int                      // HTTP status code of the final response
	ContentType    string                   // response Content-Type header
	FinalURL       string                   // final URL after any redirects
}

// extractHTMLSurfaces parses rawHTML and returns all non-visible text surfaces
// that are present in the raw HTML but stripped by readability (comments,
// placeholders, meta tags, noscript, hidden inputs, alt/aria/title/data attributes,
// textarea pre-fill). Empty values are omitted.
// Returns nil on parse failure — callers treat it as no surfaces found.
func extractHTMLSurfaces(rawHTML []byte) []surfaces.HiddenSurface {
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	add := func(source, text string) {
		if t := strings.TrimSpace(text); t != "" {
			found = append(found, surfaces.HiddenSurface{Source: source, Text: t})
		}
	}
	attr := func(n *html.Node, key string) string {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	attrPrefix := func(n *html.Node, prefix string) []html.Attribute {
		var out []html.Attribute
		for _, a := range n.Attr {
			if strings.HasPrefix(a.Key, prefix) {
				out = append(out, a)
			}
		}
		return out
	}
	// textContent collects all text node children of n into one string.
	textContent := func(n *html.Node) string {
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				b.WriteString(c.Data)
			}
		}
		return b.String()
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.CommentNode:
			add(surfaces.SourceHTMLComment, n.Data)

		case html.ElementNode:
			tag := strings.ToLower(n.Data)
			switch tag {
			case "meta":
				// <meta name/property content="..."> — description, keywords, og:description, etc.
				nameOrProp := attr(n, "name")
				if nameOrProp == "" {
					nameOrProp = attr(n, "property")
				}
				if content := attr(n, "content"); content != "" {
					// Skip purely structural meta tags (charset, viewport, robots, http-equiv)
					skip := map[string]bool{
						"viewport": true, "charset": true, "robots": true,
						"theme-color": true, "msapplication-tilecolor": true,
					}
					if !skip[strings.ToLower(nameOrProp)] {
						add(surfaces.SourceHTMLMeta, nameOrProp+": "+content)
					}
				}

			case "noscript":
				add(surfaces.SourceHTMLNoscript, textContent(n))

			case "textarea":
				add(surfaces.SourceHTMLTextarea, textContent(n))

			case "input":
				if strings.EqualFold(attr(n, "type"), "hidden") {
					if v := attr(n, "value"); v != "" {
						add(surfaces.SourceHTMLHiddenInput, v)
					}
				}
				if ph := attr(n, "placeholder"); ph != "" {
					add(surfaces.SourceHTMLPlaceholder, ph)
				}

			default:
				// placeholder on non-input elements (search, contenteditable, etc.)
				if ph := attr(n, "placeholder"); ph != "" {
					add(surfaces.SourceHTMLPlaceholder, ph)
				}
			}

			// alt attribute on img, area, input[type=image]
			if tag == "img" || tag == "area" {
				if alt := attr(n, "alt"); alt != "" {
					add(surfaces.SourceHTMLAlt, alt)
				}
			}

			// aria-label on any element
			if v := attr(n, "aria-label"); v != "" {
				add(surfaces.SourceHTMLAriaLabel, v)
			}

			// title attribute on any element (tooltip text)
			if v := attr(n, "title"); v != "" {
				add(surfaces.SourceHTMLTitleAttr, v)
			}

			// data-* attributes: only include values longer than 20 chars to
			// reduce noise from short identifiers like data-id="abc".
			for _, da := range attrPrefix(n, "data-") {
				if len(strings.TrimSpace(da.Val)) > 20 {
					add(surfaces.SourceHTMLDataAttr, da.Key+"="+da.Val)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// doHardenedRequest performs an SSRF-, redirect-, and timeout-hardened GET of
// rawURL and returns the live 2xx response together with the parsed request URL
// (for relative-link resolution). SSRF validation runs at dial time via
// safeDialContext for the initial request and every redirect hop, closing the
// DNS-rebinding TOCTOU; CheckRedirect enforces the 5-hop cap. ctx cancels the
// in-flight request and propagates to every dial. The caller owns resp.Body and
// must close it. Only http and https schemes are accepted; a non-2xx status
// returns ErrHTTPError (with the body already closed).
func doHardenedRequest(ctx context.Context, rawURL string, timeout time.Duration) (*http.Response, *url.URL, error) {
	if timeout <= 0 {
		return nil, nil, fmt.Errorf("timeout must be positive, got %v", timeout)
	}
	// Validate scheme — block file://, ftp://, etc.
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, nil, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: safeDialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects (%d) fetching %q: %w", len(via), req.URL, ErrRedirectLimit)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "tldt/2.0 (https://github.com/gleicon/tldt)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching %q: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, nil, fmt.Errorf("HTTP %d fetching %q: %w", resp.StatusCode, rawURL, ErrHTTPError)
	}
	return resp, u, nil
}

// Fetch fetches rawURL and returns the main article text plus response metadata.
// ctx cancels the in-flight request. timeout applies to the entire HTTP
// round-trip (http.Client level). maxBytes caps the response body read to
// prevent memory exhaustion; it must be positive.
//
// Only http and https schemes are accepted. Non-2xx status codes (ErrHTTPError)
// and non-HTML content types (ErrNonHTML) are returned as errors. HTTP redirects
// are followed with SSRF + 5-hop guard.
func Fetch(ctx context.Context, rawURL string, timeout time.Duration, maxBytes int64) (Result, error) {
	if maxBytes <= 0 {
		return Result{}, fmt.Errorf("maxBytes must be positive, got %d", maxBytes)
	}
	resp, u, err := doHardenedRequest(ctx, rawURL, timeout)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Content-Type guard — use Contains because real headers are
	// "text/html; charset=utf-8".
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return Result{}, fmt.Errorf("unsupported content type %q at %q (expected text/html): %w", ct, rawURL, ErrNonHTML)
	}

	// Buffer the body so we can both parse HTML comments and run readability
	// on the same bytes without a second network request.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return Result{}, fmt.Errorf("reading body from %q: %w", rawURL, err)
	}

	// Extract all non-visible HTML surfaces before readability discards them.
	hiddenSurfaces := extractHTMLSurfaces(bodyBytes)

	// Extract article text — strips nav/ads/footers via Readability scoring.
	// Use FromReader, NOT FromURL: FromURL bypasses our size cap and client.
	// Second arg is *url.URL for relative-link resolution, not a raw string.
	article, err := readability.FromReader(bytes.NewReader(bodyBytes), u)
	if err != nil {
		return Result{}, fmt.Errorf("extracting content from %q: %w", rawURL, err)
	}

	text := strings.TrimSpace(article.TextContent)
	partial := Result{
		Text:           text,
		HiddenSurfaces: hiddenSurfaces,
		StatusCode:     resp.StatusCode,
		ContentType:    ct,
		FinalURL:       resp.Request.URL.String(),
	}
	if text == "" {
		// Return the partial result (with comments) so callers can still run
		// security scans on HTML comment nodes even when the page has no
		// summarizable text (e.g. a JS SPA where content is rendered client-side).
		return partial, fmt.Errorf("%w at %q", ErrNoTextContent, rawURL)
	}
	return partial, nil
}

// FetchRaw fetches rawURL with the same SSRF, redirect, and size hardening as
// Fetch but applies no content-type gate and no text extraction: it returns the
// raw response body (capped at maxBytes) plus response metadata. Use it for JSON
// or other non-HTML resources. ctx cancels the in-flight request; maxBytes must
// be positive. The returned Result.Text is empty; the body is the []byte return
// value.
func FetchRaw(ctx context.Context, rawURL string, timeout time.Duration, maxBytes int64) ([]byte, Result, error) {
	if maxBytes <= 0 {
		return nil, Result{}, fmt.Errorf("maxBytes must be positive, got %d", maxBytes)
	}
	resp, _, err := doHardenedRequest(ctx, rawURL, timeout)
	if err != nil {
		return nil, Result{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, Result{}, fmt.Errorf("reading body from %q: %w", rawURL, err)
	}
	return body, Result{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    resp.Request.URL.String(),
	}, nil
}
