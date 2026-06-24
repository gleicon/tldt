package aidetect

import (
	"testing"
)

// ── sentenceLengthCV ──────────────────────────────────────────────────────────

func TestSentenceLengthCV_UniformSentences(t *testing.T) {
	// Identical word counts → zero variance → CV = 0 (maximally AI-like).
	sentences := []string{
		"The cat sat on the mat.",    // 6 words
		"The dog ran in the park.",   // 6 words
		"The bird flew up the tree.", // 6 words
	}
	cv := sentenceLengthCV(sentences)
	if cv > 0.05 {
		t.Errorf("uniform sentences: want CV ≈ 0, got %.4f", cv)
	}
}

func TestSentenceLengthCV_VariedSentences(t *testing.T) {
	// Very different lengths → high CV (human-like).
	sentences := []string{
		"Hi.",
		"This is a somewhat longer sentence with many more words in it than the first one.",
		"Medium length here too.",
		"Another very long sentence that keeps going and going and going with lots and lots of words.",
		"Short.",
	}
	cv := sentenceLengthCV(sentences)
	if cv < 0.40 {
		t.Errorf("varied sentences: want CV > 0.40, got %.4f", cv)
	}
}

func TestSentenceLengthCV_SingleSentence(t *testing.T) {
	if cv := sentenceLengthCV([]string{"Just one sentence here."}); cv != 0 {
		t.Errorf("single sentence: want CV=0, got %.4f", cv)
	}
}

// ── compressionRatio ──────────────────────────────────────────────────────────

func TestCompressionRatio_EmptyString(t *testing.T) {
	if r := compressionRatio(""); r != 0 {
		t.Errorf("empty: want 0, got %.4f", r)
	}
}

func TestCompressionRatio_RepetitiveTextMoreCompressible(t *testing.T) {
	// Repeat the same sentence 60 times — maximally compressible.
	rep := ""
	for i := 0; i < 60; i++ {
		rep += "The quick brown fox jumps over the lazy dog. "
	}
	// Build a long text from 60 distinct sentences with unique vocabulary.
	uniqueSentences := []string{
		"The stock market closed higher after a volatile trading session yesterday.",
		"Scientists discovered a previously unknown species of deep-sea jellyfish.",
		"Quantum entanglement enables faster-than-classical information processing.",
		"Renaissance painters used egg tempera before oil paints became widespread.",
		"Arctic permafrost thaw releases methane accelerating global temperature rise.",
		"Byzantine mosaics survive intact beneath medieval plaster in Istanbul churches.",
		"Echolocation in dolphins involves frequencies beyond human hearing thresholds.",
		"Fermentation transforms grape sugars into ethanol through yeast metabolism.",
		"Geothermal vents host extremophile bacteria thriving without sunlight entirely.",
		"Hieroglyphics encode both phonetic sounds and ideographic meaning simultaneously.",
	}
	varied := ""
	for i := 0; i < 6; i++ {
		for _, s := range uniqueSentences {
			varied += s + " "
		}
	}

	rRep := compressionRatio(rep)
	rVar := compressionRatio(varied)
	if rRep >= rVar {
		t.Errorf("repetitive (%.4f) should compress better than varied (%.4f)", rRep, rVar)
	}
}

func TestCompressionRatio_InRange(t *testing.T) {
	text := "This is a normal sentence. It has some words. Another sentence follows."
	r := compressionRatio(text)
	// For any real text, ratio must be in (0, 2] — gzip adds header overhead on tiny texts.
	if r <= 0 || r > 2.0 {
		t.Errorf("compression ratio out of expected range: %.4f", r)
	}
}

// ── discourseDensity ─────────────────────────────────────────────────────────

func TestDiscourseDensity_LLMStyleText(t *testing.T) {
	sentences := []string{
		"Furthermore, the results demonstrate clear patterns.",
		"Moreover, we observe significant improvements.",
		"In conclusion, the findings support our hypothesis.",
		"Additionally, these results align with prior work.",
	}
	d := discourseDensity(sentences)
	if d < 0.75 {
		t.Errorf("discourse-heavy text: want density ≥ 0.75, got %.3f", d)
	}
}

func TestDiscourseDensity_HumanText(t *testing.T) {
	sentences := []string{
		"The cat sat on the mat.",
		"She read a book in the afternoon.",
		"The bus arrived late.",
		"They walked home instead.",
	}
	d := discourseDensity(sentences)
	if d != 0 {
		t.Errorf("plain human text: want density=0, got %.3f", d)
	}
}

func TestDiscourseDensity_Empty(t *testing.T) {
	if d := discourseDensity(nil); d != 0 {
		t.Errorf("nil input: want 0, got %.3f", d)
	}
}

func TestDiscourseDensity_CaseInsensitive(t *testing.T) {
	upper := []string{"FURTHERMORE, this matters.", "The result holds."}
	lower := []string{"furthermore, this matters.", "The result holds."}
	if discourseDensity(upper) != discourseDensity(lower) {
		t.Error("discourse density must be case-insensitive")
	}
}

// ── lexicalDiversity ─────────────────────────────────────────────────────────

func TestLexicalDiversity_NoRepetition(t *testing.T) {
	// All unique words → TTR = 1.0, hapax = 1.0.
	ttr, hapax := lexicalDiversity("alpha beta gamma delta epsilon")
	if ttr != 1.0 {
		t.Errorf("all-unique: want TTR=1.0, got %.4f", ttr)
	}
	if hapax != 1.0 {
		t.Errorf("all-unique: want hapax=1.0, got %.4f", hapax)
	}
}

func TestLexicalDiversity_FullRepetition(t *testing.T) {
	// Same word repeated → TTR low, hapax = 0.
	ttr, hapax := lexicalDiversity("the the the the the the the the the the")
	if ttr > 0.15 {
		t.Errorf("full repetition: want TTR≈0.1, got %.4f", ttr)
	}
	if hapax != 0 {
		t.Errorf("full repetition: want hapax=0, got %.4f", hapax)
	}
}

func TestLexicalDiversity_Empty(t *testing.T) {
	ttr, hapax := lexicalDiversity("")
	if ttr != 0 || hapax != 0 {
		t.Errorf("empty: want (0,0), got (%.4f,%.4f)", ttr, hapax)
	}
}

// ── computeLinguistic (integration) ──────────────────────────────────────────

func TestComputeLinguistic_AITextHigherScore(t *testing.T) {
	aiSents := tokenizeSentences(sampleAIText)
	humanSents := tokenizeSentences(sampleHumanText)

	aiLing := computeLinguistic(sampleAIText, aiSents)
	humanLing := computeLinguistic(sampleHumanText, humanSents)

	// AI text should score higher on the linguistic composite.
	if aiLing.Score <= humanLing.Score {
		t.Errorf("AI linguistic score (%.3f) should exceed human (%.3f)",
			aiLing.Score, humanLing.Score)
	}
}

func TestComputeLinguistic_TooFewSentencesReturnsZero(t *testing.T) {
	sents := []string{"One sentence only."}
	ling := computeLinguistic("One sentence only.", sents)
	if ling.Score != 0 || ling.SentenceLengthCV != 0 {
		t.Errorf("< 3 sentences: want zero LinguisticSignals, got %+v", ling)
	}
}

func TestComputeLinguistic_ScoreInBounds(t *testing.T) {
	sents := tokenizeSentences(sampleAIText)
	ling := computeLinguistic(sampleAIText, sents)
	if ling.Score < 0 || ling.Score > 1 {
		t.Errorf("linguistic score out of [0,1]: %.4f", ling.Score)
	}
	if ling.TypeTokenRatio < 0 || ling.TypeTokenRatio > 1 {
		t.Errorf("TTR out of [0,1]: %.4f", ling.TypeTokenRatio)
	}
	if ling.HapaxRatio < 0 || ling.HapaxRatio > 1 {
		t.Errorf("hapax ratio out of [0,1]: %.4f", ling.HapaxRatio)
	}
}

// ── CombinedScore ─────────────────────────────────────────────────────────────

func TestCombinedScore_FewSentencesReturnKobak(t *testing.T) {
	// Fewer than 5 sentences → CombinedScore() == Score (Kobak only).
	r := Result{Score: 0.75, Sentences: 3}
	if cs := r.CombinedScore(); cs != 0.75 {
		t.Errorf("< 5 sentences: want CombinedScore=KobakScore=0.75, got %.4f", cs)
	}
}

func TestCombinedScore_BlendedWhenEnoughSentences(t *testing.T) {
	r := Result{
		Score:     0.80,
		Sentences: 10,
		Linguistic: LinguisticSignals{Score: 0.40},
	}
	want := 0.65*0.80 + 0.35*0.40
	got := r.CombinedScore()
	const epsilon = 1e-9
	if diff := got - want; diff > epsilon || diff < -epsilon {
		t.Errorf("CombinedScore() = %.8f, want %.8f (0.65*0.80 + 0.35*0.40)", got, want)
	}
}

func TestCombinedScore_AITextDetect(t *testing.T) {
	r, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.CombinedScore() < 0.40 {
		t.Errorf("AI text CombinedScore=%.3f: want ≥0.40", r.CombinedScore())
	}
}

// ── clamp01 ───────────────────────────────────────────────────────────────────

func TestClamp01(t *testing.T) {
	cases := [][2]float64{{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {2, 1}}
	for _, tc := range cases {
		if got := clamp01(tc[0]); got != tc[1] {
			t.Errorf("clamp01(%.1f) = %.1f, want %.1f", tc[0], got, tc[1])
		}
	}
}
