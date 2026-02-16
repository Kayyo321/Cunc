package editor

import (
	"strings"
	"sync"

	"github.com/trustmaster/go-aspell"
)

var (
	speller      aspell.Speller
	speller_once sync.Once
	speller_err  error
)

// init_speller initializes the aspell speller (singleton pattern)
func init_speller() (aspell.Speller, error) {
	speller_once.Do(func() {
		speller, speller_err = aspell.NewSpeller(map[string]string{
			"lang": "en_US",
		})
	})
	return speller, speller_err
}

// CheckTypos extracts words and checks spelling using aspell
func CheckTypos(text string, max_typos_to_show int) []string {
	// Initialize speller
	speller, err := init_speller()
	if err != nil {
		// If aspell is not available, return empty list (fail gracefully)
		return []string{}
	}

	// Extract words and check spelling
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '\'' || r == '-')
	})

	var typos []string
	seen_words := make(map[string]bool) // Avoid duplicate typos

	for _, word := range words {
		// Only check words longer than 2 characters
		if len(word) <= 2 {
			continue
		}

		// Skip if we've already checked this word
		lower := strings.ToLower(word)
		if seen_words[lower] {
			continue
		}
		seen_words[lower] = true

		// Skip proper nouns (words starting with capital letter in the middle of text)
		if is_likely_proper_noun(word) {
			continue
		}

		// Check spelling with aspell
		if !speller.Check(word) {
			typos = append(typos, word)
			if len(typos) >= max_typos_to_show {
				break
			}
		}
	}

	return typos
}

func is_likely_proper_noun(word string) bool {
	// Words starting with capital letter might be proper nouns
	if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
		return true
	}
	return false
}
