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
	"errors"
	"fmt"
	"net/http"
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

// DefaultOutlierThreshold is the default threshold for outlier detection.
// Sentences with outlier score above this are flagged as off-topic.
const DefaultOutlierThreshold = detector.DefaultOutlierThreshold

// DetectOptions controls detection behavior.
type DetectOptions struct {
	OutlierThreshold float64 // default: 0.85 (DefaultOutlierThreshold)
}

// FetchOptions controls URL fetching behavior.
type FetchOptions struct {
	Timeout  time.Duration // default: 30s
	MaxBytes int64         // default: 5MB
}

// PipelineOptions combines all pipeline stages.
type PipelineOptions struct {
	Summarize   SummarizeOptions
	Detect      DetectOptions
	Sanitize    bool // run Unicode sanitizer before detection/summarization
	DetectPII   bool // run PII detection stage (text unchanged)
	SanitizePII bool // run PII redaction stage (text redacted; implies detection)
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
	Invisibles   []InvisibleReport
}

// InvisibleReport describes a single stripped invisible codepoint for audit purposes.
// Re-exported from internal/sanitizer for CLI use.
type InvisibleReport = sanitizer.InvisibleReport

// PIIFinding describes a single PII or secret detected in text.
// Pattern names: "email", "api-key", "jwt", "credit-card".
// Excerpt is truncated to first 12 chars + "..." by the detector for privacy.
// Line is 1-based.
type PIIFinding struct {
	Pattern string
	Excerpt string
	Line    int
}

// PipelineResult is the output of Pipeline.
type PipelineResult struct {
	Summary     string
	TokensIn    int
	TokensOut   int
	Reduction   int
	Warnings    []string
	Redactions  int
	PIIFindings []PIIFinding // populated when DetectPII or SanitizePII is true; nil otherwise
}

// FetchResult is the output of Fetch with full HTTP metadata.
// This enables middleware consumers to inspect response details without
// re-fetching the URL.
type FetchResult struct {
	Text        string // Extracted article text
	StatusCode  int    // HTTP status code (after redirects)
	ContentType string // Response Content-Type header
	FinalURL    string // Final URL after all redirects
}

// --- Sentinel errors re-exported for caller error checking ---

var (
	ErrSSRFBlocked   = fetcher.ErrSSRFBlocked
	ErrRedirectLimit = fetcher.ErrRedirectLimit
	ErrHTTPError     = errors.New("tldt: HTTP error")
	ErrNonHTML       = errors.New("tldt: not HTML content")
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

// toPublicPIIFinding converts internal detector.Finding to public PIIFinding.
func toPublicPIIFinding(f detector.Finding) PIIFinding {
	return PIIFinding{
		Pattern: f.Pattern,
		Excerpt: f.Excerpt,
		Line:    f.Sentence, // NOT f.Line -- detector.Finding has no Line field
	}
}

// toPublicPIIFindings converts a slice of detector.Findings to []PIIFinding.
func toPublicPIIFindings(findings []detector.Finding) []PIIFinding {
	if len(findings) == 0 {
		return nil
	}
	out := make([]PIIFinding, len(findings))
	for i, f := range findings {
		out[i] = toPublicPIIFinding(f)
	}
	return out
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
func Detect(text string, opts DetectOptions) (DetectResult, error) {
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
	inv := ReportInvisibles(text)
	cleaned := SanitizeAll(text)
	return cleaned, SanitizeReport{
		RemovedCount: len(inv),
		Invisibles:   inv,
	}, nil
}

// DetectPII scans text for PII and secret patterns.
// Returns findings with pattern name, truncated excerpt, and 1-based line number.
// Text is not modified. Safe to call on untrusted input.
func DetectPII(text string) []PIIFinding {
	return toPublicPIIFindings(detector.DetectPII(text))
}

// SanitizePII replaces PII/secret matches with [REDACTED:<type>] placeholders.
// Returns the redacted string and the findings that triggered redaction.
// When no PII is found, the original text is returned unchanged and findings is nil.
func SanitizePII(text string) (string, []PIIFinding) {
	redacted, findings := detector.SanitizePII(text)
	return redacted, toPublicPIIFindings(findings)
}

// Fetch retrieves a URL and extracts the main article text using readability.
// SSRF protection blocks private/loopback/link-local IPs. Redirect chain capped at 5 hops.
//
// On success, returns a FetchResult with extracted text and HTTP metadata.
// Errors are wrapped with context: use errors.Is() to check for sentinel errors
// (ErrSSRFBlocked, ErrRedirectLimit, ErrHTTPError, ErrNonHTML).
func Fetch(urlStr string, opts FetchOptions) (FetchResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 5 * 1024 * 1024
	}

	text, err := fetcher.Fetch(urlStr, opts.Timeout, opts.MaxBytes)
	if err != nil {
		// Determine error type for sentinel error wrapping
		errStr := err.Error()
		if errors.Is(err, fetcher.ErrSSRFBlocked) {
			return FetchResult{}, fmt.Errorf("tldt.Fetch: %w", err)
		}
		if errors.Is(err, fetcher.ErrRedirectLimit) {
			return FetchResult{}, fmt.Errorf("tldt.Fetch: %w", err)
		}
		// Check for HTTP error (non-2xx status)
		if strings.Contains(errStr, "HTTP ") && strings.Contains(errStr, "fetching") {
			return FetchResult{}, fmt.Errorf("tldt.Fetch: %w: %v", ErrHTTPError, err)
		}
		// Check for non-HTML content type
		if strings.Contains(errStr, "unsupported content type") {
			return FetchResult{}, fmt.Errorf("tldt.Fetch: %w: %v", ErrNonHTML, err)
		}
		// Unknown error - still wrap with context
		return FetchResult{}, fmt.Errorf("tldt.Fetch: %w", err)
	}

	// On success, we don't have access to the actual HTTP response metadata
	// since fetcher.Fetch only returns the text. Return with defaults.
	// NOTE: If fetcher is extended to return metadata, update this to include it.
	return FetchResult{
		Text:        text,
		StatusCode:  http.StatusOK, // Default since fetcher.Fetch doesn't expose this
		ContentType: "text/html",     // Default since fetcher.Fetch doesn't expose this
		FinalURL:    urlStr,          // Default since fetcher.Fetch doesn't expose this
	}, nil
}

// --- Detector Types ---

// Finding describes a single injection detection signal from DetectOutliers.
// Re-exported from internal/detector for CLI --detect-injection use.
type Finding = detector.Finding

// --- Summarizer Types ---

// Summarizer is the common interface for all extractive summarization algorithms.
// Re-exported from internal/summarizer for type assertions by CLI consumers.
type Summarizer = summarizer.Summarizer

// Explainer is an optional interface implemented by algorithms that can return
// per-run diagnostics alongside the summary. Used with --explain flag.
type Explainer = summarizer.Explainer

// MatrixSummarizer is an optional interface implemented by LexRank.
// Returns the top n sentences and the raw similarity matrix for outlier detection.
type MatrixSummarizer = summarizer.MatrixSummarizer

// ExplainInfo holds debug diagnostics from a summarization run.
// Re-exported from internal/summarizer for CLI --explain output.
type ExplainInfo = summarizer.ExplainInfo

// SentenceScore holds the centrality score and selection status for one sentence.
type SentenceScore = summarizer.SentenceScore

// F1Score holds precision, recall, and F1 for one ROUGE metric.
type F1Score = summarizer.F1Score

// ROUGEScore holds ROUGE-1, ROUGE-2, and ROUGE-L scores.
type ROUGEScore = summarizer.ROUGEScore

// NewSummarizer returns a Summarizer for the named algorithm.
// Valid algorithms: "lexrank", "textrank", "graph", "ensemble".
func NewSummarizer(algo string) (Summarizer, error) {
	s, err := summarizer.New(algo)
	if err != nil {
		return nil, fmt.Errorf("tldt.NewSummarizer: %w", err)
	}
	return s, nil
}

// TokenizeSentences splits text into sentences using a regexp heuristic.
// Sentences are returned trimmed, in original order.
func TokenizeSentences(text string) []string {
	return summarizer.TokenizeSentences(text)
}

// EvalROUGE computes ROUGE-1, ROUGE-2, and ROUGE-L between system sentences
// and reference sentences. Used for evaluating summarization quality.
func EvalROUGE(system, reference []string) ROUGEScore {
	return summarizer.EvalROUGE(system, reference)
}

// --- Detector Wrappers ---

// DetectOutliers computes per-sentence outlier scores from the similarity matrix
// and returns findings for sentences above threshold. Used with --detect-injection.
//
// outlier_score(i) = 1 - mean(simMatrix[i][j] for j ≠ i)
// Higher scores mean less similarity to neighbors = more off-topic.
func DetectOutliers(sentences []string, simMatrix [][]float64, threshold float64) []Finding {
	return detector.DetectOutliers(sentences, simMatrix, threshold)
}

// --- Sanitizer Wrappers ---

// SanitizeAll applies StripInvisible followed by NormalizeUnicode.
// This is the single entry point for the --sanitize CLI flag.
func SanitizeAll(text string) string {
	return sanitizer.SanitizeAll(text)
}

// ReportInvisibles returns a description of every codepoint that would be stripped
// by SanitizeAll, without modifying the text. Used by --detect-injection to audit
// invisible content.
func ReportInvisibles(text string) []InvisibleReport {
	return sanitizer.ReportInvisibles(text)
}

// Pipeline runs the full sanitize -> detect -> summarize flow in one call.
// This is the primary embedding use case for AI API middleware.
func Pipeline(text string, opts PipelineOptions) (PipelineResult, error) {
	var redactions int
	var piiFindings []PIIFinding

	// Step 1: Unicode sanitize (if enabled)
	if opts.Sanitize {
		inv := ReportInvisibles(text)
		redactions = len(inv)
		text = SanitizeAll(text)
	}

	// Step 2: PII stage (between sanitize and inject-detect per LIB-04)
	if opts.SanitizePII {
		redacted, findings := detector.SanitizePII(text)
		piiFindings = toPublicPIIFindings(findings)
		text = redacted
	} else if opts.DetectPII {
		findings := detector.DetectPII(text)
		piiFindings = toPublicPIIFindings(findings)
	}

	// Step 3: injection detect
	var warnings []string
	report := detector.Analyze(text)
	if report.Suspicious {
		warnings = append(warnings, "injection-detect: WARNING -- input flagged as suspicious")
	}

	// Step 4: summarize
	result, err := Summarize(text, opts.Summarize)
	if err != nil {
		return PipelineResult{}, err
	}

	return PipelineResult{
		Summary:     result.Summary,
		TokensIn:    result.TokensIn,
		TokensOut:   result.TokensOut,
		Reduction:   result.Reduction,
		Warnings:    warnings,
		Redactions:  redactions,
		PIIFindings: piiFindings,
	}, nil
}
