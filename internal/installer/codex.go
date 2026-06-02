package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Codex distributes skills and hooks as plugins, not as loose files: a standalone
// ~/.codex/hooks.json is not loaded. A plugin is a directory with a
// .codex-plugin/plugin.json manifest plus a root hooks.json (wrapped format,
// plugin-root-relative commands) and a skills/ tree, served from a local
// marketplace and installed via `codex plugin add`. One plugin carries both the
// reader skill and the advisory UserPromptSubmit hook. The hook's stdin/stdout
// contract is identical to Claude (.prompt in, hookSpecificOutput.additionalContext
// out), so the same tldt-hook.sh is reused unchanged.
const (
	codexMarketplaceName = "tldt-local"
	codexPluginSelector  = "tldt@tldt-local"
	codexMarketplaceDir  = "tldt-marketplace"
)

// codexBaseDir resolves the Codex config directory base:
// explicit --config-dir > $CODEX_HOME > the ~/.codex platform default.
func codexBaseDir(homeDir string, opts Options) string {
	if opts.ConfigDir != "" {
		return opts.ConfigDir
	}
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".codex")
}

// codexTargeted reports whether this install run should install the Codex plugin.
// Codex is excluded from --skill-dir (custom skill-only) and --project (Claude-only)
// runs, and is included on the default/all run or an explicit --target codex.
func codexTargeted(opts Options) bool {
	if opts.SkillDir != "" || opts.Project {
		return false
	}
	return opts.Target == "" || opts.Target == "all" || opts.Target == "codex"
}

// installCodexPlugin writes the tldt plugin tree under <codexBase>/tldt-marketplace/
// and registers it with Codex. Registration is best-effort: when the codex binary is
// absent the plugin files are still written and the two registration commands are
// printed for the user to run.
func installCodexPlugin(codexBase string) error {
	marketplaceDir := filepath.Join(codexBase, codexMarketplaceDir)
	if err := writeCodexPluginTree(marketplaceDir); err != nil {
		return err
	}
	registered, err := registerCodexPlugin(marketplaceDir)
	if err != nil {
		return err
	}
	if registered {
		fmt.Printf("installed to codex: %s (plugin %q)\n", marketplaceDir, codexPluginSelector)
	} else {
		fmt.Printf("wrote codex plugin to %s (registration pending — see above)\n", marketplaceDir)
	}
	return nil
}

// writeCodexPluginTree writes the marketplace manifest and the tldt plugin
// (manifest, wrapped hooks.json, shared advisory hook, reader skill) under
// marketplaceDir. Overwrites existing files so re-running upgrades in place.
func writeCodexPluginTree(marketplaceDir string) error {
	pluginDir := filepath.Join(marketplaceDir, "plugins", "tldt")
	files := []struct {
		embedded string
		dest     string
		mode     os.FileMode
	}{
		{"codex/marketplace.json", filepath.Join(marketplaceDir, ".agents", "plugins", "marketplace.json"), 0644},
		{"codex/plugin.json", filepath.Join(pluginDir, ".codex-plugin", "plugin.json"), 0644},
		{"codex/hooks.json", filepath.Join(pluginDir, "hooks.json"), 0644},
		{"hooks/tldt-hook.sh", filepath.Join(pluginDir, "hooks", "tldt-hook.sh"), 0755},
		{"skills/tldt/SKILL.md", filepath.Join(pluginDir, "skills", "tldt", "SKILL.md"), 0644},
	}
	for _, f := range files {
		data, err := EmbeddedFiles.ReadFile(f.embedded)
		if err != nil {
			return fmt.Errorf("embedded %q not found: %w", f.embedded, err)
		}
		if err := os.MkdirAll(filepath.Dir(f.dest), 0755); err != nil {
			return fmt.Errorf("creating directory for %q: %w", f.dest, err)
		}
		if err := os.WriteFile(f.dest, data, f.mode); err != nil {
			return fmt.Errorf("writing %q: %w", f.dest, err)
		}
	}
	return nil
}

// registerCodexPlugin registers the local marketplace and installs the plugin via
// the codex CLI. It is idempotent: any prior registration is dropped first so an
// upgrade picks up rewritten files. When codex is not on PATH it prints the manual
// steps and returns nil — the plugin files are already written.
func registerCodexPlugin(marketplaceDir string) (bool, error) {
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		fmt.Printf("  codex not found on PATH; to finish the Codex install, run:\n")
		fmt.Printf("    codex plugin marketplace add %q\n", marketplaceDir)
		fmt.Printf("    codex plugin add %s\n", codexPluginSelector)
		return false, nil
	}

	// Best-effort cleanup so a re-install picks up updated files; removing an entry
	// that does not exist returns a non-zero exit we intentionally ignore.
	_ = exec.Command(codexPath, "plugin", "remove", codexPluginSelector).Run()
	_ = exec.Command(codexPath, "plugin", "marketplace", "remove", codexMarketplaceName).Run()

	if out, err := exec.Command(codexPath, "plugin", "marketplace", "add", marketplaceDir).CombinedOutput(); err != nil {
		return false, fmt.Errorf("codex plugin marketplace add: %w: %s", err, out)
	}
	if out, err := exec.Command(codexPath, "plugin", "add", codexPluginSelector).CombinedOutput(); err != nil {
		return false, fmt.Errorf("codex plugin add: %w: %s", err, out)
	}
	return true, nil
}
