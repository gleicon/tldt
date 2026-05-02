package fetcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second
const testMaxBytes = 1 << 20 // 1MB

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
}

func TestFetch_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	_, err := Fetch(ts.URL, testTimeout, testMaxBytes)
	if err == nil {
		t.Error("Fetch: expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Fetch: expected '404' in error message, got %q", err.Error())
	}
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

	text, err := Fetch(ts.URL+"/old", testTimeout, testMaxBytes)
	if err != nil {
		t.Fatalf("Fetch redirect: unexpected error: %v", err)
	}
	if !strings.Contains(text, "Redirected content") {
		t.Errorf("Fetch redirect: expected 'Redirected content' in text, got %q", text)
	}
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

	_, err := Fetch(ts.URL, testTimeout, testMaxBytes)
	if err == nil {
		t.Error("Fetch: expected error for application/pdf content-type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("Fetch: expected 'unsupported content type' in error, got %q", err.Error())
	}
}
