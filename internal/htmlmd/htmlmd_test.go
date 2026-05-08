package htmlmd

import (
	"strings"
	"testing"
)

func TestConvertBasicHTML(t *testing.T) {
	html := `<h1>Title</h1><p>This is a paragraph.</p>`

	md, err := ConvertString(html, DefaultOptions())
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}

	if !strings.Contains(md, "Title") {
		t.Errorf("Expected markdown to contain 'Title', got: %s", md)
	}
	if !strings.Contains(md, "paragraph") {
		t.Errorf("Expected markdown to contain 'paragraph', got: %s", md)
	}

	t.Logf("Output: %s", md)
}

func TestConvertWithReadability(t *testing.T) {
	// HTML with navigation noise that should be stripped
	html := `
<html>
<head><title>Article Title</title></head>
<body>
<nav><a href="/">Home</a> | <a href="/about">About</a></nav>
<article>
<h1>The Real Article</h1>
<p>This is the main content of the article. It should be extracted.</p>
<p>Second paragraph with more content.</p>
</article>
<footer>Copyright 2024</footer>
</body>
</html>`

	opts := DefaultOptions()
	md, err := ConvertString(html, opts)
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}

	// Should contain article content
	if !strings.Contains(md, "main content") {
		t.Errorf("Expected markdown to contain 'main content', got: %s", md)
	}

	t.Logf("Output:\n%s", md)
}

func TestConvertWithoutReadability(t *testing.T) {
	html := `<p>This is <strong>important</strong> text.</p>`

	opts := Options{
		ExtractContent: false,
		IncludeTitle:   false,
	}

	md, err := ConvertString(html, opts)
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}

	// Should contain the emphasized text
	if !strings.Contains(md, "important") {
		t.Errorf("Expected markdown to contain 'important', got: %s", md)
	}

	t.Logf("Output: %s", md)
}

func TestConvertEmpty(t *testing.T) {
	md, err := ConvertString("", DefaultOptions())
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}
	if md != "" {
		t.Errorf("Expected empty string, got: %s", md)
	}
}

func TestConvertWithMaxLength(t *testing.T) {
	html := `<p>This is a very long paragraph that should be truncated when the MaxLength option is set to a small value.</p>`

	opts := Options{
		ExtractContent: false,
		IncludeTitle:   false,
		MaxLength:      50,
	}

	md, err := ConvertString(html, opts)
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}

	if len(md) > 60 { // Allow some buffer for the "..."
		t.Errorf("Expected output to be truncated, got length %d: %s", len(md), md)
	}

	if !strings.HasSuffix(md, "...") {
		t.Errorf("Expected output to end with '...', got: %s", md)
	}

	t.Logf("Output: %s", md)
}

func TestConvertComplexHTML(t *testing.T) {
	html := `
<h1>Main Title</h1>
<h2>Section 1</h2>
<p>First paragraph with <a href="http://example.com">a link</a>.</p>
<ul>
<li>Item one</li>
<li>Item two</li>
</ul>
<h2>Section 2</h2>
<p>Another paragraph with <code>inline code</code>.</p>
<pre><code>func main() {
    fmt.Println("Hello")
}</code></pre>
`

	opts := DefaultOptions()
	md, err := ConvertString(html, opts)
	if err != nil {
		t.Fatalf("ConvertString failed: %v", err)
	}

	// Check that various elements are converted
	if !strings.Contains(md, "Main Title") {
		t.Errorf("Expected H1 to be preserved")
	}
	if !strings.Contains(md, "Section 1") {
		t.Errorf("Expected H2 to be preserved")
	}
	if !strings.Contains(md, "Item one") {
		t.Errorf("Expected list items to be preserved")
	}

	t.Logf("Output:\n%s", md)
}
