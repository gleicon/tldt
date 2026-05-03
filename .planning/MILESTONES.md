# tldt — Milestones

## v1.0: Core CLI (Complete — 2026-05-01)

**Goal:** Transform the Go web API into a pure CLI summarization tool with dual graph algorithms.

**Shipped:**
- Phase 1: Foundation — go modules, CLI skeleton, graph baseline, test data
- Phase 2: Algorithms — LexRank + TextRank native implementation, `--algorithm` / `--sentences` / `--paragraphs` flags
- Phase 3: Polish — TTY detection, JSON/markdown output, pipe safety, O(n²) cap, ensemble algorithm, ROUGE evaluation

**Stats:** 3 phases, 11 plans, 192 tests

---

## v2.0: Extensions (Complete — 2026-05-02)

**Goal:** Expand tldt's reach — URL fetch, persistent config, compression presets, AI skill, injection defense.

**Shipped:**
- Phase 4: URL Input — `--url` fetches webpage, strips HTML via go-readability, httptest-based tests
- Phase 5: Configuration — `~/.tldt.toml` with `flag.Visit` override detection; `--level lite|standard|aggressive`
- Phase 6: AI Integration — Claude Code skill installer (`--install-skill`); auto-trigger hook (`tldt-hook.sh`)
- Phase 7: Injection Defense — `--sanitize` (Unicode strip + NFKC), `--detect-injection` (regex + encoding + cosine outlier + UTS#39 homoglyphs)

**Stats:** 4 phases, 11 plans, 292 tests

---

## v1.2.0: OWASP Security Hardening (In Progress — 2026-05-02)

**Goal:** Close four concrete OWASP LLM Top 10 2025 gaps in tldt's middleware role.

**Target:**
- Phase 8: Network Hardening + Hook Defense (LLM01 + LLM10)
- Phase 9: PII Detection + Output Guard + Security Docs (LLM02 + LLM05 + DOC)

**Status:** Requirements defined — roadmap pending
