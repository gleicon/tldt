# Plan

Source: `.project/SPEC.md` — Full Code Audit (Behavior-Preserving)

## Now

**State** — ALL audit recommendations applied. 16 follow-up commits on `cleanup` (`163cd34`→`47712ec`): comment-ID strip, R15, R3, R6, R5, R7(A), R8, R9, R10, R11, R12, R13, R14, C1, C2/C3, plus doc updates. Combined with the earlier 9 commits, the whole audit (A-groups, R1–R15, C1–C3) is shipped. Tree green on every commit: lint 0, `test -race` all pass, 15/15 golden byte-identical, `pkg/tldt` API stable. Nothing from sections B/C remains open — see AUDIT.md "Section B + C — ALL APPLIED".

**Next** — AUDIT COMPLETE. Pre-pass (`18f2f32`), stray-binary untrack (`fa54cc0`), and the full 6-review final pass (`e10f3b9`, 4 behavior-preserving fixes) all shipped. No roadmap items remain. Decision for maintainer: act on the recommend-only residuals in AUDIT.md "Final review pass" — chiefly **S1 (High): `--sanitize-pii` redaction coverage is narrower than the README promise** (Slack/AWS-secret/base64 secrets pass through). Everything else is minor/optional. Branch `cleanup` ready to merge to `main` once S1 is decided.

**Open questions** — Only S1 (redaction-coverage vs doc-promise) needs a product call: expand patterns, feed DetectEncoding spans into scanPII, or narrow the doc. Maintainer prior choices: R7 option A, R6 incl. SSN+Luhn, R13 full legacy removal, C1–C3 done.

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
- [x] Strip leaked planning IDs/lingo from code comments (commit 163cd34) — GSD IDs + audit R1 IDs removed across 13 files; comments only, golden/API unchanged.
- [x] R15 (commit 20d1c23): `go mod tidy` examples basic/pipeline/openapi-client (pre-existing stale go.mod). main module untouched.
- [x] R3 (commit 9228002): SSRF blocklist — unspecified/CGNAT/NAT64/benchmark; IPv4-mapped covered by regression tests.
- [x] R6 (commit 27ec822): PII coverage — modern sk-/GitHub/PEM/SSN + Luhn credit-card; detect/redact kept consistent.
- [x] Reviewed and applied ALL remaining B/C items one-by-one with maintainer: R3,R5,R6,R7(A),R8,R9,R10,R11,R12,R13,R14 + C1,C2,C3 (commits 9228002→68c1f01). See AUDIT.md "Section B + C — ALL APPLIED". Tree green throughout; golden + API parity held on every commit.
- [x] **Pre-pass hygiene** (commit 18f2f32): `go fix ./...` across all 5 modules — `interface{}`→`any`, C-style loops→range-int, `strings.Split`→`strings.SplitSeq`. Zero gopls hints remain anywhere. Verified build+vet+lint(0), `test -race` all pass, 15/15 golden byte-identical, `pkg/tldt` API func/type set unchanged. Rebuilt example binaries (tracked artifacts) restored, not committed.
- [x] Final `/ds-*` pass over the full audit diff — all six run sequentially (deslop last), each delegated:
  1. [x] `/ds-code-quality-review` — PASS; C1/C2/C3 delete duplication, no file near 1k, no spaghetti.
  2. [x] `/ds-go-review` — PASS; SSRF dial path + DetectOutliers idiomatic; 1 major (example enc.Encode) + minor nits.
  3. [x] `/ds-bug-review` — PASS; SSRF/Luhn/outlier/clamps verified by repro. 1 reproduced advisory-only edge (base64 re-pad).
  4. [x] `/ds-security-review` — PASS; TOCTOU closed, ReDoS impossible, installer safe. 1 High coverage gap (S1).
  5. [x] `/ds-test-quality-review` — PASS; security paths well-covered. 2 test-only fixes applied (T1/T2).
  6. [x] `/ds-deslop` — PASS; code clean (prior deslop did the work). 1 trivial fix (D2), 4 candidates refuted.
  - [x] Consolidated into AUDIT.md "Final review pass"; 4 behavior-preserving fixes applied (`e10f3b9`), rest recommended.
