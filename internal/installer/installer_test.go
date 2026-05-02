package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillFile_WritesFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-installer-skill-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	destPath := filepath.Join(tmpDir, "skills", "tldt", "SKILL.md")
	if err := installSkillFile(destPath); err != nil {
		t.Fatalf("installSkillFile: unexpected error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading installed SKILL.md: %v", err)
	}
	if len(data) == 0 {
		t.Error("installed SKILL.md is empty")
	}
	if !strings.Contains(string(data), "name: tldt") {
		t.Error("SKILL.md missing 'name: tldt' frontmatter")
	}
}

func TestInstallSkillFile_MkdirAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-installer-mkdirall-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	destPath := filepath.Join(tmpDir, "a", "b", "c", "SKILL.md")
	if err := installSkillFile(destPath); err != nil {
		t.Errorf("installSkillFile(deep path): unexpected error: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("SKILL.md not found at deep path: %v", err)
	}
}

func TestInstallHookFile_WritesExecutable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-installer-hook-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	destPath := filepath.Join(tmpDir, "hooks", "tldt-hook.sh")
	if err := installHookFile(destPath); err != nil {
		t.Fatalf("installHookFile: unexpected error: %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat hook file: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook file is not executable, mode=%v", info.Mode())
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading installed hook: %v", err)
	}
	if !strings.Contains(string(data), "tldt --print-threshold") {
		t.Error("hook missing 'tldt --print-threshold'")
	}
	if !strings.Contains(string(data), "tldt --verbose") {
		t.Error("hook missing 'tldt --verbose'")
	}
}

func TestPatchSettingsJSON_CreatesFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-create-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	hookCmd := "/usr/local/bin/tldt-hook.sh"

	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("PatchSettingsJSON (create): %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading created settings.json: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("settings.json is not valid JSON after patch: %v", err)
	}

	if !strings.Contains(string(data), hookCmd) {
		t.Errorf("settings.json missing hookCmd %q", hookCmd)
	}
	if !strings.Contains(string(data), "UserPromptSubmit") {
		t.Error("settings.json missing 'UserPromptSubmit' key")
	}
}

func TestPatchSettingsJSON_MergesExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-merge-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	existing := `{"someKey":"someValue","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/other/hook.sh"}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("writing existing settings: %v", err)
	}

	hookCmd := "/usr/local/bin/tldt-hook.sh"
	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("PatchSettingsJSON (merge): %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading merged settings.json: %v", err)
	}

	// Must preserve existing keys
	if !strings.Contains(string(data), "someKey") {
		t.Error("settings.json lost 'someKey' after merge")
	}
	if !strings.Contains(string(data), "PreToolUse") {
		t.Error("settings.json lost 'PreToolUse' after merge")
	}
	// Must add new hook
	if !strings.Contains(string(data), "UserPromptSubmit") {
		t.Error("settings.json missing 'UserPromptSubmit' after merge")
	}
	if !strings.Contains(string(data), hookCmd) {
		t.Errorf("settings.json missing hookCmd %q after merge", hookCmd)
	}
}

func TestPatchSettingsJSON_Idempotent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-idempotent-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	hookCmd := "/usr/local/bin/tldt-hook.sh"

	// First patch
	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("first PatchSettingsJSON: %v", err)
	}

	// Second patch — must be no-op
	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("second PatchSettingsJSON: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	// hookCmd must appear exactly once
	count := strings.Count(string(data), hookCmd)
	if count != 1 {
		t.Errorf("hookCmd appears %d times in settings.json, want exactly 1 (idempotency)", count)
	}
}

func TestResolveTargets_AlwaysIncludesClaude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-targets-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	targets := resolveTargets(tmpDir, Options{})
	if len(targets) == 0 {
		t.Fatal("resolveTargets returned empty list")
	}
	if targets[0].name != "claude" {
		t.Errorf("first target name = %q, want \"claude\"", targets[0].name)
	}
	expectedSkill := filepath.Join(tmpDir, ".claude", "skills", "tldt", "SKILL.md")
	if targets[0].skillDest != expectedSkill {
		t.Errorf("claude skillDest = %q, want %q", targets[0].skillDest, expectedSkill)
	}
}

func TestResolveTargets_SkillDirOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-skilldir-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	customDir := filepath.Join(tmpDir, "custom-skills")
	targets := resolveTargets(tmpDir, Options{SkillDir: customDir})

	if len(targets) != 1 {
		t.Errorf("SkillDir override: got %d targets, want 1", len(targets))
	}
	if targets[0].name != "custom" {
		t.Errorf("SkillDir override target name = %q, want \"custom\"", targets[0].name)
	}
	expected := filepath.Join(customDir, "tldt", "SKILL.md")
	if targets[0].skillDest != expected {
		t.Errorf("SkillDir override skillDest = %q, want %q", targets[0].skillDest, expected)
	}
	if targets[0].hookDest != "" {
		t.Error("SkillDir override should have no hookDest")
	}
}

func TestResolveTargets_DetectsOptionalApps(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-optional-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Create .cursor dir to simulate Cursor being installed
	if err := os.MkdirAll(filepath.Join(tmpDir, ".cursor"), 0755); err != nil {
		t.Fatalf("creating .cursor dir: %v", err)
	}

	targets := resolveTargets(tmpDir, Options{})
	found := false
	for _, t2 := range targets {
		if t2.name == "cursor" {
			found = true
			if t2.hookDest != "" {
				t.Error("cursor target should have no hookDest (only Claude Code supports UserPromptSubmit)")
			}
		}
	}
	if !found {
		t.Error("cursor target not found even though ~/.cursor dir exists")
	}
}
