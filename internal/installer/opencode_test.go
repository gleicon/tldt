package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallOpenCodePlugin_WritesAdvisoryPlugin(t *testing.T) {
	destPath := filepath.Join(t.TempDir(), "plugins", "tldt-advisory.js")
	if err := installOpenCodePlugin(destPath); err != nil {
		t.Fatalf("installOpenCodePlugin: unexpected error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading installed plugin: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"chat.message"`) {
		t.Error("plugin missing chat.message hook")
	}
	if !strings.Contains(body, "tldt") || !strings.Contains(body, "--detect-injection") {
		t.Error("plugin missing tldt detection invocation")
	}
	if !strings.Contains(body, "showToast") {
		t.Error("plugin missing showToast advisory surface")
	}
}

// OpenCode gets the advisory plugin alongside its skill; per OpenCode docs
// the plugin lives in <config>/plugins/. Cursor stays skill-only.
func TestResolveTargets_OpenCodeGetsPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".config", "opencode"), 0755); err != nil {
		t.Fatalf("creating opencode dir: %v", err)
	}

	targets, err := resolveTargets(tmpDir, Options{})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}

	var oc *installTarget
	for i := range targets {
		if targets[i].name == "opencode" {
			oc = &targets[i]
		}
	}
	if oc == nil {
		t.Fatal("opencode target not found even though ~/.config/opencode exists")
	}
	wantPlugin := filepath.Join(tmpDir, ".config", "opencode", "plugins", "tldt-advisory.js")
	if oc.pluginDest != wantPlugin {
		t.Errorf("opencode pluginDest = %q, want %q", oc.pluginDest, wantPlugin)
	}
	if oc.skillDest == "" {
		t.Error("opencode target should still install the skill")
	}
	if oc.hookDest != "" {
		t.Error("opencode target should not register a shell hook")
	}
}

func TestResolveTargets_CursorHasNoPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".cursor"), 0755); err != nil {
		t.Fatalf("creating cursor dir: %v", err)
	}

	targets, err := resolveTargets(tmpDir, Options{})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	for _, tg := range targets {
		if tg.name == "cursor" && tg.pluginDest != "" {
			t.Errorf("cursor should stay skill-only, got pluginDest %q", tg.pluginDest)
		}
	}
}
