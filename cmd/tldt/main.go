package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gleicon/tldt/internal/summarizer"
)

func main() {
	filePath := flag.String("f", "", "input file path")
	algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph")
	sentences := flag.Int("sentences", 5, "number of output sentences")
	paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
	explain := flag.Bool("explain", false, "print algorithm metrics and per-sentence scores to stderr (debug)")
	noCap := flag.Bool("no-cap", false, "disable 2000-sentence cap (allows O(n^2) processing)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [-algorithm lexrank|textrank|graph] [-sentences N] [-paragraphs N] [-explain] [-no-cap] [text...]")
		fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	rawBytes, err := resolveInputBytes(flag.Args(), *filePath)
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

	s, err := summarizer.New(*algorithm)
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
			result, info, err2 = ex.SummarizeExplain(text, *sentences)
			if err2 != nil {
				fmt.Fprintln(os.Stderr, "summarization failed:", err2)
				os.Exit(1)
			}
			if info != nil {
				fmt.Fprint(os.Stderr, info.Format())
			}
		} else {
			// Graph or future algorithms without Explainer: fall back to normal summarize
			fmt.Fprintf(os.Stderr, "note: --explain not supported for algorithm %q; running without diagnostics\n", *algorithm)
			var err2 error
			result, err2 = s.Summarize(text, *sentences)
			if err2 != nil {
				fmt.Fprintln(os.Stderr, "summarization failed:", err2)
				os.Exit(1)
			}
		}
	} else {
		var err2 error
		result, err2 = s.Summarize(text, *sentences)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "summarization failed:", err2)
			os.Exit(1)
		}
	}

	// Token stats to stderr (TOK-01, TOK-02, TOK-03, D-09, D-10)
	charsOut := len(strings.Join(result, " "))
	tokIn := charsIn / 4
	tokOut := charsOut / 4
	reduction := 0
	if tokIn > 0 {
		reduction = int(float64(tokIn-tokOut) / float64(tokIn) * 100)
	}
	isTTY := stdoutIsTerminal()
	if isTTY {
		fmt.Fprintf(os.Stderr, "~%s -> ~%s tokens (%d%% reduction)\n",
			formatTokens(tokIn), formatTokens(tokOut), reduction)
	}

	// Output: one sentence per line (D-08) or grouped into paragraphs (D-05)
	if *paragraphs > 0 {
		fmt.Println(groupIntoParagraphs(result, *paragraphs))
	} else {
		fmt.Println(strings.Join(result, "\n"))
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

// resolveInput determines the text source using explicit precedence:
//  1. stdin pipe (stdin is not a TTY)
//  2. -f file flag
//  3. positional arguments joined with spaces
//  4. error (no input provided)
func resolveInput(args []string, filePath string) (string, error) {
	// 1. stdin pipe: check if stdin is a pipe/redirect (not a TTY)
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	// 2. -f flag
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading file %q: %w", filePath, err)
		}
		return string(data), nil
	}
	// 3. positional argument
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	return "", fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
}

// resolveInputBytes is like resolveInput but returns raw bytes for validation.
func resolveInputBytes(args []string, filePath string) ([]byte, error) {
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

// stdoutIsTerminal reports whether stdout is connected to a terminal (not piped/redirected).
// Uses the same os.ModeCharDevice check as resolveInput for stdin.
func stdoutIsTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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
