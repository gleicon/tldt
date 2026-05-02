package summarizer

import (
	"math"
	"strings"
	"testing"
)

// ── IDF / vocab ──────────────────────────────────────────────────────────────

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

	wantCatIDF := math.Log(2.0 / 1.0)
	if math.Abs(idf[0]-wantCatIDF) > 0.001 {
		t.Errorf("idf[cat] = %f, want %f", idf[0], wantCatIDF)
	}
	wantDogIDF := math.Log(2.0 / 1.0)
	if math.Abs(idf[1]-wantDogIDF) > 0.001 {
		t.Errorf("idf[dog] = %f, want %f", idf[1], wantDogIDF)
	}
	wantTheIDF := 0.0
	if math.Abs(idf[2]-wantTheIDF) > 0.001 {
		t.Errorf("idf[the] = %f, want %f", idf[2], wantTheIDF)
	}
}

// ── buildTFVector ─────────────────────────────────────────────────────────────

func TestBuildTFVector_Basic(t *testing.T) {
	// words: ["cat","cat","dog"] → TF(cat)=2/3, TF(dog)=1/3
	// vocab: ["cat","dog"]
	wordIdx := map[string]int{"cat": 0, "dog": 1}
	words := []string{"cat", "cat", "dog"}
	v := buildTFVector(words, wordIdx, 2)
	if len(v) != 2 {
		t.Fatalf("vector length = %d, want 2", len(v))
	}
	if math.Abs(v[0]-2.0/3) > 0.001 {
		t.Errorf("TF(cat) = %f, want %f", v[0], 2.0/3)
	}
	if math.Abs(v[1]-1.0/3) > 0.001 {
		t.Errorf("TF(dog) = %f, want %f", v[1], 1.0/3)
	}
}

func TestBuildTFVector_EmptyWords(t *testing.T) {
	wordIdx := map[string]int{"cat": 0}
	v := buildTFVector([]string{}, wordIdx, 1)
	if len(v) != 1 {
		t.Fatalf("vector length = %d, want 1", len(v))
	}
	if v[0] != 0.0 {
		t.Errorf("TF for empty words = %f, want 0.0", v[0])
	}
}

// ── cosine similarity ─────────────────────────────────────────────────────────

func TestLexRank_CosineIdentical(t *testing.T) {
	v := []float64{1.0, 2.0, 3.0}
	idf := []float64{1.0, 1.0, 1.0}
	got := idfCosine(v, v, idf)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("idfCosine(v, v, idf) = %f, want 1.0", got)
	}
}

func TestLexRank_CosineOrthogonal(t *testing.T) {
	v1 := []float64{1.0, 0.0}
	v2 := []float64{0.0, 1.0}
	idf := []float64{1.0, 1.0}
	got := idfCosine(v1, v2, idf)
	if math.Abs(got-0.0) > 0.0001 {
		t.Errorf("idfCosine(orthogonal) = %f, want 0.0", got)
	}
}

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

// ── rowNormalize ──────────────────────────────────────────────────────────────

func TestRowNormalize_NormalRow(t *testing.T) {
	m := [][]float64{{2.0, 2.0, 2.0}}
	rowNormalize(m)
	for j, v := range m[0] {
		if math.Abs(v-1.0/3) > 0.0001 {
			t.Errorf("m[0][%d] = %f, want 0.333", j, v)
		}
	}
}

func TestRowNormalize_DanglingRow(t *testing.T) {
	// All-zero row must become uniform (1/n)
	m := [][]float64{
		{0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
	}
	rowNormalize(m)
	n := 3
	for j, v := range m[0] {
		want := 1.0 / float64(n)
		if math.Abs(v-want) > 0.0001 {
			t.Errorf("dangling row m[0][%d] = %f, want %f", j, v, want)
		}
	}
}

// ── powerIterate ─────────────────────────────────────────────────────────────

func TestPowerIterate_UniformMatrix(t *testing.T) {
	n := 3
	m := make([][]float64, n)
	for i := range m {
		m[i] = []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
	}
	got, _, _ := powerIterate(m, 0.0001, 1000)
	for i, v := range got {
		if math.Abs(v-1.0/3) > 0.001 {
			t.Errorf("scores[%d] = %f, want ~0.333", i, v)
		}
	}
}

func TestPowerIterate_AsymmetricMatrix(t *testing.T) {
	// Stationary distribution: pi[0]=1/3, pi[1]=2/3
	m := [][]float64{
		{0.5, 0.5},
		{0.25, 0.75},
	}
	got, _, _ := powerIterate(m, 0.0001, 1000)
	if math.Abs(got[0]-1.0/3) > 0.001 {
		t.Errorf("scores[0] = %f, want ~0.333", got[0])
	}
	if math.Abs(got[1]-2.0/3) > 0.001 {
		t.Errorf("scores[1] = %f, want ~0.667", got[1])
	}
}

func TestPowerIterate_ScoresSumToOne(t *testing.T) {
	n := 4
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n)
		for j := range m[i] {
			m[i][j] = 1.0 / float64(n)
		}
	}
	scores, _, _ := powerIterate(m, 0.0001, 1000)
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("power iteration scores sum = %f, want ~1.0", sum)
	}
}

// ── selectTopN ────────────────────────────────────────────────────────────────

func TestSelectTopN_DocumentOrder(t *testing.T) {
	sentences := []string{"A", "B", "C", "D", "E"}
	// Score order: C > E > A > B > D → top-3 are indices {0,2,4}
	scores := []float64{0.3, 0.1, 0.5, 0.05, 0.4}
	result := selectTopN(scores, 3, sentences)
	// Must return in document order: A, C, E
	want := []string{"A", "C", "E"}
	if len(result) != len(want) {
		t.Fatalf("got %d sentences, want %d", len(result), len(want))
	}
	for i := range want {
		if result[i] != want[i] {
			t.Errorf("result[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestSelectTopN_CapToLen(t *testing.T) {
	sentences := []string{"X", "Y"}
	scores := []float64{0.6, 0.4}
	result := selectTopN(scores, 10, sentences)
	if len(result) > 2 {
		t.Errorf("selectTopN returned %d, want <= 2", len(result))
	}
}

// ── LexRank.Summarize ─────────────────────────────────────────────────────────

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

func TestLexRank_Summarize_SilentCap(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(threeSentenceText, 10)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Summarize returned %d sentences from 3-sentence input, want <= 3", len(result))
	}
}

func TestLexRank_Summarize_DocumentOrder(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(tenSentenceText, 5)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	prevIdx := -1
	for _, sentence := range result {
		idx := strings.Index(tenSentenceText, sentence)
		if idx == -1 {
			t.Errorf("returned sentence %q not found in original text", sentence)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("document order violated: %q at %d is before previous at %d", sentence, idx, prevIdx)
		}
		prevIdx = idx
	}
}

func TestLexRank_Summarize_AllSentencesFromOriginal(t *testing.T) {
	l := &LexRank{}
	result, err := l.Summarize(tenSentenceText, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range result {
		if !strings.Contains(tenSentenceText, s) {
			t.Errorf("result sentence %q not in original text", s)
		}
	}
}

func TestLexRank_Deterministic(t *testing.T) {
	l := &LexRank{}
	result1, _ := l.Summarize(tenSentenceText, 3)
	result2, _ := l.Summarize(tenSentenceText, 3)
	if len(result1) != len(result2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(result1), len(result2))
	}
	for i := range result1 {
		if result1[i] != result2[i] {
			t.Errorf("sentence[%d] differs: %q vs %q", i, result1[i], result2[i])
		}
	}
}

// ── LexRank.SummarizeExplain ──────────────────────────────────────────────────

func TestLexRank_SummarizeExplain_Basic(t *testing.T) {
	l := &LexRank{}
	result, info, err := l.SummarizeExplain(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("SummarizeExplain returned error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("want 3 sentences, got %d", len(result))
	}
	if info == nil {
		t.Fatal("ExplainInfo is nil")
	}
	if info.Algorithm != "lexrank" {
		t.Errorf("Algorithm = %q, want lexrank", info.Algorithm)
	}
	if info.InputSentences != 10 {
		t.Errorf("InputSentences = %d, want 10", info.InputSentences)
	}
	if info.SelectedN != 3 {
		t.Errorf("SelectedN = %d, want 3", info.SelectedN)
	}
	if info.VocabSize == 0 {
		t.Error("VocabSize = 0, want > 0")
	}
	if !info.Converged {
		t.Error("expected convergence for small input")
	}
	if len(info.Scores) != 10 {
		t.Errorf("Scores length = %d, want 10", len(info.Scores))
	}
}

func TestLexRank_SummarizeExplain_SelectedFlaggedCorrectly(t *testing.T) {
	l := &LexRank{}
	result, info, err := l.SummarizeExplain(tenSentenceText, 3)
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
		t.Errorf("Selected count in ExplainInfo = %d, want %d (len of result)", selectedCount, len(result))
	}
}

func TestLexRank_SummarizeExplain_Empty(t *testing.T) {
	l := &LexRank{}
	_, info, err := l.SummarizeExplain("", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("ExplainInfo nil for empty input")
	}
	if info.InputSentences != 0 {
		t.Errorf("InputSentences = %d for empty input, want 0", info.InputSentences)
	}
}
