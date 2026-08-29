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
//
// Rare/Common are single-token vocabulary markers (Kobak et al. method).
// Phrases are literal, case-insensitive multi-word tells matched against the raw
// text ("it's important to note"), which the word tokenizer cannot catch because
// it splits on apostrophes and spaces. Templates are regex patterns for
// structural tics with variable slots ("not just X, but Y"), matched
// case-insensitively.
type wordlist struct {
	Lang      string   `json:"lang"`
	Source    string   `json:"source"`
	Rare      []string `json:"rare"`
	Common    []string `json:"common"`
	Phrases   []string `json:"phrases"`
	Templates []string `json:"templates"`
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
