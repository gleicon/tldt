package summarizer

import (
	"strings"
	"testing"
)

// idfVaryText exercises both IDFMin and IDFMax update branches in SummarizeExplain.
//
// "aardvark" is first alphabetically, appears in 2/4 sentences → IDF=log(4/2)=log(2)≈0.693
// "the" appears in 3/4 sentences       → IDF=log(4/3)≈0.288 < 0.693  → triggers v < IDFMin
// single-occurrence words               → IDF=log(4)≈1.386  > 0.693  → triggers v > IDFMax
const idfVaryText = `Aardvark jumps over the fence quickly.
Aardvark and the fox run fast together.
The cat sits quietly on a soft mat.
The lazy dog rests near a warm fire.`

// ── preview ───────────────────────────────────────────────────────────────────

func TestPreview_Short(t *testing.T) {
	s := "hello world"
	got := preview(s, 72)
	if got != s {
		t.Errorf("preview short: got %q, want %q", got, s)
	}
}

func TestPreview_Truncated(t *testing.T) {
	s := strings.Repeat("x", 100)
	got := preview(s, 72)
	if len([]rune(got)) > 73 { // 72 chars + ellipsis rune
		t.Errorf("preview truncated: length %d > 73", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("preview truncated: want trailing '…', got %q", got)
	}
}

func TestPreview_ExactLen(t *testing.T) {
	s := strings.Repeat("a", 72)
	got := preview(s, 72)
	if got != s {
		t.Errorf("preview exact len: expected no truncation")
	}
}

func TestPreview_LeadingWhitespace(t *testing.T) {
	s := "   trimmed"
	got := preview(s, 72)
	if strings.HasPrefix(got, " ") {
		t.Errorf("preview: leading whitespace not stripped: %q", got)
	}
}

// ── ExplainInfo.Format ────────────────────────────────────────────────────────

func TestFormat_LexRank(t *testing.T) {
	l := &LexRank{}
	_, info, err := l.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := info.Format()
	if !strings.Contains(out, "explain: algorithm=lexrank") {
		t.Errorf("Format missing algorithm header: %q", out)
	}
	if !strings.Contains(out, "input_sentences:") {
		t.Errorf("Format missing input_sentences: %q", out)
	}
	if !strings.Contains(out, "vocab_size:") {
		t.Errorf("Format missing vocab_size (lexrank-only): %q", out)
	}
	if !strings.Contains(out, "iterations:") {
		t.Errorf("Format missing iterations: %q", out)
	}
	if !strings.Contains(out, "sentence scores") {
		t.Errorf("Format missing sentence scores table: %q", out)
	}
}

func TestFormat_TextRank(t *testing.T) {
	tr := &TextRank{}
	_, info, err := tr.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := info.Format()
	if !strings.Contains(out, "explain: algorithm=textrank") {
		t.Errorf("Format missing algorithm header: %q", out)
	}
	if !strings.Contains(out, "damping:") {
		t.Errorf("Format missing damping (textrank-only): %q", out)
	}
	if !strings.Contains(out, "nonzero_pairs:") {
		t.Errorf("Format missing nonzero_pairs: %q", out)
	}
}

func TestFormat_NoIterations(t *testing.T) {
	info := &ExplainInfo{Algorithm: "lexrank", InputSentences: 0, Iterations: 0}
	out := info.Format()
	if strings.Contains(out, "iterations:") {
		t.Errorf("Format: should not print iterations when Iterations=0, got %q", out)
	}
}

func TestFormat_NonConverged(t *testing.T) {
	// Converged=false triggers the "no (hit max N)" branch in Format.
	info := &ExplainInfo{
		Algorithm:      "lexrank",
		InputSentences: 2,
		SelectedN:      1,
		Iterations:     1000,
		Converged:      false,
		Scores: []SentenceScore{
			{Index: 0, Score: 0.5, Selected: true, Rank: 1, Preview: "test sentence one"},
			{Index: 1, Score: 0.5, Selected: false, Rank: 2, Preview: "test sentence two"},
		},
	}
	out := info.Format()
	if !strings.Contains(out, "no (hit max") {
		t.Errorf("Format non-converged: want 'no (hit max' in output, got %q", out)
	}
}

func TestFormat_SelectedMarker(t *testing.T) {
	l := &LexRank{}
	_, info, err := l.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := info.Format()
	// Selected sentences should be marked with "*"
	if !strings.Contains(out, "*") {
		t.Errorf("Format: selected sentences should be marked with '*': %q", out)
	}
}

// ── IDF min/max tracking in SummarizeExplain ──────────────────────────────────

func TestLexRank_SummarizeExplain_IDFMinMax(t *testing.T) {
	// idfVaryText has "animals" in every sentence → IDF=0, other words IDF=log(3)
	// Ensures v < IDFMin (the 0) and v > IDFMax (the log(3)) branches both execute.
	l := &LexRank{}
	_, info, err := l.SummarizeExplain(idfVaryText, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IDFMin >= info.IDFMax {
		t.Errorf("IDFMin (%f) should be < IDFMax (%f) for text with varied word freq",
			info.IDFMin, info.IDFMax)
	}
}

func TestLexRank_SummarizeExplain_CapN(t *testing.T) {
	// n > sentence count → n-cap branch in SummarizeExplain
	l := &LexRank{}
	result, info, err := l.SummarizeExplain(threeSentenceText, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("CapN: got %d sentences from 3-sentence input", len(result))
	}
	if info.SelectedN > 3 {
		t.Errorf("ExplainInfo.SelectedN = %d, want <= 3", info.SelectedN)
	}
}

// ── TextRank.SummarizeExplain missing branches ────────────────────────────────

func TestTextRank_SummarizeExplain_Empty(t *testing.T) {
	tr := &TextRank{}
	result, info, err := tr.SummarizeExplain("", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("want nil result for empty input, got %v", result)
	}
	if info == nil {
		t.Fatal("ExplainInfo nil for empty input")
	}
	if info.InputSentences != 0 {
		t.Errorf("InputSentences = %d, want 0", info.InputSentences)
	}
}

func TestTextRank_SummarizeExplain_CapN(t *testing.T) {
	tr := &TextRank{}
	result, info, err := tr.SummarizeExplain(threeSentenceText, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("CapN: got %d sentences from 3-sentence input", len(result))
	}
	if info.SelectedN > 3 {
		t.Errorf("ExplainInfo.SelectedN = %d, want <= 3", info.SelectedN)
	}
}

// ── powerIterate non-convergence ──────────────────────────────────────────────

func TestPowerIterate_NonConvergence(t *testing.T) {
	// maxIter=1 forces early exit without convergence
	m := [][]float64{
		{0.5, 0.5},
		{0.3, 0.7},
	}
	scores, iters, converged := powerIterate(m, 1e-15, 1)
	if converged {
		t.Error("expected non-convergence with maxIter=1")
	}
	if iters != 1 {
		t.Errorf("iters = %d, want 1", iters)
	}
	if len(scores) != 2 {
		t.Errorf("scores length = %d, want 2", len(scores))
	}
}

func TestPowerIterateDamped_NonConvergence(t *testing.T) {
	// maxIter=1 forces early exit without convergence
	m := [][]float64{
		{0.5, 0.5},
		{0.3, 0.7},
	}
	scores, iters, converged := powerIterateDamped(m, 0.85, 1e-15, 1)
	if converged {
		t.Error("expected non-convergence with maxIter=1")
	}
	if iters != 1 {
		t.Errorf("iters = %d, want 1", iters)
	}
	if len(scores) != 2 {
		t.Errorf("scores length = %d, want 2", len(scores))
	}
}
