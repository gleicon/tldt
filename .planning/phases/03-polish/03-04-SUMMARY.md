---
phase: 03-polish
plan: "04"
subsystem: documentation
tags: [readme, documentation, cli-flags, algorithms]
dependency_graph:
  requires: []
  provides: [README.md-v1]
  affects: []
tech_stack:
  added: []
  patterns: [developer-facing markdown documentation]
key_files:
  created: []
  modified:
    - README.md
decisions:
  - "Document --format and --no-cap flags in README even though implemented in sibling plans (03-01 to 03-03) — README describes the target v1 state, not the pre-wave state"
metrics:
  duration: "~3 min"
  completed: "2026-05-02"
  tasks_completed: 1
  tasks_total: 1
  files_changed: 1
requirements_met: [PROJ-02, PROJ-04]
---

# Phase 3 Plan 04: README Rewrite Summary

README.md completely replaced — stale Go web server template (MySQL/Redis) removed and replaced with accurate tldt v1 documentation covering install, all three input modes, all seven flags, three output formats with examples, LexRank/TextRank algorithm explanations, and comparison table.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rewrite README.md for tldt v1 | 86381eb | README.md |

## What Was Built

A complete developer-facing README.md that:

- Identifies tldt by purpose: extractive summarization to save LLM tokens before pasting into AI coding agents
- Provides `go install github.com/gleicon/tldt/cmd/tldt@latest` install instructions with Go 1.21+ prerequisite
- Shows four input usage patterns: stdin pipe, `-f` file, positional arg, YouTube transcript pipeline
- Documents all seven flags in a reference table (`-f`, `--algorithm`, `--sentences`, `--paragraphs`, `--format`, `--no-cap`, `--explain`)
- Shows all three output formats (text, json, markdown) with concrete examples including the full 9-field JSON schema
- Explains LexRank (TF-IDF cosine + eigenvector centrality) and TextRank (word overlap + PageRank) in plain language
- Includes a comparison table: similarity metric, ranking method, best-for, determinism
- Includes `go build ./...` and `go test ./...` commands
- Explains token savings display (`~N -> ~M tokens (P% reduction)` on stderr)

## Acceptance Criteria Verification

| Criterion | Status |
|-----------|--------|
| No "web server template" text | PASS (0 matches) |
| No "MySQL" or "Redis" text | PASS (0 matches) |
| Contains "tldt" | PASS |
| Contains "go install" | PASS |
| Contains "--algorithm" | PASS |
| Contains "--sentences" | PASS |
| Contains "--format" | PASS |
| Contains "--no-cap" | PASS |
| Contains "LexRank" | PASS |
| Contains "TextRank" | PASS |
| Contains "compression_ratio" | PASS |
| Contains "go build" | PASS |
| Contains "go test" | PASS |

## Deviations from Plan

None — plan executed exactly as written.

The README documents `--format` and `--no-cap` flags per the plan's `<interfaces>` section which describes the complete target v1 state (after all phase 3 plans complete). This is correct: the README describes the final state, and sibling wave plans implement those flags.

## Known Stubs

None — README is documentation only, no code stubs.

## Threat Surface

No new executable code paths. README content uses local file examples and public YouTube URLs. No credentials or private endpoints documented. Consistent with threat model T-03-08 (accept).

## Self-Check: PASSED

- README.md exists and contains all required sections
- Commit 86381eb exists in git log
- No unexpected file deletions
