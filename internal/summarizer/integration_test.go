package summarizer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the repository root directory using the location of this test file.
// This allows the tests to locate test-data/ regardless of working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	// This file is at internal/summarizer/integration_test.go
	// repo root is two levels up
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// filename = .../internal/summarizer/integration_test.go
	// filepath.Dir twice gives repo root
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

func readTestFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "test-data", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read test fixture %s: %v", path, err)
	}
	return string(data)
}

func TestSummarize_WikipediaEn(t *testing.T) {
	text := readTestFile(t, "wikipedia_en.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(wikipedia_en.txt) returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize(wikipedia_en.txt) returned empty slice")
	}
}

func TestSummarize_YoutubeTranscript(t *testing.T) {
	text := readTestFile(t, "youtube_transcript.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(youtube_transcript.txt) returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Summarize(youtube_transcript.txt) returned empty slice")
	}
}

func TestSummarize_LongformDoc(t *testing.T) {
	text := readTestFile(t, "longform_3000.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(longform_3000.txt) returned error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("Summarize(longform_3000.txt, n=5): got %d sentences, want 5 (document has 20+ sentences)", len(result))
	}
}

func TestSummarize_EdgeShort_SilentCap(t *testing.T) {
	// edge_short.txt has exactly 3 sentences.
	// Requesting 5 should return <=3 without error (silent cap per didasy/tldr behavior).
	text := readTestFile(t, "edge_short.txt")
	result, err := Summarize(text, 5)
	if err != nil {
		t.Fatalf("Summarize(edge_short.txt) returned unexpected error: %v", err)
	}
	if len(result) > 3 {
		t.Errorf("Summarize(edge_short.txt, n=5): got %d sentences from a 3-sentence doc; expected <=3", len(result))
	}
	for i, s := range result {
		if s == "" {
			t.Errorf("result[%d] is empty string", i)
		}
	}
}
