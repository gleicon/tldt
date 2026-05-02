package summarizer

import (
	"math"
	"strings"
	"testing"
)

// TestLexRank_TFIDFVectors verifies IDF computation on a known 2-sentence corpus.
// Sentences: {"the","cat"} and {"the","dog"}
// IDF("the") = log(2/2) = 0.0
// IDF("cat") = log(2/1) = log(2) ~ 0.693
// IDF("dog") = log(2/1) = log(2) ~ 0.693
// Vocab must be sorted alphabetically: ["cat","dog","the"]
func TestLexRank_TFIDFVectors(t *testing.T) {
	sentences := [][]string{
		{"the", "cat"},
		{"the", "dog"},
	}
	vocab, idf := buildVocabAndIDF(sentences)

	wantVocab := []string{"cat", "dog", "the"}
	if len(vocab) != len(wantVocab) {
		t.Fatalf("vocab length = %d, want %d", len(vocab), len(wantVocab))
	}
	for i, w := range wantVocab {
		if vocab[i] != w {
			t.Errorf("vocab[%d] = %q, want %q", i, vocab[i], w)
		}
	}

	// IDF for "cat" at index 0
	wantCatIDF := math.Log(2.0 / 1.0)
	if math.Abs(idf[0]-wantCatIDF) > 0.001 {
		t.Errorf("idf[cat] = %f, want %f", idf[0], wantCatIDF)
	}

	// IDF for "dog" at index 1
	wantDogIDF := math.Log(2.0 / 1.0)
	if math.Abs(idf[1]-wantDogIDF) > 0.001 {
		t.Errorf("idf[dog] = %f, want %f", idf[1], wantDogIDF)
	}

	// IDF for "the" at index 2 — appears in both sentences → IDF = log(2/2) = 0
	wantTheIDF := 0.0
	if math.Abs(idf[2]-wantTheIDF) > 0.001 {
		t.Errorf("idf[the] = %f, want %f", idf[2], wantTheIDF)
	}
}

// TestLexRank_CosineIdentical verifies that idfCosine(v, v, idf) == 1.0
func TestLexRank_CosineIdentical(t *testing.T) {
	v := []float64{1.0, 2.0, 3.0}
	idf := []float64{1.0, 1.0, 1.0}
	got := idfCosine(v, v, idf)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("idfCosine(v, v, idf) = %f, want 1.0", got)
	}
}

// TestLexRank_CosineOrthogonal verifies that idfCosine([1,0], [0,1], [1,1]) == 0.0
func TestLexRank_CosineOrthogonal(t *testing.T) {
	v1 := []float64{1.0, 0.0}
	v2 := []float64{0.0, 1.0}
	idf := []float64{1.0, 1.0}
	got := idfCosine(v1, v2, idf)
	if math.Abs(got-0.0) > 0.0001 {
		t.Errorf("idfCosine(orthogonal) = %f, want 0.0", got)
	}
}

// TestLexRank_CosineZeroVector verifies that idfCosine([0,0], [1,1], [1,1]) == 0.0 (no NaN/panic)
func TestLexRank_CosineZeroVector(t *testing.T) {
	v1 := []float64{0.0, 0.0}
	v2 := []float64{1.0, 1.0}
	idf := []float64{1.0, 1.0}
	got := idfCosine(v1, v2, idf)
	if math.IsNaN(got) {
		t.Error("idfCosine with zero vector returned NaN")
	}
	if math.Abs(got-0.0) > 0.0001 {
		t.Errorf("idfCosine(zero, v, idf) = %f, want 0.0", got)
	}
}

// TestPowerIterate_UniformMatrix verifies convergence of a 3x3 uniform stochastic matrix.
// Each row is [1/3, 1/3, 1/3] — stationary distribution is [1/3, 1/3, 1/3].
func TestPowerIterate_UniformMatrix(t *testing.T) {
	n := 3
	m := make([][]float64, n)
	for i := range m {
		m[i] = []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
	}
	got := powerIterate(m, 0.0001, 1000)
	for i, v := range got {
		if math.Abs(v-1.0/3) > 0.001 {
			t.Errorf("scores[%d] = %f, want ~0.333", i, v)
		}
	}
}

// TestPowerIterate_AsymmetricMatrix verifies convergence of a 2x2 asymmetric matrix.
// Matrix: [[0.5, 0.5], [0.25, 0.75]]
// Stationary distribution (solving pi*M = pi, sum=1):
//   pi[0] = 1/3, pi[1] = 2/3
func TestPowerIterate_AsymmetricMatrix(t *testing.T) {
	m := [][]float64{
		{0.5, 0.5},
		{0.25, 0.75},
	}
	got := powerIterate(m, 0.0001, 1000)
	if len(got) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(got))
	}
	if math.Abs(got[0]-1.0/3) > 0.001 {
		t.Errorf("scores[0] = %f, want ~0.333 (1/3)", got[0])
	}
	if math.Abs(got[1]-2.0/3) > 0.001 {
		t.Errorf("scores[1] = %f, want ~0.667 (2/3)", got[1])
	}
}

// TestLexRank_Summarize_Basic verifies that Summarize returns exactly 3 sentences.
func TestLexRank_Summarize_Basic(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Summarize returned %d sentences, want 3", len(result))
	}
	for _, s := range result {
		if strings.TrimSpace(s) == "" {
			t.Error("Summarize returned empty sentence in result")
		}
	}
}

// TestLexRank_Summarize_EmptyInput verifies that empty input returns nil, nil.
func TestLexRank_Summarize_EmptyInput(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize("", 5)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error for empty input: %v", err)
	}
	if result != nil {
		t.Errorf("Summarize returned %v for empty input, want nil", result)
	}
}

// TestLexRank_Summarize_SilentCap verifies that n > sentence count returns <= sentence count, no error.
func TestLexRank_Summarize_SilentCap(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(threeSentenceText, 10)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error for n > sentence count: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Summarize returned %d sentences from 3-sentence input, want <= 3", len(result))
	}
}

// TestLexRank_Summarize_DocumentOrder verifies that returned sentences appear in document order.
func TestLexRank_Summarize_DocumentOrder(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(tenSentenceText, 5)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize returned empty result")
	}

	// Find the index of each returned sentence in the original text
	prevIdx := -1
	for _, sentence := range result {
		idx := strings.Index(tenSentenceText, sentence)
		if idx == -1 {
			t.Errorf("returned sentence %q not found in original text", sentence)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("document order violated: sentence %q at position %d is before previous at %d", sentence, idx, prevIdx)
		}
		prevIdx = idx
	}
}

// TestLexRank_Deterministic verifies that two consecutive calls produce identical output.
func TestLexRank_Deterministic(t *testing.T) {
	l := &LexRank{}
	result1, err := l.Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("first Summarize call returned error: %v", err)
	}
	result2, err := l.Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("second Summarize call returned error: %v", err)
	}
	if len(result1) != len(result2) {
		t.Fatalf("different number of sentences: first=%d, second=%d", len(result1), len(result2))
	}
	for i := range result1 {
		if result1[i] != result2[i] {
			t.Errorf("sentence[%d] differs:\n  first:  %q\n  second: %q", i, result1[i], result2[i])
		}
	}
}
