package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gleicon/tldt/internal/summarizer"
)

func main() {
	filePath := flag.String("f", "", "input file path")
	algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph")
	sentences := flag.Int("sentences", 5, "number of output sentences")
	paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [-algorithm lexrank|textrank|graph] [-sentences N] [-paragraphs N] [text...]")
		fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	text, err := resolveInput(flag.Args(), *filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	s, err := summarizer.New(*algorithm)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	charsIn := len(text)
	result, err := s.Summarize(text, *sentences)
	if err != nil {
		fmt.Fprintln(os.Stderr, "summarization failed:", err)
		os.Exit(1)
	}

	// Token stats to stderr (TOK-01, TOK-02, TOK-03, D-09, D-10)
	charsOut := len(strings.Join(result, " "))
	tokIn := charsIn / 4
	tokOut := charsOut / 4
	reduction := 0
	if tokIn > 0 {
		reduction = int(float64(tokIn-tokOut) / float64(tokIn) * 100)
	}
	fmt.Fprintf(os.Stderr, "~%s \u2192 ~%s tokens (%d%% reduction)\n",
		formatTokens(tokIn), formatTokens(tokOut), reduction)

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
