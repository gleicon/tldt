---
name: tldt
description: "Summarize long text with tldt — reduces token count before sending to AI"
argument-hint: "<text to summarize>"
allowed-tools:
  - Bash
---

Summarize the text in $ARGUMENTS using the local tldt binary.

Run this command:

```bash
echo "$ARGUMENTS" | tldt --verbose 2>&1
```

The first line of output shows token savings (~X -> ~Y tokens, Z% reduction).
The remaining lines are the extractive summary.

Return the complete output — savings line first, then summary.
