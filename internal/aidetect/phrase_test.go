package aidetect

import (
	"strings"
	"testing"
)

func hasPhrase(r Result, want string) bool {
	for _, p := range r.Phrases {
		if strings.Contains(p, want) {
			return true
		}
	}
	return false
}

// TestPhraseLiteralMatch: a fixed phrase the word tokenizer cannot catch
// (apostrophes, spaces) is detected against the raw text.
func TestPhraseLiteralMatch(t *testing.T) {
	r, err := Detect("The design is fine. It is important to note the tradeoffs here.", "en", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasPhrase(r, "it is important to note") {
		t.Fatalf("literal phrase not matched; phrases=%v", r.Phrases)
	}
	if r.PhraseSignal <= 0 {
		t.Errorf("phrase signal should be positive, got %.2f", r.PhraseSignal)
	}
}

// TestTemplateMatch: the flagship "not just X, but Y" structural tic.
func TestTemplateMatch(t *testing.T) {
	r, _ := Detect("This tool is not just fast but also reliable in production.", "en", "")
	if len(r.Phrases) == 0 {
		t.Fatal("template 'not just X but Y' not matched")
	}
	if !hasPhrase(r, "not just fast but") {
		t.Errorf("template should report matched text, got %v", r.Phrases)
	}
}

// TestPhraseSignalIsAdditiveAndMonotonic is the core guarantee: adding phrases to
// a text can only raise the score, and text with NO phrases scores exactly as the
// word-only method did. The word density/variety must be untouched.
func TestPhraseSignalMonotonic(t *testing.T) {
	base := "The cat sat on the mat. It rained all day. I bought some bread."
	withPhrase := base + " It is important to note this is not just simple but also clear."

	rBase, _ := Detect(base, "en", "")
	rPhrase, _ := Detect(withPhrase, "en", "")

	if rBase.PhraseSignal != 0 {
		t.Errorf("plain text should have zero phrase signal, got %.2f (phrases=%v)", rBase.PhraseSignal, rBase.Phrases)
	}
	if rPhrase.Score < rBase.Score {
		t.Errorf("phrases lowered the score: base=%.3f withPhrase=%.3f", rBase.Score, rPhrase.Score)
	}
	if rPhrase.PhraseSignal <= 0 {
		t.Error("text with phrases should carry a positive phrase signal")
	}
}

// TestWordOnlyScoreUnchanged guards against the denominator regression: a text
// with only word markers and no phrases must score by the original
// 0.6*density + 0.4*variety formula, unmodified.
func TestWordOnlyScoreUnchanged(t *testing.T) {
	// "delve" and "intricate" are word markers; no phrase or template present.
	text := "We delve into the intricate details of the problem at hand."
	r, _ := Detect(text, "en", "")
	if len(r.Phrases) != 0 {
		t.Fatalf("expected no phrase hits, got %v", r.Phrases)
	}
	want := 0.6*r.Density + 0.4*r.Variety
	if r.Score != want {
		t.Errorf("word-only score %.4f != 0.6*density+0.4*variety %.4f — phrase layer altered the base", r.Score, want)
	}
}

// TestPhraseScoreCap: the phrase signal is bounded so phrases alone cannot pin a
// score at 1.0.
func TestPhraseScoreCap(t *testing.T) {
	if got := phraseScore(100, 100); got != maxPhraseSignal {
		t.Errorf("phraseScore not capped: got %.2f, want %.2f", got, maxPhraseSignal)
	}
	if got := phraseScore(0, 0); got != 0 {
		t.Errorf("no hits should be zero signal, got %.2f", got)
	}
	// A single template outweighs a single fixed phrase.
	if phraseScore(0, 1) <= phraseScore(1, 0) {
		t.Error("one template should signal more than one fixed phrase")
	}
}

// TestPortuguesePhrases: the pt-BR list catches its flagship tells.
func TestPortuguesePhrases(t *testing.T) {
	r, err := Detect("Vale ressaltar que a solução não apenas acelera mas também melhora o resultado.", "pt-BR", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Phrases) < 2 {
		t.Fatalf("expected 'vale ressaltar' phrase and 'não apenas X mas Y' template; got %v", r.Phrases)
	}
}

// TestPhrasesDeduplicated: the same phrase repeated counts once.
func TestPhrasesDeduplicated(t *testing.T) {
	r, _ := Detect("It is important to note X. It is important to note Y. It is important to note Z.", "en", "")
	count := 0
	for _, p := range r.Phrases {
		if p == "it is important to note" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("repeated phrase should count once, got %d", count)
	}
}

// TestMalformedTemplateSkipped: a bad regex pattern must be skipped without
// aborting the scan, and a valid pattern beside it must still match.
func TestMalformedTemplateSkipped(t *testing.T) {
	wl := wordlist{Templates: []string{"(unclosed", "valid.*pattern"}}
	_, templates := matchPhrases("this has a valid then pattern in it", wl)
	if len(templates) != 1 {
		t.Fatalf("expected the valid template to match past the malformed one, got %v", templates)
	}
	if !strings.Contains(templates[0], "valid") {
		t.Errorf("matched text %q should come from the valid pattern", templates[0])
	}
}
