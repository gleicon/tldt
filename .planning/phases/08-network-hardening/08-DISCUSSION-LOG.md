# Phase 8: Network Hardening + Hook Defense - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-02
**Phase:** 8-Network Hardening + Hook Defense
**Areas discussed:** SSRF check architecture, Hook stderr splitting, Output guard mechanism, additionalContext structure

---

## SSRF Check Architecture

### Q1: Should SSRF blocking cover the initial URL too?

| Option | Description | Selected |
|--------|-------------|----------|
| Both — initial + every hop | Resolve initial URL before request, re-check in CheckRedirect for each hop | ✓ |
| Redirect hops only | Only block via CheckRedirect; initial private IP would slip through | |
| You decide | Claude chooses safest approach | |

**User's choice:** Both — initial + every hop (Recommended)

---

### Q2: Single vs separate CheckRedirect function?

| Option | Description | Selected |
|--------|-------------|----------|
| Single combined CheckRedirect | One func: count hops + resolve hostname + check IPs | ✓ |
| Separate: redirect counter + SSRF helper | CheckRedirect only counts; blockPrivateIP() called separately | |

**User's choice:** Single combined CheckRedirect (Recommended)

---

### Q3: Error format when SSRF triggered?

| Option | Description | Selected |
|--------|-------------|----------|
| Descriptive string — no custom type | fmt.Errorf("SSRF blocked: ..."); matches existing style | |
| Typed sentinel errors | var ErrSSRFBlocked, ErrRedirectLimit — allows errors.Is() checks | ✓ |

**User's choice:** Typed sentinel errors

---

## Hook Stderr Splitting

### Q1: How to split WARNING lines from token stats?

| Option | Description | Selected |
|--------|-------------|----------|
| Grep WARNING prefix | grep ^WARNING / grep -v ^WARNING in bash; zero tldt changes | ✓ |
| Two invocations | Run tldt twice; doubles CPU cost | |
| New tldt flag --warnings-file | Cleaner but adds scope to Phase 8 | |

**User's choice:** Grep WARNING prefix (Recommended)

---

### Q2: Silent when clean vs always include status?

| Option | Description | Selected |
|--------|-------------|----------|
| Silent when clean | Only append WARNINGs if they exist | ✓ |
| Always include status | Append "no injection detected" on clean runs | |

**User's choice:** Silent when clean (Recommended)

---

## Output Guard Mechanism

### Q1: How to detect without re-summarizing?

| Option | Description | Selected |
|--------|-------------|----------|
| --sentences 999 | Requests 999 sentences so all ~5 summary sentences pass through | ✓ |
| Accept re-summarization | Summary is short; may return same sentences anyway | |

**User's choice:** --sentences 999 (Recommended)

---

### Q2: If output guard flags the summary?

| Option | Description | Selected |
|--------|-------------|----------|
| Warn and still emit | Append WARNINGs to additionalContext; summary still included | ✓ |
| Suppress the summary | Drop summary if flagged; emit only WARNINGs | |

**User's choice:** Warn and still emit (Recommended) — consistent with SEC-07 advisory contract

---

## additionalContext Structure

### Q1: How to compose savings + warnings + summary?

| Option | Description | Selected |
|--------|-------------|----------|
| Labeled sections | [Token savings] / [Security warnings - input] / [Security warnings - summary] / [Summary] | ✓ |
| Flat concat, no headers | Savings line, blank line, WARNINGs, blank line, summary | |

**User's choice:** Labeled sections (Recommended)

---

### Q2: When no warnings — omit warning sections or keep headers?

| Option | Description | Selected |
|--------|-------------|----------|
| Stats + summary only — no warning sections | Warning sections omitted when empty | ✓ |
| Keep all section headers | Always render all 4 sections; empty sections show "(none)" | |

**User's choice:** Stats + summary only — no warning sections (Recommended)

---

## Claude's Discretion

None — all decisions were made by user.

## Deferred Ideas

None — discussion stayed within phase scope.
