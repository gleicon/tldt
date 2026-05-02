package summarizer

import (
	"strings"
	"testing"
)

// ── TokenizeSentences ────────────────────────────────────────────────────────

func TestTokenizeSentences_Empty(t *testing.T) {
	got := TokenizeSentences("")
	if got != nil {
		t.Errorf("TokenizeSentences(\"\") = %v, want nil", got)
	}
}

func TestTokenizeSentences_WhitespaceOnly(t *testing.T) {
	got := TokenizeSentences("   ")
	if got != nil {
		t.Errorf("TokenizeSentences(whitespace) = %v, want nil", got)
	}
}

func TestTokenizeSentences_Single(t *testing.T) {
	got := TokenizeSentences("Just one sentence.")
	if len(got) != 1 {
		t.Errorf("got %d sentences, want 1", len(got))
	}
}

func TestTokenizeSentences_MultiSentence(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	got := TokenizeSentences(text)
	if len(got) != 3 {
		t.Errorf("got %d sentences from 3-sentence input, want 3", len(got))
	}
}

func TestTokenizeSentences_QuestionMark(t *testing.T) {
	text := "Is this a question? Yes it is. Another sentence here."
	got := TokenizeSentences(text)
	if len(got) < 2 {
		t.Errorf("got %d sentences from question-mark text, want >= 2", len(got))
	}
}

func TestTokenizeSentences_ExclamationMark(t *testing.T) {
	text := "Watch out! Something happened. All is well now."
	got := TokenizeSentences(text)
	if len(got) < 2 {
		t.Errorf("got %d sentences from exclamation text, want >= 2", len(got))
	}
}

func TestTokenizeSentences_NoTerminalPunct(t *testing.T) {
	got := TokenizeSentences("No period at end")
	if len(got) != 1 {
		t.Errorf("got %d sentences, want 1", len(got))
	}
}

func TestTokenizeSentences_ResultNotEmpty(t *testing.T) {
	text := "Alpha. Beta. Gamma."
	got := TokenizeSentences(text)
	for i, s := range got {
		if strings.TrimSpace(s) == "" {
			t.Errorf("sentence[%d] is empty or whitespace", i)
		}
	}
}

func TestTokenizeSentences_MultiParagraph(t *testing.T) {
	text := "First paragraph here.\n\nSecond paragraph here. With two sentences.\n\nThird paragraph."
	got := TokenizeSentences(text)
	if len(got) < 3 {
		t.Errorf("got %d sentences from multi-paragraph input, want >= 3", len(got))
	}
}

// ── tokenizeWords ────────────────────────────────────────────────────────────

func TestTokenizeWords_Basic(t *testing.T) {
	got := tokenizeWords("The Cat sat.")
	want := []string{"the", "cat", "sat"}
	if len(got) != len(want) {
		t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("word[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestTokenizeWords_Empty(t *testing.T) {
	got := tokenizeWords("")
	if len(got) != 0 {
		t.Errorf("got %v, want empty slice", got)
	}
}

func TestTokenizeWords_Lowercase(t *testing.T) {
	got := tokenizeWords("HELLO WORLD")
	for _, w := range got {
		for _, r := range w {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("word %q contains uppercase character", w)
			}
		}
	}
}

func TestTokenizeWords_StripsPunctuation(t *testing.T) {
	got := tokenizeWords("hello, world!")
	for _, w := range got {
		if strings.ContainsAny(w, ",.!?;:") {
			t.Errorf("word %q still contains punctuation", w)
		}
	}
}

func TestTokenizeWords_SkipsBlankTokens(t *testing.T) {
	got := tokenizeWords("  spaces   between  words  ")
	for _, w := range got {
		if strings.TrimSpace(w) == "" {
			t.Errorf("tokenizeWords returned blank token")
		}
	}
}

func TestTokenizeWords_AllPunctuation(t *testing.T) {
	// A string that is only punctuation should yield zero non-empty tokens
	got := tokenizeWords("... !!! ???")
	for _, w := range got {
		if w != "" {
			t.Errorf("expected no non-empty tokens from all-punct input, got %q", w)
		}
	}
}

// ── normalizeWord ────────────────────────────────────────────────────────────

func TestNormalizeWord_Lowercase(t *testing.T) {
	got := normalizeWord("HELLO")
	if got != "hello" {
		t.Errorf("normalizeWord(\"HELLO\") = %q, want \"hello\"", got)
	}
}

func TestNormalizeWord_StripPunctuation(t *testing.T) {
	got := normalizeWord("hello,")
	if got != "hello" {
		t.Errorf("normalizeWord(\"hello,\") = %q, want \"hello\"", got)
	}
}

func TestNormalizeWord_LeadingPunct(t *testing.T) {
	got := normalizeWord("\"word\"")
	if strings.ContainsAny(got, "\"'") {
		t.Errorf("normalizeWord(%q) = %q, expected quotes stripped", "\"word\"", got)
	}
}

func TestNormalizeWord_AlreadyNormal(t *testing.T) {
	got := normalizeWord("hello")
	if got != "hello" {
		t.Errorf("normalizeWord(already normal) = %q, want %q", got, "hello")
	}
}
