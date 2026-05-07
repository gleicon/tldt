// Package tldt provides an embeddable Go API for text summarization,
// prompt injection detection, PII scanning, and Unicode sanitization.
//
// This is the only public API surface of the tldt module. All functions
// are stateless with no global mutable state. Options are passed via plain
// structs; zero-value fields receive sensible defaults.
//
// Import: github.com/gleicon/tldt/pkg/tldt
//
// Quick Start
//
// Basic summarization with defaults (LexRank, 5 sentences):
//
//	result, err := tldt.Summarize(longText, tldt.SummarizeOptions{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Summary)
//
// PII-Aware Pipeline
//
// Run the full processing pipeline with PII redaction and injection detection:
//
//	result, err := tldt.Pipeline(untrustedText, tldt.PipelineOptions{
//	    Sanitize:    true,  // strip invisible Unicode
//	    SanitizePII: true,  // redact emails, API keys, JWTs
//	    Detect:      tldt.DetectOptions{OutlierThreshold: 0.85},
//	    Summarize:   tldt.SummarizeOptions{Algorithm: "ensemble", Sentences: 3},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access findings
//	fmt.Printf("Summary: %s\n", result.Summary)
//	fmt.Printf("Redactions: %d\n", result.Redactions)
//	fmt.Printf("Token reduction: %d%%\n", result.Reduction)
//
//	for _, finding := range result.PIIFindings {
//	    fmt.Printf("Found %s on line %d: %s\n",
//	        finding.Pattern, finding.Line, finding.Excerpt)
//	}
//
// URL Fetching
//
// Fetch and summarize a webpage:
//
//	result, err := tldt.Fetch("https://example.com/article", tldt.FetchOptions{
//	    Timeout: 30 * time.Second,
//	})
//	if err != nil {
//	    if errors.Is(err, tldt.ErrSSRFBlocked) {
//	        log.Fatal("SSRF protection triggered")
//	    }
//	    log.Fatal(err)
//	}
//
//	summary, err := tldt.Summarize(result.Text, tldt.SummarizeOptions{
//	    Sentences: 3,
//	})
//
// Individual Operations
//
// Each stage can be used independently:
//
//	// Sanitize Unicode (invisible characters, NFKC normalization)
//	clean, report, _ := tldt.Sanitize(dirtyText)
//	fmt.Printf("Removed %d invisible codepoints\n", report.RemovedCount)
//
//	// Detect prompt injection patterns
//	detectResult, _ := tldt.Detect(text, tldt.DetectOptions{})
//	if detectResult.Report.Suspicious {
//	    for _, w := range detectResult.Warnings {
//	        log.Println(w)
//	    }
//	}
//
//	// Detect PII without redacting
//	findings := tldt.DetectPII(text)
//	for _, f := range findings {
//	    fmt.Printf("Found %s on line %d\n", f.Pattern, f.Line)
//	}
//
//	// Redact PII before processing
//	redacted, findings := tldt.SanitizePII(text)
//	fmt.Printf("Redacted %d findings\n", len(findings))
//
// Algorithm Selection
//
// Four algorithms are available:
//   - "lexrank": TF-IDF cosine similarity + eigenvector centrality (default)
//   - "textrank": Word overlap + PageRank damping
//   - "graph": Bag-of-words baseline (didasy/tldr library)
//   - "ensemble": Average of LexRank and TextRank scores
//
//	result, _ := tldt.Summarize(text, tldt.SummarizeOptions{
//	    Algorithm: "ensemble",
//	    Sentences: 7,
//	})
//
// Error Handling
//
// All errors are wrapped with context. Use errors.Is for sentinel errors:
//
//	if errors.Is(err, tldt.ErrSSRFBlocked) {
//	    // Handle SSRF block
//	}
//	if errors.Is(err, tldt.ErrRedirectLimit) {
//	    // Too many redirects
//	}
//
// Thread Safety
//
// All functions are safe for concurrent use. There is no shared state.
// Each call operates on its own data.
//
package tldt
