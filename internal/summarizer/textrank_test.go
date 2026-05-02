package summarizer

import (
	"math"
	"strings"
	"testing"
)

// ── wordOverlapSim ────────────────────────────────────────────────────────────

func TestWordOverlapSim_CommonWords(t *testing.T) {
	s1 := []string{"the", "cat", "sat"}
	s2 := []string{"the", "cat", "ran"}
	got := wordOverlapSim(s1, s2)
	want := 2.0 / (math.Log(float64(len(s1))) + math.Log(float64(len(s2))))
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("wordOverlapSim = %f, want %f", got, want)
	}
}

func TestWordOverlapSim_NoOverlap(t *testing.T) {
	s1 := []string{"cat", "sat"}
	s2 := []string{"dog", "ran"}
	got := wordOverlapSim(s1, s2)
	if got != 0.0 {
		t.Errorf("wordOverlapSim no overlap = %f, want 0.0", got)
	}
}

func TestWordOverlapSim_SingleWord(t *testing.T) {
	// len <= 1 → log(1)=0 → division-by-zero guard must return 0.0
	s1 := []string{"cat"}
	s2 := []string{"cat", "dog"}
	got := wordOverlapSim(s1, s2)
	if got != 0.0 {
		t.Errorf("wordOverlapSim len(s1)==1 = %f, want 0.0", got)
	}
}

func TestWordOverlapSim_EmptySlice(t *testing.T) {
	got := wordOverlapSim([]string{}, []string{"cat"})
	if got != 0.0 {
		t.Errorf("wordOverlapSim empty = %f, want 0.0", got)
	}
}

func TestWordOverlapSim_IdenticalSentences(t *testing.T) {
	s := []string{"the", "quick", "brown", "fox"}
	got := wordOverlapSim(s, s)
	if got <= 0 {
		t.Errorf("identical sentences similarity = %f, want > 0", got)
	}
}

// ── trRowNormalize ────────────────────────────────────────────────────────────

func TestTRRowNormalize_NormalRow(t *testing.T) {
	m := [][]float64{{1.0, 3.0}}
	trRowNormalize(m)
	if math.Abs(m[0][0]-0.25) > 0.0001 {
		t.Errorf("m[0][0] = %f, want 0.25", m[0][0])
	}
	if math.Abs(m[0][1]-0.75) > 0.0001 {
		t.Errorf("m[0][1] = %f, want 0.75", m[0][1])
	}
}

func TestTRRowNormalize_DanglingRow(t *testing.T) {
	// All-zero row must become uniform (1/n where n = len(matrix) = 3 rows)
	m := [][]float64{
		{0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
	}
	trRowNormalize(m)
	n := 3 // len(matrix)
	for j, v := range m[0] {
		want := 1.0 / float64(n)
		if math.Abs(v-want) > 0.0001 {
			t.Errorf("dangling row m[0][%d] = %f, want %f", j, v, want)
		}
	}
}

// ── powerIterateDamped ────────────────────────────────────────────────────────

func TestPowerIterateDamped_UniformMatrix(t *testing.T) {
	n := 3
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		for j := range matrix[i] {
			matrix[i][j] = 1.0 / float64(n)
		}
	}
	scores, _, _ := powerIterateDamped(matrix, 0.85, 0.0001, 1000)
	if len(scores) != n {
		t.Fatalf("expected %d scores, got %d", n, len(scores))
	}
	expected := 1.0 / float64(n)
	for i, s := range scores {
		if math.Abs(s-expected) > 0.01 {
			t.Errorf("scores[%d] = %f, want ~%f", i, s, expected)
		}
	}
}

func TestPowerIterateDamped_ScoresSumToOne(t *testing.T) {
	n := 5
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		for j := range matrix[i] {
			matrix[i][j] = 1.0 / float64(n)
		}
	}
	scores, _, _ := powerIterateDamped(matrix, 0.85, 0.0001, 1000)
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	if math.Abs(sum-1.0) > 0.01 {
		t.Errorf("scores sum = %f, want ~1.0", sum)
	}
}

// ── trSelectTopN ──────────────────────────────────────────────────────────────

func TestTRSelectTopN_DocumentOrder(t *testing.T) {
	sentences := []string{"A", "B", "C", "D"}
	// Highest scores at index 3,1 → top-2 in doc order = B, D
	scores := []float64{0.1, 0.4, 0.2, 0.5}
	result := trSelectTopN(scores, 2, sentences)
	want := []string{"B", "D"}
	if len(result) != 2 {
		t.Fatalf("got %d results, want 2", len(result))
	}
	for i, w := range want {
		if result[i] != w {
			t.Errorf("result[%d] = %q, want %q", i, result[i], w)
		}
	}
}

// ── TextRank.Summarize ────────────────────────────────────────────────────────

func TestTextRank_Summarize_Basic(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 sentences, got %d", len(result))
	}
	for _, s := range result {
		if strings.TrimSpace(s) == "" {
			t.Error("Summarize returned empty sentence")
		}
	}
}

func TestTextRank_Summarize_EmptyInput(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize("", 5)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestTextRank_Summarize_SilentCap(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize(threeSentenceText, 10)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("expected <= 3 sentences, got %d", len(result))
	}
}

func TestTextRank_Summarize_DocumentOrder(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize(tenSentenceText, 5)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	sentences := TokenizeSentences(tenSentenceText)
	indexOf := func(s string) int {
		for i, orig := range sentences {
			if orig == s {
				return i
			}
		}
		return -1
	}
	lastIdx := -1
	for _, s := range result {
		idx := indexOf(s)
		if idx == -1 {
			t.Errorf("result sentence not in original: %q", s)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("document order violated: idx %d after %d", idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestTextRank_Summarize_AllSentencesFromOriginal(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize(tenSentenceText, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range result {
		if !strings.Contains(tenSentenceText, s) {
			t.Errorf("result sentence %q not in original text", s)
		}
	}
}

func TestTextRank_Deterministic(t *testing.T) {
	tr := &TextRank{}
	r1, err1 := tr.Summarize(tenSentenceText, 4)
	r2, err2 := tr.Summarize(tenSentenceText, 4)
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v, %v", err1, err2)
	}
	if len(r1) != len(r2) {
		t.Fatalf("different lengths: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("result[%d] differs: %q vs %q", i, r1[i], r2[i])
		}
	}
}

// ── TextRank.SummarizeExplain ─────────────────────────────────────────────────

func TestTextRank_SummarizeExplain_Basic(t *testing.T) {
	tr := &TextRank{}
	result, info, err := tr.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("SummarizeExplain returned error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("want 3 sentences, got %d", len(result))
	}
	if info == nil {
		t.Fatal("ExplainInfo is nil")
	}
	if info.Algorithm != "textrank" {
		t.Errorf("Algorithm = %q, want textrank", info.Algorithm)
	}
	if info.InputSentences != 10 {
		t.Errorf("InputSentences = %d, want 10", info.InputSentences)
	}
	if info.SelectedN != 3 {
		t.Errorf("SelectedN = %d, want 3", info.SelectedN)
	}
	if info.DampingFactor != textRankDamping {
		t.Errorf("DampingFactor = %f, want %f", info.DampingFactor, textRankDamping)
	}
	if !info.Converged {
		t.Error("expected convergence for small input")
	}
	if len(info.Scores) != 10 {
		t.Errorf("Scores length = %d, want 10", len(info.Scores))
	}
}

func TestTextRank_SummarizeExplain_SelectedFlaggedCorrectly(t *testing.T) {
	tr := &TextRank{}
	result, info, err := tr.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	selectedCount := 0
	for _, sc := range info.Scores {
		if sc.Selected {
			selectedCount++
		}
	}
	if selectedCount != len(result) {
		t.Errorf("Selected count = %d, want %d", selectedCount, len(result))
	}
}

func TestTextRank_SummarizeExplain_SimilarityPairsCount(t *testing.T) {
	tr := &TextRank{}
	_, info, err := tr.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 sentences → 10*9=90 pairs
	wantPairs := 10 * 9
	if info.SimilarityPairs != wantPairs {
		t.Errorf("SimilarityPairs = %d, want %d", info.SimilarityPairs, wantPairs)
	}
}
