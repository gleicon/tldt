# Roadmap: tldt — Too Long, Didn't Tokenize

## Overview

A brownfield Go web API is transformed into a pure CLI summarization tool. The three phases follow a natural delivery arc: get the project skeleton and module structure working (Foundation), implement the graph algorithms that do the actual work (Algorithms), then harden the CLI for real-world pipeline use (Polish). Each phase ships a verifiable, runnable binary milestone.

## Phases

- [x] **Phase 1: Foundation** - Modernize to go modules, clean CLI skeleton, baseline graph algorithm, test data
- [x] **Phase 2: Algorithms** - Implement LexRank and TextRank natively, expose algorithm/sentence/paragraph flags, full test suite
- [x] **Phase 3: Polish** - TTY detection, output formats (JSON/markdown), pipe safety, O(n²) cap, README

## Phase Details

### Phase 1: Foundation
**Goal**: A working go-modules project with a CLI binary that accepts text input and produces extractive summaries via the graph baseline algorithm.
**Depends on**: Nothing (first phase)
**Requirements**: PROJ-01, CLI-01, CLI-02, CLI-03, CLI-04, SUM-08, TEST-07
**Success Criteria** (what must be TRUE):
  1. `go build ./...` and `go test ./...` succeed with no errors from the repo root
  2. `echo "text..." | tldt` runs without panicking and returns non-empty output
  3. `tldt -f article.txt` and `tldt "text..."` both produce output without error
  4. `github.com/didasy/tldr` graph algorithm is selectable and produces output on all test-data/ files
**Plans**: 3 plans

**Wave 1**
- [x] 01-01-PLAN.md — Go module init, dependency fetch, Makefile replacement, directory scaffolds

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 01-02-PLAN.md — Graph summarizer wrapper (internal/summarizer) and CLI entry point (cmd/tldt/main.go)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 01-03-PLAN.md — Four English test-data files and integration tests covering all test-data fixtures
**UI hint**: no

### Phase 2: Algorithms
**Goal**: LexRank and TextRank are implemented natively in Go and selectable via flags, with a deterministic, fully-tested summarization pipeline.
**Depends on**: Phase 1
**Requirements**: SUM-01, SUM-02, SUM-03, SUM-04, SUM-05, SUM-06, SUM-07, TOK-01, TOK-02, TOK-03, TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06
**Success Criteria** (what must be TRUE):
  1. `tldt --algorithm lexrank --sentences 3 -f article.txt` returns exactly 3 sentences in original document order
  2. `tldt --algorithm textrank --sentences 5 -f article.txt` returns a different (but valid) 5-sentence summary
  3. `go test ./...` passes all unit tests including TF-IDF vectors, cosine similarity, and power iteration convergence
  4. Running the same input twice always produces identical output (deterministic)
**Plans**: 4 plans

Plans:
- [x] 02-01-PLAN.md — Sentence tokenizer, Summarizer interface + registry, Graph struct wrapper
- [x] 02-02-PLAN.md — LexRank algorithm (TF-IDF, cosine similarity, power iteration) with unit tests
- [x] 02-03-PLAN.md — TextRank algorithm (word overlap, damped PageRank iteration) with unit tests
- [x] 02-04-PLAN.md — CLI flag wiring (--algorithm, --sentences, --paragraphs), token stats, integration tests

**Wave 1**
- [x] 02-01-PLAN.md — Sentence tokenizer, Summarizer interface + registry, Graph struct wrapper

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 02-02-PLAN.md — LexRank algorithm with TF-IDF, cosine similarity, power iteration, and full unit tests
- [x] 02-03-PLAN.md — TextRank algorithm with word overlap similarity, damped power iteration, and full unit tests

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 02-04-PLAN.md — CLI flags, token stats to stderr, paragraph grouping, integration tests for all algorithms
**UI hint**: no

### Phase 3: Polish
**Goal**: The binary is pipe-safe and production-ready: TTY-aware stats output, structured output formats, and input validation.
**Depends on**: Phase 2
**Requirements**: CLI-05, CLI-06, CLI-07, CLI-08, OUT-01, OUT-02, OUT-03, PROJ-02, PROJ-03, PROJ-04
**Success Criteria** (what must be TRUE):
  1. `cat article.txt | tldt | wc -l` captures only summary lines — no stats, no decoration on stdout
  2. Running `tldt -f article.txt` in a terminal shows `~12,400 -> ~1,380 tokens (89% reduction)` on stderr
  3. `tldt --format json -f article.txt` outputs valid JSON with all required fields (summary, algorithm, compression_ratio, etc.)
  4. `tldt` given empty input exits 0 with no output; binary/non-text input prints an error to stderr and exits non-zero
**Plans**: 4 plans

Plans:
- [x] 03-01-PLAN.md — TTY gate, input validation (empty/binary), sentence cap, stats format fix
- [x] 03-02-PLAN.md — internal/formatter package (FormatText, FormatJSON, FormatMarkdown) with unit tests
- [x] 03-03-PLAN.md — Wire --format flag into main.go, formatter dispatch, JSON stats suppression
- [x] 03-04-PLAN.md — README rewrite for tldt v1

**Wave 1** *(parallel — no shared files)*
- [x] 03-01-PLAN.md — TTY gate, input validation, sentence cap, stats format fix (cmd/tldt/main.go)
- [x] 03-02-PLAN.md — internal/formatter package with all three format functions and unit tests
- [x] 03-04-PLAN.md — README rewrite

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 03-03-PLAN.md — Wire --format flag into main.go using formatter package
**UI hint**: no

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete | 2026-05-01 |
| 2. Algorithms | 4/4 | Complete | 2026-05-01 |
| 3. Polish | 4/4 | Complete | 2026-05-02 |

---

## Milestone v2.0: Extensions

### Overview

v2.0 expands tldt's reach in three focused phases: URL input adds a new content source without touching the core summarization pipeline; Configuration persists user defaults and introduces compression presets as a friendlier interface over raw `--sentences`; AI Integration ships tldt as an installable Claude Code skill with an auto-trigger hook. Each phase is independently deliverable and depends on the previous.

### Phases

- [x] **Phase 4: URL Input** - User can pass a URL to tldt and receive an extractive summary of the fetched page
- [ ] **Phase 5: Configuration** - User preferences persist across invocations via ~/.tldt.toml; compression presets simplify common sentence counts
- [ ] **Phase 6: AI Integration** - tldt ships as an installable Claude Code skill file with an auto-trigger hook that fires when input exceeds a token threshold

## Phase Details

### Phase 4: URL Input
**Goal**: Users can summarize a live webpage by passing its URL to tldt — no manual copy-paste required.
**Depends on**: Phase 3
**Requirements**: INP-01, INP-02
**Success Criteria** (what must be TRUE):
  1. `tldt --url https://example.com/article` fetches the page, strips HTML boilerplate, and prints an extractive summary to stdout
  2. `tldt --url https://example.com/article | wc -l` produces only summary text on stdout — no headers, no HTML, no decoration
  3. `tldt --url https://httpstat.us/404` exits non-zero and prints a descriptive error to stderr
  4. `tldt --url https://httpstat.us/301` follows the redirect transparently and still produces a summary
**Plans**: 2 plans

**Wave 1**
- [x] 04-01-PLAN.md — go-readability dependency + internal/fetcher package (Fetch function + 5 unit tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 04-02-PLAN.md — Wire --url flag into main.go; fix 5 existing resolveInputBytes call sites; add 4 URL integration tests

**Cross-cutting constraints:**
- `--url` is highest-priority branch in `resolveInputBytes(urlStr, file, args)` — both plans must honor this precedence
- All tests use `httptest.NewServer` — no real network calls permitted
**UI hint**: no

### Phase 5: Configuration
**Goal**: Users can persist their preferred flags in ~/.tldt.toml and use named compression presets instead of raw sentence counts.
**Depends on**: Phase 4
**Requirements**: CFG-01, CFG-02, CFG-03, CFG-04, CFG-05
**Success Criteria** (what must be TRUE):
  1. After writing `algorithm = "ensemble"` and `sentences = 7` to `~/.tldt.toml`, running `tldt -f article.txt` (no extra flags) uses those values
  2. `tldt --sentences 3 -f article.txt` overrides a `sentences = 7` entry in `~/.tldt.toml` and returns exactly 3 sentences
  3. Deleting or corrupting `~/.tldt.toml` does not cause an error — the tool silently falls back to built-in defaults
  4. `tldt --level aggressive -f article.txt` returns 10 sentences; `--level lite` returns 3; `--level standard` returns 5
**Plans**: 2 plans

Plans:
- [x] 05-01-PLAN.md — internal/config package (Config struct, Load, DefaultConfig, LevelPresets) + unit tests
- [x] 05-02-PLAN.md — Wire config + --level flag into main.go; flag.Visit override logic; integration tests

**Wave 1**
- [x] 05-01-PLAN.md — BurntSushi/toml dependency + internal/config package with Config, Load, DefaultConfig, LevelPresets, ConfigPath + 10 unit tests

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 05-02-PLAN.md — Wire config loading into main.go, add --level flag, flag.Visit override detection, replace raw flag dereferences with effective vars, 11 integration tests

**Cross-cutting constraints:**
- `Load()` NEVER returns an error — absorbs missing and malformed TOML silently (CFG-03)
- `flag.Visit` (NOT `flag.VisitAll`) detects explicitly-set CLI flags for override precedence (CFG-02)
- Level preset resolution happens BEFORE --sentences override check (CFG-05)
- All config tests use `t.Setenv("HOME", t.TempDir())` for isolation — no real ~/.tldt.toml access
**UI hint**: no

### Phase 6: AI Integration
**Goal**: tldt is installable as a Claude Code skill and fires automatically when pasted or file-sourced text exceeds a configurable token threshold.
**Depends on**: Phase 5
**Requirements**: AI-01, AI-02, AI-03, AI-04
**Success Criteria** (what must be TRUE):
  1. A user can copy the shipped skill file into their Claude Code skills directory and invoke tldt on selected text from within a Claude Code session — the summary appears inline in the conversation
  2. The skill passes text to tldt via stdin and the returned summary replaces the raw input in the conversation context
  3. With the auto-trigger hook installed and threshold set to 2000 tokens, pasting a 3000-token block causes tldt to summarize it automatically before it enters the AI context
  4. After auto-trigger fires, the tool reports the token savings (e.g. `~3,200 -> ~480 tokens (85% reduction)`) before inserting the summary
**Plans**: 4 plans

Plans:
- [x] 06-01-PLAN.md — Extend internal/config with HookConfig struct (Hook.Threshold, default 2000) + unit tests
- [x] 06-02-PLAN.md — Create SKILL.md and tldt-hook.sh templates in internal/installer/ for go:embed
- [x] 06-03-PLAN.md — internal/installer package (embed.go, installer.go, installer_test.go)
- [x] 06-04-PLAN.md — Wire --print-threshold and --install-skill flags into main.go; add Makefile install-skill target

**Wave 1** *(parallel — no shared files)*
- [x] 06-01-PLAN.md — HookConfig + Hook.Threshold in internal/config/config.go + 5 unit tests
- [x] 06-02-PLAN.md — internal/installer/skills/tldt/SKILL.md + internal/installer/hooks/tldt-hook.sh templates

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 06-03-PLAN.md — internal/installer package: embed.go (go:embed) + installer.go + installer_test.go (8 tests)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 06-04-PLAN.md — Wire --print-threshold, --install-skill, --skill-dir, --target flags into main.go + Makefile install-skill target

**Cross-cutting constraints:**
- go:embed paths must be relative to the source file; templates live in internal/installer/skills/ and internal/installer/hooks/ (NOT repo root)
- Hook script MUST call `tldt --verbose` to capture token stats (stats suppressed by default when stdout is pipe)
- Hook script parses .prompt field from JSON stdin using jq with python3 fallback (control char safety)
- PatchSettingsJSON uses read-merge-write with atomic temp-file rename and idempotency guard
- Settings.json hook entry uses absolute expanded path for the hook command (subdirectory hook bug workaround)
**UI hint**: no

### Phase 7: Injection Defense
**Goal**: tldt can detect and sanitize prompt injection patterns in untrusted text before it enters an AI context, with advisory-only output to stderr that never breaks pipes.
**Depends on**: Phase 6
**Requirements**: SEC-01, SEC-02, SEC-03, SEC-04, SEC-05, SEC-06, SEC-07, SEC-08, SEC-09
**Success Criteria** (what must be TRUE):
  1. `cat untrusted.txt | tldt --sanitize` strips invisible Unicode (Cf category, bidi controls, zero-width, PUA, Tags block) and NFKC-normalizes before summarization; stdout is unaffected if no invisible chars found
  2. `cat untrusted.txt | tldt --detect-injection` reports pattern matches (direct-override, role-injection, delimiter, jailbreak, exfiltration) and encoding anomalies (base64, hex-escape, hex-string, ctrl-char-density) to stderr; stdout always contains only the summary
  3. Detection output never appears on stdout — detection is advisory and never blocks summarization
  4. `--detect-injection` with `--algorithm lexrank` also reports statistically off-topic sentences using the LexRank cosine similarity matrix (outlier score = 1 - mean neighbor similarity)
  5. `--injection-threshold 0.90` adjusts the outlier sensitivity; default is 0.85
**Plans**: 3 plans (retroactive — implemented before formal plan)

**Wave 1** *(parallel — no shared files)*
- [x] 07-01-PLAN.md — internal/sanitizer package: StripInvisible, NormalizeUnicode, SanitizeAll, ReportInvisibles + 31 tests
- [x] 07-02-PLAN.md — internal/detector package: DetectPatterns (6 categories, 16 regexes), DetectEncoding (base64/hex/ctrl), DetectOutliers (cosine outlier), Analyze + 28 tests

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 07-03-PLAN.md — Wire --sanitize, --detect-injection, --injection-threshold into main.go; expose LexRank similarity matrix via MatrixSummarizer interface; wire DetectOutliers; README injection defense section

**Cross-cutting constraints:**
- Detection is ALWAYS advisory — never modifies stdout, never blocks summarization
- DetectOutliers uses pre-normalization cosine similarity matrix (not stochastic/row-normalized values)
- NFKC normalization does NOT collapse cross-script homoglyphs (Cyrillic 'а' ≠ Latin 'a') — documented limitation
- Pattern regexes use multi-word phrases to minimize false positives on common single words
**UI hint**: no

## v2.0 Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 4. URL Input | 2/2 | Complete | 2026-05-02 |
| 5. Configuration | 2/2 | Complete | 2026-05-02 |
| 6. AI Integration | 4/4 | Complete | 2026-05-02 |
| 7. Injection Defense | 3/3 | Complete | 2026-05-02 |

---

## Milestone v1.2.0: OWASP Security Hardening

### Overview

v1.2.0 closes four concrete OWASP LLM Top 10 2025 gaps in tldt's role as AI middleware. The work splits naturally along two delivery boundaries: network-layer and hook-layer hardening are surgical changes to existing files (fetcher.go and tldt-hook.sh) with no new packages; PII detection introduces a new scanner and requires new flags wired into main.go alongside the README security section. Each phase is independently verifiable and the two phases have no shared files, enabling clean sequential delivery.

### Phases

- [x] **Phase 8: Network Hardening + Hook Defense** - SSRF protection and redirect cap in the URL fetcher; hook wires --sanitize --detect-injection by default and guards its own output before emitting to Claude context (2026-05-03)
- [ ] **Phase 9: PII Detection + Output Guard + Docs** - --detect-pii and --sanitize-pii flags for email, API keys, JWTs, and credit card patterns; hook output guard re-runs injection check on summary; README Security section

## Phase Details

### Phase 8: Network Hardening + Hook Defense
**Goal**: The URL fetcher cannot be weaponized for SSRF attacks and the auto-trigger hook defends every summarization pass against injection by default.
**Depends on**: Phase 7
**Requirements**: SEC-11, SEC-12, SEC-13, SEC-16
**Success Criteria** (what must be TRUE):
  1. `tldt --url http://192.168.1.1/admin` exits non-zero with a descriptive SSRF-block error to stderr and produces no summary output
  2. `tldt --url http://169.254.169.254/latest/meta-data/` exits non-zero with a cloud-metadata-block error to stderr
  3. A URL that redirects more than 5 times causes tldt to exit non-zero with a redirect-limit error; a URL with exactly 5 hops succeeds
  4. When the installed hook processes a document, any WARNING lines emitted by --detect-injection appear in the additionalContext returned to Claude alongside the summary
**Plans**: 4 plans

Plans:
- [x] 08-01-PLAN.md — SSRF IP block + redirect cap in internal/fetcher/fetcher.go (SEC-11, SEC-12)
- [x] 08-02-PLAN.md — Hook defense: --sanitize --detect-injection by default, output guard (SEC-13, SEC-16)
- [x] 08-03-PLAN.md — Security documentation (docs/security.md) + landing page security section (D-10, D-11)
- [x] 08-04-PLAN.md — Embeddable Go library pkg/tldt/ with Summarize, Detect, Sanitize, Fetch, Pipeline (D-12)

**Wave 1** *(parallel — no shared files)*
- [x] 08-01-PLAN.md — SSRF IP block + redirect cap in internal/fetcher/fetcher.go; fetcher unit tests using httptest.NewServer
- [x] 08-03-PLAN.md — docs/security.md (OWASP LLM Top 10 2025 reference) + docs/index.html security section
- [x] 08-04-PLAN.md — pkg/tldt/tldt.go + pkg/tldt/tldt_test.go (embeddable Go library)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 08-02-PLAN.md — Update internal/installer/hooks/tldt-hook.sh: invoke `tldt --sanitize --detect-injection --verbose` by default; capture stderr WARNING lines and append to additionalContext; add output guard that re-runs --detect-injection on the summary before emitting

**Cross-cutting constraints:**
- SSRF block must resolve the hostname (net.LookupHost) after redirect, not just the initial URL — block applies at every hop
- Cloud metadata range is 169.254.169.254/32 (IPv4) and fd00:ec2::254/128 (IPv6)
- Redirect cap is enforced via a custom http.Client CheckRedirect func; the 5-hop limit is inclusive (5 redirects allowed, 6th is rejected)
- All new fetcher tests use httptest.NewServer — no live network calls
- Hook changes are in the embedded template source (internal/installer/hooks/tldt-hook.sh), not a deployed copy
- Use `grep 'WARNING'` (unanchored) in hook — tldt emits `injection-detect: WARNING —` format (not line-starting `WARNING:`)
**UI hint**: no

### Phase 9: PII Detection + Output Guard + Docs
**Goal**: Users can detect or redact PII and secrets in source text before summarization, the hook guards its output against injection, and the README documents tldt's architectural immunity to three OWASP LLM categories.
**Depends on**: Phase 8
**Requirements**: SEC-14, SEC-15, DOC-01
**Success Criteria** (what must be TRUE):
  1. `echo "Contact alice@example.com or use key sk-abc123" | tldt --detect-pii` prints WARNING lines to stderr identifying the email and API key pattern; stdout contains only the summary
  2. `echo "Card: 4111111111111111 JWT: eyJ..." | tldt --sanitize-pii` replaces the credit card and JWT with `[REDACTED:credit-card]` and `[REDACTED:jwt]` before summarization; stderr reports the count of redactions
  3. `--sanitize-pii` without `--detect-pii` still performs redaction (--sanitize-pii implies detection)
  4. README contains a `## Security` section with rationale entries for LLM04, LLM08, and LLM09 architectural immunity
**Plans**: 3 plans
**UI hint**: no

**Wave 1** *(parallel — no shared files)*
- [ ] 09-01-PLAN.md — internal/pii package (or extend internal/detector): DetectPII with patterns for email, API key prefixes (Bearer/sk-/AIza/AKIA), JWT (three-segment base64url), and 13-16-digit credit card sequences; unit tests covering all pattern categories (SEC-14)
- [ ] 09-02-PLAN.md — README.md `## Security` section: LLM04 (no ML weights), LLM08 (no vector store), LLM09 (extractive = no hallucination) with one-paragraph rationale each (DOC-01)

**Wave 2** *(blocked on Wave 1 completion)*
- [ ] 09-03-PLAN.md — Wire --detect-pii and --sanitize-pii flags into cmd/tldt/main.go; SanitizePII redaction with [REDACTED:<type>] placeholders; stderr redaction count; integration tests (SEC-15)

**Cross-cutting constraints:**
- `--detect-pii` is advisory only — never modifies stdout, never blocks summarization (mirrors SEC-07 contract)
- `--sanitize-pii` implies detection: if --sanitize-pii is set, PII detection runs automatically even without --detect-pii
- Redaction replaces the matched span in-place before the text reaches the summarizer; original text is never logged
- API key patterns: `Bearer\s+[A-Za-z0-9._~+/-]+=*`, `sk-[A-Za-z0-9]{20,}`, `AIza[A-Za-z0-9_-]{35}`, `AKIA[A-Z0-9]{16}`
- JWT pattern: three base64url segments separated by dots with minimum length guard to reduce false positives
- Credit card pattern: 13-16 consecutive digits (Luhn check optional but preferred to reduce false positives)

## v1.2.0 Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 8. Network Hardening + Hook Defense | 4/4 | Complete | 2026-05-03 |
| 9. PII Detection + Output Guard + Docs | 0/3 | Not started | - |
