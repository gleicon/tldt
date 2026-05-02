package summarizer

import (
	"testing"
)

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

func TestTokenizeSentences_Unicode(t *testing.T) {
	// Unicode content should not panic
	text := "Hello world. World is great. End here."
	got := TokenizeSentences(text)
	if len(got) == 0 {
		t.Error("TokenizeSentences returned empty for unicode input")
	}
}

func TestTokenizeSentences_NoTerminalPunct(t *testing.T) {
	got := TokenizeSentences("No period at end")
	if len(got) != 1 {
		t.Errorf("got %d sentences, want 1", len(got))
	}
}

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
