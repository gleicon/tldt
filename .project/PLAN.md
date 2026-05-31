# Plan

Source: `.project/SPEC.md` — Full Code Audit (Behavior-Preserving)

## Now

**State** — Audit's main work is shipped: 9 commits on `cleanup` (`a6e4591`→`852f6b0`), all behavior-preserving A-groups + approved behavior-changing R1/R2/R4. Tree is green (lint 0, `test -race` all pass, golden I/O identical, API identical, `go.mod` unchanged). Two "Ending cleanup" tasks remain in the roadmap and are **not yet started**.

**Next** — Strip leaked planning IDs from code comments (GSD `CFG-/SUM-/D-/INP-/SEC-/LIB-/Pitfall N/Phase N` + the audit's own `R1/R2/R4` in new test comments) across ~9 non-test + 2 test files; keep the explanation, drop the reference; commit as one behavior-preserving group, verify build+lint+golden. Then run the final `/ds-*` pass (deslop, code-quality, go-review, bug-review, security-review) over the diff.

**Open questions** — (1) Should the comment cleanup also drop the `R1/R2/R4` audit IDs (current plan: yes)? (2) Fix the 3 examples' pre-existing stale `go.mod` via `go mod tidy` (R15), or leave as flagged? Awaiting user "go".

## Roadmap

### Baseline
- [x] Behavior baseline captured: build/vet/`test -race` all green; golangci-lint baseline = 19 issues (17 errcheck, 2 staticcheck)
- [x] Public API snapshot captured: `go doc -all ./pkg/tldt` saved (`.audit-baseline/pkg-tldt-api.txt`)
- [x] Golden I/O snapshot captured: 15 scenarios (stdin, `-f`, 4 algorithms × 3 formats, verbose, sanitize/detect) — stdout SHAs + stderr recorded; `--url` excluded per D-5

### Review pass (collect findings, no edits)
- [x] `/ds-bug-review` findings collected over active code
- [x] `/ds-security-review` findings collected over active code
- [x] `/ds-go-review` findings collected over active code
- [x] `/ds-code-quality-review` findings collected over active code
- [x] `/ds-test-quality-review` findings collected
- [x] `/ds-doc-quality-review` findings collected (README, docs/, package docs)
- [x] `/ds-deslop` findings collected
- [x] All findings consolidated and classified → `.project/AUDIT.md` (A=apply, B=recommend, C=opt-in refactor)
- [~] **CHECKPOINT: maintainer reviews AUDIT.md before any edit**

### Apply safe fixes (behavior-preserving only) — DONE
- [x] golangci-lint gate adopted (commit d2b03c9): `.golangci.yml` v2 defaults, `make lint` wired, tree green
- [x] Doc accuracy fixes (commit 3b88964): threshold drift, missing flags, dup/stale doc cleanup
- [x] Deslop + dead-code fixes (commit dd1aa2a): QuarantinedIdxs, false comments, micro-idioms
- [x] Test-quality fixes (commit f65f1b1): assertion-free tests converted; 9 coverage tests added
- [x] `go.mod`/`go.sum` unchanged (no imports became unused)

### Apply approved behavior-changing fixes (R1, R2, R4 — user-approved) — DONE
- [x] R1 (commit f2298ed): reject `--sentences < 1`; summarizer select clamps negative n
- [x] R4 (commit 829a816): fetcher returns real metadata + sentinel errors; pkg/tldt.Fetch uses errors.Is
- [x] R2 (commit ff1790c): SSRF dial-time IP validation (closes DNS-rebinding TOCTOU)

### Verify — DONE
- [x] API snapshot unchanged: exported `pkg/tldt` func/type set identical (FetchResult shape unchanged; values now truthful per R4)
- [x] Golden I/O unchanged: all 15 baseline scenarios match byte-for-byte
- [x] Suite parity: `go test -race ./...` passes (366 test funcs)
- [x] Build + vet + lint green on final tree
- [x] `go.mod`/`go.sum` unchanged after `go mod tidy` (NFR-4)
- [~] examples: `html-processor` builds; basic/pipeline/openapi-client have pre-existing stale go.mod (flagged R15)

### Report & ship — DONE
- [x] Consolidated audit report produced → `.project/AUDIT.md` (applied + remaining recommendations)
- [x] Fixes committed in 7 independently-revertible groups on `cleanup`
- [x] Remaining behavior-changing recommendations (R3, R5–R15, C1–C3) handed to maintainer in AUDIT.md

### Ending cleanup
- [ ] Strip leaked planning IDs/lingo from code comments — keep the substantive explanation, drop the reference. Patterns: `CFG-NN`, `SUM-NN`, `D-NN`, `INP-NN`, `SEC-NN`, `LIB-*`, `CLI-NN`, `Pitfall N`, `Phase N`, milestone refs (GSD); also the audit's own `R1/R2/R4` IDs that leaked into the new test comments. Scope: ~9 non-test files (cmd/tldt/main.go, internal/config, internal/fetcher, internal/installer ×2, internal/summarizer ×4, pkg/tldt) + 2 test files. Behavior-preserving (comments only) → verify build + lint + golden I/O unchanged.
- [ ] Final `/ds-*` pass over the full audit diff (catch anything the changes introduced):
  - [ ] `/ds-deslop` — slop in new/edited code
  - [ ] `/ds-code-quality-review` — maintainability of the changes
  - [ ] `/ds-go-review` — Go idioms (new fetcher transport, error wrapping, clamps)
  - [ ] `/ds-bug-review` — correctness of the behavior-changing commits (R1/R2/R4)
  - [ ] `/ds-security-review` — re-verify the rewritten SSRF dial path (R2) and fetcher error/metadata surface (R4)
  - [ ] Consolidate residual findings into AUDIT.md; apply behavior-preserving fixes, recommend the rest
