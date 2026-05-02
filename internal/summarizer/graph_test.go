package summarizer

import (
	"strings"
	"testing"
)

// tenSentenceText is a simple 10-sentence English text for testing.
const tenSentenceText = `The quick brown fox jumps over the lazy dog.
Pack my box with five dozen liquor jugs.
How vexingly quick daft zebras jump.
The five boxing wizards jump quickly.
Sphinx of black quartz, judge my vow.
Jackdaws love my big sphinx of quartz.
The jay, pig, fox, zebra and my wolves quack.
Blowzy red vixens fight for a quick jump.
Amazingly few discotheques provide juicy bass.
Heavy boxes perform quick waltzes and jigs.`

// threeSentenceText tests the silent-cap behavior when n > sentence count.
const threeSentenceText = `This is the first sentence.
This is the second sentence.
This is the third and final sentence.`

func TestSummarize_ReturnsNonEmpty(t *testing.T) {
	result, err := Summarize(tenSentenceText, 3)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize returned empty slice for multi-sentence input")
	}
}

func TestSummarize_RespectsNLimit(t *testing.T) {
	result, err := Summarize(tenSentenceText, 2)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error: %v", err)
	}
	if len(result) > 2 {
		t.Errorf("Summarize returned %d sentences but n=2 was requested", len(result))
	}
}

func TestSummarize_SilentCapOnShortInput(t *testing.T) {
	// threeSentenceText has 3 sentences; requesting 10 should return <=3, no error
	result, err := Summarize(threeSentenceText, 10)
	if err != nil {
		t.Fatalf("Summarize returned unexpected error for n > sentence count: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Summarize returned %d sentences from a 3-sentence input", len(result))
	}
}

func TestSummarize_ResultContainsRealSentences(t *testing.T) {
	result, err := Summarize(threeSentenceText, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range result {
		s = strings.TrimSpace(s)
		if s == "" {
			t.Error("Summarize returned an empty string in result slice")
		}
	}
}
