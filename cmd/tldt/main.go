package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gleicon/tldt/internal/config"
	"github.com/gleicon/tldt/internal/fetcher"
	"github.com/gleicon/tldt/internal/formatter"
	"github.com/gleicon/tldt/internal/summarizer"
)

func main() {
	filePath := flag.String("f", "", "input file path")
	urlFlag := flag.String("url", "", "URL of a webpage to fetch and summarize")
	algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph|ensemble")
	sentences := flag.Int("sentences", 5, "number of output sentences")
	level := flag.String("level", "", "named preset: lite (3)|standard (5)|aggressive (10)")
	paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
	explain := flag.Bool("explain", false, "print algorithm metrics and per-sentence scores to stderr (debug)")
	noCap := flag.Bool("no-cap", false, "disable 2000-sentence cap (allows O(n^2) processing)")
	format := flag.String("format", "text", "output format: text|json|markdown")
	verbose := flag.Bool("verbose", false, "print token stats to stderr (suppressed by default; use when stderr is not redirected)")
	rouge := flag.String("rouge", "", "path to reference summary file; prints ROUGE-1/2/L scores to stderr")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [-url url] [-algorithm lexrank|textrank|graph|ensemble] [-sentences N] [-level lite|standard|aggressive] [-paragraphs N] [-explain] [-no-cap] [-format text|json|markdown] [-verbose] [-rouge ref.txt] [text...]")
		fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	// Load config file — silent fallback to defaults on any error (CFG-03).
	cfgPath, _ := config.ConfigPath()
	cfg := config.Load(cfgPath)

	// Detect which flags the user explicitly provided (CFG-02).
	// flag.Visit (NOT flag.VisitAll) visits only explicitly-set flags.
	flagsSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

	// Resolve effective parameters: config -> level preset -> explicit flags.
	effectiveAlgorithm := cfg.Algorithm
	effectiveSentences := cfg.Sentences
	effectiveFormat := cfg.Format
	effectiveLevel := cfg.Level

	// --level flag overrides config level (CFG-04).
	if flagsSet["level"] {
		effectiveLevel = *level
	}
	// Validate --level value if set (Pitfall 5 from research).
	if effectiveLevel != "" {
		if n, ok := config.LevelPresets[effectiveLevel]; ok {
			effectiveSentences = n
		} else {
			fmt.Fprintf(os.Stderr, "unknown --level %q: valid values are lite, standard, aggressive\n", effectiveLevel)
			os.Exit(1)
		}
	}
	// Explicit --sentences always wins over level preset (CFG-02, CFG-05).
	if flagsSet["sentences"] {
		effectiveSentences = *sentences
	}
	// Explicit --algorithm and --format override config values (CFG-02).
	if flagsSet["algorithm"] {
		effectiveAlgorithm = *algorithm
	}
	if flagsSet["format"] {
		effectiveFormat = *format
	}

	rawBytes, err := resolveInputBytes(flag.Args(), *filePath, *urlFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	text, isEmpty, err := validateInput(rawBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if isEmpty {
		os.Exit(0)
	}

	const defaultSentenceCap = 2000
	if !*noCap {
		text = applySentenceCap(text, defaultSentenceCap)
	}

	s, err := summarizer.New(effectiveAlgorithm)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	charsIn := len(text)
	var result []string
	if *explain {
		if ex, ok := s.(summarizer.Explainer); ok {
			var info *summarizer.ExplainInfo
			var err2 error
			result, info, err2 = ex.SummarizeExplain(text, effectiveSentences)
			if err2 != nil {
				fmt.Fprintln(os.Stderr, "summarization failed:", err2)
				os.Exit(1)
			}
			if info != nil {
				fmt.Fprint(os.Stderr, info.Format())
			}
		} else {
			// Graph or future algorithms without Explainer: fall back to normal summarize
			fmt.Fprintf(os.Stderr, "note: --explain not supported for algorithm %q; running without diagnostics\n", effectiveAlgorithm)
			var err2 error
			result, err2 = s.Summarize(text, effectiveSentences)
			if err2 != nil {
				fmt.Fprintln(os.Stderr, "summarization failed:", err2)
				os.Exit(1)
			}
		}
	} else {
		var err2 error
		result, err2 = s.Summarize(text, effectiveSentences)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "summarization failed:", err2)
			os.Exit(1)
		}
	}

	// ROUGE evaluation against reference file (if --rouge provided)
	if *rouge != "" {
		refData, err := os.ReadFile(*rouge)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rouge: cannot read reference file:", err)
			os.Exit(1)
		}
		refSents := summarizer.TokenizeSentences(string(refData))
		scores := summarizer.EvalROUGE(result, refSents)
		fmt.Fprintf(os.Stderr, "rouge-1  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGE1.Precision, scores.ROUGE1.Recall, scores.ROUGE1.F1)
		fmt.Fprintf(os.Stderr, "rouge-2  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGE2.Precision, scores.ROUGE2.Recall, scores.ROUGE2.F1)
		fmt.Fprintf(os.Stderr, "rouge-l  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGEL.Precision, scores.ROUGEL.Recall, scores.ROUGEL.F1)
	}

	// Token stats to stderr (TOK-01, TOK-02, TOK-03, D-09, D-10)
	charsOut := len(strings.Join(result, " "))
	tokIn := charsIn / 4
	tokOut := charsOut / 4
	reduction := 0
	if tokIn > 0 {
		reduction = int(float64(tokIn-tokOut) / float64(tokIn) * 100)
	}
	if *verbose && effectiveFormat != "json" {
		fmt.Fprintf(os.Stderr, "~%s -> ~%s tokens (%d%% reduction)\n",
			formatTokens(tokIn), formatTokens(tokOut), reduction)
	}

	// Build metadata for structured formats
	meta := formatter.SummaryMeta{
		Algorithm:          effectiveAlgorithm,
		SentencesIn:        len(summarizer.TokenizeSentences(text)),
		SentencesOut:       len(result),
		CharsIn:            charsIn,
		CharsOut:           charsOut,
		TokensEstimatedIn:  tokIn,
		TokensEstimatedOut: tokOut,
		CompressionRatio:   float64(tokIn-tokOut) / float64(tokIn+1), // +1 guards divide-by-zero
	}

	switch effectiveFormat {
	case "json":
		out, err := formatter.FormatJSON(result, meta)
		if err != nil {
			fmt.Fprintln(os.Stderr, "format error:", err)
			os.Exit(1)
		}
		fmt.Println(out)
	case "markdown":
		fmt.Print(formatter.FormatMarkdown(result, meta))
	default: // "text" and anything unrecognised
		if *paragraphs > 0 {
			fmt.Println(groupIntoParagraphs(result, *paragraphs))
		} else {
			fmt.Println(strings.Join(result, "\n"))
		}
	}
}

func formatTokens(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteByte(',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

func groupIntoParagraphs(sentences []string, n int) string {
	if n <= 0 || len(sentences) == 0 {
		return strings.Join(sentences, "\n")
	}
	if n > len(sentences) {
		n = len(sentences) // D-06: silent cap
	}
	size := len(sentences) / n
	rem := len(sentences) % n
	var b strings.Builder
	start := 0
	for i := 0; i < n; i++ {
		end := start + size
		if i < rem {
			end++
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.Join(sentences[start:end], "\n"))
		start = end
	}
	return b.String()
}

// resolveInputBytes reads raw input bytes from --url, stdin pipe, -f file, or positional args.
func resolveInputBytes(args []string, filePath string, urlStr string) ([]byte, error) {
	// --url branch: highest priority — most explicit input source (INP-01, INP-02)
	if urlStr != "" {
		text, err := fetcher.Fetch(urlStr, 30*time.Second, 5<<20) // 5MB cap
		if err != nil {
			return nil, fmt.Errorf("fetching URL: %w", err)
		}
		return []byte(text), nil
	}
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return data, nil
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading file %q: %w", filePath, err)
		}
		return data, nil
	}
	if len(args) > 0 {
		return []byte(strings.Join(args, " ")), nil
	}
	return nil, fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
}

// validateInput checks raw input bytes for binary content and whitespace-only input.
// Returns (text, isEmpty, error).
// isEmpty==true means the caller must exit 0 with no output.
// error != nil means binary input detected; caller must print error to stderr and exit 1.
func validateInput(data []byte) (string, bool, error) {
	if bytes.IndexByte(data, 0) >= 0 {
		return "", false, fmt.Errorf("binary input: NUL byte found")
	}
	if !utf8.Valid(data) {
		return "", false, fmt.Errorf("binary input: invalid UTF-8 encoding")
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return "", true, nil
	}
	return text, false, nil
}

// applySentenceCap limits text to at most cap sentences to prevent O(n^2) hang.
// Returns text unchanged if sentence count is within cap.
func applySentenceCap(text string, cap int) string {
	sents := summarizer.TokenizeSentences(text)
	if len(sents) <= cap {
		return text
	}
	return strings.Join(sents[:cap], " ")
}
