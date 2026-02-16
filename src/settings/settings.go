package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// default_settings holds default values for each setting (and headers)
var default_settings = map[string]string{
	"_header_Account":  "",
	"email":            "",
	"password":         "",
	"2fa app-password": "",

	"_header_Preferences":         "",
	"emails per page":             "40",
	"should delete draft on send": "n",
	"max typos displayed":         "3",
	"prevent send with typos":     "y",
}

// field_order defines the exact order in which headers and settings appear
var field_order = []string{
	"_header_Account",
	"email",
	"password",
	"2fa app-password",

	"_header_Preferences",
	"emails per page",
	"should delete draft on send",
	"max typos displayed",
	"prevent send with typos",
}

// sensitive_fields marks which settings should not be saved to normal JSON
var sensitive_fields = map[string]bool{
	"password": true,
}

// Model holds the settings UI state
type Model struct {
	settings    map[string]string
	fields      []string // ordered list including headers
	focused     int
	width       int
	height      int
	edit_values map[int]*textinput.Model
}

// InitialModel creates a new settings model
func InitialModel() Model {
	settings_map := load_settings()
	fields := field_order // preserve the order

	edit_values := make(map[int]*textinput.Model)

	// Automatically focus first non-header field
	focused_set := false

	for i, field := range fields {
		if strings.HasPrefix(field, "_header_") {
			continue
		}

		input := textinput.New()
		input.SetValue(settings_map[field])

		if !focused_set {
			input.Focus()
			focused_set = true
		}

		if field == "password" {
			input.EchoMode = textinput.EchoPassword
		}

		edit_values[i] = &input
	}

	return Model{
		settings:    settings_map,
		fields:      fields,
		focused:     0,
		width:       80,
		height:      24,
		edit_values: edit_values,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) GetSetting(key string) string {
	if value, exists := m.settings[key]; exists {
		return value
	}
	return ""
}

// Update handles UI events and navigation
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+q":
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "ctrl+s":
			m.save_settings()
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "up":
			for m.focused > 0 {
				if m.edit_values[m.focused] != nil {
					m.edit_values[m.focused].Blur()
				}
				m.focused--
				if !strings.HasPrefix(m.fields[m.focused], "_header_") {
					if m.edit_values[m.focused] != nil {
						m.edit_values[m.focused].Focus()
					}
					break
				}
			}

		case "down":
			for m.focused < len(m.fields)-1 {
				if m.edit_values[m.focused] != nil {
					m.edit_values[m.focused].Blur()
				}
				m.focused++
				if !strings.HasPrefix(m.fields[m.focused], "_header_") {
					if m.edit_values[m.focused] != nil {
						m.edit_values[m.focused].Focus()
					}
					break
				}
			}
		}
	}

	// Update value of focused field
	if m.focused < len(m.fields) && m.edit_values[m.focused] != nil {
		var cmd tea.Cmd
		*m.edit_values[m.focused], cmd = m.edit_values[m.focused].Update(msg)
		field_name := m.fields[m.focused]
		m.settings[field_name] = m.edit_values[m.focused].Value()
		return m, cmd
	}

	return m, nil
}

// View renders the settings UI
func (m Model) View() string {
	if len(m.fields) == 0 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(m.width - 2).
			Height(max(m.height-6, 3))
		content := box.Render("No settings configured.")
		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 1).
			Width(m.width - 2).
			Render("ctrl+s save • ctrl+q quit")
		return lipgloss.JoinVertical(lipgloss.Left, content, footer)
	}

	var settings_lines string
	for i, field := range m.fields {
		var line string

		if strings.HasPrefix(field, "_header_") {
			header_name := strings.TrimPrefix(field, "_header_")
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Bold(true).
				Render(header_name)
		} else {
			if i == m.focused && m.edit_values[i] != nil {
				line = fmt.Sprintf("  %s: %s", field, m.edit_values[i].View())
			} else {
				value := ""
				if m.edit_values[i] != nil {
					if field == "password" {
						value = strings.Repeat("*", len(m.edit_values[i].Value()))
					} else {
						value = m.edit_values[i].Value()
					}
				}
				line = fmt.Sprintf("  %s: %s", field, value)
			}
		}

		if i == m.focused {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4")).
				Render(line)
		}

		if i > 0 {
			settings_lines += "\n"
		}
		settings_lines += line
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(max(m.height-6, 3))
	content := box.Render(settings_lines)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("↑/↓ navigate • edit value • ctrl+s save • ctrl+q quit")

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

// save_settings saves both normal and sensitive fields
func (m *Model) save_settings() {
	settings_dir := get_settings_dir()
	_ = os.MkdirAll(settings_dir, 0755)

	// Save non-sensitive settings
	settings_to_save := make(map[string]string)
	for _, k := range field_order {
		if !sensitive_fields[k] && !strings.HasPrefix(k, "_header_") {
			if v, exists := m.settings[k]; exists {
				settings_to_save[k] = v
			}
		}
	}

	_ = os.WriteFile(filepath.Join(settings_dir, "settings.json"),
		must_marshal_indent(settings_to_save), 0644)

	// Save sensitive fields
	sensitive_to_save := make(map[string]string)
	for k := range sensitive_fields {
		if v, exists := m.settings[k]; exists {
			sensitive_to_save[k] = v
		}
	}

	_ = os.WriteFile(filepath.Join(settings_dir, "sensitive.json"),
		must_marshal_indent(sensitive_to_save), 0600)
}

// load_settings loads saved settings and merges with defaults
func load_settings() map[string]string {
	settings_map := make(map[string]string)
	for k, v := range default_settings {
		settings_map[k] = v
	}

	settings_dir := get_settings_dir()
	settings_path := filepath.Join(settings_dir, "settings.json")
	if data, err := os.ReadFile(settings_path); err == nil {
		var saved map[string]string
		if err := json.Unmarshal(data, &saved); err == nil {
			for k, v := range saved {
				if _, exists := default_settings[k]; exists && !sensitive_fields[k] && !strings.HasPrefix(k, "_header_") {
					settings_map[k] = v
				}
			}
		}
	}

	sensitive_path := filepath.Join(settings_dir, "sensitive.json")
	if data, err := os.ReadFile(sensitive_path); err == nil {
		var sensitive map[string]string
		if err := json.Unmarshal(data, &sensitive); err == nil {
			for k, v := range sensitive {
				if sensitive_fields[k] {
					settings_map[k] = v
				}
			}
		}
	}

	return settings_map
}

func get_settings_dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_settings"
	}
	return filepath.Join(home, ".local", "share", "cunc")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// must_marshal_indent is a small helper for json.MarshalIndent
func must_marshal_indent(v interface{}) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}
