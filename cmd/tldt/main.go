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
	"github.com/gleicon/tldt/internal/detector"
	"github.com/gleicon/tldt/internal/fetcher"
	"github.com/gleicon/tldt/internal/formatter"
	"github.com/gleicon/tldt/internal/installer"
	"github.com/gleicon/tldt/internal/sanitizer"
	"github.com/gleicon/tldt/internal/summarizer"
)

func main() {
	filePath := flag.String("f", "", "input file path")
	urlFlag := flag.String("url", "", "URL of a webpage to fetch and summarize")
	algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph|ensemble")
	sentences := flag.Int("sentences", 5, "number of output sentences")
	level := flag.String("level", "", "named preset: aggressive (3)|standard (5)|lite (10)")
	paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
	explain := flag.Bool("explain", false, "print algorithm metrics and per-sentence scores to stderr (debug)")
	noCap := flag.Bool("no-cap", false, "disable 2000-sentence cap (allows O(n^2) processing)")
	format := flag.String("format", "text", "output format: text|json|markdown")
	verbose := flag.Bool("verbose", false, "print token stats to stderr (suppressed by default; use when stderr is not redirected)")
	rouge           := flag.String("rouge", "", "path to reference summary file; prints ROUGE-1/2/L scores to stderr")
	printThreshold    := flag.Bool("print-threshold", false, "print configured hook token threshold to stdout and exit")
	installSkill      := flag.Bool("install-skill", false, "install tldt Claude Code skill and UserPromptSubmit hook")
	skillDir          := flag.String("skill-dir", "", "override skill install directory (default: all detected app dirs)")
	skillTarget       := flag.String("target", "", "install target app: claude|cursor|opencode|agents|all (default: all detected)")
	sanitizeFlag      := flag.Bool("sanitize", false, "strip invisible Unicode and apply NFKC normalization before summarization")
	detectInjection   := flag.Bool("detect-injection", false, "report injection patterns and encoding anomalies to stderr (advisory)")
	injectionThreshold := flag.Float64("injection-threshold", detector.DefaultOutlierThreshold, "outlier score [0,1] above which sentences are flagged")
	detectPII    := flag.Bool("detect-pii", false, "report PII and secret patterns (email, API keys, JWTs, credit cards) to stderr (advisory)")
	sanitizePII  := flag.Bool("sanitize-pii", false, "redact PII in input before summarization; reports redaction count to stderr")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tldt [--print-threshold] [--install-skill [--skill-dir path] [--target app]]")
		fmt.Fprintln(os.Stderr, "       tldt [-f file] [--url url] [-algorithm lexrank|textrank|graph|ensemble] [-sentences N] [--detect-pii] [--sanitize-pii]")
		fmt.Fprintln(os.Stderr, "       tldt [-level aggressive|standard|lite] [-paragraphs N] [-format text|json|markdown]")
		fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
		flag.PrintDefaults()
		os.Exit(2) // POSIX convention
	}
	flag.Parse()

	// Load config file — silent fallback to defaults on any error (CFG-03).
	cfgPath, _ := config.ConfigPath()
	cfg := config.Load(cfgPath)

	// Detect which flags the user explicitly provided (CFG-02).
	// flag.Visit (NOT flag.VisitAll) visits only explicitly-set flags.
	flagsSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

	// --print-threshold: print configured hook token threshold to stdout and exit (D-10, D-12)
	// Prints bare integer only — no label — so hook script can capture it directly.
	if *printThreshold {
		fmt.Println(cfg.Hook.Threshold)
		os.Exit(0)
	}

	// --install-skill: write skill + hook templates and patch settings.json, then exit (D-13, D-16)
	if *installSkill {
		if err := installer.Install(installer.Options{
			SkillDir: *skillDir,
			Target:   *skillTarget,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "install-skill:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

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
	// Validate effectiveFormat — covers both CLI flag and config file paths.
	validFormats := map[string]bool{"text": true, "json": true, "markdown": true}
	if !validFormats[effectiveFormat] {
		fmt.Fprintf(os.Stderr, "unknown --format %q: valid values are text, json, markdown\n", effectiveFormat)
		os.Exit(1)
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

	// --sanitize: strip invisible Unicode and NFKC-normalize before summarization.
	if *sanitizeFlag {
		stripped := sanitizer.SanitizeAll(text)
		if stripped != text {
			if inv := sanitizer.ReportInvisibles(text); len(inv) > 0 {
				fmt.Fprintf(os.Stderr, "sanitize: removed %d invisible codepoint(s)\n", len(inv))
			}
		}
		text = stripped
	}

	// --sanitize-pii: redact PII and secrets before summarization (D-06).
	// Implies detection: redaction count always reported to stderr.
	// --sanitize-pii and --sanitize stack independently (D-07).
	if *sanitizePII {
		redacted, findings := detector.SanitizePII(text)
		count := len(findings)
		fmt.Fprintf(os.Stderr, "pii-detect: %d redaction(s) applied\n", count)
		text = redacted
	}

	// --detect-pii: advisory PII scan; never modifies text or blocks summarization (mirrors SEC-07 contract, D-05).
	// When --sanitize-pii is also set, this block runs on the already-redacted text — findings will be empty
	// (since redaction already replaced matches). This is correct behavior: detection post-redaction is safe.
	if *detectPII {
		findings := detector.DetectPII(text)
		if len(findings) == 0 {
			fmt.Fprintln(os.Stderr, "pii-detect: no findings")
		} else {
			fmt.Fprintf(os.Stderr, "pii-detect: %d finding(s)\n", len(findings))
			for _, f := range findings {
				fmt.Fprintf(os.Stderr, "pii-detect: WARNING — [%s] %s (line %d)\n", f.Pattern, f.Excerpt, f.Sentence)
			}
		}
	}

	// --detect-injection: report pattern, encoding, and invisible-char findings to stderr.
	if *detectInjection {
		if inv := sanitizer.ReportInvisibles(text); len(inv) > 0 {
			fmt.Fprintf(os.Stderr, "injection-detect: %d invisible Unicode codepoint(s) found\n", len(inv))
			for _, r := range inv {
				fmt.Fprintf(os.Stderr, "  offset %d: U+%04X %s (%s)\n", r.Offset, r.Rune, r.Name, r.Category)
			}
		}
		report := detector.Analyze(text)
		if len(report.Findings) > 0 {
			fmt.Fprintf(os.Stderr, "injection-detect: %d finding(s), max confidence %.2f\n", len(report.Findings), report.MaxScore)
			for _, f := range report.Findings {
				fmt.Fprintf(os.Stderr, "  [%s] %s (score=%.2f): %s\n", f.Category, f.Pattern, f.Score, f.Excerpt)
			}
			if report.Suspicious {
				fmt.Fprintln(os.Stderr, "injection-detect: WARNING — input flagged as suspicious")
			}
		} else {
			fmt.Fprintln(os.Stderr, "injection-detect: no findings")
		}
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
	var simMatrix [][]float64 // populated if algorithm supports MatrixSummarizer
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
	} else if ms, ok := s.(summarizer.MatrixSummarizer); ok {
		var err2 error
		result, simMatrix, err2 = ms.SummarizeWithMatrix(text, effectiveSentences)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "summarization failed:", err2)
			os.Exit(1)
		}
	} else {
		var err2 error
		result, err2 = s.Summarize(text, effectiveSentences)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "summarization failed:", err2)
			os.Exit(1)
		}
	}

	// --detect-injection outlier layer: run after summarization to use LexRank matrix.
	if *detectInjection && simMatrix != nil {
		sentences := summarizer.TokenizeSentences(text)
		outliers := detector.DetectOutliers(sentences, simMatrix, *injectionThreshold)
		if len(outliers) > 0 {
			fmt.Fprintf(os.Stderr, "injection-detect: %d outlier sentence(s) above threshold %.2f\n", len(outliers), *injectionThreshold)
			for _, f := range outliers {
				fmt.Fprintf(os.Stderr, "  [outlier] sentence %d (score=%.2f): %s\n", f.Sentence, f.Score, f.Excerpt)
			}
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
			fmt.Println(formatter.FormatText(result))
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
