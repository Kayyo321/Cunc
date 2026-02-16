package contacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// contactentry represents a contact the user has emailed, shocking
type ContactEntry struct {
	Email      string    `json:"email"`
	LastUsed   time.Time `json:"last_used"`
	UsageCount int       `json:"usage_count"`
}

// contacthistory manages the list of known contacts, aka the usual suspects
type ContactHistory struct {
	Contacts []ContactEntry `json:"contacts"`
}

// getcontactspath returns the path to the contacts file, wherever it hides
func getContactsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_contacts.json"
	}
	return filepath.Join(home, ".local", "share", "cunc", "contacts.json")
}

// load reads the contact history from disk, if it exists
func Load() *ContactHistory {
	path := getContactsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &ContactHistory{Contacts: []ContactEntry{}}
	}

	var history ContactHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return &ContactHistory{Contacts: []ContactEntry{}}
	}

	return &history
}

// save writes the contact history to disk, fingers crossed
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

// addcontact adds or updates a contact in the history, so we remember
func (h *ContactHistory) AddContact(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}

	// find existing contact, because duplicates are boring
	for i := range h.Contacts {
		if strings.ToLower(h.Contacts[i].Email) == email {
			h.Contacts[i].LastUsed = time.Now()
			h.Contacts[i].UsageCount++
			return
		}
	}

	// add new contact, welcome to the list
	h.Contacts = append(h.Contacts, ContactEntry{
		Email:      email,
		LastUsed:   time.Now(),
		UsageCount: 1,
	})
}

// isknown checks if an email address is in the contact history
func (h *ContactHistory) IsKnown(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, contact := range h.Contacts {
		if strings.ToLower(contact.Email) == email {
			return true
		}
	}
	return false
}

// getsuggestions returns contact emails matching the given prefix
// results are sorted by usage count, then recency, because stats
func (h *ContactHistory) GetSuggestions(prefix string) []string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return h.getAllSorted()
	}

	var matches []ContactEntry
	for _, contact := range h.Contacts {
		if strings.HasPrefix(strings.ToLower(contact.Email), prefix) {
			matches = append(matches, contact)
		}
	}

	// sort by usage count, then last used, because order matters again
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

// getallsorted returns all contacts sorted by usage, surprise
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
