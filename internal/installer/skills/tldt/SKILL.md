---
name: tldt
description: "Compress long prose into a short extractive summary with the local tldt binary (no LLM cost) before adding it to context. Use for long articles, web pages, transcripts, docs, or pasted prose you only need the gist of. Do NOT use when the exact/verbatim text matters: code, config, logs to grep, or anything you will quote or edit."
argument-hint: "<url | file path | text to summarize>"
allowed-tools:
  - Bash
---

Summarize the source in $ARGUMENTS with the local `tldt` binary. It runs entirely
locally (no API/token cost) and returns exact sentences from the source.

Pick the input form from $ARGUMENTS and run the matching command. The summary goes
to stdout; the savings line (`~X -> ~Y tokens (Z% reduction)`) goes to stderr —
`2>&1` keeps both.

URL (starts with `http://` or `https://`):

```bash
tldt --url "$ARGUMENTS" --verbose 2>&1
```

File path (an existing file):

```bash
tldt -f "$ARGUMENTS" --verbose 2>&1
```

Raw text (anything else):

```bash
printf '%s' "$ARGUMENTS" | tldt --verbose 2>&1
```

Return the complete output: the savings line first, then the extractive summary.
