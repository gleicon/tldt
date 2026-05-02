package formatter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatText_MultiSentence(t *testing.T) {
	result := FormatText([]string{"A.", "B.", "C."})
	expected := "A.\nB.\nC."
	if result != expected {
		t.Errorf("FormatText multi-sentence: got %q, want %q", result, expected)
	}
}

func TestFormatText_Empty(t *testing.T) {
	result := FormatText(nil)
	if result != "" {
		t.Errorf("FormatText nil input: got %q, want %q", result, "")
	}
	result2 := FormatText([]string{})
	if result2 != "" {
		t.Errorf("FormatText empty slice: got %q, want %q", result2, "")
	}
}

func TestFormatJSON_ValidJSON(t *testing.T) {
	meta := SummaryMeta{
		Algorithm:          "lexrank",
		SentencesIn:        2,
		SentencesOut:       1,
		CharsIn:            100,
		CharsOut:           10,
		TokensEstimatedIn:  25,
		TokensEstimatedOut: 3,
		CompressionRatio:   0.12,
	}
	result, err := FormatJSON([]string{"Summary sentence."}, meta)
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}
	if !json.Valid([]byte(result)) {
		t.Errorf("FormatJSON output is not valid JSON: %q", result)
	}
}

func TestFormatJSON_RequiredFields(t *testing.T) {
	meta := SummaryMeta{
		Algorithm:          "lexrank",
		SentencesIn:        2,
		SentencesOut:       1,
		CharsIn:            100,
		CharsOut:           10,
		TokensEstimatedIn:  25,
		TokensEstimatedOut: 3,
		CompressionRatio:   0.12,
	}
	result, err := FormatJSON([]string{"Summary sentence."}, meta)
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("FormatJSON json.Unmarshal failed: %v", err)
	}
	requiredFields := []string{
		"summary", "algorithm", "sentences_in", "sentences_out",
		"chars_in", "chars_out", "tokens_estimated_in", "tokens_estimated_out",
		"compression_ratio",
	}
	for _, field := range requiredFields {
		if _, ok := out[field]; !ok {
			t.Errorf("FormatJSON missing required field: %q", field)
		}
	}
	if algo, ok := out["algorithm"].(string); !ok || algo != "lexrank" {
		t.Errorf("FormatJSON algorithm field: got %v, want %q", out["algorithm"], "lexrank")
	}
}

func TestFormatJSON_NilSummaryBecomesArray(t *testing.T) {
	meta := SummaryMeta{Algorithm: "lexrank"}
	result, err := FormatJSON(nil, meta)
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}
	if !strings.Contains(result, `"summary": []`) {
		t.Errorf("FormatJSON nil sentences: expected %q in output, got %q", `"summary": []`, result)
	}
}

func TestFormatMarkdown_Header(t *testing.T) {
	meta := SummaryMeta{
		Algorithm:        "lexrank",
		CompressionRatio: 0.89,
	}
	result := FormatMarkdown([]string{"First sentence."}, meta)
	if !strings.HasPrefix(result, "<!-- tldt | algorithm: lexrank") {
		t.Errorf("FormatMarkdown header: output does not start with expected header, got %q", result)
	}
}

func TestFormatMarkdown_BlockquotePrefix(t *testing.T) {
	meta := SummaryMeta{Algorithm: "textrank", CompressionRatio: 0.5}
	sentences := []string{"First sentence.", "Second sentence."}
	result := FormatMarkdown(sentences, meta)
	lines := strings.Split(result, "\n")
	// Lines after the header should start with "> " (sentence) or be ">" (blank separator) or empty
	for _, line := range lines[1:] {
		if line == "" || line == ">" {
			continue
		}
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("FormatMarkdown blockquote: line %q does not start with '> '", line)
		}
	}
}

func TestFormatMarkdown_BlankLineBetweenSentences(t *testing.T) {
	meta := SummaryMeta{Algorithm: "lexrank", CompressionRatio: 0.5}
	sentences := []string{"First sentence.", "Second sentence."}
	result := FormatMarkdown(sentences, meta)
	if !strings.Contains(result, ">\n") {
		t.Errorf("FormatMarkdown: expected blank blockquote line ('>\\n') between sentences, got %q", result)
	}
}
