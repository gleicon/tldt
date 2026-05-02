package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SummaryMeta holds all computed metadata for a summarization run.
// Callers (main.go) populate this from the raw char/token counts.
type SummaryMeta struct {
	Algorithm          string
	SentencesIn        int
	SentencesOut       int
	CharsIn            int
	CharsOut           int
	TokensEstimatedIn  int
	TokensEstimatedOut int
	CompressionRatio   float64
}

// JSONOutput is the struct marshalled for --format json.
// All field names match REQUIREMENTS.md OUT-02 exactly.
type JSONOutput struct {
	Summary            []string `json:"summary"`
	Algorithm          string   `json:"algorithm"`
	SentencesIn        int      `json:"sentences_in"`
	SentencesOut       int      `json:"sentences_out"`
	CharsIn            int      `json:"chars_in"`
	CharsOut           int      `json:"chars_out"`
	TokensEstimatedIn  int      `json:"tokens_estimated_in"`
	TokensEstimatedOut int      `json:"tokens_estimated_out"`
	CompressionRatio   float64  `json:"compression_ratio"`
}

// FormatText returns sentences joined by newlines. Safe to pipe — no metadata.
// Returns empty string for nil/empty input.
func FormatText(sentences []string) string {
	return strings.Join(sentences, "\n")
}

// FormatJSON serialises summary + metadata as indented JSON (OUT-02).
// summary field is always a JSON array, never null (use []string{} not nil).
func FormatJSON(sentences []string, meta SummaryMeta) (string, error) {
	if sentences == nil {
		sentences = []string{}
	}
	out := JSONOutput{
		Summary:            sentences,
		Algorithm:          meta.Algorithm,
		SentencesIn:        meta.SentencesIn,
		SentencesOut:       meta.SentencesOut,
		CharsIn:            meta.CharsIn,
		CharsOut:           meta.CharsOut,
		TokensEstimatedIn:  meta.TokensEstimatedIn,
		TokensEstimatedOut: meta.TokensEstimatedOut,
		CompressionRatio:   meta.CompressionRatio,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatter: JSON marshal failed: %w", err)
	}
	return string(b), nil
}

// FormatMarkdown wraps sentences as a markdown blockquote with an HTML comment
// metadata header (OUT-03). Header format:
//
//	<!-- tldt | algorithm: X | sentences: N | compression: P% -->
//
// Sentences are separated by a blank blockquote line (>).
func FormatMarkdown(sentences []string, meta SummaryMeta) string {
	compressionPct := int(meta.CompressionRatio * 100)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- tldt | algorithm: %s | sentences: %d | compression: %d%% -->\n",
		meta.Algorithm, len(sentences), compressionPct)
	for i, s := range sentences {
		if i > 0 {
			b.WriteString(">\n")
		}
		fmt.Fprintf(&b, "> %s\n", s)
	}
	return b.String()
}
