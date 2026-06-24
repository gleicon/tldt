package aidetect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"
)

//go:embed wordlists/en.json
var enJSON []byte

//go:embed wordlists/pt-BR.json
var ptBRJSON []byte

//go:embed wordlists/es.json
var esJSON []byte

// wordlist holds a language's marker sets.
type wordlist struct {
	Lang   string   `json:"lang"`
	Source string   `json:"source"`
	Rare   []string `json:"rare"`
	Common []string `json:"common"`
}

// SupportedLangs lists the languages with embedded wordlists.
var SupportedLangs = []string{"en", "pt-BR", "es"}

// loadWordlist returns the wordlist for lang. If overrideDir is non-empty,
// it looks for <overrideDir>/<lang>.json before falling back to the embedded list.
func loadWordlist(lang, overrideDir string) (wordlist, error) {
	if overrideDir != "" {
		path := filepath.Join(overrideDir, lang+".json")
		data, err := os.ReadFile(path)
		if err == nil {
			return parseWordlist(data)
		}
		// If the file doesn't exist, fall through to embedded.
	}

	switch strings.ToLower(strings.ReplaceAll(lang, "_", "-")) {
	case "en", "english":
		return parseWordlist(enJSON)
	case "pt-br", "pt_br", "portuguese":
		return parseWordlist(ptBRJSON)
	case "es", "spanish":
		return parseWordlist(esJSON)
	default:
		return wordlist{}, fmt.Errorf("aidetect: unsupported language %q; supported: %s", lang, strings.Join(SupportedLangs, ", "))
	}
}

func parseWordlist(data []byte) (wordlist, error) {
	var wl wordlist
	if err := json.Unmarshal(data, &wl); err != nil {
		return wordlist{}, fmt.Errorf("aidetect: parse wordlist: %w", err)
	}
	return wl, nil
}
