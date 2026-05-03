package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second
const testMaxBytes = 1 << 20 // 1MB

// withLookup temporarily overrides the package-level lookupHost for test isolation.
// Use for SSRF-specific tests where the DNS response controls the test outcome.
// It restores the original after the test function returns.
func withLookup(fn func(string) ([]string, error), test func()) {
	orig := lookupHost
	lookupHost = fn
	defer func() { lookupHost = orig }()
	test()
}

// withServer overrides both lookupHost and dialTCP so that httptest-based tests
// can exercise non-SSRF fetcher behavior (404, redirect following, content-type,
// etc.) while the SSRF filter still runs.
//
// Background: httptest.NewServer binds to 127.0.0.1. With the DialContext transport
// (WR-02 fix), the SSRF filter runs at TCP dial time. If the real loopback IP reaches
// blockPrivateIP it is rejected — correct SSRF behavior but wrong for tests that need
// to exercise the HTTP layer.
//
// Solution: lookupHost returns a public IP so the SSRF filter passes, and dialTCP
// redirects all TCP connections to the real test server address so the request
// actually reaches the httptest server.
func withServer(ts *httptest.Server, test func()) {
	origLookup := lookupHost
	origDial := dialTCP

	realAddr := ts.Listener.Addr().String()
	lookupHost = func(host string) ([]string, error) {
		return []string{"93.184.216.34"}, nil // public IP — passes blockPrivateIP
	}
	dialTCP = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Ignore resolved addr; always connect to the real test server.
		return (&net.Dialer{}).DialContext(ctx, network, realAddr)
	}

	defer func() {
		lookupHost = origLookup
		dialTCP = origDial
	}()
	test()
}

// publicLookup always returns a public IP, bypassing the SSRF filter.
// Use with withLookup for SSRF tests that need to inject a specific IP separately.
func publicLookup(host string) ([]string, error) {
	return []string{"93.184.216.34"}, nil
}

// privateLookup returns a lookup function that always returns the given private IP.
// Use this for SSRF integration tests.
func privateLookup(ip string) func(string) ([]string, error) {
	return func(host string) ([]string, error) {
		return []string{ip}, nil
	}
}

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

	withServer(ts, func() {
		text, err := Fetch(ts.URL, testTimeout, testMaxBytes)
		if err != nil {
			t.Fatalf("Fetch: unexpected error: %v", err)
		}
		if strings.TrimSpace(text) == "" {
			t.Error("Fetch: expected non-empty text content, got empty string")
		}
		if strings.Contains(text, "Navigation junk") {
			t.Errorf("Fetch: nav junk leaked into text content: %q", text)
		}
	})
}

func TestFetch_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	withServer(ts, func() {
		_, err := Fetch(ts.URL, testTimeout, testMaxBytes)
		if err == nil {
			t.Error("Fetch: expected error for 404 response, got nil")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("Fetch: expected '404' in error message, got %q", err.Error())
		}
	})
}

func TestFetch_Redirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><article><p>Redirected content successfully arrived here.</p></article></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	withServer(ts, func() {
		text, err := Fetch(ts.URL+"/old", testTimeout, testMaxBytes)
		if err != nil {
			t.Fatalf("Fetch redirect: unexpected error: %v", err)
		}
		if !strings.Contains(text, "Redirected content") {
			t.Errorf("Fetch redirect: expected 'Redirected content' in text, got %q", text)
		}
	})
}

func TestFetch_InvalidScheme(t *testing.T) {
	_, err := Fetch("file:///etc/passwd", testTimeout, testMaxBytes)
	if err == nil {
		t.Error("Fetch: expected error for file:// scheme, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Errorf("Fetch: expected 'unsupported URL scheme' in error, got %q", err.Error())
	}
}

func TestFetch_NonHTMLContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, "%PDF-1.4 fake pdf content")
	}))
	defer ts.Close()

	withServer(ts, func() {
		_, err := Fetch(ts.URL, testTimeout, testMaxBytes)
		if err == nil {
			t.Error("Fetch: expected error for application/pdf content-type, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported content type") {
			t.Errorf("Fetch: expected 'unsupported content type' in error, got %q", err.Error())
		}
	})
}

// TestBlockPrivateIP is a unit test for the blockPrivateIP helper directly.
// No DNS lookup needed — passes raw IP strings.
func TestBlockPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		addrs   []string
		wantErr bool
	}{
		{"loopback", "localhost", []string{"127.0.0.1"}, true},
		{"ipv6-loopback", "localhost", []string{"::1"}, true},
		{"private-10", "internal", []string{"10.0.0.1"}, true},
		{"private-172", "internal", []string{"172.16.0.1"}, true},
		{"private-192", "internal", []string{"192.168.1.1"}, true},
		{"link-local", "metadata", []string{"169.254.169.254"}, true},
		{"cloud-meta-v6", "metadata", []string{"fd00:ec2::254"}, true},
		{"unspecified-v4", "zero", []string{"0.0.0.0"}, true},
		{"cgn", "cgn", []string{"100.64.1.1"}, true},
		{"public-ip", "example.com", []string{"93.184.216.34"}, false},
		{"nil-parse", "bad", []string{"not-an-ip"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := blockPrivateIP(tt.host, tt.addrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("blockPrivateIP(%q, %v) error = %v, wantErr %v", tt.host, tt.addrs, err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked, got: %v", err)
			}
		})
	}
}

func TestFetch_SSRFBlockPrivateIP(t *testing.T) {
	withLookup(privateLookup("192.168.1.1"), func() {
		_, err := Fetch("http://example.com/admin", testTimeout, testMaxBytes)
		if err == nil {
			t.Fatal("expected SSRF block error for private IP, got nil")
		}
		if !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("expected ErrSSRFBlocked, got: %v", err)
		}
	})
}

func TestFetch_SSRFBlockLoopback(t *testing.T) {
	withLookup(privateLookup("127.0.0.1"), func() {
		_, err := Fetch("http://example.com/secret", testTimeout, testMaxBytes)
		if err == nil {
			t.Fatal("expected SSRF block on loopback, got nil")
		}
		if !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("expected ErrSSRFBlocked, got: %v", err)
		}
	})
}

func TestFetch_SSRFBlockCloudMeta(t *testing.T) {
	withLookup(privateLookup("169.254.169.254"), func() {
		_, err := Fetch("http://example.com/latest/meta-data/", testTimeout, testMaxBytes)
		if err == nil {
			t.Fatal("expected SSRF block on cloud metadata IP, got nil")
		}
		if !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("expected ErrSSRFBlocked, got: %v", err)
		}
	})
}

// TestFetch_SSRFBlockViaRedirect tests that SSRF is detected on redirect hops.
// The initial lookup returns a public IP (passes pre-check), but the redirect
// lookup returns a private IP (triggering ErrSSRFBlocked in CheckRedirect).
func TestFetch_SSRFBlockViaRedirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusMovedPermanently)
	}))
	defer ts.Close()

	callCount := 0
	withServer(ts, func() {
		withLookup(func(host string) ([]string, error) {
			callCount++
			if callCount == 1 {
				return []string{"93.184.216.34"}, nil // initial: public — passes SSRF check
			}
			return []string{"10.0.0.1"}, nil // redirect: private — blocked
		}, func() {
			_, err := Fetch(ts.URL+"/start", testTimeout, testMaxBytes)
			if err == nil {
				t.Fatal("expected SSRF block on redirect to private IP, got nil")
			}
			if !errors.Is(err, ErrSSRFBlocked) && !errors.Is(err, ErrRedirectLimit) {
				t.Errorf("expected ErrSSRFBlocked or ErrRedirectLimit, got: %v", err)
			}
		})
	})
}

// TestFetch_RedirectLimitExceeded tests that redirect chains > 5 hops are rejected.
// Uses withServer so the SSRF filter passes and the redirect cap is exercised.
func TestFetch_RedirectLimitExceeded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
	}))
	defer ts.Close()

	withServer(ts, func() {
		_, err := Fetch(ts.URL, testTimeout, testMaxBytes)
		if err == nil {
			t.Fatal("expected redirect limit error, got nil")
		}
		if !errors.Is(err, ErrRedirectLimit) {
			t.Errorf("expected ErrRedirectLimit, got: %v", err)
		}
	})
}
