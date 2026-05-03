# Phase 9 Discussion Log

**Date:** 2026-05-03
**Phase:** 9 — PII Detection + Output Guard + Docs

## Areas Selected

User selected all four gray areas for discussion.

---

## Area 1: PII package location

**Options presented:**
- Extend `internal/detector` (recommended)
- New `internal/pii` package

**User selected:** Extend `internal/detector`

**Rationale:** Detector already has `Category`, `Finding`, `Report` types. PII findings are interoperable with injection/encoding findings. One import for callers. No type duplication.

---

## Area 2: Hook PII guard

**Options presented:**
- Add `--detect-pii` to guard pass (recommended)
- Keep guard injection-only

**User selected:** Add `--detect-pii` to guard

**Notes:** Guard becomes `--detect-injection --detect-pii --sentences 999`. PII WARNING lines in summary surface in `[Security warnings - summary]`. Existing `grep 'WARNING'` filter catches both prefixes.

---

## Area 3: README Security scope

**Options presented:**
- Standalone mini-section with 3 paragraphs + link to security.md (recommended)
- Link-only callout

**User selected:** Standalone mini-section

**Notes:** LLM04, LLM08, LLM09 get one paragraph each inline. Link to `docs/security.md` for full OWASP coverage (LLM01/02/05/10 already there). No duplication.

---

## Area 4: WARNING output format

**Options presented:**
- `pii-detect: WARNING —` prefix (parallel to injection-detect) (recommended)
- Unified `WARNING —` prefix

**User selected:** `pii-detect: WARNING —`

**Notes:** grep-able by source. Hook's `grep 'WARNING'` catches both automatically. Users can filter by `pii-detect` or `injection-detect` in separate alerting pipelines.

---

## Claude's Discretion

- Redaction count stderr format: `pii-detect: N redaction(s) applied`
- PII excerpt truncation: first 12 chars + `...` for long values in WARNING messages
- `--sanitize-pii` flag order relative to `--sanitize`: Unicode normalization then PII redaction (consistent with existing pipeline order)
- README `## Security` placement: after flags table, before installation

## Deferred Ideas

None.
