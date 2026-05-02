package summarizer

import (
	"strings"
	"testing"
)

const ensembleText = `The quick brown fox jumps over the lazy dog.
A fox is a clever animal that lives in many environments.
Dogs are domesticated animals and loyal companions to humans.
Scientists study animal behavior to understand evolution.
The relationship between predators and prey shapes ecosystems.
Many animals have adapted to urban environments over centuries.
Research shows that foxes are increasingly common in cities.`

// ── Ensemble.Summarize ────────────────────────────────────────────────────────

func TestEnsemble_Summarize_Basic(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize(ensembleText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("want 3 sentences, got %d", len(result))
	}
}

func TestEnsemble_Summarize_Empty(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize("", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("want nil for empty input, got %v", result)
	}
}

func TestEnsemble_Summarize_SilentCap(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize("One sentence only.", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) > 1 {
		t.Fatalf("want at most 1 sentence, got %d", len(result))
	}
}

func TestEnsemble_Summarize_DocumentOrder(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize(ensembleText, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sentences := TokenizeSentences(ensembleText)
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
			t.Errorf("document order violated: got index %d after %d", idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestEnsemble_Summarize_AllSentencesFromOriginal(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize(ensembleText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range result {
		if !strings.Contains(ensembleText, s) {
			t.Errorf("result sentence %q not in original text", s)
		}
	}
}

func TestEnsemble_Summarize_NoBlanks(t *testing.T) {
	e := &Ensemble{}
	result, err := e.Summarize(ensembleText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range result {
		if strings.TrimSpace(s) == "" {
			t.Errorf("empty sentence in result")
		}
	}
}

func TestEnsemble_Deterministic(t *testing.T) {
	e := &Ensemble{}
	r1, err := e.Summarize(ensembleText, 3)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	r2, err := e.Summarize(ensembleText, 3)
	if err != nil {
		t.Fatalf("second call error: %v", err)
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

// ── New("ensemble") ───────────────────────────────────────────────────────────

func TestEnsemble_ViaNew(t *testing.T) {
	s, err := New("ensemble")
	if err != nil {
		t.Fatalf("New(ensemble) failed: %v", err)
	}
	result, err := s.Summarize(ensembleText, 2)
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("want 2, got %d", len(result))
	}
}

// ── internal score helpers ────────────────────────────────────────────────────

func TestLexrankScores_Length(t *testing.T) {
	sentences := TokenizeSentences(ensembleText)
	scores := lexrankScores(sentences)
	if len(scores) != len(sentences) {
		t.Errorf("lexrankScores returned %d scores for %d sentences", len(scores), len(sentences))
	}
}

func TestTextrankScores_Length(t *testing.T) {
	sentences := TokenizeSentences(ensembleText)
	scores := textrankScores(sentences)
	if len(scores) != len(sentences) {
		t.Errorf("textrankScores returned %d scores for %d sentences", len(scores), len(sentences))
	}
}

func TestLexrankScores_NonNegative(t *testing.T) {
	sentences := TokenizeSentences(ensembleText)
	for i, s := range lexrankScores(sentences) {
		if s < 0 {
			t.Errorf("lexrankScores[%d] = %f, want >= 0", i, s)
		}
	}
}

func TestTextrankScores_NonNegative(t *testing.T) {
	sentences := TokenizeSentences(ensembleText)
	for i, s := range textrankScores(sentences) {
		if s < 0 {
			t.Errorf("textrankScores[%d] = %f, want >= 0", i, s)
		}
	}
}
