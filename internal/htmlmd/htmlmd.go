// Package htmlmd converts HTML content to clean Markdown for text processing.
// It combines go-readability for content extraction with html-to-markdown
// for generating clean, readable Markdown output.
//
// This is useful when processing HTML from:
//   - curl commands that return HTML
//   - Web scraping tools
//   - Saved HTML files
//   - HTML emails
//
// The conversion pipeline:
//   1. Parse HTML and extract article content (readability algorithm)
//   2. Convert to clean Markdown (removing most HTML tags)
//   3. Normalize whitespace and trim
package htmlmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/go-shiori/go-readability"
)

// Options configures HTML to Markdown conversion behavior.
type Options struct {
	// ExtractContent applies readability algorithm to extract main article
	// content before converting to Markdown. This removes navigation,
	// ads, sidebars, and other boilerplate.
	// Default: true
	ExtractContent bool

	// IncludeTitle adds the article title as an H1 heading at the start.
	// Default: true
	IncludeTitle bool

	// MaxLength limits the output length. 0 means no limit.
	// Default: 0
	MaxLength int
}

// DefaultOptions returns sensible defaults for HTML to Markdown conversion.
func DefaultOptions() Options {
	return Options{
		ExtractContent: true,
		IncludeTitle:   true,
		MaxLength:      0,
	}
}

// Convert transforms HTML content to clean Markdown text.
// It uses readability to extract the main content, then converts to Markdown.
//
// Example:
//
//	html := `<html><body><article><h1>Title</h1><p>Content...</p></article></body></html>`
//	md, err := htmlmd.Convert(strings.NewReader(html), htmlmd.DefaultOptions())
//
// Returns error if HTML parsing fails or content extraction returns empty.
func Convert(r io.Reader, opts Options) (string, error) {
	// Read all input
	htmlBytes, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("htmlmd.Convert: read input: %w", err)
	}

	if len(htmlBytes) == 0 {
		return "", nil
	}

	var textContent string

	if opts.ExtractContent {
		// Use readability to extract main article content
		article, err := readability.FromReader(bytes.NewReader(htmlBytes), nil)
		if err != nil {
			// Fallback: use raw HTML if readability fails
			textContent = string(htmlBytes)
		} else {
			// Build HTML from extracted content
			var buf bytes.Buffer
			if opts.IncludeTitle && article.Title != "" {
				buf.WriteString("<h1>")
				buf.WriteString(escapeHTML(article.Title))
				buf.WriteString("</h1>\n")
			}
			buf.WriteString(article.Content)
			textContent = buf.String()
		}
	} else {
		textContent = string(htmlBytes)
	}

	if strings.TrimSpace(textContent) == "" {
		return "", nil
	}

	// Convert to Markdown
	markdown, err := htmltomarkdown.ConvertString(textContent)
	if err != nil {
		return "", fmt.Errorf("htmlmd.Convert: convert to markdown: %w", err)
	}

	// Clean up the output
	markdown = cleanMarkdown(markdown)

	// Apply length limit if specified
	if opts.MaxLength > 0 && len(markdown) > opts.MaxLength {
		markdown = markdown[:opts.MaxLength]
		// Try to end at a word boundary
		if idx := strings.LastIndex(markdown, " "); idx > opts.MaxLength/2 {
			markdown = markdown[:idx] + "..."
		} else {
			markdown = markdown + "..."
		}
	}

	return markdown, nil
}

// ConvertString is a convenience wrapper for Convert with a string input.
func ConvertString(html string, opts Options) (string, error) {
	return Convert(strings.NewReader(html), opts)
}

// escapeHTML escapes special HTML characters in text.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// cleanMarkdown normalizes and cleans up Markdown output.
func cleanMarkdown(s string) string {
	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Remove excessive blank lines (more than 2 consecutive)
	for strings.Contains(s, "\n\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n\n", "\n\n\n")
	}

	// Trim leading/trailing whitespace
	s = strings.TrimSpace(s)

	return s
}
