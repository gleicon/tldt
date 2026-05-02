package summarizer

import "strings"

// F1Score holds precision, recall, and F1 for one ROUGE metric.
type F1Score struct {
	Precision float64
	Recall    float64
	F1        float64
}

// ROUGEScore holds ROUGE-1, ROUGE-2, and ROUGE-L scores.
type ROUGEScore struct {
	ROUGE1 F1Score
	ROUGE2 F1Score
	ROUGEL F1Score
}

// EvalROUGE computes ROUGE-1, ROUGE-2, and ROUGE-L between system sentences
// and reference sentences. Both inputs are joined into a single string for scoring.
func EvalROUGE(system, reference []string) ROUGEScore {
	sysToks := tokenizeWords(strings.Join(system, " "))
	refToks := tokenizeWords(strings.Join(reference, " "))
	return ROUGEScore{
		ROUGE1: ngramF1(sysToks, refToks, 1),
		ROUGE2: ngramF1(sysToks, refToks, 2),
		ROUGEL: lcsSimilarity(sysToks, refToks),
	}
}

// ngramF1 computes precision, recall, and F1 for n-gram overlap.
// Precondition: sysGrams/refGrams non-empty iff len(tokens) >= n, so
// sysTotal/refTotal are always > 0 when we reach that point.
func ngramF1(sys, ref []string, n int) F1Score {
	sysGrams := ngramCounts(sys, n)
	refGrams := ngramCounts(ref, n)

	if len(refGrams) == 0 || len(sysGrams) == 0 {
		return F1Score{}
	}

	overlap := 0
	for g, sc := range sysGrams {
		if rc, ok := refGrams[g]; ok {
			if sc < rc {
				overlap += sc
			} else {
				overlap += rc
			}
		}
	}

	sysTotal := len(sys) - n + 1
	refTotal := len(ref) - n + 1

	p := float64(overlap) / float64(sysTotal)
	r := float64(overlap) / float64(refTotal)
	f1 := 0.0
	if p+r > 0 {
		f1 = 2 * p * r / (p + r)
	}
	return F1Score{Precision: p, Recall: r, F1: f1}
}

// ngramCounts returns a count map of all n-grams in tokens.
func ngramCounts(tokens []string, n int) map[string]int {
	counts := make(map[string]int)
	for i := 0; i+n <= len(tokens); i++ {
		g := strings.Join(tokens[i:i+n], " ")
		counts[g]++
	}
	return counts
}

// lcsSimilarity computes ROUGE-L F1 using the longest common subsequence length.
func lcsSimilarity(sys, ref []string) F1Score {
	if len(sys) == 0 || len(ref) == 0 {
		return F1Score{}
	}
	lcsLen := lcs(sys, ref)
	p := float64(lcsLen) / float64(len(sys))
	r := float64(lcsLen) / float64(len(ref))
	f1 := 0.0
	if p+r > 0 {
		f1 = 2 * p * r / (p + r)
	}
	return F1Score{Precision: p, Recall: r, F1: f1}
}

// lcs returns the length of the longest common subsequence of a and b.
// Uses O(min(m,n)) space via two-row DP.
func lcs(a, b []string) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	m := len(a)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for _, bw := range b {
		for j, aw := range a {
			if aw == bw {
				curr[j+1] = prev[j] + 1
			} else if prev[j+1] > curr[j] {
				curr[j+1] = prev[j+1]
			} else {
				curr[j+1] = curr[j]
			}
		}
		prev, curr = curr, prev
		for i := range curr {
			curr[i] = 0
		}
	}
	return prev[m]
}
