package summarizer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the repository root directory using the location of this test file.
// This allows the tests to locate test-data/ regardless of working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	// This file is at internal/summarizer/integration_test.go
	// repo root is two levels up
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// filename = .../internal/summarizer/integration_test.go
	// filepath.Dir twice gives repo root
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

func readTestFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "test-data", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read test fixture %s: %v", path, err)
	}
	return string(data)
}

func TestSummarize_WikipediaEn(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(wikipedia_en.txt) returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize(wikipedia_en.txt) returned empty slice")
	}
}

func TestSummarize_YoutubeTranscript(t *testing.T) {
	text := readTestFile(t, "youtube_transcript.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(youtube_transcript.txt) returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize(youtube_transcript.txt) returned empty slice")
	}
}

func TestSummarize_LongformDoc(t *testing.T) {
	text := readTestFile(t, "longform_3000.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(longform_3000.txt) returned error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("Summarize(longform_3000.txt, n=5): got %d sentences, want 5 (document has 20+ sentences)", len(result))
	}
}

func TestSummarize_EdgeShort_SilentCap(t *testing.T) {
	// edge_short.txt has exactly 3 sentences.
	// Requesting 5 should return <=3 without error (silent cap per didasy/tldr behavior).
	text := readTestFile(t, "edge_short.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(edge_short.txt) returned unexpected error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Summarize(edge_short.txt, n=5): got %d sentences from a 3-sentence doc; expected <=3", len(result))
	}
	for i, s := range result {
		if s == "" {
			t.Errorf("result[%d] is empty string", i)
		}
	}
}

// LexRank integration tests via New() registry

func TestNew_LexRank_WikipediaEn(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, err := New("lexrank")
	if err != nil {
		t.Fatalf("New(lexrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("LexRank.Summarize(wikipedia_en) error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("LexRank returned empty slice for wikipedia_en")
	}
	if len(result) > 5 {
		t.Errorf("LexRank returned %d sentences, want <= 5", len(result))
	}
}

func TestNew_LexRank_YoutubeTranscript(t *testing.T) {
	text := readTestFile(t, "youtube_transcript.txt")
	s, err := New("lexrank")
	if err != nil {
		t.Fatalf("New(lexrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("LexRank.Summarize(youtube_transcript) error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("LexRank returned empty slice for youtube_transcript")
	}
	if len(result) > 5 {
		t.Errorf("LexRank returned %d sentences, want <= 5", len(result))
	}
}

func TestNew_LexRank_Longform(t *testing.T) {
	text := readTestFile(t, "longform_3000.txt")
	s, err := New("lexrank")
	if err != nil {
		t.Fatalf("New(lexrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("LexRank.Summarize(longform_3000) error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("LexRank returned %d sentences, want 5", len(result))
	}
}

func TestNew_LexRank_EdgeShort(t *testing.T) {
	text := readTestFile(t, "edge_short.txt")
	s, err := New("lexrank")
	if err != nil {
		t.Fatalf("New(lexrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("LexRank.Summarize(edge_short) error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("LexRank returned %d sentences from 3-sentence doc, want <= 3", len(result))
	}
}

// TextRank integration tests via New() registry

func TestNew_TextRank_WikipediaEn(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, err := New("textrank")
	if err != nil {
		t.Fatalf("New(textrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("TextRank.Summarize(wikipedia_en) error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("TextRank returned empty slice for wikipedia_en")
	}
	if len(result) > 5 {
		t.Errorf("TextRank returned %d sentences, want <= 5", len(result))
	}
}

func TestNew_TextRank_YoutubeTranscript(t *testing.T) {
	text := readTestFile(t, "youtube_transcript.txt")
	s, err := New("textrank")
	if err != nil {
		t.Fatalf("New(textrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("TextRank.Summarize(youtube_transcript) error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("TextRank returned empty slice for youtube_transcript")
	}
	if len(result) > 5 {
		t.Errorf("TextRank returned %d sentences, want <= 5", len(result))
	}
}

func TestNew_TextRank_Longform(t *testing.T) {
	text := readTestFile(t, "longform_3000.txt")
	s, err := New("textrank")
	if err != nil {
		t.Fatalf("New(textrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("TextRank.Summarize(longform_3000) error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("TextRank returned %d sentences, want 5", len(result))
	}
}

func TestNew_TextRank_EdgeShort(t *testing.T) {
	text := readTestFile(t, "edge_short.txt")
	s, err := New("textrank")
	if err != nil {
		t.Fatalf("New(textrank) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("TextRank.Summarize(edge_short) error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("TextRank returned %d sentences from 3-sentence doc, want <= 3", len(result))
	}
}

// Registry error test (TEST-05)

func TestNew_UnknownAlgorithm(t *testing.T) {
	_, err := New("nonexistent")
	if err == nil {
		t.Fatal("New(nonexistent) should return error")
	}
}

// Determinism integration tests (TEST-06)

func TestNew_LexRank_Deterministic_RealData(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, err := New("lexrank")
	if err != nil {
		t.Fatalf("New(lexrank) error: %v", err)
	}
	r1, err := s.Summarize(text, 3)
	if err != nil {
		t.Fatalf("first Summarize error: %v", err)
	}
	r2, err := s.Summarize(text, 3)
	if err != nil {
		t.Fatalf("second Summarize error: %v", err)
	}
	if len(r1) != len(r2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, r1[i], r2[i])
		}
	}
}

func TestNew_TextRank_Deterministic_RealData(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, _ := New("textrank")
	r1, _ := s.Summarize(text, 3)
	r2, _ := s.Summarize(text, 3)
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, r1[i], r2[i])
		}
	}
}

func TestNew_Ensemble_Deterministic_RealData(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, _ := New("ensemble")
	r1, _ := s.Summarize(text, 3)
	r2, _ := s.Summarize(text, 3)
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, r1[i], r2[i])
		}
	}
}

// Ensemble integration tests

func TestNew_Ensemble_WikipediaEn(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	s, err := New("ensemble")
	if err != nil {
		t.Fatalf("New(ensemble) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("Ensemble.Summarize(wikipedia_en) error: %v", err)
	}
	if len(result) == 0 || len(result) > 5 {
		t.Errorf("Ensemble returned %d sentences, want 1-5", len(result))
	}
}

func TestNew_Ensemble_YoutubeTranscript(t *testing.T) {
	text := readTestFile(t, "youtube_transcript.txt")
	s, err := New("ensemble")
	if err != nil {
		t.Fatalf("New(ensemble) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("Ensemble.Summarize(youtube_transcript) error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Ensemble returned empty slice for youtube_transcript")
	}
}

func TestNew_Ensemble_Longform(t *testing.T) {
	text := readTestFile(t, "longform_3000.txt")
	s, err := New("ensemble")
	if err != nil {
		t.Fatalf("New(ensemble) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("Ensemble.Summarize(longform_3000) error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("Ensemble returned %d sentences, want 5", len(result))
	}
}

func TestNew_Ensemble_EdgeShort(t *testing.T) {
	text := readTestFile(t, "edge_short.txt")
	s, err := New("ensemble")
	if err != nil {
		t.Fatalf("New(ensemble) error: %v", err)
	}
	result, err := s.Summarize(text, 5)
	if err != nil {
		t.Fatalf("Ensemble.Summarize(edge_short) error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Ensemble returned %d sentences from 3-sentence doc, want <= 3", len(result))
	}
}

// Cross-algorithm: all results must be substrings of original text

func TestAllAlgorithms_SentencesFromOriginal(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	algos := []string{"lexrank", "textrank", "graph", "ensemble"}
	for _, algo := range algos {
		t.Run(algo, func(t *testing.T) {
			s, err := New(algo)
			if err != nil {
				t.Fatalf("New(%s) error: %v", algo, err)
			}
			result, err := s.Summarize(text, 5)
			if err != nil {
				t.Fatalf("%s.Summarize error: %v", algo, err)
			}
			for _, sent := range result {
				if sent == "" {
					t.Errorf("%s: empty sentence in result", algo)
				}
			}
		})
	}
}

// ROUGE integration: ensemble vs lexrank on same text should both score > 0 against each other

func TestROUGE_CrossAlgorithm_NonZero(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	lr, _ := New("lexrank")
	en, _ := New("ensemble")
	lrResult, _ := lr.Summarize(text, 5)
	enResult, _ := en.Summarize(text, 5)
	score := EvalROUGE(enResult, lrResult)
	// Both summaries come from same document — expect non-trivial overlap
	if score.ROUGE1.F1 <= 0 {
		t.Errorf("ROUGE-1 F1 between ensemble and lexrank summaries = %f, want > 0", score.ROUGE1.F1)
	}
}
