// tldt-advisory.js — OpenCode advisory plugin.
//
// On each submitted user message, runs local tldt injection + PII detection
// (no LLM, no network) and shows a TUI toast ONLY when something is flagged.
// It never summarizes, replaces, or blocks the prompt. It degrades silently
// when the tldt binary is absent.
//
// Install location (per OpenCode docs): ~/.config/opencode/plugins/ (global) or
// .opencode/plugins/ (project).
import { spawnSync } from "node:child_process"

export const TldtAdvisory = async ({ client }) => {
  return {
    "chat.message": async (_input, output) => {
      const text = (output.parts || [])
        .filter((p) => p && p.type === "text" && typeof p.text === "string")
        .map((p) => p.text)
        .join("\n")
        .trim()
      if (!text) return

      // Detection-only: no summary, no usage log. tldt emits a structured
      // {flagged, findings[]} JSON contract on stdout (outliers excluded).
      const res = spawnSync(
        "tldt",
        ["--detect-injection", "--detect-pii", "--detect-only", "--format", "json"],
        { input: text, encoding: "utf8" },
      )
      // tldt missing, failed to spawn, or errored (no stdout) — degrade silently.
      if (res.error || res.status !== 0) return

      let report
      try {
        report = JSON.parse(res.stdout || "")
      } catch {
        return // malformed output — fail closed, no false advisory
      }
      if (!report || !report.flagged) return

      // User-facing toast: a one-line summary per finding (kind + pattern +
      // location). Excerpts are available here but kept short for the toast.
      const lines = (report.findings || []).map((f) => {
        const loc = f.line ? ` (line ${f.line})` : ""
        const score = typeof f.score === "number" ? ` score ${f.score.toFixed(2)}` : ""
        return `- ${f.kind}: ${f.pattern}${score}${loc}`
      })

      await client.tui.showToast({
        body: {
          title: "tldt security advisory",
          message: "Untrusted input flagged:\n" + lines.join("\n"),
          variant: "warning",
        },
      })
    },
  }
}

export default TldtAdvisory
