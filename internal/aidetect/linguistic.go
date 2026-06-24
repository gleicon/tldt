// Linguistic/stylometric AI-detection signals.
//
// These complement the Kobak et al. excess-vocabulary method with structural
// features from classical NLP and stylometry research:
//
//   - Sentence length regularity (CV): LLMs produce suspiciously uniform
//     sentence lengths. Kim & Mukherjee (2011) showed burstiness separates
//     human from machine-generated text.
//
//   - Compression ratio (gzip): a model-free proxy for perplexity. LLM text
//     is more predictable and therefore more compressible. Mitchell et al.
//     (2023, DetectGPT) established the perplexity-detection link; compression
//     is a zero-dependency approximation.
//
//   - Discourse connector density: LLMs systematically open sentences with
//     structural transition phrases ("Furthermore,", "In conclusion,") to
//     simulate academic flow. Measured at sentence-initial position only.
//
//   - Type-token ratio (TTR): unique_words / total_words. LLMs recycle a
//     narrower vocabulary per passage than human writers.
//
//   - Hapax ratio: words_appearing_once / unique_words. Human text has more
//     rare one-off words; LLMs reuse a core vocabulary.
//
// All signals are language-agnostic (computed on tokenized surface form).
package aidetect

import (
	"bytes"
	"compress/gzip"
	"math"
	"strings"
)

// LinguisticSignals holds structural/stylometric features of one text.
// Each raw metric is preserved alongside its normalized AI-signal in Score.
type LinguisticSignals struct {
	SentenceLengthCV float64 // σ/μ of per-sentence word counts; lower = more uniform = AI-like
	CompressionRatio float64 // gzip(text)/len(text); lower = more compressible = AI-like
	DiscourseDensity float64 // fraction of sentences starting with a discourse connector
	TypeTokenRatio   float64 // unique_words / total_words; lower = AI-like
	HapaxRatio       float64 // words_once / unique_words; lower = AI-like
	Score            float64 // composite [0,1]: weighted combination of the five signals above
}

// discourseConnectors are sentence-initial transition phrases overused by LLMs
// to simulate academic discourse structure. These are position-sensitive — the
// same word in mid-sentence is not counted.
var discourseConnectors = []string{
	"furthermore,", "moreover,", "additionally,", "consequently,",
	"therefore,", "notably,", "importantly,", "thus,",
	"in conclusion,", "in conclusion.", "in summary,", "in summary.",
	"to summarize,", "to summarize.", "in other words,",
	"in addition,", "as a result,", "to illustrate,",
	"for instance,", "in essence,", "to begin with,",
	"first and foremost,", "last but not least,",
	"it is worth noting", "it is important to note",
	"it should be noted", "as mentioned", "as noted",
	"as previously", "building on this", "with this in mind",
}

// computeLinguistic derives stylometric AI signals from text and its pre-split
// sentences. Returns a zero-value LinguisticSignals when sentences < 3 (too
// few for reliable statistics).
func computeLinguistic(text string, sentences []string) LinguisticSignals {
	if len(sentences) < 3 {
		return LinguisticSignals{}
	}

	cv := sentenceLengthCV(sentences)
	comp := compressionRatio(text)
	disc := discourseDensity(sentences)
	ttr, hap := lexicalDiversity(text)

	// Per-signal AI scores: 1.0 = strongly AI-like, 0.0 = human-like.
	cvSig := clamp01(1.0 - cv/0.60)

	compSig := 0.0
	if len(text) >= 150 { // too short → unreliable
		compSig = clamp01((0.55 - comp) / 0.20)
	}

	discSig := clamp01(disc / 0.30)
	ttrSig := clamp01((0.80 - ttr) / 0.40)
	hapSig := clamp01((0.65 - hap) / 0.35)

	score := 0.25*cvSig + 0.30*compSig + 0.20*discSig + 0.15*ttrSig + 0.10*hapSig

	return LinguisticSignals{
		SentenceLengthCV: cv,
		CompressionRatio: comp,
		DiscourseDensity: disc,
		TypeTokenRatio:   ttr,
		HapaxRatio:       hap,
		Score:            score,
	}
}

// sentenceLengthCV returns the coefficient of variation (σ/μ) of word counts
// across sentences. Returns 0 when fewer than 2 sentences are present.
func sentenceLengthCV(sentences []string) float64 {
	if len(sentences) < 2 {
		return 0
	}
	counts := make([]float64, len(sentences))
	for i, s := range sentences {
		counts[i] = float64(len(tokenizeWords(s)))
	}
	mean := 0.0
	for _, c := range counts {
		mean += c
	}
	mean /= float64(len(counts))
	if mean == 0 {
		return 0
	}
	variance := 0.0
	for _, c := range counts {
		d := c - mean
		variance += d * d
	}
	variance /= float64(len(counts))
	return math.Sqrt(variance) / mean
}

// compressionRatio returns gzip(text) size / len(text).
// Lower = more compressible = more predictable = more AI-like.
func compressionRatio(text string) float64 {
	if len(text) == 0 {
		return 0
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write([]byte(text))
	_ = w.Close()
	return float64(buf.Len()) / float64(len(text))
}

// discourseDensity returns the fraction of sentences that begin with a
// discourse connector phrase. Position-sensitive: only sentence-initial usage
// is counted.
func discourseDensity(sentences []string) float64 {
	if len(sentences) == 0 {
		return 0
	}
	count := 0
	for _, s := range sentences {
		lower := strings.ToLower(strings.TrimSpace(s))
		for _, dc := range discourseConnectors {
			if strings.HasPrefix(lower, dc) {
				count++
				break
			}
		}
	}
	return float64(count) / float64(len(sentences))
}

// lexicalDiversity returns type-token ratio and hapax ratio for text.
func lexicalDiversity(text string) (ttr, hapaxRatio float64) {
	words := tokenizeWords(text)
	if len(words) == 0 {
		return 0, 0
	}
	freq := make(map[string]int, len(words))
	for _, w := range words {
		freq[w]++
	}
	unique := len(freq)
	hapax := 0
	for _, cnt := range freq {
		if cnt == 1 {
			hapax++
		}
	}
	ttr = float64(unique) / float64(len(words))
	if unique > 0 {
		hapaxRatio = float64(hapax) / float64(unique)
	}
	return ttr, hapaxRatio
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
