# tldt Requirements

---

## v2.0 Requirements — Extensions

**Milestone goal:** Expand tldt's reach — fetch URLs, persist user defaults, add compression presets, integrate as an AI assistant skill with auto-trigger support, and defend against prompt injection in untrusted text.

### Input Sources (INP)

- [x] **INP-01**: User can run `tldt --url <url>` to fetch a webpage, strip boilerplate HTML, and receive an extractive summary on stdout
- [x] **INP-02**: URL fetcher handles HTTP redirects; returns non-zero exit code with error to stderr on fetch failure

### Configuration (CFG)

- [x] **CFG-01**: User can create `~/.tldt.toml` with default values for `algorithm`, `sentences`, `format`, and `level` flags
- [x] **CFG-02**: CLI flags always override values from `~/.tldt.toml`
- [x] **CFG-03**: Missing or malformed `~/.tldt.toml` is not an error — defaults apply silently
- [x] **CFG-04**: User can run `tldt --level aggressive` (3 sentences, most compression), `--level standard` (5 sentences), or `--level lite` (10 sentences, least compression)
- [x] **CFG-05**: `--level` can be set as the default in `~/.tldt.toml`; explicit `--sentences N` overrides it

### AI Integration (AI)

- [x] **AI-01**: User can install a Claude Code skill file that invokes the local `tldt` binary on selected or pasted text
- [x] **AI-02**: AI skill passes text to `tldt` via stdin and returns the summary inline in the conversation
- [x] **AI-03**: Auto-trigger hook fires when input text or a file exceeds a configurable token count threshold
- [x] **AI-04**: Auto-trigger summarizes the oversized input and reports token savings before inserting the summary into the AI context

### Security / Injection Defense (SEC)

- [x] **SEC-01**: `--sanitize` strips invisible Unicode (Cf category, bidi controls U+202A–U+202E, zero-width U+200B–U+200F, PUA, Tags block U+E0000–U+E01FF) before summarization
- [x] **SEC-02**: `--sanitize` applies NFKC normalization (collapses fullwidth, ligatures, compatibility variants)
- [x] **SEC-03**: `--sanitize` reports count of removed codepoints to stderr; stdout unchanged if nothing removed
- [x] **SEC-04**: `--detect-injection` detects direct instruction overrides, role injection, delimiter injection, jailbreaks, and exfiltration requests via multi-word regex patterns
- [x] **SEC-05**: `--detect-injection` detects encoding anomalies: base64 payloads (entropy-gated), `\x`-escaped hex sequences, raw hex strings, abnormal control character density
- [x] **SEC-06**: `--detect-injection` with `--algorithm lexrank` reports statistically off-topic sentences using the LexRank cosine similarity matrix (outlier_score = 1 - mean neighbor similarity)
- [x] **SEC-07**: All detection output goes to stderr only; detection never blocks or modifies stdout summarization output
- [x] **SEC-08**: `--injection-threshold <float>` configures the outlier score cutoff (default: 0.85); higher = fewer false positives
- [x] **SEC-09**: Sanitizer and detector packages are independently importable with no dependency on cmd/tldt
- [x] **SEC-10**: `--detect-injection` detects cross-script homoglyph substitution (e.g., Cyrillic `а` for Latin `a`) using UTS#39 confusables.txt (Unicode 17.0, embedded in binary)

---

## v2.0 Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INP-01 | Phase 4: URL Input | Complete |
| INP-02 | Phase 4: URL Input | Complete |
| CFG-01 | Phase 5: Configuration | Complete |
| CFG-02 | Phase 5: Configuration | Complete |
| CFG-03 | Phase 5: Configuration | Complete |
| CFG-04 | Phase 5: Configuration | Complete |
| CFG-05 | Phase 5: Configuration | Complete |
| AI-01 | Phase 6: AI Integration | Complete |
| AI-02 | Phase 6: AI Integration | Complete |
| AI-03 | Phase 6: AI Integration | Complete |
| AI-04 | Phase 6: AI Integration | Complete |
| SEC-01 | Phase 7: Injection Defense | Complete |
| SEC-02 | Phase 7: Injection Defense | Complete |
| SEC-03 | Phase 7: Injection Defense | Complete |
| SEC-04 | Phase 7: Injection Defense | Complete |
| SEC-05 | Phase 7: Injection Defense | Complete |
| SEC-06 | Phase 7: Injection Defense | Complete |
| SEC-07 | Phase 7: Injection Defense | Complete |
| SEC-08 | Phase 7: Injection Defense | Complete |
| SEC-09 | Phase 7: Injection Defense | Complete |
| SEC-10 | Phase 7: Injection Defense | Complete |
| SEC-08 | Phase 7: Injection Defense | Complete |
| SEC-09 | Phase 7: Injection Defense | Complete |

---

## v2.0 Future / Deferred

- Clipboard auto-read (`pbpaste`/`xclip`) when invoked with no args — deferred
- `--url` authentication headers / cookie support — deferred
- TOML validation/lint command (`tldt --check-config`) — deferred
- MCP server mode for direct tool-call integration — deferred

---

## v1.0 Requirements (all validated — historical record)

### Core CLI (v1)

- [x] **CLI-01**: User can invoke `tldt` as a standalone binary from PATH
- [x] **CLI-02**: User can pipe text via stdin: `cat file.txt | tldt`
- [x] **CLI-03**: User can specify input file: `tldt -f article.txt`
- [x] **CLI-04**: User can pass text as positional argument: `tldt "long text..."`
- [x] **CLI-05**: When stdout is piped, output contains ONLY summary text (no metadata, no decoration)
- [x] **CLI-06**: When stdout is a TTY, output includes compression stats to stderr
- [x] **CLI-07**: Empty or whitespace-only input exits 0 with no output (pipe-safe)
- [x] **CLI-08**: Binary/non-text input detected and rejected with error to stderr

### Summarization (v1)

- [x] **SUM-01**: User can control output sentence count: `--sentences N` (default: 5)
- [x] **SUM-02**: User can group output sentences into paragraphs: `--paragraphs N`
- [x] **SUM-03**: User can select algorithm: `--algorithm lexrank|textrank|graph|ensemble` (default: lexrank)
- [x] **SUM-04**: When N > available sentences, return all sentences without error
- [x] **SUM-05**: Output sentences appear in original document order (not score order)
- [x] **SUM-06**: LexRank algorithm implemented natively with IDF-modified cosine similarity
- [x] **SUM-07**: TextRank algorithm implemented natively with word-overlap + PageRank
- [x] **SUM-08**: `graph` algorithm delegates to `github.com/didasy/tldr` as baseline
- [x] **SUM-09**: `ensemble` algorithm averages LexRank + TextRank score vectors

### Token Awareness (v1)

- [x] **TOK-01**: Tool displays estimated token count before and after: `~12,400 → ~1,380 tokens (89% reduction)`
- [x] **TOK-02**: Token estimate uses chars/4 heuristic, labeled as estimated
- [x] **TOK-03**: Token stats displayed to stderr (never stdout) so they don't break pipes

### Output Formats (v1)

- [x] **OUT-01**: Default output is plain text (pipe-safe)
- [x] **OUT-02**: `--format json` outputs structured JSON with all stats fields
- [x] **OUT-03**: `--format markdown` wraps summary in a markdown blockquote with metadata header
- [x] **OUT-04**: `--rouge <reference_file>` prints ROUGE-1/2/L scores to stderr

### Quality & Testing (v1)

- [x] **TEST-01–07**: Full unit + integration test suite (192 tests, 86% coverage)

### Project Hygiene (v1)

- [x] **PROJ-01**: Modern go modules (`go.mod` at repo root)
- [x] **PROJ-02**: README updated with all features and examples
- [x] **PROJ-03**: Sentence count cap at 2000 for O(n²) safety
- [x] **PROJ-04**: Build via `go build ./...`, test via `go test ./...`

---

## Out of Scope (all milestones)

- HTTP server / web API — dropped in v1.0
- Redis / database storage — CLI is stateless by design
- Authentication / rate limiting — not applicable
- LLM integration — antithetical to tool's purpose
- Abstractive summarization — extractive only
