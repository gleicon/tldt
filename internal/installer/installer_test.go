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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

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
	if !strings.Contains(string(data), "tldt --detect-injection --detect-pii --detect-only") {
		t.Error("hook missing advisory invocation 'tldt --detect-injection --detect-pii --detect-only'")
	}
}

func TestPatchSettingsJSON_CreatesFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-create-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	hookCmd := "/usr/local/bin/tldt-hook.sh"

	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("PatchSettingsJSON (create): %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading created settings.json: %v", err)
	}

	var result map[string]any
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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

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

// TestPatchSettingsJSON_RejectsRelativeHookCmd pins G9: the doc says hookCmd MUST
// be absolute, so a relative path is rejected rather than silently registered.
func TestPatchSettingsJSON_RejectsRelativeHookCmd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-relhook-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	if err := PatchSettingsJSON(settingsPath, "relative/tldt-hook.sh"); err == nil {
		t.Error("PatchSettingsJSON: expected error for relative hookCmd, got nil")
	}
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("PatchSettingsJSON: must not create settings.json when hookCmd is invalid")
	}
}

// TestPatchSettingsJSON_RefusesMalformedHooks pins G8: when settings.json has a
// "hooks" key of an unexpected type, PatchSettingsJSON errors instead of silently
// overwriting the user's config.
func TestPatchSettingsJSON_RefusesMalformedHooks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-malformed-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	original := `{"hooks":"not-an-object"}`
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("writing settings.json: %v", err)
	}

	if err := PatchSettingsJSON(settingsPath, "/usr/local/bin/tldt-hook.sh"); err == nil {
		t.Error("PatchSettingsJSON: expected error for non-object \"hooks\", got nil")
	}
	// The user's file must be left untouched.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if string(data) != original {
		t.Errorf("PatchSettingsJSON clobbered malformed settings.json: got %q, want %q", data, original)
	}
}

// TestResolveTargets_ExplicitTargetMkdirFails pins G7: an explicit --target whose
// base directory cannot be created surfaces a loud error rather than a false success.
func TestResolveTargets_ExplicitTargetMkdirFails(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-mkdirfail-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// opencode's base dir is <home>/.config/opencode. Plant a regular file at
	// <home>/.config so MkdirAll of the opencode dir fails (a parent is a file).
	if err := os.WriteFile(filepath.Join(tmpDir, ".config"), []byte("x"), 0644); err != nil {
		t.Fatalf("planting .config file: %v", err)
	}

	if _, err := resolveTargets(tmpDir, Options{Target: "opencode"}); err == nil {
		t.Error("resolveTargets: expected error when explicit-target base dir can't be created, got nil")
	}
}

func TestResolveTargets_AlwaysIncludesClaude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-targets-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
	t.Setenv("CLAUDE_CONFIG_DIR", "") // isolate from ambient env so the default path is exercised

	targets, err := resolveTargets(tmpDir, Options{})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
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

func TestResolveTargets_HonorsClaudeConfigDirEnv(t *testing.T) {
	// AC-17: CLAUDE_CONFIG_DIR set and no --config-dir → artifacts under the env path.
	tmpDir := t.TempDir()
	envBase := filepath.Join(tmpDir, "cc-env")
	t.Setenv("CLAUDE_CONFIG_DIR", envBase)

	targets, err := resolveTargets(tmpDir, Options{Target: "claude"})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	wantSkill := filepath.Join(envBase, "skills", "tldt", "SKILL.md")
	if targets[0].skillDest != wantSkill {
		t.Errorf("skillDest = %q, want %q (CLAUDE_CONFIG_DIR honored)", targets[0].skillDest, wantSkill)
	}
	if want := filepath.Join(envBase, "hooks", "tldt-hook.sh"); targets[0].hookDest != want {
		t.Errorf("hookDest = %q, want %q", targets[0].hookDest, want)
	}
	if want := filepath.Join(envBase, "settings.json"); targets[0].settingsPath != want {
		t.Errorf("settingsPath = %q, want %q", targets[0].settingsPath, want)
	}
}

func TestResolveTargets_ConfigDirFlagBeatsEnv(t *testing.T) {
	// AC-18: --config-dir and CLAUDE_CONFIG_DIR both set → the flag wins.
	tmpDir := t.TempDir()
	flagBase := filepath.Join(tmpDir, "cc-flag")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmpDir, "cc-env"))

	targets, err := resolveTargets(tmpDir, Options{Target: "claude", ConfigDir: flagBase})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	wantSkill := filepath.Join(flagBase, "skills", "tldt", "SKILL.md")
	if targets[0].skillDest != wantSkill {
		t.Errorf("skillDest = %q, want %q (--config-dir must win over env)", targets[0].skillDest, wantSkill)
	}
}

func TestResolveTargets_SpecificOptionalExcludesClaude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-target-opencode-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	targets, err := resolveTargets(tmpDir, Options{Target: "opencode"})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("--target opencode: got %d targets, want 1: %+v", len(targets), targets)
	}
	if targets[0].name != "opencode" {
		t.Errorf("--target opencode: target name = %q, want \"opencode\"", targets[0].name)
	}
	for _, tg := range targets {
		if tg.name == "claude" {
			t.Error("--target opencode must not install Claude (no hook, no settings patch)")
		}
		if tg.hookDest != "" || tg.settingsPath != "" {
			t.Errorf("--target opencode: %q should have no hook/settings, got hook=%q settings=%q", tg.name, tg.hookDest, tg.settingsPath)
		}
	}
}

func TestResolveTargets_TargetClaudeOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-target-claude-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Even with an optional app present, --target claude installs only claude.
	if err := os.MkdirAll(filepath.Join(tmpDir, ".cursor"), 0755); err != nil {
		t.Fatalf("creating .cursor dir: %v", err)
	}
	targets, err := resolveTargets(tmpDir, Options{Target: "claude"})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	if len(targets) != 1 || targets[0].name != "claude" {
		t.Errorf("--target claude: got %+v, want exactly [claude]", targets)
	}
}

func TestResolveTargets_SkillDirOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-skilldir-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	customDir := filepath.Join(tmpDir, "custom-skills")
	targets, err := resolveTargets(tmpDir, Options{SkillDir: customDir})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}

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
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .cursor dir to simulate Cursor being installed
	if err := os.MkdirAll(filepath.Join(tmpDir, ".cursor"), 0755); err != nil {
		t.Fatalf("creating .cursor dir: %v", err)
	}

	targets, err := resolveTargets(tmpDir, Options{})
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
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
