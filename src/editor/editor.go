package editor

import (
	"cunc/src/contacts"
	"cunc/src/sending"
	"cunc/src/settings"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	to              textinput.Model
	subject         textinput.Model
	body            textarea.Model
	focus           int
	width           int
	height          int
	draft_id        string
	last_save       time.Time
	confirming_send bool

	// Autocomplete fields
	contactHistory *contacts.ContactHistory
	suggestions    []string
	selectedSugg   int
	showingSugg    bool

	// Typo detection fields
	typos             []string
	lastTypoCheckTime time.Time
	typoCheckInterval time.Duration
	lastSubjectValue  string
	lastBodyValue     string
	maxTyposToShow    int

	// Error display
	lastSendError string
	showingError  bool
}

func ComposeFrom(m Model) {
	p := tea.NewProgram(InitialModel())
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}

func Compose() {
	ComposeFrom(InitialModel())
}

func InitialModel() Model {
	to := textinput.New()
	to.Placeholder = "Recipient email"
	to.Focus()

	subject := textinput.New()
	subject.Placeholder = "Subject"

	body := textarea.New()
	body.Placeholder = "Write your message..."

	sett := settings.InitialModel()
	maxTypos := 3 // default
	if maxTyposStr := sett.GetSetting("max typos displayed"); maxTyposStr != "" {
		if parsed, err := strconv.Atoi(maxTyposStr); err == nil && parsed > 0 {
			maxTypos = parsed
		}
	}

	return Model{
		to:                to,
		subject:           subject,
		body:              body,
		focus:             0,
		width:             80,
		height:            24,
		draft_id:          fmt.Sprintf("%d", time.Now().UnixNano()),
		confirming_send:   false,
		contactHistory:    contacts.Load(),
		suggestions:       []string{},
		selectedSugg:      0,
		showingSugg:       false,
		typos:             []string{},
		lastTypoCheckTime: time.Now(),
		typoCheckInterval: 1 * time.Second,
		lastSubjectValue:  "",
		lastBodyValue:     "",
		maxTyposToShow:    maxTypos,
		lastSendError:     "",
		showingError:      false,
	}
}

func LoadDraft(draft_id, to, subject, body string) Model {
	to_input := textinput.New()
	to_input.SetValue(to)
	to_input.Placeholder = "Recipient email"
	to_input.Focus()

	subject_input := textinput.New()
	subject_input.SetValue(subject)
	subject_input.Placeholder = "Subject"

	body_input := textarea.New()
	body_input.SetValue(body)
	body_input.Placeholder = "Write your message..."

	sett := settings.InitialModel()
	maxTypos := 3 // default
	if maxTyposStr := sett.GetSetting("max typos displayed"); maxTyposStr != "" {
		if parsed, err := strconv.Atoi(maxTyposStr); err == nil && parsed > 0 {
			maxTypos = parsed
		}
	}

	return Model{
		to:                to_input,
		subject:           subject_input,
		body:              body_input,
		focus:             0,
		width:             80,
		height:            24,
		draft_id:          draft_id,
		confirming_send:   false,
		contactHistory:    contacts.Load(),
		suggestions:       []string{},
		selectedSugg:      0,
		showingSugg:       false,
		typos:             []string{},
		lastTypoCheckTime: time.Now(),
		typoCheckInterval: 1 * time.Second,
		lastSubjectValue:  subject,
		lastBodyValue:     body,
		maxTyposToShow:    maxTypos,
		lastSendError:     "",
		showingError:      false,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Refocus() {
	m.to.Blur()
	m.subject.Blur()
	m.body.Blur()

	switch m.focus {
	case 0:
		m.to.Focus()
	case 1:
		m.subject.Focus()
	case 2:
		m.body.Focus()
	}
}

func (m *Model) SaveAsDraft() {
	draft_dir := get_draft_dir()
	if err := os.MkdirAll(draft_dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating draft directory: %v\n", err)
		return
	}

	draft := map[string]string{
		"to":      m.GetTo(),
		"subject": m.GetSubject(),
		"body":    m.GetBody(),
	}

	draft_path := filepath.Join(draft_dir, m.draft_id+".json")
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling draft: %v\n", err)
		return
	}

	if err := os.WriteFile(draft_path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving draft: %v\n", err)
		return
	}

	m.last_save = time.Now()
}

func get_draft_dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_drafts"
	}
	return filepath.Join(home, ".local", "share", "cunc", "drafts")
}

func (m *Model) SendEmail() bool {
	// Check if we should prevent sending with typos
	sett := settings.InitialModel()
	preventWithTypos := sett.GetSetting("prevent send with typos") == "y"

	// Check for typos in subject and body
	subjectTypos := CheckTypos(m.GetSubject(), m.maxTyposToShow)
	bodyTypos := CheckTypos(m.GetBody(), m.maxTyposToShow)
	hasTypos := len(subjectTypos) > 0 || len(bodyTypos) > 0

	if preventWithTypos && hasTypos {
		m.lastSendError = "Email contains typos in subject/body.\n\nPlease fix them or change\n'prevent send with typos' to 'n' in settings."
		return false
	}

	err := sending.Send(m.GetTo(), m.GetSubject(), m.GetBody())
	if err != nil {
		m.lastSendError = "Failed to send email:\n\n" + err.Error()
		return false
	}

	// Delete draft if setting is enabled
	shouldDeleteDraft := sett.GetSetting("should delete draft on send") == "y"
	if shouldDeleteDraft {
		draft_dir := get_draft_dir()
		draft_path := filepath.Join(draft_dir, m.draft_id+".json")
		if err := os.Remove(draft_path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not delete draft file: %v\n", err)
		}
	}

	return true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.to.Width = msg.Width - 4
		m.subject.Width = msg.Width - 4
		m.body.SetWidth(msg.Width - 4)
		m.body.SetHeight(msg.Height - 12)

	case tea.KeyMsg:
		// Handle error dismissal
		if m.showingError {
			m.showingError = false
			m.lastSendError = ""
			return m, nil
		}

		handled := false
		switch msg.String() {

		case "ctrl+q":
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "tab":
			if m.showingSugg && m.focus == 0 && len(m.suggestions) > 0 {
				// Select current suggestion
				m.to.SetValue(m.suggestions[m.selectedSugg])
				m.showingSugg = false
				m.suggestions = []string{}
				handled = true
			} else if !m.confirming_send {
				m.focus = (m.focus + 1) % 3
				m.Refocus()
				m.showingSugg = false
				handled = true
			}

		case "shift+tab":
			if !m.confirming_send {
				m.focus = (m.focus - 1 + 3) % 3
				m.Refocus()
				m.showingSugg = false
				handled = true
			}

		case "esc":
			if m.showingSugg {
				m.showingSugg = false
				m.suggestions = []string{}
				handled = true
			}

		case "down":
			if m.showingSugg && m.focus == 0 {
				m.selectedSugg = (m.selectedSugg + 1) % len(m.suggestions)
				handled = true
			}

		case "up":
			if m.showingSugg && m.focus == 0 {
				m.selectedSugg = (m.selectedSugg - 1 + len(m.suggestions)) % len(m.suggestions)
				handled = true
			}

		case "enter":
			if m.showingSugg && m.focus == 0 && len(m.suggestions) > 0 {
				// Select current suggestion
				m.to.SetValue(m.suggestions[m.selectedSugg])
				m.showingSugg = false
				m.suggestions = []string{}
				handled = true
			}

		case "ctrl+s":
			if !m.confirming_send {
				m.SaveAsDraft()
				fmt.Print("\033[2J")
				return m, tea.Quit
			}

		case "ctrl+y":
			if !m.confirming_send {
				m.confirming_send = true
			} else {
				handled = true
			}

		case "y":
			if m.confirming_send {
				if m.SendEmail() {
					fmt.Print("\033[2J")
					return m, tea.Quit
				}
				// If send failed, show error
				m.confirming_send = false
				m.showingError = true
				handled = true
			}

		case "n":
			if m.confirming_send {
				m.confirming_send = false
				handled = true
			}
		}

		if handled {
			return m, nil
		}
	}

	var cmd tea.Cmd

	// Store old value of "to" field to detect changes
	oldToValue := m.to.Value()

	switch m.focus {
	case 0:
		m.to, cmd = m.to.Update(msg)
		// Update autocomplete suggestions if "to" field changed
		if m.to.Value() != oldToValue {
			m.updateSuggestions()
		}
	case 1:
		m.subject, cmd = m.subject.Update(msg)
	case 2:
		m.body, cmd = m.body.Update(msg)
	}

	// Check for typos periodically if subject or body changed
	if time.Since(m.lastTypoCheckTime) > m.typoCheckInterval {
		if m.focus == 1 && m.subject.Value() != m.lastSubjectValue {
			m.typos = CheckTypos(m.subject.Value(), m.maxTyposToShow)
			m.lastSubjectValue = m.subject.Value()
			m.lastTypoCheckTime = time.Now()
		} else if m.focus == 2 && m.body.Value() != m.lastBodyValue {
			m.typos = CheckTypos(m.body.Value(), m.maxTyposToShow)
			m.lastBodyValue = m.body.Value()
			m.lastTypoCheckTime = time.Now()
		}
	}

	return m, cmd
}

func (m *Model) updateSuggestions() {
	toValue := m.to.Value()
	if toValue == "" {
		m.showingSugg = false
		m.suggestions = []string{}
		return
	}

	if m.contactHistory == nil {
		m.showingSugg = false
		m.suggestions = []string{}
		return
	}

	suggestions := m.contactHistory.GetSuggestions(toValue)
	if len(suggestions) > 0 {
		m.suggestions = suggestions
		if len(m.suggestions) > 5 {
			m.suggestions = m.suggestions[:5] // Limit to 5 suggestions
		}
		m.showingSugg = true
		m.selectedSugg = 0
	} else {
		m.showingSugg = false
		m.suggestions = []string{}
	}
}

func (m Model) View() string {
	if m.showingError {
		error_box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(60).
			BorderForeground(lipgloss.Color("1")).
			Render(m.lastSendError + "\n\n(press any key to dismiss)")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, error_box)
	}

	if m.confirming_send {
		confirm_box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(40).
			BorderForeground(lipgloss.Color("3")).
			Render("Send email to " + m.to.Value() + "?\n\n(y/n)")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, confirm_box)
	}

	// Build the content with autocomplete suggestions and colored cc/bcc if showing
	toField := "To:\n" + m.buildToFieldWithColors()

	if m.showingSugg && len(m.suggestions) > 0 {
		suggStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 2)

		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Background(lipgloss.Color("4")).
			Padding(0, 2)

		toField += "\n"
		for i, sugg := range m.suggestions {
			if i == m.selectedSugg {
				toField += selectedStyle.Render("→ "+sugg) + "\n"
			} else {
				toField += suggStyle.Render("  "+sugg) + "\n"
			}
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(m.height - 6)

	subjectField := m.subject.View()
	if m.focus == 1 && len(m.typos) > 0 {
		typoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // Red
		subjectField += "\n" + typoStyle.Render("✗ Typos: "+strings.Join(m.typos, ", "))
	}

	bodyField := m.body.View()
	if m.focus == 2 && len(m.typos) > 0 {
		typoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // Red
		bodyField += "\n" + typoStyle.Render("✗ Typos: "+strings.Join(m.typos, ", "))
	}

	content := box.Render(
		toField +
			"\n\nSubject:\n" + subjectField +
			"\n\nBody:\n" + bodyField,
	)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("ctrl+q quit • tab/shift+tab navigate • ctrl+s save as draft • ctrl+y send email")

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m Model) buildToFieldWithColors() string {
	toValue := m.to.Value()

	// If not focused, show with colors; if focused, show editor with colors overlaid
	if m.focus != 0 {
		// Not focused on to field, show with colors
		return m.colorizeToField(toValue)
	}

	// Focused on to field, show the input box
	return m.to.View()
}

func (m Model) colorizeToField(toValue string) string {
	if toValue == "" {
		return m.to.View()
	}

	// If there's no cc: or bcc: markers, just return the value as-is
	if !strings.Contains(toValue, "cc:") && !strings.Contains(toValue, "bcc:") {
		return toValue
	}

	ccStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // Green
	bccStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // Magenta

	var result strings.Builder
	remaining := toValue

	// Process segments: find cc: and bcc: markers and color accordingly
	for len(remaining) > 0 {
		// Look for next cc: or bcc:
		ccIdx := strings.Index(remaining, "cc:")
		bccIdx := strings.Index(remaining, "bcc:")

		// Determine which marker comes first
		var nextIdx int
		var markerLen int
		var isCC bool

		switch {
		case ccIdx == -1 && bccIdx == -1:
			// No more markers, append the rest
			result.WriteString(remaining)
			remaining = ""
			continue
		case ccIdx == -1:
			// Only bcc: found
			nextIdx = bccIdx
			markerLen = 4
			isCC = false
		case bccIdx == -1:
			// Only cc: found
			nextIdx = ccIdx
			markerLen = 3
			isCC = true
		case ccIdx < bccIdx:
			// cc: comes first
			nextIdx = ccIdx
			markerLen = 3
			isCC = true
		default:
			// bcc: comes first
			nextIdx = bccIdx
			markerLen = 4
			isCC = false
		}

		// Append text before the marker (uncolored)
		if nextIdx > 0 {
			result.WriteString(remaining[:nextIdx])
		}

		// Skip the marker
		if nextIdx+markerLen > len(remaining) {
			// Safety check: marker is at the end
			remaining = remaining[nextIdx:]
			result.WriteString(remaining)
			break
		}

		remaining = remaining[nextIdx+markerLen:]

		// Find where this segment ends (at the next cc:, bcc:, or end of string)
		nextCCIdx := strings.Index(remaining, "cc:")
		nextBCCIdx := strings.Index(remaining, "bcc:")

		segmentEnd := len(remaining)
		if nextCCIdx >= 0 && nextBCCIdx >= 0 {
			segmentEnd = minInt(nextCCIdx, nextBCCIdx)
		} else if nextCCIdx >= 0 {
			segmentEnd = nextCCIdx
		} else if nextBCCIdx >= 0 {
			segmentEnd = nextBCCIdx
		}

		segment := remaining[:segmentEnd]
		remaining = remaining[segmentEnd:]

		// Trim and colorize the segment
		trimmedSegment := strings.TrimSpace(segment)

		if trimmedSegment != "" {
			// Color and append the segment
			if isCC {
				result.WriteString(ccStyle.Render("cc: " + trimmedSegment))
			} else {
				result.WriteString(bccStyle.Render("bcc: " + trimmedSegment))
			}
		}

		// Add spacing between sections if there's more
		if len(remaining) > 0 {
			result.WriteString("  ")
		}
	}

	return result.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m Model) GetTo() string {
	return m.to.Value()
}

func (m Model) GetSubject() string {
	return m.subject.Value()
}

func (m Model) GetBody() string {
	return m.body.Value()
}
