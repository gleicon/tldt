// Package tldt provides an embeddable Go API for text summarization,
// prompt injection detection, and Unicode sanitization. It wraps the
// internal packages and is the only public API surface of the tldt module.
//
// All functions are stateless -- no global mutable state. Options are passed
// explicitly via plain structs; zero-value fields receive sensible defaults.
//
// Import path: github.com/gleicon/tldt/pkg/tldt
package tldt

import (
	"fmt"
	"strings"
	"time"

	"github.com/gleicon/tldt/internal/detector"
	"github.com/gleicon/tldt/internal/fetcher"
	"github.com/gleicon/tldt/internal/sanitizer"
	"github.com/gleicon/tldt/internal/summarizer"
)

// --- Option types ---

// SummarizeOptions controls summarization behavior.
type SummarizeOptions struct {
	Algorithm string // "lexrank"|"textrank"|"graph"|"ensemble" (default: "lexrank")
	Sentences int    // number of output sentences (default: 5)
}

// DetectOptions controls detection behavior.
type DetectOptions struct {
	// OutlierThreshold is reserved for future use.
	// The underlying detector.Analyze function runs pattern, encoding, and confusable
	// checks only; statistical outlier detection (which requires a precomputed LexRank
	// similarity matrix) is not performed by Detect or Pipeline.
	// Setting this field to a non-zero value returns an error to prevent silent
	// misconfiguration. Leave at 0 (zero value) to use the built-in defaults.
	OutlierThreshold float64
}

// FetchOptions controls URL fetching behavior.
type FetchOptions struct {
	Timeout  time.Duration // default: 30s
	MaxBytes int64         // default: 5MB
}

// PipelineOptions combines all pipeline stages.
type PipelineOptions struct {
	Summarize SummarizeOptions
	Detect    DetectOptions
	Sanitize  bool // run sanitizer before detection/summarization
}

// --- Result types ---

// Result is the output of Summarize.
type Result struct {
	Summary   string
	TokensIn  int
	TokensOut int
	Reduction int // percentage
}

// DetectResult is the output of Detect.
type DetectResult struct {
	Report   detector.Report
	Warnings []string // human-readable WARNING lines (same format as CLI stderr)
}

// SanitizeReport is the output metadata from Sanitize.
type SanitizeReport struct {
	RemovedCount int
	Invisibles   []sanitizer.InvisibleReport
}

// PipelineResult is the output of Pipeline.
type PipelineResult struct {
	Summary    string
	TokensIn   int
	TokensOut  int
	Reduction  int
	Warnings   []string
	Redactions int
}

// --- Sentinel errors re-exported for caller error checking ---

var (
	ErrSSRFBlocked   = fetcher.ErrSSRFBlocked
	ErrRedirectLimit = fetcher.ErrRedirectLimit
)

// --- Default helpers ---

func applySummarizeDefaults(opts *SummarizeOptions) {
	if opts.Algorithm == "" {
		opts.Algorithm = "lexrank"
	}
	if opts.Sentences == 0 {
		opts.Sentences = 5
	}
}

// --- Exported functions ---

// Summarize runs the extractive summarization pipeline on text.
// Returns the summary, token counts, and compression ratio.
func Summarize(text string, opts SummarizeOptions) (Result, error) {
	applySummarizeDefaults(&opts)
	s, err := summarizer.New(opts.Algorithm)
	if err != nil {
		return Result{}, fmt.Errorf("tldt.Summarize: %w", err)
	}
	sentences, err := s.Summarize(text, opts.Sentences)
	if err != nil {
		return Result{}, fmt.Errorf("tldt.Summarize: %w", err)
	}
	tokIn := len(text) / 4
	summary := strings.Join(sentences, " ")
	tokOut := len(summary) / 4
	reduction := 0
	if tokIn > 0 {
		reduction = 100 - (tokOut*100)/tokIn
	}
	return Result{
		Summary:   summary,
		TokensIn:  tokIn,
		TokensOut: tokOut,
		Reduction: reduction,
	}, nil
}

// Detect runs injection and encoding detection on text without summarizing.
// Returns findings and human-readable warning lines.
//
// If opts.OutlierThreshold is non-zero, Detect returns an error: outlier
// detection requires a precomputed similarity matrix and is not available
// through this function. This error prevents silent misconfiguration.
func Detect(text string, opts DetectOptions) (DetectResult, error) {
	if opts.OutlierThreshold != 0 {
		return DetectResult{}, fmt.Errorf("tldt.Detect: OutlierThreshold is not supported (outlier detection requires a precomputed similarity matrix); set to 0 to use built-in defaults")
	}
	report := detector.Analyze(text)
	var warnings []string
	if report.Suspicious {
		warnings = append(warnings, "injection-detect: WARNING -- input flagged as suspicious")
	}
	return DetectResult{Report: report, Warnings: warnings}, nil
}

// Sanitize strips invisible Unicode characters and applies NFKC normalization.
// Returns the cleaned text and a report of what was removed.
func Sanitize(text string) (string, SanitizeReport, error) {
	inv := sanitizer.ReportInvisibles(text)
	cleaned := sanitizer.SanitizeAll(text)
	return cleaned, SanitizeReport{
		RemovedCount: len(inv),
		Invisibles:   inv,
	}, nil
}

// Fetch retrieves a URL and extracts the main article text using readability.
// SSRF protection blocks private/loopback/link-local IPs. Redirect chain capped at 5 hops.
func Fetch(url string, opts FetchOptions) (string, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 5 * 1024 * 1024
	}
	return fetcher.Fetch(url, opts.Timeout, opts.MaxBytes)
}

// Pipeline runs the full sanitize -> detect -> summarize flow in one call.
// This is the primary embedding use case for AI API middleware.
//
// If opts.Detect.OutlierThreshold is non-zero, Pipeline returns an error.
// See Detect for details.
func Pipeline(text string, opts PipelineOptions) (PipelineResult, error) {
	if opts.Detect.OutlierThreshold != 0 {
		return PipelineResult{}, fmt.Errorf("tldt.Pipeline: OutlierThreshold is not supported (outlier detection requires a precomputed similarity matrix); set to 0 to use built-in defaults")
	}

	var redactions int

	// Step 1: sanitize (if enabled)
	if opts.Sanitize {
		inv := sanitizer.ReportInvisibles(text)
		redactions = len(inv)
		text = sanitizer.SanitizeAll(text)
	}

	// Step 2: detect
	var warnings []string
	report := detector.Analyze(text)
	if report.Suspicious {
		warnings = append(warnings, "injection-detect: WARNING -- input flagged as suspicious")
	}

	// Step 3: summarize
	result, err := Summarize(text, opts.Summarize)
	if err != nil {
		return PipelineResult{}, err
	}

	return PipelineResult{
		Summary:    result.Summary,
		TokensIn:   result.TokensIn,
		TokensOut:  result.TokensOut,
		Reduction:  result.Reduction,
		Warnings:   warnings,
		Redactions: redactions,
	}, nil
}
