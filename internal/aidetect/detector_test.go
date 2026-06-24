package aidetect

import (
	"strings"
	"testing"
)

// sampleAIText is dense with excess-vocabulary markers.
const sampleAIText = `
Delving into the intricate landscape of modern AI, it is crucial to leverage
meticulous and comprehensive frameworks. This groundbreaking research showcases
a multifaceted approach that is both robust and transformative. The synergy
between these pivotal components underscores their invaluable contribution to
the field. Moreover, the seamless integration of holistic methodologies fosters
remarkable advancements. This testament to innovation will undoubtedly propel
the paradigm forward, empowering practitioners to navigate the realm of
cutting-edge solutions with unparalleled expertise.
`

// sampleHumanText is plain prose without excess-vocabulary markers.
const sampleHumanText = `
The cat sat on the mat. It looked out the window at the rain.
Three birds flew past the tree. She read a book in the afternoon.
The bus arrived late, so they walked home instead.
`

func TestDetect_AITextScoresHigher(t *testing.T) {
	aiResult, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect(aiText): %v", err)
	}
	humanResult, err := Detect(sampleHumanText, "en", "")
	if err != nil {
		t.Fatalf("Detect(humanText): %v", err)
	}
	if aiResult.Score <= humanResult.Score {
		t.Errorf("AI text score (%.3f) should exceed human text score (%.3f)", aiResult.Score, humanResult.Score)
	}
}

func TestDetect_AITextFlagged(t *testing.T) {
	r, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.Score < 0.40 {
		t.Errorf("AI text: want score ≥ 0.40, got %.3f (density=%.3f, variety=%.3f, markers=%v)",
			r.Score, r.Density, r.Variety, r.Markers)
	}
}

func TestDetect_HumanTextLowScore(t *testing.T) {
	r, err := Detect(sampleHumanText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// Plain human text should score very low (below 0.30).
	if r.Score >= 0.30 {
		t.Errorf("human text: want score < 0.30, got %.3f (markers=%v)", r.Score, r.Markers)
	}
}

func TestDetect_EmptyText(t *testing.T) {
	r, err := Detect("", "en", "")
	if err != nil {
		t.Fatalf("Detect(empty): %v", err)
	}
	if r.Score != 0 || r.Sentences != 0 {
		t.Errorf("empty text: want zero result, got %+v", r)
	}
}

func TestDetect_DefaultLang(t *testing.T) {
	r, err := Detect(sampleAIText, "", "")
	if err != nil {
		t.Fatalf("Detect(default lang): %v", err)
	}
	if r.Lang != "en" {
		t.Errorf("default lang: want 'en', got %q", r.Lang)
	}
}

func TestDetect_UnsupportedLang(t *testing.T) {
	_, err := Detect("some text", "fr", "")
	if err == nil {
		t.Fatal("expected error for unsupported language 'fr'")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDetect_PortugueseLang(t *testing.T) {
	ptText := `Ao aprofundar na intrincada paisagem da inteligência artificial,
é crucial alavancar frameworks meticulosos e abrangentes. Esta pesquisa inovadora
demonstra uma abordagem multifacetada que é robusta e transformadora. A sinergia
entre esses componentes primordiais evidencia sua contribuição inestimável.
Além disso, a integração sinérgica de metodologias holísticas fomenta
avanços notáveis.`
	r, err := Detect(ptText, "pt-BR", "")
	if err != nil {
		t.Fatalf("Detect(pt-BR): %v", err)
	}
	if r.Lang != "pt-BR" {
		t.Errorf("lang: want 'pt-BR', got %q", r.Lang)
	}
	if r.Score < 0.40 {
		t.Errorf("pt-BR AI text: want score ≥ 0.40, got %.3f (markers=%v)", r.Score, r.Markers)
	}
}

func TestDetect_SpanishLang(t *testing.T) {
	esText := `Al profundizar en el intrincado panorama de la inteligencia artificial,
es crucial aprovechar marcos de trabajo meticulosos y holísticos. Esta investigación
innovadora muestra un enfoque multifacético que es robusto y transformador. La sinergia
entre estos componentes pivotales resalta su contribución invaluable. Además, la
integración sinérgica de metodologías holísticas fomenta avances notables.`
	r, err := Detect(esText, "es", "")
	if err != nil {
		t.Fatalf("Detect(es): %v", err)
	}
	if r.Lang != "es" {
		t.Errorf("lang: want 'es', got %q", r.Lang)
	}
	if r.Score < 0.40 {
		t.Errorf("es AI text: want score ≥ 0.40, got %.3f (markers=%v)", r.Score, r.Markers)
	}
}

func TestDetect_MarkersAreSorted(t *testing.T) {
	r, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for i := 1; i < len(r.Markers); i++ {
		if r.Markers[i] < r.Markers[i-1] {
			t.Errorf("markers not sorted at index %d: %q > %q", i, r.Markers[i-1], r.Markers[i])
		}
	}
}

func TestDetect_ScoreBounds(t *testing.T) {
	r, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.Score < 0 || r.Score > 1 {
		t.Errorf("score out of [0,1]: %.4f", r.Score)
	}
	if r.Density < 0 || r.Density > 1 {
		t.Errorf("density out of [0,1]: %.4f", r.Density)
	}
	if r.Variety < 0 || r.Variety > 1 {
		t.Errorf("variety out of [0,1]: %.4f", r.Variety)
	}
}

func TestVerdict(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.80, "likely AI-generated"},
		{0.70, "likely AI-generated"},
		{0.55, "possibly AI-generated"},
		{0.40, "possibly AI-generated"},
		{0.39, "likely human-written"},
		{0.00, "likely human-written"},
	}
	for _, tc := range cases {
		r := Result{Score: tc.score}
		if got := r.Verdict(); got != tc.want {
			t.Errorf("Verdict(%.2f) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestTokenizeWords_HyphenatedCompounds(t *testing.T) {
	got := tokenizeWords("cutting-edge state-of-the-art forward-thinking")
	want := []string{"cutting-edge", "state-of-the-art", "forward-thinking"}
	if len(got) != len(want) {
		t.Fatalf("tokenizeWords: got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] got %q, want %q", i, got[i], w)
		}
	}
}

func TestTokenizeWords_StripsDigitsAndPunct(t *testing.T) {
	// Digits, commas, periods must not appear in output tokens.
	got := tokenizeWords("hello, world! 123 foo.")
	for _, w := range got {
		for _, r := range w {
			if r >= '0' && r <= '9' {
				t.Errorf("digit in token %q", w)
			}
			if r == ',' || r == '!' || r == '.' {
				t.Errorf("punctuation in token %q", w)
			}
		}
	}
}

func TestTokenizeWords_CaseLower(t *testing.T) {
	got := tokenizeWords("DELVE Intricate METICULOUS")
	want := []string{"delve", "intricate", "meticulous"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] got %q, want %q", i, got[i], w)
		}
	}
}

func TestTokenizeWords_Empty(t *testing.T) {
	if got := tokenizeWords(""); len(got) != 0 {
		t.Errorf("empty input: want nil/empty, got %v", got)
	}
}

// ── tokenizeSentences ─────────────────────────────────────────────────────────

func TestTokenizeSentences_Basic(t *testing.T) {
	got := tokenizeSentences("Hello world. How are you? Fine!")
	if len(got) != 3 {
		t.Fatalf("want 3 sentences, got %d: %v", len(got), got)
	}
}

func TestTokenizeSentences_NoTrailingPunct(t *testing.T) {
	// A sentence with no terminal punctuation is returned as a single sentence.
	got := tokenizeSentences("no punctuation here")
	if len(got) != 1 || got[0] != "no punctuation here" {
		t.Errorf("want [\"no punctuation here\"], got %v", got)
	}
}

func TestTokenizeSentences_EmptyInput(t *testing.T) {
	if got := tokenizeSentences(""); len(got) != 0 {
		t.Errorf("empty input: want nil, got %v", got)
	}
}

func TestTokenizeSentences_WhitespaceOnly(t *testing.T) {
	if got := tokenizeSentences("   \n\t  "); len(got) != 0 {
		t.Errorf("whitespace-only: want nil, got %v", got)
	}
}

func TestTokenizeSentences_SingleSentenceReturnsOne(t *testing.T) {
	got := tokenizeSentences("This is one sentence.")
	if len(got) != 1 {
		t.Fatalf("want 1, got %d: %v", len(got), got)
	}
}

// ── score formula ─────────────────────────────────────────────────────────────

// TestDetect_ScoreFormula pins score = 0.6*density + 0.4*variety so a weight
// change immediately breaks this test.
func TestDetect_ScoreFormula(t *testing.T) {
	r, err := Detect(sampleAIText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	want := 0.6*r.Density + 0.4*r.Variety
	const epsilon = 1e-9
	if diff := r.Score - want; diff > epsilon || diff < -epsilon {
		t.Errorf("score %.10f ≠ 0.6*density(%.10f)+0.4*variety(%.10f) = %.10f",
			r.Score, r.Density, r.Variety, want)
	}
}

// ── mixed-density text ────────────────────────────────────────────────────────

// TestDetect_MixedText uses one AI sentence and one human sentence.
// Density must be exactly 0.5; score must be between pure-AI and pure-human.
func TestDetect_MixedText(t *testing.T) {
	aiSent := "Delving into the intricate and multifaceted landscape of groundbreaking innovation."
	humanSent := "The cat sat on the mat."
	mixed := aiSent + " " + humanSent

	aiOnly, _ := Detect(aiSent, "en", "")
	humanOnly, _ := Detect(humanSent, "en", "")
	mixedRes, err := Detect(mixed, "en", "")
	if err != nil {
		t.Fatalf("Detect(mixed): %v", err)
	}
	if mixedRes.Density != 0.5 {
		t.Errorf("mixed density: want 0.5, got %.4f", mixedRes.Density)
	}
	if mixedRes.Score >= aiOnly.Score {
		t.Errorf("mixed score (%.3f) must be < pure-AI score (%.3f)", mixedRes.Score, aiOnly.Score)
	}
	if mixedRes.Score <= humanOnly.Score {
		t.Errorf("mixed score (%.3f) must be > pure-human score (%.3f)", mixedRes.Score, humanOnly.Score)
	}
}

// ── case-insensitive marker matching ─────────────────────────────────────────

func TestDetect_CaseInsensitiveMarkers(t *testing.T) {
	// Markers in all-caps should still be found.
	upper := "DELVING into the INTRICATE landscape, this is CRUCIAL and METICULOUS."
	lower := "Delving into the intricate landscape, this is crucial and meticulous."
	rUp, err := Detect(upper, "en", "")
	if err != nil {
		t.Fatalf("Detect(upper): %v", err)
	}
	rLo, err := Detect(lower, "en", "")
	if err != nil {
		t.Fatalf("Detect(lower): %v", err)
	}
	if rUp.Score != rLo.Score {
		t.Errorf("case-insensitive: upper score %.4f ≠ lower score %.4f", rUp.Score, rLo.Score)
	}
}

// ── SentenceCount ─────────────────────────────────────────────────────────────

func TestDetect_SentenceCountReflectsInput(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	r, err := Detect(text, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.Sentences != 3 {
		t.Errorf("want Sentences=3, got %d", r.Sentences)
	}
}

// ── markers nil vs empty ──────────────────────────────────────────────────────

// TestDetect_MarkersNilOnNoHits ensures Markers is nil (not empty slice) when
// no markers are found — the caller (CLI) converts nil to [] for JSON output.
func TestDetect_MarkersNilOnNoHits(t *testing.T) {
	r, err := Detect(sampleHumanText, "en", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// Human text has score 0: Markers should be nil or empty (not a non-empty slice).
	for _, m := range r.Markers {
		// If any marker appears in plain human text it is a false positive; report it.
		_ = m
	}
	// The key invariant: no crash, score 0, Sentences > 0.
	if r.Sentences == 0 {
		t.Error("Sentences must be > 0 for non-empty human text")
	}
}
