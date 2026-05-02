package summarizer

import (
	"math"
	"strings"
	"testing"
)

func TestWordOverlapSim_CommonWords(t *testing.T) {
	s1 := []string{"the", "cat", "sat"}
	s2 := []string{"the", "cat", "ran"}
	got := wordOverlapSim(s1, s2)
	want := 2.0 / (math.Log(float64(len(s1))) + math.Log(float64(len(s2))))
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("wordOverlapSim = %f, want %f", got, want)
	}
	if got <= 0 {
		t.Errorf("wordOverlapSim with 2 common words should be > 0, got %f", got)
	}
}

func TestWordOverlapSim_NoOverlap(t *testing.T) {
	s1 := []string{"cat", "sat"}
	s2 := []string{"dog", "ran"}
	got := wordOverlapSim(s1, s2)
	if got != 0.0 {
		t.Errorf("wordOverlapSim with no overlap = %f, want 0.0", got)
	}
}

func TestWordOverlapSim_SingleWord(t *testing.T) {
	// len <= 1 guard: log(1) = 0, division by zero — must return 0.0
	s1 := []string{"cat"}
	s2 := []string{"cat", "dog"}
	got := wordOverlapSim(s1, s2)
	if got != 0.0 {
		t.Errorf("wordOverlapSim with len(s1)==1 = %f, want 0.0", got)
	}
}

func TestWordOverlapSim_EmptySlice(t *testing.T) {
	got := wordOverlapSim([]string{}, []string{"cat"})
	if got != 0.0 {
		t.Errorf("wordOverlapSim with empty slice = %f, want 0.0", got)
	}
}

func TestPowerIterateDamped_UniformMatrix(t *testing.T) {
	// 3x3 uniform matrix: each entry = 1/3
	n := 3
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		for j := range matrix[i] {
			matrix[i][j] = 1.0 / float64(n)
		}
	}
	scores := powerIterateDamped(matrix, 0.85, 0.0001, 1000)
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
			t.Error("Summarize returned empty sentence in result")
		}
	}
}

func TestTextRank_Summarize_EmptyInput(t *testing.T) {
	tr := &TextRank{}
	result, err := tr.Summarize("", 5)
	if err != nil {
		t.Errorf("expected nil error for empty input, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}
}

func TestTextRank_Summarize_SilentCap(t *testing.T) {
	// SUM-04: n > sentence count returns <= sentence count with no error
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
	// SUM-05: returned sentences must appear in original document order
	tr := &TextRank{}
	result, err := tr.Summarize(tenSentenceText, 5)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	sentences := TokenizeSentences(tenSentenceText)
	// Find the index of each result sentence in the original list
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
			t.Errorf("result sentence not found in original: %q", s)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("document order violated: got index %d after %d", idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestTextRank_Deterministic(t *testing.T) {
	// TEST-06: two consecutive calls produce identical output
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
