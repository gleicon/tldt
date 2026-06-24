package aidetect

import (
	"os"
	"path/filepath"
	"testing"
)

// ── embedded wordlists ────────────────────────────────────────────────────────

func TestLoadWordlist_AllEmbeddedLangsParseClean(t *testing.T) {
	for _, lang := range SupportedLangs {
		wl, err := loadWordlist(lang, "")
		if err != nil {
			t.Errorf("loadWordlist(%q): %v", lang, err)
			continue
		}
		if len(wl.Rare) == 0 {
			t.Errorf("loadWordlist(%q): rare list is empty", lang)
		}
		if len(wl.Common) == 0 {
			t.Errorf("loadWordlist(%q): common list is empty", lang)
		}
		if wl.Lang == "" {
			t.Errorf("loadWordlist(%q): lang field is empty", lang)
		}
		if wl.Source == "" {
			t.Errorf("loadWordlist(%q): source field is empty", lang)
		}
	}
}

func TestLoadWordlist_LangAliases(t *testing.T) {
	aliases := map[string]string{
		"en":       "en",
		"english":  "en",
		"pt-br":    "pt-BR",
		"pt_br":    "pt-BR",
		"pt-BR":    "pt-BR",
		"es":       "es",
		"spanish":  "es",
	}
	for alias, wantLang := range aliases {
		wl, err := loadWordlist(alias, "")
		if err != nil {
			t.Errorf("loadWordlist(%q): unexpected error: %v", alias, err)
			continue
		}
		if wl.Lang != wantLang {
			t.Errorf("loadWordlist(%q): want lang=%q, got %q", alias, wantLang, wl.Lang)
		}
	}
}

func TestLoadWordlist_UnsupportedLangError(t *testing.T) {
	_, err := loadWordlist("fr", "")
	if err == nil {
		t.Fatal("expected error for unsupported lang 'fr'")
	}
}

// ── override dir ─────────────────────────────────────────────────────────────

func TestLoadWordlist_OverrideDirUsedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	custom := `{"lang":"en","source":"test","rare":["testmarker"],"common":["also"]}`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(custom), 0o644); err != nil {
		t.Fatalf("write custom wordlist: %v", err)
	}
	wl, err := loadWordlist("en", dir)
	if err != nil {
		t.Fatalf("loadWordlist with override dir: %v", err)
	}
	if len(wl.Rare) != 1 || wl.Rare[0] != "testmarker" {
		t.Errorf("expected custom rare=['testmarker'], got %v", wl.Rare)
	}
}

func TestLoadWordlist_OverrideDirMissingFileFallsBackToEmbedded(t *testing.T) {
	dir := t.TempDir() // exists but has no en.json
	wl, err := loadWordlist("en", dir)
	if err != nil {
		t.Fatalf("expected fallback to embedded, got error: %v", err)
	}
	// Embedded en.json has far more than 1 rare word.
	if len(wl.Rare) < 10 {
		t.Errorf("fallback: expected embedded wordlist with ≥10 rare words, got %d", len(wl.Rare))
	}
}

func TestLoadWordlist_MalformedCustomJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write malformed json: %v", err)
	}
	_, err := loadWordlist("en", dir)
	if err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
}

// ── Detect with override dir ──────────────────────────────────────────────────

func TestDetect_CustomWordlistDir(t *testing.T) {
	dir := t.TempDir()
	custom := `{"lang":"en","source":"test","rare":["xyzzy","frobnicate"],"common":[]}`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(custom), 0o644); err != nil {
		t.Fatalf("write custom wordlist: %v", err)
	}
	// Text with standard AI markers — should score 0 with custom wordlist.
	r, err := Detect("Delving into the intricate and multifaceted realm.", "en", dir)
	if err != nil {
		t.Fatalf("Detect with custom dir: %v", err)
	}
	if r.Score != 0 {
		t.Errorf("custom wordlist has no standard markers: want score=0, got %.4f (markers=%v)", r.Score, r.Markers)
	}
	// Text with custom markers should score > 0.
	r2, err := Detect("The function xyzzy returns a frobnicate value.", "en", dir)
	if err != nil {
		t.Fatalf("Detect with custom markers: %v", err)
	}
	if r2.Score == 0 {
		t.Error("custom markers not found: want score>0")
	}
}
