package contacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ContactEntry represents a contact that the user has emailed
type ContactEntry struct {
	Email      string    `json:"email"`
	LastUsed   time.Time `json:"last_used"`
	UsageCount int       `json:"usage_count"`
}

// ContactHistory manages the list of known contacts
type ContactHistory struct {
	Contacts []ContactEntry `json:"contacts"`
}

// getContactsPath returns the path to the contacts file
func getContactsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_contacts.json"
	}
	return filepath.Join(home, ".local", "share", "cunc", "contacts.json")
}

// Load reads the contact history from disk
func Load() *ContactHistory {
	path := getContactsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist yet, return empty history
		return &ContactHistory{Contacts: []ContactEntry{}}
	}

	var history ContactHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return &ContactHistory{Contacts: []ContactEntry{}}
	}

	return &history
}

// Save writes the contact history to disk
func (h *ContactHistory) Save() error {
	path := getContactsPath()
	dir := filepath.Dir(path)
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// AddContact adds or updates a contact in the history
func (h *ContactHistory) AddContact(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}

	// Find existing contact
	for i := range h.Contacts {
		if strings.ToLower(h.Contacts[i].Email) == email {
			h.Contacts[i].LastUsed = time.Now()
			h.Contacts[i].UsageCount++
			return
		}
	}

	// Add new contact
	h.Contacts = append(h.Contacts, ContactEntry{
		Email:      email,
		LastUsed:   time.Now(),
		UsageCount: 1,
	})
}

// IsKnown checks if an email address is in the contact history
func (h *ContactHistory) IsKnown(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, contact := range h.Contacts {
		if strings.ToLower(contact.Email) == email {
			return true
		}
	}
	return false
}

// GetSuggestions returns contact emails matching the given prefix
// Results are sorted by usage count (most used first), then by recency
func (h *ContactHistory) GetSuggestions(prefix string) []string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		// Return all contacts sorted by usage
		return h.getAllSorted()
	}

	var matches []ContactEntry
	for _, contact := range h.Contacts {
		if strings.HasPrefix(strings.ToLower(contact.Email), prefix) {
			matches = append(matches, contact)
		}
	}

	// Sort by usage count (descending), then by last used (descending)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].UsageCount != matches[j].UsageCount {
			return matches[i].UsageCount > matches[j].UsageCount
		}
		return matches[i].LastUsed.After(matches[j].LastUsed)
	})

	suggestions := make([]string, len(matches))
	for i, match := range matches {
		suggestions[i] = match.Email
	}

	return suggestions
}

// getAllSorted returns all contacts sorted by usage
func (h *ContactHistory) getAllSorted() []string {
	contacts := make([]ContactEntry, len(h.Contacts))
	copy(contacts, h.Contacts)

	sort.Slice(contacts, func(i, j int) bool {
		if contacts[i].UsageCount != contacts[j].UsageCount {
			return contacts[i].UsageCount > contacts[j].UsageCount
		}
		return contacts[i].LastUsed.After(contacts[j].LastUsed)
	})

	result := make([]string, len(contacts))
	for i, contact := range contacts {
		result[i] = contact.Email
	}
	return result
}
