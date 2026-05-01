# Pitfalls Research — NLP + Go CLI Gotchas

## Text Processing Gotchas

### Sentence Tokenization Edge Cases
- **Abbreviations**: "Dr. Smith went..." — naive period-split breaks here. Fix: abbreviation list or regex lookahead `(?<!\b[A-Z][a-z]\b)\.`
- **URLs**: "See https://example.com/path.html for details" — period in URL triggers false split. Fix: strip/replace URLs before tokenizing.
- **Code snippets**: Technical docs often contain inline code. `fmt.Println("hello.")` ends with period inside string. Fix: treat code blocks as single sentence.
- **Ellipsis**: "Wait..." should not produce 3 splits. Fix: collapse `\.{2,}` before splitting.
- **Numbers**: "v1.5.2 released" — periods in version numbers. Fix: protect digit-dot-digit sequences.
- **Quotes**: `"He said, 'Stop.'"` — period inside quote. Fix: close-quote after period = sentence boundary.

### Unicode / Encoding
- YouTube transcripts often contain: em-dash (—), smart quotes (" "), non-breaking spaces (\u00a0)
- Non-breaking space breaks word tokenization. Fix: `strings.Map` to normalize whitespace.
- Right-to-left text (Arabic, Hebrew) — TF-IDF still works but sentence order matters for output. Out of scope for v1.
- BOM at start of file (Windows UTF-8): `\xef\xbb\xbf` — strip before processing.

### Edge Cases for Short Input
- Input has fewer sentences than `--sentences N`: return all sentences, no error.
- Input is single sentence: return it as-is.
- Empty input: print nothing, exit 0 (pipe-safe).
- Input is only whitespace: exit 0, no output.

## Performance Considerations

### O(n²) Similarity Matrix
- 100 sentences → 10,000 comparisons: ~1ms
- 1,000 sentences → 1,000,000 comparisons: ~100ms (acceptable)
- 5,000 sentences → 25,000,000 comparisons: ~5s (needs warning or cap)
- Fix: default cap at 2,000 sentences; `--no-cap` flag to override.
- Optimization: sparse matrix (skip pairs with similarity < threshold 0.1) — cuts 80%+ of work for typical text.

### Memory
- TF-IDF vectors: map[string]float64 per sentence. For 1,000 sentences with 500 unique words: ~4MB. Fine.
- Similarity matrix: float64[n][n]. 5,000² × 8 bytes = 200MB. Use float32 to halve; or sparse representation.

### Power Iteration Convergence
- Default max iterations: 100. Typical convergence: 20-50 iterations.
- Convergence check: sum of |rank[i] - prevRank[i]| < epsilon (1e-6). Don't skip this — infinite loop risk.

## Go Modules Migration Issues

### Old GOPATH → go.mod
- Project uses GOPATH-style `go get` with no `go.mod`. Must `go mod init github.com/gleicon/tldt` at repo root.
- `github.com/JesusIslam/tldr` — if we replace it, remove from deps. If we keep it, `go get github.com/JesusIslam/tldr`.
- `github.com/BurntSushi/toml` — used by old conf.go. Drop (no config needed for CLI).
- `github.com/fiorix/go-redis/redis` — used by old main.go. Drop entirely.
- Old `src/` subdirectory with its own build: collapse to root-level `main.go` or `cmd/tldt/main.go`.

### Build Tags
- Old Makefile uses `go build -v -o NAME -ldflags "-X main.VERSION=..."`. Modernize: use `go build -ldflags "-X main.version=..."`.

## Testing Strategy for NLP

### Non-Determinism Risk
- LexRank uses power iteration — converges to stable ranking. Given same input, output is deterministic.
- BUT: map iteration order in Go is random. If TF-IDF uses `map[string]float64`, computing IDF across corpus can produce float64 rounding differences. Fix: sort word list before building vectors.
- Test with fixed seed sentences to ensure deterministic output across runs.

### What to Test
1. **Unit**: TF-IDF computation for known sentences with known expected vectors
2. **Unit**: Cosine similarity between known vectors (0.0 for orthogonal, 1.0 for identical)
3. **Unit**: Power iteration convergence on toy 3x3 matrix
4. **Integration**: Full pipeline on `test-data/` files — verify top sentences include known key content
5. **Regression**: Snapshot test — summarize body.txt, compare to golden output. Update intentionally.
6. **Edge cases**: Empty input, single sentence, N > sentence count, unicode input

### Snapshot Testing Pattern in Go
```go
// In test: write golden file on first run, compare thereafter
// go test -update-golden to regenerate
var updateGolden = flag.Bool("update-golden", false, "update golden files")
```

## CLI Robustness

### Stdin Handling
- If stdin is a TTY (no pipe), don't hang waiting. Detect: `fi, _ := os.Stdin.Stat(); (fi.Mode() & os.ModeCharDevice) != 0`.
- If TTY + no file arg + no positional arg: print usage and exit 1.
- Large stdin: don't buffer entire input before processing — but sentence tokenization requires full text. Document: max ~10MB practical limit.

### Non-text Input
- If input is binary (PDF, image), tokenization produces garbage. Fast check: if > 30% non-printable bytes in first 512 bytes → error "binary input detected, use text".

### Pipe Safety
- Always exit 0 on empty/no output (don't break `cat file | tldt | next-step` pipelines).
- Write errors to stderr, output to stdout. Never mix.

## Prevention Strategies Summary

| Pitfall | Prevention |
|---------|------------|
| Abbreviation false splits | Abbreviation list + lookahead regex |
| URLs in text | Strip URL pattern before tokenize |
| O(n²) for large docs | Sparse similarity + sentence cap (default 2000) |
| Non-determinism from map iteration | Sort keys before vector computation |
| Hanging on TTY stdin | Detect `ModeCharDevice` before reading |
| Binary input | Magic byte check on first 512 bytes |
| Short input < N sentences | Return all available sentences, no error |
| go.mod migration | `go mod init` + tidy + remove old deps |
