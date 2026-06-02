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

      // Detection-only: no summary, no usage log. Findings go to stderr.
      const res = spawnSync(
        "tldt",
        ["--detect-injection", "--detect-pii", "--detect-only"],
        { input: text, encoding: "utf8" },
      )
      // tldt missing or failed to spawn — degrade silently.
      if (res.error || res.status === null) return

      // Flagged = stderr minus the two clean "no findings" lines and blanks.
      const findings = (res.stderr || "")
        .split("\n")
        .filter((l) => l.trim() && !/(pii|injection)-detect: no findings/.test(l))
        .join("\n")
        .trim()
      if (!findings) return

      await client.tui.showToast({
        body: {
          title: "tldt security advisory",
          message: "Untrusted input flagged:\n" + findings,
          variant: "warning",
        },
      })
    },
  }
}

export default TldtAdvisory
