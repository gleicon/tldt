package summarizer

import (
	"math"
	"testing"
)

func approxF(a, b, tol float64) bool { return math.Abs(a-b) < tol }

// ── ngramCounts ───────────────────────────────────────────────────────────────

func TestNgramCounts_Unigrams(t *testing.T) {
	counts := ngramCounts([]string{"a", "b", "a"}, 1)
	if counts["a"] != 2 {
		t.Errorf("count(a) = %d, want 2", counts["a"])
	}
	if counts["b"] != 1 {
		t.Errorf("count(b) = %d, want 1", counts["b"])
	}
}

func TestNgramCounts_Bigrams(t *testing.T) {
	counts := ngramCounts([]string{"a", "b", "c"}, 2)
	if _, ok := counts["a b"]; !ok {
		t.Error("bigram 'a b' missing")
	}
	if _, ok := counts["b c"]; !ok {
		t.Error("bigram 'b c' missing")
	}
	if len(counts) != 2 {
		t.Errorf("expected 2 bigrams, got %d", len(counts))
	}
}

func TestNgramCounts_NLargerThanInput(t *testing.T) {
	counts := ngramCounts([]string{"a"}, 2)
	if len(counts) != 0 {
		t.Errorf("expected 0 bigrams for 1-token input, got %d", len(counts))
	}
}

// ── lcs ───────────────────────────────────────────────────────────────────────

func TestLCS_FullMatch(t *testing.T) {
	a := []string{"a", "b", "c"}
	got := lcs(a, a)
	if got != 3 {
		t.Errorf("lcs(a,a) = %d, want 3", got)
	}
}

func TestLCS_NoMatch(t *testing.T) {
	got := lcs([]string{"x", "y"}, []string{"a", "b"})
	if got != 0 {
		t.Errorf("lcs no match = %d, want 0", got)
	}
}

func TestLCS_PartialMatch(t *testing.T) {
	// LCS of [a,b,c,d] and [a,c,d] = 3
	got := lcs([]string{"a", "b", "c", "d"}, []string{"a", "c", "d"})
	if got != 3 {
		t.Errorf("partial LCS = %d, want 3", got)
	}
}

func TestLCS_Asymmetric(t *testing.T) {
	// lcs(a,b) == lcs(b,a)
	a := []string{"a", "b", "c"}
	b := []string{"b", "c", "d", "e"}
	got1 := lcs(a, b)
	got2 := lcs(b, a)
	if got1 != got2 {
		t.Errorf("lcs not symmetric: %d vs %d", got1, got2)
	}
}

// ── ROUGE-1 ───────────────────────────────────────────────────────────────────

func TestROUGE1_PerfectMatch(t *testing.T) {
	sys := []string{"the cat sat on the mat"}
	ref := []string{"the cat sat on the mat"}
	score := EvalROUGE(sys, ref)
	if !approxF(score.ROUGE1.F1, 1.0, 0.001) {
		t.Errorf("perfect ROUGE-1 F1 = %f, want ~1.0", score.ROUGE1.F1)
	}
	if !approxF(score.ROUGE1.Precision, 1.0, 0.001) {
		t.Errorf("perfect ROUGE-1 Precision = %f, want ~1.0", score.ROUGE1.Precision)
	}
	if !approxF(score.ROUGE1.Recall, 1.0, 0.001) {
		t.Errorf("perfect ROUGE-1 Recall = %f, want ~1.0", score.ROUGE1.Recall)
	}
}

func TestROUGE1_NoOverlap(t *testing.T) {
	score := EvalROUGE([]string{"alpha beta gamma"}, []string{"delta epsilon zeta"})
	if score.ROUGE1.F1 != 0.0 {
		t.Errorf("no-overlap ROUGE-1 F1 = %f, want 0.0", score.ROUGE1.F1)
	}
}

func TestROUGE1_PrecisionRecallBalance(t *testing.T) {
	// sys shorter than ref → recall < 1, precision might be high
	sys := []string{"the cat sat"}
	ref := []string{"the cat sat on the mat with a hat"}
	score := EvalROUGE(sys, ref)
	if score.ROUGE1.Recall >= score.ROUGE1.Precision {
		t.Errorf("expected precision > recall when sys is subset of ref: P=%f R=%f",
			score.ROUGE1.Precision, score.ROUGE1.Recall)
	}
}

func TestROUGE1_SysLongerThanRef(t *testing.T) {
	// sys longer → precision lower than recall
	sys := []string{"the cat sat on the mat with extra words here"}
	ref := []string{"the cat sat"}
	score := EvalROUGE(sys, ref)
	if score.ROUGE1.Precision >= score.ROUGE1.Recall {
		t.Errorf("expected recall > precision when ref is subset of sys: P=%f R=%f",
			score.ROUGE1.Precision, score.ROUGE1.Recall)
	}
}

// ── ROUGE-2 ───────────────────────────────────────────────────────────────────

func TestROUGE2_PerfectMatch(t *testing.T) {
	sys := []string{"the cat sat on the mat"}
	ref := []string{"the cat sat on the mat"}
	score := EvalROUGE(sys, ref)
	if !approxF(score.ROUGE2.F1, 1.0, 0.001) {
		t.Errorf("perfect ROUGE-2 F1 = %f, want ~1.0", score.ROUGE2.F1)
	}
}

func TestROUGE2_PartialOverlap(t *testing.T) {
	sys := []string{"the cat sat on the mat"}
	ref := []string{"the cat sat on the rug"}
	score := EvalROUGE(sys, ref)
	if score.ROUGE2.F1 <= 0 || score.ROUGE2.F1 >= 1 {
		t.Errorf("ROUGE-2 F1 out of (0,1): %f", score.ROUGE2.F1)
	}
	// ROUGE-2 must be <= ROUGE-1 for partial match
	if score.ROUGE2.F1 > score.ROUGE1.F1+0.001 {
		t.Errorf("ROUGE-2 (%f) > ROUGE-1 (%f), unexpected", score.ROUGE2.F1, score.ROUGE1.F1)
	}
}

func TestROUGE2_NoOverlap(t *testing.T) {
	score := EvalROUGE([]string{"alpha beta"}, []string{"gamma delta"})
	if score.ROUGE2.F1 != 0 {
		t.Errorf("no-overlap ROUGE-2 F1 = %f, want 0", score.ROUGE2.F1)
	}
}

// ── ROUGE-L ───────────────────────────────────────────────────────────────────

func TestROUGEL_PerfectMatch(t *testing.T) {
	sys := []string{"a b c d"}
	ref := []string{"a b c d"}
	score := EvalROUGE(sys, ref)
	if !approxF(score.ROUGEL.F1, 1.0, 0.001) {
		t.Errorf("perfect ROUGE-L F1 = %f, want ~1.0", score.ROUGEL.F1)
	}
}

func TestROUGEL_PartialLCS(t *testing.T) {
	// sys="a b c", ref="a x b x c" → LCS tokens: a,b,c
	sys := []string{"a b c"}
	ref := []string{"a x b x c"}
	score := EvalROUGE(sys, ref)
	if score.ROUGEL.F1 <= 0 || score.ROUGEL.F1 > 1 {
		t.Errorf("ROUGE-L F1 out of range: %f", score.ROUGEL.F1)
	}
}

// ── empty / edge ──────────────────────────────────────────────────────────────

func TestROUGE_Empty(t *testing.T) {
	score := EvalROUGE(nil, nil)
	if score.ROUGE1.F1 != 0 || score.ROUGE2.F1 != 0 || score.ROUGEL.F1 != 0 {
		t.Errorf("empty input should yield zero scores")
	}
}

func TestROUGE_EmptySys(t *testing.T) {
	score := EvalROUGE(nil, []string{"reference text here"})
	if score.ROUGE1.F1 != 0 {
		t.Errorf("empty sys ROUGE-1 = %f, want 0", score.ROUGE1.F1)
	}
}

func TestROUGE_MultiSentence(t *testing.T) {
	sys := []string{"the cat sat", "dogs are loyal"}
	ref := []string{"cats sit on mats", "dogs are loyal companions"}
	score := EvalROUGE(sys, ref)
	// Just verify scores are in valid range
	for name, f1 := range map[string]float64{
		"ROUGE1": score.ROUGE1.F1,
		"ROUGE2": score.ROUGE2.F1,
		"ROUGEL": score.ROUGEL.F1,
	} {
		if f1 < 0 || f1 > 1 {
			t.Errorf("%s F1 out of [0,1]: %f", name, f1)
		}
	}
}

func TestROUGE_F1IsHarmonicMean(t *testing.T) {
	sys := []string{"the cat sat on the mat"}
	ref := []string{"the cat sat on the rug"}
	score := EvalROUGE(sys, ref)
	// Verify F1 = 2PR/(P+R) for ROUGE-1
	p, r := score.ROUGE1.Precision, score.ROUGE1.Recall
	if p+r == 0 {
		t.Skip("P+R=0, skip harmonic mean check")
	}
	wantF1 := 2 * p * r / (p + r)
	if !approxF(score.ROUGE1.F1, wantF1, 0.0001) {
		t.Errorf("ROUGE-1 F1 = %f, harmonic mean of P/R = %f", score.ROUGE1.F1, wantF1)
	}
}
