package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DefaultSettings defines all available settings with their default values
var DefaultSettings = map[string]string{
	"email":    "",
	"password": "",
}

// SensitiveFields marks which settings should not be saved to the JSON file
var SensitiveFields = map[string]bool{
	"password": true,
}

type Model struct {
	settings    map[string]string
	fields      []string // ordered list of field names
	focused     int
	width       int
	height      int
	edit_values map[int]*textinput.Model // input models for each field
}

func InitialModel() Model {
	settings_map := load_settings()
	fields := get_sorted_keys(settings_map)

	edit_values := make(map[int]*textinput.Model)
	for i, field := range fields {
		input := textinput.New()
		input.SetValue(settings_map[field])
		if i == 0 {
			input.Focus()
		}
		// Mask password field
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

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) GetSetting(key string) string {
	if value, exists := m.settings[key]; exists {
		return value
	}
	return ""
}

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
			if m.focused > 0 {
				m.edit_values[m.focused].Blur()
				m.focused--
				m.edit_values[m.focused].Focus()
			}

		case "down":
			if m.focused < len(m.fields)-1 {
				m.edit_values[m.focused].Blur()
				m.focused++
				m.edit_values[m.focused].Focus()
			}
		}
	}

	// Handle editing of focused field
	if m.focused < len(m.fields) && m.edit_values[m.focused] != nil {
		var cmd tea.Cmd
		*m.edit_values[m.focused], cmd = m.edit_values[m.focused].Update(msg)
		field_name := m.fields[m.focused]
		m.settings[field_name] = m.edit_values[m.focused].Value()
		return m, cmd
	}

	return m, nil
}

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
		if i == m.focused && m.edit_values[i] != nil {
			// Show the focused field with its input view (including cursor)
			line = fmt.Sprintf("  %s: %s", field, m.edit_values[i].View())
		} else {
			// Show non-focused fields as plain text
			value := ""
			if m.edit_values[i] != nil {
				if field == "password" {
					// Show masked password
					password_value := m.edit_values[i].Value()
					value = strings.Repeat("*", len(password_value))
				} else {
					value = m.edit_values[i].Value()
				}
			}
			line = fmt.Sprintf("  %s: %s", field, value)
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

func (m *Model) save_settings() {
	settings_dir := get_settings_dir()
	if err := os.MkdirAll(settings_dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating settings directory: %v\n", err)
		return
	}

	// Only save non-sensitive defined settings
	settings_to_save := make(map[string]string)
	for k := range DefaultSettings {
		if !SensitiveFields[k] {
			if v, exists := m.settings[k]; exists {
				settings_to_save[k] = v
			}
		}
	}

	settings_path := filepath.Join(settings_dir, "settings.json")
	data, err := json.MarshalIndent(settings_to_save, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling settings: %v\n", err)
		return
	}

	if err := os.WriteFile(settings_path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
		return
	}

	// Save sensitive fields to a restricted file (0600: owner read/write only)
	sensitive_to_save := make(map[string]string)
	for field := range SensitiveFields {
		if value, exists := m.settings[field]; exists {
			sensitive_to_save[field] = value
		}
	}

	sensitive_path := filepath.Join(settings_dir, "sensitive.json")
	sensitive_data, err := json.MarshalIndent(sensitive_to_save, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling sensitive settings: %v\n", err)
		return
	}

	if err := os.WriteFile(sensitive_path, sensitive_data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving sensitive settings: %v\n", err)
		return
	}
}

func load_settings() map[string]string {
	// Start with defaults
	settings_map := make(map[string]string)
	for k, v := range DefaultSettings {
		settings_map[k] = v
	}

	// Merge with saved settings from JSON
	settings_dir := get_settings_dir()
	settings_path := filepath.Join(settings_dir, "settings.json")

	data, err := os.ReadFile(settings_path)
	if err == nil {
		var saved_settings map[string]string
		if err := json.Unmarshal(data, &saved_settings); err == nil {
			// Override defaults with saved values (only for defined keys)
			for k, v := range saved_settings {
				if _, exists := DefaultSettings[k]; exists && !SensitiveFields[k] {
					settings_map[k] = v
				}
			}
		}
	}

	// Load sensitive fields from restricted file
	sensitive_path := filepath.Join(settings_dir, "sensitive.json")
	sensitive_data, err := os.ReadFile(sensitive_path)
	if err == nil {
		var sensitive_settings map[string]string
		if err := json.Unmarshal(sensitive_data, &sensitive_settings); err == nil {
			for field, value := range sensitive_settings {
				if SensitiveFields[field] {
					settings_map[field] = value
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

func get_sorted_keys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
