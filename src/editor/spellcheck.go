package editor

import (
	"strings"
	"sync"

	"github.com/trustmaster/go-aspell"
)

var (
	speller     aspell.Speller
	spellerOnce sync.Once
	spellerErr  error
)

// initSpeller initializes the aspell speller (singleton pattern)
func initSpeller() (aspell.Speller, error) {
	spellerOnce.Do(func() {
		speller, spellerErr = aspell.NewSpeller(map[string]string{
			"lang": "en_US",
		})
	})
	return speller, spellerErr
}

// CheckTypos extracts words and checks spelling using aspell
func CheckTypos(text string, maxTyposToShow int) []string {
	// Initialize speller
	speller, err := initSpeller()
	if err != nil {
		// If aspell is not available, return empty list (fail gracefully)
		return []string{}
	}

	// Extract words and check spelling
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '\'' || r == '-')
	})

	var typos []string
	seenWords := make(map[string]bool) // Avoid duplicate typos

	for _, word := range words {
		// Only check words longer than 2 characters
		if len(word) <= 2 {
			continue
		}

		// Skip if we've already checked this word
		lower := strings.ToLower(word)
		if seenWords[lower] {
			continue
		}
		seenWords[lower] = true

		// Skip proper nouns (words starting with capital letter in the middle of text)
		if isLikelyProperNoun(word) {
			continue
		}

		// Check spelling with aspell
		if !speller.Check(word) {
			typos = append(typos, word)
			if len(typos) >= maxTyposToShow {
				break
			}
		}
	}

	return typos
}

func isLikelyProperNoun(word string) bool {
	// Words starting with capital letter might be proper nouns
	if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
		return true
	}
	return false
}
