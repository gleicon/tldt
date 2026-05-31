# Handoff — tldt full code audit

## Goal
Behavior-preserving full code audit of `tldt` using `/ds-*` reviews: apply safe fixes, recommend behavior-changing ones. Full spec in `.project/SPEC.md`; executed roadmap + current state in `.project/PLAN.md` (`## Now`); consolidated findings in `.project/AUDIT.md`. Decisions came from a `/ds-grill-me` session (D-1…D-8 in SPEC.md).

## Done (9 commits on branch `cleanup`, `a6e4591`→`852f6b0`)
- `a6e4591` GSD→devskills migration (removed `.planning/`, stopped ignoring `.project/`)
- `d2b03c9` **lint gate** — `.golangci.yml` v2 defaults, `make lint` wired, gofmt, all 19 baseline lint findings fixed
- `3b88964` **docs** — threshold drift 0.85→0.99, fixed `--detect-pii` example, added 5 missing README flags, dup/stale cleanup
- `dd1aa2a` **dead-code/slop** — removed `QuarantinedIdxs`, false comments, micro-idioms
- `f65f1b1` **test quality** — 2 assertion-free tests converted, +9 coverage tests
- `f2298ed` **R1** — reject `--sentences < 1` (was a panic); summarizer select clamps negative n
- `829a816` **R4** — fetcher returns real metadata + sentinel errors (`ErrHTTPError`/`ErrNonHTML`); pkg/tldt.Fetch uses `errors.Is`, no fabricated values
- `ff1790c` **R2** — dial-time SSRF validation via `safeDialContext` (closes DNS-rebinding TOCTOU); pre-check + per-hop CheckRedirect-SSRF removed, 5-hop cap kept
- `040365e` / `852f6b0` — `.project/` audit docs + ending tasks

## Remains (in PLAN.md `## Roadmap` → "Ending cleanup")
1. **Strip leaked planning IDs from comments** — GSD IDs (`CFG-NN`, `SUM-NN`, `D-NN`, `INP-NN`, `SEC-NN`, `LIB-*`, `Pitfall N`, `Phase N`) + the audit's own `R1/R2/R4` in the new test comments. Scope: cmd/tldt/main.go, internal/config, internal/fetcher, internal/installer (×2: installer.go, embed.go), internal/summarizer (×4: summarizer.go, lexrank.go, textrank.go, ensemble.go), pkg/tldt/tldt.go + cmd/tldt/main_test.go, pkg/tldt/tldt_test.go. Keep the substantive explanation, drop only the ID/reference. Behavior-preserving → verify build+lint+golden unchanged, one commit.
2. **Final `/ds-*` pass** over the diff: deslop, code-quality, go-review, bug-review, security-review (run as parallel subagents like the first round; scope = active code, exclude `src/`). Consolidate residuals into AUDIT.md; apply behavior-preserving fixes, recommend the rest.

## Remaining recommendations (NOT applied — see AUDIT.md section B)
R3 (SSRF blocklist gaps), R5 (PII single-pass), R6 (PII coverage), R7 (`Detect` unused opts — API change), R8 (UTF-8 byte-slice truncation), R9 (installer `--target`), R10–R13, R14 (openapi example fetches JSON → `ErrNonHTML`), R15 (3 examples' stale go.mod, pre-existing). Plus 🔁 C1–C3 refactors (collapse duplicated scoring pipeline; decompose ~400-line `main`) — user deferred.

## How to verify (the audit's safety net)
Baseline snapshots in `.audit-baseline/` (gitignored): `pkg-tldt-api.txt` (API), `golden/` (15 stdout SHAs). After any change:
- `go build ./... && go vet ./... && golangci-lint run ./...` (or `make lint`) — must be 0 issues
- `go test -race ./...` — all pass (366 test funcs)
- golden I/O: rebuild `./cmd/tldt`, run the 15 scenarios (stdin/`-f`, 4 algos × 3 formats, `--verbose`, `--sanitize --detect-injection --detect-pii` over `test-data/wikipedia_en.txt`), shasum-compare against `.audit-baseline/golden/*.stdout`
- API parity: `go doc -all ./pkg/tldt` func/type set vs `.audit-baseline/pkg-tldt-api.txt`

## Gotchas
- **httptest + SSRF**: httptest servers bind loopback, which the real guard blocks. Functional fetcher tests override the injectable `blockIP` var via `withBlockIP(allowAllIPs, ...)`; SSRF tests inject `lookupHost` (private) with the real `blockIP`. Don't revert to the old `publicLookup` pattern — it relied on the rebinding gap that R2 closed.
- **gofmt vs the gate**: golangci v2 defaults don't include gofmt, so `golangci-lint run` can pass while files are unformatted. Always `gofmt -w ./cmd ./internal ./pkg` before committing (Group 1 established a gofmt-clean tree).
- **`FetchResult` shape unchanged** by R4 — only the *values* are now truthful; exported API is identical (AC-5 held).
- Edits require a prior Read of the file in-session (`sed`/`cat` don't count for the Edit tool's read-tracking).
- Working on `cleanup`; commit per group, keep them independently revertible (NFR-3).
