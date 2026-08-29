package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tldt "github.com/gleicon/tldt/pkg/tldt"

	"github.com/gleicon/tldt/internal/config"
	"github.com/gleicon/tldt/internal/extractor"
	"github.com/gleicon/tldt/internal/formatter"
	"github.com/gleicon/tldt/internal/installer"
	usagelog "github.com/gleicon/tldt/internal/usage"
)

var version = "dev" // set at build time via -ldflags "-X main.version=vX.Y.Z"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats(os.Args[2:])
		return
	}

	versionFlag := flag.Bool("version", false, "print version and exit")
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
	rouge := flag.String("rouge", "", "path to reference summary file; prints ROUGE-1/2/L scores to stderr")
	installSkill := flag.Bool("install-skill", false, "install tldt Claude Code skill and UserPromptSubmit hook")
	skillDir := flag.String("skill-dir", "", "override skill install directory (default: all detected app dirs)")
	skillTarget := flag.String("target", "", "install target app: claude|codex|cursor|opencode|agents|all (default: all detected)")
	configDir := flag.String("config-dir", "", "override Claude config dir base (precedence: --config-dir > $CLAUDE_CONFIG_DIR > ~/.claude)")
	projectInstall := flag.Bool("project", false, "install repo-locally under ./.claude/ and register the hook in .claude/settings.local.json")
	sanitizeFlag := flag.Bool("sanitize", false, "strip invisible Unicode and apply NFKC normalization before summarization")
	detectInjection := flag.Bool("detect-injection", false, "report injection patterns and encoding anomalies to stderr (advisory)")
	detectOnly := flag.Bool("detect-only", false, "run requested detection stages then exit before summarizing (no summary, no usage log)")
	hookOutput := flag.Bool("hook-output", false, "UserPromptSubmit hook mode: read the {prompt} stdin envelope, detect injection+PII, emit a metadata-only advisory envelope when flagged (else nothing)")
	injectionThreshold := flag.Float64("injection-threshold", tldt.DefaultOutlierThreshold, "outlier score [0,1] above which sentences are flagged")
	detectPII := flag.Bool("detect-pii", false, "report PII and secret patterns (emails, API keys, tokens, private keys, JWTs, SSNs, credit cards) to stderr (advisory)")
	sanitizePII := flag.Bool("sanitize-pii", false, "redact PII in input before summarization; reports redaction count to stderr")
	fromHTML := flag.Bool("from-html", false, "convert HTML input to Markdown before summarization (uses readability + html-to-markdown)")
	detectAI := flag.Bool("detect-ai", false, "score text for AI-generated content using excess-vocabulary method (Kobak et al. 2024, arXiv:2406.07016)")
	aiLang := flag.String("lang", "en", "language for AI detection wordlist: en, pt-BR, es")
	aiWordlistDir := flag.String("wordlist-dir", "", "directory with custom <lang>.json wordlist files (overrides embedded lists)")
	detectExfil := flag.Bool("detect-exfil", false, "report Markdown links and images shaped to carry data outward (advisory)")
	detectPositional := flag.Bool("detect-positional", false, "report instructions placed after whitespace gaps or in the document tail (weak-prior, advisory)")
	detectScript := flag.Bool("detect-script-mismatch", false, "report sentences whose dominant Unicode script differs from the document (weak-prior, advisory)")
	foldObfuscation := flag.Bool("fold-obfuscation", false, "match injection phrases through leetspeak and character substitutions (advisory)")
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("tldt " + version)
		return
	}

	// Load config file — silent fallback to defaults on any error.
	cfgPath, _ := config.ConfigPath()
	cfg := config.Load(cfgPath)

	flagsSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

	if *installSkill {
		if err := installer.Install(installer.Options{
			SkillDir:  *skillDir,
			Target:    *skillTarget,
			ConfigDir: *configDir,
			Project:   *projectInstall,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "install-skill:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// fails safe: always exits 0
	if *hookOutput {
		runHookOutput(*injectionThreshold)
		return
	}

	effectiveAlgorithm, effectiveSentences, effectiveFormat := resolveSettings(
		cfg, flagsSet, *level, *algorithm, *format, *sentences)

	rawBytes, hiddenSurfaces, err := resolveInputBytes(flag.Args(), *filePath, *urlFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	secOpts := resolveSecurityOpts(cfg, flagsSet,
		*fromHTML, *sanitizeFlag, *sanitizePII, *detectPII,
		*detectInjection, *injectionThreshold,
		*detectAI, *aiLang, *aiWordlistDir)
	if flagsSet["detect-exfil"] {
		secOpts.detectExfil = *detectExfil
	}
	if flagsSet["detect-positional"] {
		secOpts.detectPositional = *detectPositional
	}
	if flagsSet["detect-script-mismatch"] {
		secOpts.detectScript = *detectScript
	}
	if flagsSet["fold-obfuscation"] {
		secOpts.foldObfuscation = *foldObfuscation
	}

	// HTML comment injection check runs before the empty-text guard so that
	// JS SPAs (no readable body text, but comments carrying injection payloads)
	// still get scanned when --detect-injection is set.
	if secOpts.detectInjection && len(hiddenSurfaces) > 0 {
		reportHiddenSurfaces(hiddenSurfaces, secOpts)
	}

	text, isEmpty, err := validateInput(rawBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if isEmpty {
		os.Exit(0)
	}

	// Detection runs on the original bytes, before any mutating stage. A
	// sanitizer that strips zero-width characters or the Unicode Tags block would
	// otherwise destroy exactly the payload the decoder is meant to reconstruct,
	// and redaction would erase spans the detectors need to see. Mutation is a
	// separate concern on the path to the summarizer.
	//
	// --detect-only --format json emits the structured detection contract to
	// stdout and exits. Machine consumers (the OpenCode plugin) read this instead
	// of parsing stderr prose.
	if *detectOnly && effectiveFormat == "json" {
		emitDetectJSON(text, secOpts)
	}
	runDetectionStderr(text, secOpts)

	if *detectOnly {
		os.Exit(0)
	}

	text = applyMutatingStages(text, secOpts)

	const defaultSentenceCap = 2000
	if !*noCap {
		text = applySentenceCap(text, defaultSentenceCap)
	}

	charsIn := len(text)
	result := summarize(effectiveAlgorithm, text, effectiveSentences, *explain)

	// ROUGE evaluation against reference file (if --rouge provided)
	if *rouge != "" {
		refData, err := os.ReadFile(*rouge)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rouge: cannot read reference file:", err)
			os.Exit(1)
		}
		refSents := tldt.TokenizeSentences(string(refData))
		scores := tldt.EvalROUGE(result, refSents)
		fmt.Fprintf(os.Stderr, "rouge-1  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGE1.Precision, scores.ROUGE1.Recall, scores.ROUGE1.F1)
		fmt.Fprintf(os.Stderr, "rouge-2  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGE2.Precision, scores.ROUGE2.Recall, scores.ROUGE2.F1)
		fmt.Fprintf(os.Stderr, "rouge-l  P=%.4f R=%.4f F1=%.4f\n", scores.ROUGEL.Precision, scores.ROUGEL.Recall, scores.ROUGEL.F1)
	}

	// Token stats to stderr.
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
		SentencesIn:        len(tldt.TokenizeSentences(text)),
		SentencesOut:       len(result),
		CharsIn:            charsIn,
		CharsOut:           charsOut,
		TokensEstimatedIn:  tokIn,
		TokensEstimatedOut: tokOut,
		CompressionRatio:   float64(tokIn-tokOut) / float64(tokIn+1), // +1 guards divide-by-zero
	}

	writeOutput(effectiveFormat, result, meta, *paragraphs)

	// A log-write failure must never affect stdout or exit code — so errors are dropped.
	if cfg.Stats.Enabled {
		if logPath, err := usagelog.Path(); err == nil {
			_, statErr := os.Stat(logPath)
			firstWrite := os.IsNotExist(statErr)
			if appendErr := usagelog.Append(logPath, usagelog.Record{
				TS:    time.Now().Format(time.RFC3339),
				In:    tokIn,
				Out:   tokOut,
				Saved: tokIn - tokOut,
			}); appendErr == nil && firstWrite {
				// One-time consent notice on first log creation. Counts only; the
				// user can opt out without ever having content recorded.
				fmt.Fprintf(os.Stderr, "tldt: usage stats now logged to %s (counts only — no content). "+
					"Disable with [stats] enabled = false in ~/.tldt.toml; clear with tldt stats --reset\n", logPath)
			}
		}
	}
}

// resolveSettings merges the effective algorithm, sentence count, and output
// resolveSecurityOpts merges config defaults with explicit CLI flags.
// Config values apply when the corresponding flag was not explicitly set.
func resolveSecurityOpts(
	cfg config.Config, flagsSet map[string]bool,
	fromHTML, sanitize, sanitizePII, detectPII, detectInjection bool,
	injectionThreshold float64,
	detectAI bool, aiLang, aiWordlistDir string,
) securityOpts {
	o := securityOpts{
		fromHTML:           fromHTML,
		sanitize:           cfg.Security.Sanitize,
		sanitizePII:        cfg.Security.SanitizePII,
		detectPII:          cfg.Security.DetectPII,
		detectInjection:    cfg.Security.DetectInjection,
		injectionThreshold: cfg.Security.InjectionThreshold,
		detectAI:           cfg.AIDetection.Enabled,
		aiLang:             cfg.AIDetection.Lang,
		aiWordlistDir:      cfg.AIDetection.WordlistDir,
		detectExfil:        cfg.Security.DetectExfil,
		detectPositional:   cfg.Security.DetectPositional,
		detectScript:       cfg.Security.DetectScript,
		foldObfuscation:    cfg.Security.FoldObfuscation,
	}
	if flagsSet["sanitize"] {
		o.sanitize = sanitize
	}
	if flagsSet["sanitize-pii"] {
		o.sanitizePII = sanitizePII
	}
	if flagsSet["detect-pii"] {
		o.detectPII = detectPII
	}
	if flagsSet["detect-injection"] {
		o.detectInjection = detectInjection
	}
	if flagsSet["injection-threshold"] {
		o.injectionThreshold = injectionThreshold
	}
	if flagsSet["detect-ai"] {
		o.detectAI = detectAI
	}
	if flagsSet["lang"] {
		o.aiLang = aiLang
	}
	if flagsSet["wordlist-dir"] {
		o.aiWordlistDir = aiWordlistDir
	}
	return o
}

// format from config, an optional level preset, and explicit flags (flags win).
// It validates the result and exits the process on an unknown level/format or a
// non-positive sentence count.
func resolveSettings(cfg config.Config, flagsSet map[string]bool, level, algorithm, format string, sentences int) (algo string, n int, outFormat string) {
	algo = cfg.Algorithm
	n = cfg.Sentences
	outFormat = cfg.Format
	effectiveLevel := cfg.Level

	// --level flag overrides config level.
	if flagsSet["level"] {
		effectiveLevel = level
	}
	if effectiveLevel != "" {
		preset, ok := config.LevelPresets[effectiveLevel]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown --level %q: valid values are lite, standard, aggressive\n", effectiveLevel)
			os.Exit(1)
		}
		n = preset
	}
	// Explicit flags always win over config/level preset.
	if flagsSet["sentences"] {
		n = sentences
	}
	if flagsSet["algorithm"] {
		algo = algorithm
	}
	if flagsSet["format"] {
		outFormat = format
	}

	// Validate format — covers both CLI flag and config file paths.
	validFormats := map[string]bool{"text": true, "json": true, "markdown": true}
	if !validFormats[outFormat] {
		fmt.Fprintf(os.Stderr, "unknown --format %q: valid values are text, json, markdown\n", outFormat)
		os.Exit(1)
	}
	// Sentence count must be positive — covers CLI flag, level preset, and config.
	if n < 1 {
		fmt.Fprintf(os.Stderr, "--sentences must be >= 1 (got %d)\n", n)
		os.Exit(1)
	}
	return algo, n, outFormat
}

// applyMutatingStages applies the requested text-modifying stages —
// HTML-to-Markdown conversion, Unicode sanitization, and PII redaction — in
// order, reporting each to stderr, and returns the possibly-modified text.
// Exits the process on HTML conversion failure.
func applyMutatingStages(text string, o securityOpts) string {
	// --from-html: convert HTML to Markdown before processing.
	if o.fromHTML {
		converted, err := tldt.ConvertHTML(text, tldt.HTMLConvertOptions{
			ExtractContent: true,
			IncludeTitle:   true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "html-convert: %v\n", err)
			os.Exit(1)
		}
		srcLen := len(text)
		dstLen := len(converted)
		reduction := 0
		if srcLen > 0 {
			reduction = (srcLen - dstLen) * 100 / srcLen
		}
		fmt.Fprintf(os.Stderr, "html-convert: %d → %d bytes (%d%% reduction)\n", srcLen, dstLen, reduction)
		text = converted
	}

	// --sanitize: strip invisible Unicode and NFKC-normalize before summarization.
	if o.sanitize {
		stripped := tldt.SanitizeAll(text)
		if stripped != text {
			if inv := tldt.ReportInvisibles(text); len(inv) > 0 {
				fmt.Fprintf(os.Stderr, "sanitize: removed %d invisible codepoint(s)\n", len(inv))
			}
		}
		text = stripped
	}

	// --sanitize-pii: redact PII and secrets before summarization. Implies
	// detection: redaction count always reported. Stacks with --sanitize.
	if o.sanitizePII {
		redacted, findings := tldt.SanitizePII(text)
		fmt.Fprintf(os.Stderr, "pii-detect: %d redaction(s) applied\n", len(findings))
		text = redacted
	}
	return text
}

// runDetectionStderr runs the advisory PII and injection detectors on text and
// reports findings to stderr. It never modifies text. A no-op when neither
// detect flag is set. Exits the process on a detector failure.
func runDetectionStderr(text string, o securityOpts) {
	// --detect-pii: advisory PII scan; never modifies text. When --sanitize-pii is
	// also set this runs on already-redacted text, so findings will be empty.
	if o.detectPII {
		findings := tldt.DetectPII(text)
		if len(findings) == 0 {
			fmt.Fprintln(os.Stderr, "pii-detect: no findings")
		} else {
			fmt.Fprintf(os.Stderr, "pii-detect: %d finding(s)\n", len(findings))
			for _, f := range findings {
				fmt.Fprintf(os.Stderr, "pii-detect: WARNING — [%s] %s (line %d)\n", f.Pattern, f.Excerpt, f.Line)
			}
		}
	}

	// --detect-injection: report pattern, encoding, and invisible-char findings.
	if o.detectInjection {
		reportInjection(text, o)
	}

	// --detect-ai: excess-vocabulary AI content scoring (Kobak et al. 2024).
	if o.detectAI {
		reportAIDetection(text, o.aiLang, o.aiWordlistDir)
	}
}

// reportAIDetection scores text for AI-generated content and writes to stderr.
func reportAIDetection(text, lang, wordlistDir string) {
	r, err := tldt.DetectAI(text, lang, wordlistDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ai-detect:", err)
		os.Exit(1)
	}
	combined := r.CombinedScore()
	fmt.Fprintf(os.Stderr, "ai-detect: score=%.3f (kobak=%.3f, linguistic=%.3f) [%s] — %s\n",
		combined, r.Score, r.Linguistic.Score, r.Lang, r.Verdict())
	if r.Sentences >= 3 {
		ling := r.Linguistic
		fmt.Fprintf(os.Stderr, "ai-detect: linguistic: cv=%.3f comp=%.3f disc=%.3f ttr=%.3f hapax=%.3f\n",
			ling.SentenceLengthCV, ling.CompressionRatio, ling.DiscourseDensity,
			ling.TypeTokenRatio, ling.HapaxRatio)
	}
	if len(r.Markers) > 0 {
		fmt.Fprintf(os.Stderr, "ai-detect: %d excess-vocabulary marker(s): %s\n",
			len(r.Markers), strings.Join(r.Markers, ", "))
	}
	if len(r.Phrases) > 0 {
		fmt.Fprintf(os.Stderr, "ai-detect: %d phrase/template tell(s) (+%.2f signal): %s\n",
			len(r.Phrases), r.PhraseSignal, strings.Join(r.Phrases, " | "))
	}
	if combined >= 0.40 {
		fmt.Fprintln(os.Stderr, "ai-detect: WARNING — text may be AI-generated")
	}
}

// reportInjection runs invisible-character and Detect analysis on text and writes
// the findings to stderr, splitting pattern findings from outlier sentences.
func reportInjection(text string, o securityOpts) {
	threshold := o.injectionThreshold
	if inv := tldt.ReportInvisibles(text); len(inv) > 0 {
		fmt.Fprintf(os.Stderr, "injection-detect: %d invisible Unicode codepoint(s) found\n", len(inv))
		for _, r := range inv {
			fmt.Fprintf(os.Stderr, "  offset %d: U+%04X %s (%s)\n", r.Offset, r.Rune, r.Name, r.Category)
		}
	}
	layers := o.resolveLayers()
	dresult, err := tldt.Detect(text, tldt.DetectOptions{OutlierThreshold: threshold, Layers: &layers})
	if err != nil {
		fmt.Fprintln(os.Stderr, "detection error:", err)
		os.Exit(1)
	}
	report := dresult.Report
	// Outlier findings use a dissimilarity score on a different scale than
	// pattern confidence, so report them in their own block.
	var patternFindings, outlierFindings []tldt.Finding
	for _, f := range report.Findings {
		if f.Category == tldt.CategoryOutlier {
			outlierFindings = append(outlierFindings, f)
		} else {
			patternFindings = append(patternFindings, f)
		}
	}
	if len(patternFindings) > 0 {
		fmt.Fprintf(os.Stderr, "injection-detect: %d finding(s), max confidence %.2f\n", len(patternFindings), report.MaxScore)
		for _, f := range patternFindings {
			fmt.Fprintf(os.Stderr, "  [%s] %s (score=%.2f): %s\n", f.Category, f.Pattern, f.Score, f.Excerpt)
		}
		if report.Suspicious {
			fmt.Fprintln(os.Stderr, "injection-detect: WARNING — input flagged as suspicious")
		}
	} else {
		fmt.Fprintln(os.Stderr, "injection-detect: no findings")
	}
	if len(outlierFindings) > 0 {
		fmt.Fprintf(os.Stderr, "injection-detect: %d outlier sentence(s) above threshold %.2f\n", len(outlierFindings), threshold)
		for _, f := range outlierFindings {
			fmt.Fprintf(os.Stderr, "  [outlier] sentence %d (score=%.2f): %s\n", f.Sentence+1, f.Score, f.Excerpt)
		}
	}
}

// reportHiddenSurfaces runs injection detection on all hidden surfaces extracted
// from a fetched URL or document file. Surfaces are invisible to content extractors
// (readability, document converters) but present in raw HTML/PDF/DOCX/XLSX and
// readable by LLMs — this is the dedicated scan for that blind spot.
// Each surface is scanned individually; findings are labeled with source and index.
func reportHiddenSurfaces(surfs []tldt.HiddenSurface, o securityOpts) {
	threshold := o.injectionThreshold
	layers := o.resolveLayers()
	var totalFindings int
	for i, s := range surfs {
		dresult, err := tldt.Detect(s.Text, tldt.DetectOptions{OutlierThreshold: threshold, Layers: &layers})
		if err != nil {
			fmt.Fprintf(os.Stderr, "injection-detect[%s:%d]: detection error: %v\n", s.Source, i, err)
			continue
		}
		report := dresult.Report
		var patternFindings []tldt.Finding
		for _, f := range report.Findings {
			if f.Category != tldt.CategoryOutlier {
				patternFindings = append(patternFindings, f)
			}
		}
		if len(patternFindings) > 0 {
			totalFindings += len(patternFindings)
			fmt.Fprintf(os.Stderr, "injection-detect[%s:%d]: %d finding(s), max confidence %.2f\n",
				s.Source, i, len(patternFindings), report.MaxScore)
			for _, f := range patternFindings {
				fmt.Fprintf(os.Stderr, "  [%s] %s (score=%.2f): %s\n", f.Category, f.Pattern, f.Score, f.Excerpt)
			}
			if report.Suspicious {
				fmt.Fprintf(os.Stderr, "injection-detect[%s:%d]: WARNING — surface flagged as suspicious\n", s.Source, i)
			}
		}
	}
	if totalFindings == 0 {
		fmt.Fprintln(os.Stderr, "injection-detect[hidden-surfaces]: no findings")
	}
}

// extractDocumentSurfaces detects the document type by file extension and
// extracts hidden injection surfaces from DOCX, XLSX, and PDF files.
// Returns nil for unknown types — callers skip surface scanning gracefully.
func extractDocumentSurfaces(filePath string, data []byte) []tldt.HiddenSurface {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".docx"):
		return extractor.ExtractDOCX(data)
	case strings.HasSuffix(lower, ".xlsx"), strings.HasSuffix(lower, ".xls"):
		return extractor.ExtractXLSX(data)
	case strings.HasSuffix(lower, ".pdf"):
		return extractor.ExtractPDF(data)
	case strings.HasSuffix(lower, ".pptx"):
		return extractor.ExtractPPTX(data)
	case strings.HasSuffix(lower, ".ipynb"):
		return extractor.ExtractIPYNB(data)
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"):
		return extractor.ExtractMarkdown(data)
	case strings.HasSuffix(lower, ".epub"):
		return extractor.ExtractEPUB(data)
	case strings.HasSuffix(lower, ".eml"):
		return extractor.ExtractEML(data)
	case strings.HasSuffix(lower, ".svg"):
		return extractor.ExtractSVG(data)
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"),
		strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".tiff"),
		strings.HasSuffix(lower, ".webp"):
		return extractor.ExtractImageMetadata(data)
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		surfs := extractor.ExtractHTML(data)
		return append(surfs, extractor.DifferentialHTML(data)...)
	}
	return nil
}

// summarize builds the summarizer for algo and returns the summary. With explain
// set it prints algorithm diagnostics to stderr when the algorithm supports them,
// otherwise notes the fallback. Exits the process on failure.
func summarize(algo, text string, n int, explain bool) []string {
	s, err := tldt.NewSummarizer(algo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if explain {
		if ex, ok := s.(tldt.Explainer); ok {
			result, info, err := ex.SummarizeExplain(text, n)
			if err != nil {
				fmt.Fprintln(os.Stderr, "summarization failed:", err)
				os.Exit(1)
			}
			if info != nil {
				fmt.Fprint(os.Stderr, info.Format())
			}
			return result
		}
		// Graph or future algorithms without Explainer: fall back to normal summarize.
		fmt.Fprintf(os.Stderr, "note: --explain not supported for algorithm %q; running without diagnostics\n", algo)
	}
	result, err := s.Summarize(text, n)
	if err != nil {
		fmt.Fprintln(os.Stderr, "summarization failed:", err)
		os.Exit(1)
	}
	return result
}

// writeOutput renders result to stdout in the requested format.
func writeOutput(format string, result []string, meta formatter.SummaryMeta, paragraphs int) {
	switch format {
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
		if paragraphs > 0 {
			fmt.Println(groupIntoParagraphs(result, paragraphs))
		} else {
			fmt.Println(formatter.FormatText(result))
		}
	}
}

// usage prints the full help text to stderr and exits. Wired as flag.Usage.
func usage() {
	fmt.Fprintln(os.Stderr, "tldt - Text summarization and security preprocessing for LLM input")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "USAGE:")
	fmt.Fprintln(os.Stderr, "  tldt [options] [text...]")
	fmt.Fprintln(os.Stderr, "  cat file.txt | tldt [options]")
	fmt.Fprintln(os.Stderr, "  tldt -f article.txt [options]")
	fmt.Fprintln(os.Stderr, "  tldt --url https://example.com/article [options]")
	fmt.Fprintln(os.Stderr, "  tldt stats [--reset] [--top N]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "CORE OPTIONS:")
	fmt.Fprintln(os.Stderr, "  -f, -file string       Read input from file")
	fmt.Fprintln(os.Stderr, "  --url string           Fetch and summarize URL content")
	fmt.Fprintln(os.Stderr, "  --algorithm string     Summarization algorithm: lexrank (default), textrank, graph, ensemble")
	fmt.Fprintln(os.Stderr, "  --sentences int        Number of output sentences (default: 5)")
	fmt.Fprintln(os.Stderr, "  --level string         Compression preset: aggressive (3), standard (5), lite (10)")
	fmt.Fprintln(os.Stderr, "  --version              Print version and exit")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "SECURITY OPTIONS:")
	fmt.Fprintln(os.Stderr, "  --sanitize             Strip invisible Unicode characters and NFKC-normalize")
	fmt.Fprintln(os.Stderr, "  --detect-injection     Report prompt injection patterns to stderr (advisory)")
	fmt.Fprintln(os.Stderr, "  --injection-threshold float  Outlier detection threshold (default: 0.99)")
	fmt.Fprintln(os.Stderr, "  --detect-pii           Report PII/secrets (emails, API keys, tokens, private keys, JWTs, SSNs, cards)")
	fmt.Fprintln(os.Stderr, "  --sanitize-pii         Redact PII/secrets (detected patterns plus high-entropy key material)")
	fmt.Fprintln(os.Stderr, "  --detect-only          Run detection then exit; no summary, no usage log (pair with --format json for machine output)")
	fmt.Fprintln(os.Stderr, "  --hook-output          UserPromptSubmit hook mode: read {prompt} JSON from stdin, emit metadata-only advisory when flagged (else no output)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "AI CONTENT DETECTION:")
	fmt.Fprintln(os.Stderr, "  --detect-ai            Score text for AI-generated content using excess-vocabulary method")
	fmt.Fprintln(os.Stderr, "                         (Kobak et al. 2024, arXiv:2406.07016); reports score + verdict to stderr")
	fmt.Fprintln(os.Stderr, "  --lang string          Language for AI detection wordlist: en (default), pt-BR, es")
	fmt.Fprintln(os.Stderr, "  --wordlist-dir string  Directory with custom <lang>.json wordlist files (overrides embedded lists)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Score interpretation:")
	fmt.Fprintln(os.Stderr, "    ≥ 0.70  likely AI-generated")
	fmt.Fprintln(os.Stderr, "    ≥ 0.40  possibly AI-generated")
	fmt.Fprintln(os.Stderr, "    < 0.40  likely human-written")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  With --detect-only --format json: ai_detection block added to JSON output.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "FORMAT OPTIONS:")
	fmt.Fprintln(os.Stderr, "  --format string        Output format: text (default), json, markdown")
	fmt.Fprintln(os.Stderr, "  --verbose              Print token statistics to stderr")
	fmt.Fprintln(os.Stderr, "  --paragraphs int       Group output sentences into N paragraphs")
	fmt.Fprintln(os.Stderr, "  --no-cap               Disable 2000-sentence processing limit")
	fmt.Fprintln(os.Stderr, "  --explain              Print per-sentence scores and algorithm metrics to stderr (debug)")
	fmt.Fprintln(os.Stderr, "  --rouge string         Path to reference summary file; prints ROUGE-1/2/L scores to stderr")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "HTML PROCESSING:")
	fmt.Fprintln(os.Stderr, "  --from-html            Convert HTML input to Markdown before summarization")
	fmt.Fprintln(os.Stderr, "                         (uses readability extraction + html-to-markdown)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "CONFIGURATION:")
	fmt.Fprintln(os.Stderr, "  --install-skill        Install Claude Code skill and auto-trigger hook")
	fmt.Fprintln(os.Stderr, "  --skill-dir string     Override skill install directory")
	fmt.Fprintln(os.Stderr, "  --target string        Install target: claude|codex|cursor|opencode|agents|all")
	fmt.Fprintln(os.Stderr, "  --config-dir string    Override Claude config dir (else $CLAUDE_CONFIG_DIR, else ~/.claude)")
	fmt.Fprintln(os.Stderr, "  --project              Install repo-locally under ./.claude/ (hook in settings.local.json)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "EMBEDDED AI ASSISTANT SKILLS:")
	fmt.Fprintln(os.Stderr, "  The binary contains embedded skill templates for AI assistants.")
	fmt.Fprintln(os.Stderr, "  Skills are extracted and installed when you run --install-skill.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  SKILL.md - Manual /tldt command (all assistants)")
	fmt.Fprintln(os.Stderr, "    - Claude Code: ~/.claude/skills/tldt/SKILL.md")
	fmt.Fprintln(os.Stderr, "    - OpenCode:    ~/.config/opencode/skills/tldt/SKILL.md")
	fmt.Fprintln(os.Stderr, "    - Cursor:      ~/.cursor/skills/tldt/SKILL.md")
	fmt.Fprintln(os.Stderr, "    - Agents:      ~/.agents/skills/tldt/SKILL.md")
	fmt.Fprintln(os.Stderr, "    - Usage: Type /tldt <long text> inside the assistant")
	fmt.Fprintln(os.Stderr, "    - Returns: Token savings + extractive summary")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  tldt-hook.sh - Advisory security hook (Claude Code & Codex)")
	fmt.Fprintln(os.Stderr, "    - Location: ~/.claude/hooks/tldt-hook.sh (Claude); bundled in the Codex plugin")
	fmt.Fprintln(os.Stderr, "    - Delegates to: tldt --hook-output (reads the prompt envelope, no jq/python needed)")
	fmt.Fprintln(os.Stderr, "    - Adds a metadata-only warning to context when input is flagged; never summarizes or blocks")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "INSTALLATION:")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Auto-detect (installs to all assistants with existing directories):")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Target specific assistant (auto-creates directory if needed):")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target claude    # SKILL.md + hook + settings.json")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target codex     # plugin (skill + advisory hook) via local marketplace")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target opencode  # SKILL.md + advisory plugin (auto-creates dir)")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target cursor    # SKILL.md only (auto-creates dir)")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target agents    # SKILL.md only (auto-creates dir)")
	fmt.Fprintln(os.Stderr, "    tldt --install-skill --target all       # All assistants")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Notes:")
	fmt.Fprintln(os.Stderr, "    - Claude Code & Codex get the advisory hook (UserPromptSubmit); OpenCode gets an advisory plugin")
	fmt.Fprintln(os.Stderr, "    - Cursor and Agents get SKILL.md only (manual /tldt command)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "EXAMPLES:")
	fmt.Fprintln(os.Stderr, "  cat article.txt | tldt")
	fmt.Fprintln(os.Stderr, "  tldt -f transcript.txt --algorithm textrank --sentences 10")
	fmt.Fprintln(os.Stderr, "  tldt --url https://example.com/article --sanitize --detect-pii")
	fmt.Fprintln(os.Stderr, "  curl -s https://example.com | tldt --from-html --sentences 3")
	fmt.Fprintln(os.Stderr, "  tldt \"Long text to summarize\" --format json --verbose")
	fmt.Fprintln(os.Stderr, "  cat essay.txt | tldt --detect-ai --lang en --detect-only")
	fmt.Fprintln(os.Stderr, "  cat essay.txt | tldt --detect-ai --detect-injection --detect-pii --detect-only --format json")
	fmt.Fprintln(os.Stderr, "  tldt stats")
	fmt.Fprintln(os.Stderr, "  tldt stats --reset")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "CONFIG FILE (~/.tldt.toml):")
	fmt.Fprintln(os.Stderr, "  Core defaults:")
	fmt.Fprintln(os.Stderr, "    algorithm  = \"lexrank\"   # lexrank|textrank|graph|ensemble")
	fmt.Fprintln(os.Stderr, "    sentences  = 5")
	fmt.Fprintln(os.Stderr, "    format     = \"text\"      # text|json|markdown")
	fmt.Fprintln(os.Stderr, "    level      = \"\"          # aggressive|standard|lite")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Security defaults (all off by default):")
	fmt.Fprintln(os.Stderr, "    [security]")
	fmt.Fprintln(os.Stderr, "    detect_injection    = false")
	fmt.Fprintln(os.Stderr, "    injection_threshold = 0.99")
	fmt.Fprintln(os.Stderr, "    detect_pii          = false")
	fmt.Fprintln(os.Stderr, "    sanitize            = false")
	fmt.Fprintln(os.Stderr, "    sanitize_pii        = false")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  AI content detection defaults:")
	fmt.Fprintln(os.Stderr, "    [ai_detection]")
	fmt.Fprintln(os.Stderr, "    enabled      = false")
	fmt.Fprintln(os.Stderr, "    lang         = \"en\"      # en|pt-BR|es")
	fmt.Fprintln(os.Stderr, "    wordlist_dir = \"\"        # path to custom <lang>.json files")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Usage logging:")
	fmt.Fprintln(os.Stderr, "    [stats]")
	fmt.Fprintln(os.Stderr, "    enabled = true          # set false to opt out; clear with: tldt stats --reset")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "For more information: https://github.com/gleicon/tldt")
	os.Exit(0)
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
		n = len(sentences) // silent cap
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
// For --url fetches it also returns hidden HTML surfaces extracted from the raw HTML — these are
// invisible to readability but can carry prompt injection payloads. Non-URL paths return nil surfaces.
func resolveInputBytes(args []string, filePath string, urlStr string) ([]byte, []tldt.HiddenSurface, error) {
	// --url branch: highest priority — most explicit input source
	if urlStr != "" {
		fresult, err := tldt.Fetch(context.Background(), urlStr, tldt.FetchOptions{
			Timeout:  30 * time.Second,
			MaxBytes: 5 << 20, // 5MB cap
		})
		if err != nil {
			// ErrNoTextContent means a JS SPA or empty page — no summarizable text,
			// but hidden surfaces may still carry injection payloads. Return nil text
			// with surfaces so the caller can still run security scanning.
			if errors.Is(err, tldt.ErrNoTextContent) {
				return nil, fresult.HiddenSurfaces, nil
			}
			return nil, nil, fmt.Errorf("fetching URL: %w", err)
		}
		return []byte(fresult.Text), fresult.HiddenSurfaces, nil
	}
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, nil, fmt.Errorf("reading stdin: %w", err)
		}
		return data, nil, nil
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("reading file %q: %w", filePath, err)
		}
		// For known document formats, extract hidden injection surfaces alongside
		// raw bytes (which the summarizer will receive as-is for text extraction).
		surfs := extractDocumentSurfaces(filePath, data)
		return data, surfs, nil
	}
	if len(args) > 0 {
		return []byte(strings.Join(args, " ")), nil, nil
	}
	return nil, nil, fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
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

// applySentenceCap limits text to at most maxSentences to prevent O(n^2) hang.
// Returns text unchanged if sentence count is within the cap.
func applySentenceCap(text string, maxSentences int) string {
	sents := tldt.TokenizeSentences(text)
	if len(sents) <= maxSentences {
		return text
	}
	return strings.Join(sents[:maxSentences], " ")
}
