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
	contact_history *contacts.ContactHistory
	suggestions     []string
	selected_sugg   int
	showing_sugg    bool

	// Typo detection fields
	typos                []string
	last_typo_check_time time.Time
	typo_check_interval  time.Duration
	last_subject_value   string
	last_body_value      string
	max_typos_to_show    int

	// Error display
	last_send_error string
	showing_error   bool

	// Attachments
	attachments          []string
	selected_attachment  int
	showing_file_picker  bool
	file_picker_path     string
	file_picker_files    []os.FileInfo
	file_picker_selected int
	file_picker_input    textinput.Model
	last_attachment_path string
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
	max_typos := 3 // default
	if max_typos_str := sett.GetSetting("max typos displayed"); max_typos_str != "" {
		if parsed, err := strconv.Atoi(max_typos_str); err == nil && parsed > 0 {
			max_typos = parsed
		}
	}

	home_dir, _ := os.UserHomeDir()
	if home_dir == "" {
		home_dir = "/"
	}

	file_picker_input := textinput.New()
	file_picker_input.Placeholder = "Type to filter files..."

	return Model{
		to:                   to,
		subject:              subject,
		body:                 body,
		focus:                0,
		width:                80,
		height:               24,
		draft_id:             fmt.Sprintf("%d", time.Now().UnixNano()),
		confirming_send:      false,
		contact_history:      contacts.Load(),
		suggestions:          []string{},
		selected_sugg:        0,
		showing_sugg:         false,
		typos:                []string{},
		last_typo_check_time: time.Now(),
		typo_check_interval:  1 * time.Second,
		last_subject_value:   "",
		last_body_value:      "",
		max_typos_to_show:    max_typos,
		last_send_error:      "",
		showing_error:        false,
		attachments:          []string{},
		selected_attachment:  0,
		showing_file_picker:  false,
		file_picker_path:     home_dir,
		file_picker_files:    []os.FileInfo{},
		file_picker_selected: 0,
		file_picker_input:    file_picker_input,
		last_attachment_path: home_dir,
	}
}

func LoadDraft(draft_id, to, subject, body string, attachments []string) Model {
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
	max_typos := 3 // default
	if max_typos_str := sett.GetSetting("max typos displayed"); max_typos_str != "" {
		if parsed, err := strconv.Atoi(max_typos_str); err == nil && parsed > 0 {
			max_typos = parsed
		}
	}

	home_dir, _ := os.UserHomeDir()
	if home_dir == "" {
		home_dir = "/"
	}

	// Determine last attachment path from attachments
	last_path := home_dir
	if len(attachments) > 0 {
		last_path = filepath.Dir(attachments[len(attachments)-1])
	}

	file_picker_input := textinput.New()
	file_picker_input.Placeholder = "Type to filter files..."

	return Model{
		to:                   to_input,
		subject:              subject_input,
		body:                 body_input,
		focus:                0,
		width:                80,
		height:               24,
		draft_id:             draft_id,
		confirming_send:      false,
		contact_history:      contacts.Load(),
		suggestions:          []string{},
		selected_sugg:        0,
		showing_sugg:         false,
		typos:                []string{},
		last_typo_check_time: time.Now(),
		typo_check_interval:  1 * time.Second,
		last_subject_value:   subject,
		last_body_value:      body,
		max_typos_to_show:    max_typos,
		last_send_error:      "",
		showing_error:        false,
		attachments:          attachments,
		selected_attachment:  0,
		showing_file_picker:  false,
		file_picker_path:     last_path,
		file_picker_files:    []os.FileInfo{},
		file_picker_selected: 0,
		file_picker_input:    file_picker_input,
		last_attachment_path: last_path,
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

	draft := map[string]interface{}{
		"to":          m.GetTo(),
		"subject":     m.GetSubject(),
		"body":        m.GetBody(),
		"attachments": m.attachments,
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
	sett := settings.InitialModel()
	draft_dir := sett.GetSetting("drafts directory")

	// Expand ~ to home directory
	if strings.HasPrefix(draft_dir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			draft_dir = filepath.Join(home, draft_dir[2:])
		}
	}

	// Fallback if setting is empty
	if draft_dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ".cunc_drafts"
		}
		return filepath.Join(home, ".local", "share", "cunc", "drafts")
	}

	return draft_dir
}

func (m *Model) SendEmail() bool {
	// Check if we should prevent sending with typos
	sett := settings.InitialModel()
	preventWithTypos := sett.GetSetting("prevent send with typos") == "y"

	// Check for typos in subject and body
	subjectTypos := CheckTypos(m.GetSubject(), m.max_typos_to_show)
	bodyTypos := CheckTypos(m.GetBody(), m.max_typos_to_show)
	hasTypos := len(subjectTypos) > 0 || len(bodyTypos) > 0

	if preventWithTypos && hasTypos {
		m.last_send_error = "Email contains typos in subject/body.\n\nPlease fix them or change\n'prevent send with typos' to 'n' in settings."
		return false
	}

	err := sending.Send(m.GetTo(), m.GetSubject(), m.GetBody(), m.attachments)
	if err != nil {
		m.last_send_error = "Failed to send email:\n\n" + err.Error()
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
		// Handle file picker
		if m.showing_file_picker {
			switch msg.String() {
			case "ctrl+q", "esc":
				m.showing_file_picker = false
				m.file_picker_input.Blur()
				return m, nil
			case "up", "k":
				if m.file_picker_selected > 0 {
					m.file_picker_selected--
				}
				return m, nil
			case "down", "j":
				filtered := m.get_filtered_files()
				if m.file_picker_selected < len(filtered)-1 {
					m.file_picker_selected++
				}
				return m, nil
			case "enter":
				m.select_file_picker_item()
				return m, nil
			case "backspace":
				if m.file_picker_input.Value() == "" {
					m.file_picker_go_up()
					return m, nil
				}
			}
			// Update filter input
			var cmd tea.Cmd
			m.file_picker_input, cmd = m.file_picker_input.Update(msg)
			// Reset selection when filter changes
			m.file_picker_selected = 0
			return m, cmd
		}

		// Handle error dismissal
		if m.showing_error {
			m.showing_error = false
			m.last_send_error = ""
			return m, nil
		}

		handled := false
		switch msg.String() {

		case "ctrl+q":
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "ctrl+a":
			if !m.confirming_send {
				m.open_file_picker()
				handled = true
			}

		case "ctrl+f":
			if !m.confirming_send && len(m.attachments) > 0 {
				m.focus = 3 // Focus on attachments
				handled = true
			}

		case "delete", "backspace", "x":
			if m.focus == 3 && len(m.attachments) > 0 {
				// Remove selected attachment
				if m.selected_attachment >= 0 && m.selected_attachment < len(m.attachments) {
					m.attachments = append(m.attachments[:m.selected_attachment], m.attachments[m.selected_attachment+1:]...)
					if m.selected_attachment >= len(m.attachments) && m.selected_attachment > 0 {
						m.selected_attachment--
					}
					if len(m.attachments) == 0 {
						m.focus = 0
						m.Refocus()
					}
				}
				handled = true
			}

		case "tab":
			if m.showing_sugg && m.focus == 0 && len(m.suggestions) > 0 {
				// Select current suggestion
				m.to.SetValue(m.suggestions[m.selected_sugg])
				m.showing_sugg = false
				m.suggestions = []string{}
				handled = true
			} else if !m.confirming_send {
				maxFocus := 2
				if len(m.attachments) > 0 {
					maxFocus = 3
				}
				m.focus = (m.focus + 1) % (maxFocus + 1)
				if m.focus < 3 {
					m.Refocus()
				}
				m.showing_sugg = false
				handled = true
			}

		case "shift+tab":
			if !m.confirming_send {
				maxFocus := 2
				if len(m.attachments) > 0 {
					maxFocus = 3
				}
				m.focus = (m.focus - 1 + maxFocus + 1) % (maxFocus + 1)
				if m.focus < 3 {
					m.Refocus()
				}
				m.showing_sugg = false
				handled = true
			}

		case "esc":
			if m.showing_sugg {
				m.showing_sugg = false
				m.suggestions = []string{}
				handled = true
			} else if m.focus == 3 {
				m.focus = 0
				m.Refocus()
				handled = true
			}

		case "down":
			if m.showing_sugg && m.focus == 0 {
				m.selected_sugg = (m.selected_sugg + 1) % len(m.suggestions)
				handled = true
			} else if m.focus == 3 && len(m.attachments) > 1 {
				m.selected_attachment = (m.selected_attachment + 1) % len(m.attachments)
				handled = true
			}

		case "up":
			if m.showing_sugg && m.focus == 0 {
				m.selected_sugg = (m.selected_sugg - 1 + len(m.suggestions)) % len(m.suggestions)
				handled = true
			} else if m.focus == 3 && len(m.attachments) > 1 {
				m.selected_attachment = (m.selected_attachment - 1 + len(m.attachments)) % len(m.attachments)
				handled = true
			}

		case "enter":
			if m.showing_sugg && m.focus == 0 && len(m.suggestions) > 0 {
				// Select current suggestion
				m.to.SetValue(m.suggestions[m.selected_sugg])
				m.showing_sugg = false
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
				m.showing_error = true
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
			m.update_suggestions()
		}
	case 1:
		m.subject, cmd = m.subject.Update(msg)
	case 2:
		m.body, cmd = m.body.Update(msg)
	case 3:
		// Focus on attachments, no input to update
		return m, nil
	}

	// Check for typos periodically if subject or body changed
	if time.Since(m.last_typo_check_time) > m.typo_check_interval {
		if m.focus == 1 && m.subject.Value() != m.last_subject_value {
			m.typos = CheckTypos(m.subject.Value(), m.max_typos_to_show)
			m.last_subject_value = m.subject.Value()
			m.last_typo_check_time = time.Now()
		} else if m.focus == 2 && m.body.Value() != m.last_body_value {
			m.typos = CheckTypos(m.body.Value(), m.max_typos_to_show)
			m.last_body_value = m.body.Value()
			m.last_typo_check_time = time.Now()
		}
	}

	return m, cmd
}

func (m *Model) update_suggestions() {
	toValue := m.to.Value()
	if toValue == "" {
		m.showing_sugg = false
		m.suggestions = []string{}
		return
	}

	if m.contact_history == nil {
		m.showing_sugg = false
		m.suggestions = []string{}
		return
	}

	suggestions := m.contact_history.GetSuggestions(toValue)
	if len(suggestions) > 0 {
		m.suggestions = suggestions
		if len(m.suggestions) > 5 {
			m.suggestions = m.suggestions[:5] // Limit to 5 suggestions
		}
		m.showing_sugg = true
		m.selected_sugg = 0
	} else {
		m.showing_sugg = false
		m.suggestions = []string{}
	}
}

func (m Model) View() string {
	// Show file picker if active
	if m.showing_file_picker {
		return m.render_file_picker()
	}

	if m.showing_error {
		error_box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(60).
			BorderForeground(lipgloss.Color("1")).
			Render(m.last_send_error + "\n\n(press any key to dismiss)")

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
	toField := "To:\n" + m.build_to_field_with_colors()

	if m.showing_sugg && len(m.suggestions) > 0 {
		suggStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 2)

		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Background(lipgloss.Color("4")).
			Padding(0, 2)

		toField += "\n"
		for i, sugg := range m.suggestions {
			if i == m.selected_sugg {
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
		sett := settings.InitialModel()
		unicode_support := sett.GetSetting("unicode support") == "y"
		typo_indicator := "X"
		if unicode_support {
			typo_indicator = "✗"
		}
		subjectField += "\n" + typoStyle.Render(typo_indicator+" Typos: "+strings.Join(m.typos, ", "))
	}

	bodyField := m.body.View()
	if m.focus == 2 && len(m.typos) > 0 {
		typoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // Red
		sett := settings.InitialModel()
		unicode_support := sett.GetSetting("unicode support") == "y"
		typo_indicator := "X"
		if unicode_support {
			typo_indicator = "✗"
		}
		bodyField += "\n" + typoStyle.Render(typo_indicator+" Typos: "+strings.Join(m.typos, ", "))
	}

	// Build attachments section
	attachmentsField := m.render_attachments()

	contentStr := toField +
		"\n\nSubject:\n" + subjectField +
		"\n\nBody:\n" + bodyField

	if attachmentsField != "" {
		contentStr += "\n\n" + attachmentsField
	}

	content := box.Render(contentStr)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render(m.getFooterText())

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m Model) getFooterText() string {
	if len(m.attachments) > 0 {
		return "ctrl+q quit • tab navigate • ctrl+a add attachment • ctrl+f manage attachments • ctrl+s save • ctrl+y send"
	}
	return "ctrl+q quit • tab navigate • ctrl+a add attachment • ctrl+s save draft • ctrl+y send"
}

func (m Model) build_to_field_with_colors() string {
	toValue := m.to.Value()

	// If not focused, show with colors; if focused, show editor with colors overlaid
	if m.focus != 0 {
		// Not focused on to field, show with colors
		return m.colorize_to_field(toValue)
	}

	// Focused on to field, show the input box
	return m.to.View()
}

func (m Model) colorize_to_field(toValue string) string {
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

// File picker helper functions
func (m *Model) open_file_picker() {
	m.showing_file_picker = true
	m.file_picker_path = m.last_attachment_path
	m.load_file_picker_directory()
	m.file_picker_selected = 0
	m.file_picker_input.SetValue("")
	m.file_picker_input.Focus()
}

func (m *Model) load_file_picker_directory() {
	entries, err := os.ReadDir(m.file_picker_path)
	if err != nil {
		m.file_picker_files = []os.FileInfo{}
		return
	}

	m.file_picker_files = []os.FileInfo{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		m.file_picker_files = append(m.file_picker_files, info)
	}
}

func (m *Model) get_filtered_files() []os.FileInfo {
	filter_text := strings.ToLower(m.file_picker_input.Value())
	if filter_text == "" {
		return m.file_picker_files
	}

	var filtered []os.FileInfo
	for _, f := range m.file_picker_files {
		if strings.Contains(strings.ToLower(f.Name()), filter_text) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func (m *Model) select_file_picker_item() {
	filtered := m.get_filtered_files()
	if m.file_picker_selected >= len(filtered) {
		return
	}

	selected := filtered[m.file_picker_selected]
	if selected.IsDir() {
		// Navigate into directory
		m.file_picker_path = filepath.Join(m.file_picker_path, selected.Name())
		m.load_file_picker_directory()
		m.file_picker_selected = 0
		m.file_picker_input.SetValue("")
	} else {
		// Add file as attachment
		full_path := filepath.Join(m.file_picker_path, selected.Name())
		m.attachments = append(m.attachments, full_path)
		m.last_attachment_path = m.file_picker_path
		m.showing_file_picker = false
	}
}

func (m *Model) file_picker_go_up() {
	parent := filepath.Dir(m.file_picker_path)
	if parent != m.file_picker_path {
		m.file_picker_path = parent
		m.load_file_picker_directory()
		m.file_picker_selected = 0
		m.file_picker_input.SetValue("")
	}
}

func (m Model) render_attachments() string {
	if len(m.attachments) == 0 {
		return ""
	}

	// Check unicode support
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"
	attachment_icon := "[A]"
	if unicode_support {
		attachment_icon = "📎"
	}

	attachStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")) // Cyan

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Background(lipgloss.Color("4"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	var result string

	// Always show list of attachments with filenames
	result = attachStyle.Render(fmt.Sprintf("Attachments (%d):", len(m.attachments))) + "\n"

	for i, path := range m.attachments {
		fileName := filepath.Base(path)
		var line string

		if m.focus == 3 {
			// When focused, show with removal indicator
			if i == m.selected_attachment {
				line = fmt.Sprintf("  [x] %s %s", attachment_icon, fileName)
				result += selectedStyle.Render(line) + "\n"
			} else {
				line = fmt.Sprintf("  [ ] %s %s", attachment_icon, fileName)
				result += attachStyle.Render(line) + "\n"
			}
		} else {
			// When not focused, show simple list
			line = fmt.Sprintf("  %s %s", attachment_icon, fileName)
			result += attachStyle.Render(line) + "\n"
		}
	}

	if m.focus == 3 {
		result += hintStyle.Render("  ↑/↓ select • x/delete/backspace remove • esc exit")
	} else if len(m.attachments) > 0 {
		result += hintStyle.Render("  (press ctrl+f to manage)")
	}

	return result
}

func (m Model) render_file_picker() string {
	filtered := m.get_filtered_files()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6"))

	dirStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Select File to Attach") + "\n")
	content.WriteString(normalStyle.Render("Current: "+m.file_picker_path) + "\n\n")
	content.WriteString("Filter: " + m.file_picker_input.View() + "\n\n")

	// Show files
	maxVisible := 15
	startIdx := 0
	if m.file_picker_selected >= maxVisible {
		startIdx = m.file_picker_selected - maxVisible + 1
	}

	for i := startIdx; i < len(filtered) && i < startIdx+maxVisible; i++ {
		f := filtered[i]
		displayName := f.Name()
		if f.IsDir() {
			displayName += "/"
		}

		if i == m.file_picker_selected {
			if f.IsDir() {
				content.WriteString(selectedStyle.Render("→ " + displayName))
			} else {
				content.WriteString(selectedStyle.Render("→ " + displayName))
			}
		} else {
			if f.IsDir() {
				content.WriteString(dirStyle.Render("  " + displayName))
			} else {
				content.WriteString(normalStyle.Render("  " + displayName))
			}
		}
		content.WriteString("\n")
	}

	if len(filtered) == 0 {
		content.WriteString(normalStyle.Render("(no files match filter)") + "\n")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 4).
		Height(m.height - 4)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Render("↑/↓ navigate • enter select • backspace go up • esc cancel")

	boxed := box.Render(content.String())
	return lipgloss.JoinVertical(lipgloss.Left, boxed, footer)
}
