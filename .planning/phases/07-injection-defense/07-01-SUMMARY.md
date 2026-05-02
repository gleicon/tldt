---
phase: 07
plan: 01
title: Sanitizer package
status: complete
completed: 2026-05-02
commit: 6e2793f
---

# Summary 07-01: internal/sanitizer package

## What was built

`internal/sanitizer` package with four exported functions:

- `StripInvisible(text string) string` — strips 5 categories of invisible chars: Unicode Cf, bidi controls (U+202A–U+202E), zero-width (U+200B–U+200F), PUA block, Tags block (U+E0000–U+E01FF)
- `NormalizeUnicode(text string) string` — NFKC normalization (fullwidth → ASCII, ligatures, compatibility variants)
- `SanitizeAll(text string) string` — StripInvisible + NormalizeUnicode in one call
- `ReportInvisibles(text string) []InvisibleRune` — structured report of each stripped char (offset, rune value, Unicode name, category label)

31 tests across all categories. `golang.org/x/text` upgraded to v0.36.

## Key decisions

- NFKC does NOT collapse cross-script homoglyphs (Cyrillic 'а' ≠ Latin 'a'). Documented as known limitation. UTS#39 confusables database would be required for that threat model.
- Tags block (U+E0000–U+E01FF) stripped — used in some prompt injection attacks to hide instructions in metadata-like characters.
- `golang.org/x/text` was already a transitive dependency via go-readability; upgrade from v0.22 to v0.36 added no new direct module.
