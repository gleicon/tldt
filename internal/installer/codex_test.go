package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCodexPluginTree_WritesPluginLayout(t *testing.T) {
	marketplaceDir := filepath.Join(t.TempDir(), "tldt-marketplace")
	if err := writeCodexPluginTree(marketplaceDir); err != nil {
		t.Fatalf("writeCodexPluginTree: unexpected error: %v", err)
	}

	pluginDir := filepath.Join(marketplaceDir, "plugins", "tldt")
	want := []string{
		filepath.Join(marketplaceDir, ".agents", "plugins", "marketplace.json"),
		filepath.Join(pluginDir, ".codex-plugin", "plugin.json"),
		filepath.Join(pluginDir, "hooks.json"),
		filepath.Join(pluginDir, "hooks", "tldt-hook.sh"),
		filepath.Join(pluginDir, "skills", "tldt", "SKILL.md"),
	}
	for _, p := range want {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %q: %v", p, err)
		}
	}
}

func TestWriteCodexPluginTree_HookIsExecutableAndAdvisory(t *testing.T) {
	marketplaceDir := filepath.Join(t.TempDir(), "tldt-marketplace")
	if err := writeCodexPluginTree(marketplaceDir); err != nil {
		t.Fatalf("writeCodexPluginTree: unexpected error: %v", err)
	}

	hookPath := filepath.Join(marketplaceDir, "plugins", "tldt", "hooks", "tldt-hook.sh")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook is not executable, mode=%v", info.Mode())
	}
}

// The Codex hooks.json must use the wrapped format ({"hooks":{"UserPromptSubmit":...}})
// with a plugin-root-relative command — distinct from Claude's flat settings.json.
func TestWriteCodexPluginTree_HooksJSONWrappedAndRelative(t *testing.T) {
	marketplaceDir := filepath.Join(t.TempDir(), "tldt-marketplace")
	if err := writeCodexPluginTree(marketplaceDir); err != nil {
		t.Fatalf("writeCodexPluginTree: unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(marketplaceDir, "plugins", "tldt", "hooks.json"))
	if err != nil {
		t.Fatalf("reading hooks.json: %v", err)
	}
	var parsed struct {
		Hooks struct {
			UserPromptSubmit []struct {
				Hooks []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"UserPromptSubmit"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v", err)
	}
	ups := parsed.Hooks.UserPromptSubmit
	if len(ups) != 1 || len(ups[0].Hooks) != 1 {
		t.Fatalf("expected one UserPromptSubmit hook entry, got %+v", ups)
	}
	h := ups[0].Hooks[0]
	if h.Type != "command" {
		t.Errorf("hook type = %q, want \"command\"", h.Type)
	}
	if h.Command != "./hooks/tldt-hook.sh" {
		t.Errorf("hook command = %q, want plugin-root-relative \"./hooks/tldt-hook.sh\"", h.Command)
	}
}

func TestCodexBaseDir_Precedence(t *testing.T) {
	home := "/home/user"
	t.Run("config-dir beats env", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "/env/codex")
		got := codexBaseDir(home, Options{ConfigDir: "/flag/codex"})
		if got != "/flag/codex" {
			t.Errorf("got %q, want /flag/codex", got)
		}
	})
	t.Run("env beats default", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "/env/codex")
		got := codexBaseDir(home, Options{})
		if got != "/env/codex" {
			t.Errorf("got %q, want /env/codex", got)
		}
	})
	t.Run("default", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "")
		got := codexBaseDir(home, Options{})
		if want := filepath.Join(home, ".codex"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestCodexTargeted(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want bool
	}{
		{"default run", Options{}, true},
		{"target all", Options{Target: "all"}, true},
		{"target codex", Options{Target: "codex"}, true},
		{"target claude excludes codex", Options{Target: "claude"}, false},
		{"target opencode excludes codex", Options{Target: "opencode"}, false},
		{"skill-dir excludes codex", Options{SkillDir: "/x"}, false},
		{"project excludes codex", Options{Project: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := codexTargeted(tt.opts); got != tt.want {
				t.Errorf("codexTargeted(%+v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
	}
}
