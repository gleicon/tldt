package summarizer

import (
	"fmt"
	"strings"
)

// ExplainInfo holds debug diagnostics from a summarization run.
// Printed to stderr when --explain is active.
type ExplainInfo struct {
	Algorithm string

	// Input / output counts
	InputSentences int
	SelectedN      int

	// LexRank only
	VocabSize int
	IDFMin    float64
	IDFMax    float64

	// TextRank only
	DampingFactor     float64
	SimilarityNonZero int
	SimilarityPairs   int
	SimilarityMax     float64
	SimilarityMean    float64

	// Both (0 for Graph — opaque library)
	Iterations int
	Converged  bool

	// Per-sentence scores in document order
	Scores []SentenceScore
}

// SentenceScore holds the centrality score and selection status for one sentence.
type SentenceScore struct {
	Index    int
	Score    float64
	Selected bool
	Rank     int    // 1-based rank by score (1 = highest)
	Preview  string // first 72 chars of sentence
}

// Explainer is an optional interface implemented by algorithms that can
// return per-run diagnostics alongside the summary.
type Explainer interface {
	SummarizeExplain(text string, n int) ([]string, *ExplainInfo, error)
}

// preview truncates s to maxLen characters for display.
func preview(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// PrintExplain writes a human-readable explain report to a string.
// Callers write it to stderr.
func (e *ExplainInfo) Format() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n── explain: %s ──────────────────────────────────────\n", e.Algorithm)
	fmt.Fprintf(&b, "  Input sentences : %d\n", e.InputSentences)
	fmt.Fprintf(&b, "  Selected        : %d\n", e.SelectedN)

	switch e.Algorithm {
	case "lexrank":
		fmt.Fprintf(&b, "  Vocabulary size : %d unique terms\n", e.VocabSize)
		fmt.Fprintf(&b, "  IDF range       : %.4f – %.4f\n", e.IDFMin, e.IDFMax)
	case "textrank":
		fmt.Fprintf(&b, "  Damping factor  : %.2f\n", e.DampingFactor)
		fmt.Fprintf(&b, "  Non-zero pairs  : %d / %d\n", e.SimilarityNonZero, e.SimilarityPairs)
		fmt.Fprintf(&b, "  Max similarity  : %.4f\n", e.SimilarityMax)
		fmt.Fprintf(&b, "  Mean similarity : %.4f\n", e.SimilarityMean)
	}

	if e.Iterations > 0 {
		conv := "yes"
		if !e.Converged {
			conv = fmt.Sprintf("NO (hit max %d)", e.Iterations)
		}
		fmt.Fprintf(&b, "  Power iterations: %d  converged: %s\n", e.Iterations, conv)
	}

	fmt.Fprintln(&b, "\n  Sentence scores (document order):")
	fmt.Fprintln(&b, "  idx  rank  score    sel  preview")
	fmt.Fprintln(&b, "  ────────────────────────────────────��────────────────────")
	for _, s := range e.Scores {
		sel := "   "
		if s.Selected {
			sel = "★  "
		}
		fmt.Fprintf(&b, "  %3d  %4d  %.6f  %s%s\n",
			s.Index, s.Rank, s.Score, sel, preview(s.Preview, 55))
	}
	fmt.Fprintln(&b, "─────────────────────────────────────────────────────────────")
	return b.String()
}
