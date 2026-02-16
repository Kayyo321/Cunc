package director

import (
	"fmt"

	"cunc/src/settings"
	"cunc/src/title"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	options        []Option
	selected       int
	width          int
	height         int
	Action         string // "inbox", "drafts", "settings", "compose", "quit"
	title_animator title.Animator
}

type Option struct {
	name        string
	description string
	action      string
}

func InitialModel() Model {
	// Check unicode support setting
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"

	options := []Option{
		{
			name:        get_inbox_name(unicode_support),
			description: "View and manage your emails",
			action:      "inbox",
		},
		{
			name:        get_compose_name(unicode_support),
			description: "Write a new email",
			action:      "compose",
		},
		{
			name:        get_drafts_name(unicode_support),
			description: "View and edit saved drafts",
			action:      "drafts",
		},
		{
			name:        get_settings_name(unicode_support),
			description: "Configure your email and preferences",
			action:      "settings",
		},
		{
			name:        get_quit_name(unicode_support),
			description: "Exit the application",
			action:      "quit",
		},
	}

	return Model{
		options:        options,
		selected:       0,
		width:          80,
		height:         24,
		Action:         "",
		title_animator: title.New(),
	}
}

func get_inbox_name(unicode_support bool) string {
	if unicode_support {
		return "📥 Inbox"
	}
	return "[Inbox]"
}

func get_compose_name(unicode_support bool) string {
	if unicode_support {
		return "✏️  Compose"
	}
	return "[Compose]"
}

func get_drafts_name(unicode_support bool) string {
	if unicode_support {
		return "📝 Drafts"
	}
	return "[Drafts]"
}

func get_settings_name(unicode_support bool) string {
	if unicode_support {
		return "⚙️  Settings"
	}
	return "[Settings]"
}

func get_quit_name(unicode_support bool) string {
	if unicode_support {
		return "🚪 Quit"
	}
	return "[Quit]"
}

func (m Model) Init() tea.Cmd {
	return title.TickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle title animation updates
	if cmd := m.title_animator.Update(msg); cmd != nil {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+q", "q":
			fmt.Print("\033[2J")
			m.Action = "quit"
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.options)-1 {
				m.selected++
			}

		case "enter", " ":
			m.Action = m.options[m.selected].action
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	// Render animated title
	titleView := m.title_animator.Render()

	// Welcome message
	welcomeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(1, 0, 0, 0)
	welcome := welcomeStyle.Render("Welcome! Select a mode to get started:")

	// Render options
	var optionsView string
	for i, option := range m.options {
		var line string
		if i == m.selected {
			// Selected style
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4")).
				Bold(true).
				Padding(0, 2).
				Width(m.width - 8).
				Render(fmt.Sprintf("%s\n  %s", option.name, option.description))
		} else {
			// Normal style
			nameStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true)
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))
			line = fmt.Sprintf("  %s\n  %s",
				nameStyle.Render(option.name),
				descStyle.Render(option.description))
		}

		if i > 0 {
			optionsView += "\n\n"
		}
		optionsView += line
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(max(m.height-8, 10)).
		Render(optionsView)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("↑/k up • ↓/j down • enter select • q/ctrl+q quit")

	return lipgloss.JoinVertical(lipgloss.Left, titleView, welcome, box, footer)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
