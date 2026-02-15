package editor

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	to      textinput.Model
	subject textinput.Model
	body    textarea.Model
	focus   int
	width   int
	height  int
}

func InitialModel() Model {
	to := textinput.New()
	to.Placeholder = "Recipient email"
	to.Focus()

	subject := textinput.New()
	subject.Placeholder = "Subject"

	body := textarea.New()
	body.Placeholder = "Write your message..."

	return Model{
		to:      to,
		subject: subject,
		body:    body,
		focus:   0,
		width:   80,
		height:  24,
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
		switch msg.String() {

		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.focus = (m.focus + 1) % 3
			m.Refocus()

		case "shift+tab":
			m.focus = (m.focus - 1 + 3) % 3
			m.Refocus()
		}
	}

	var cmd tea.Cmd

	switch m.focus {
	case 0:
		m.to, cmd = m.to.Update(msg)
	case 1:
		m.subject, cmd = m.subject.Update(msg)
	case 2:
		m.body, cmd = m.body.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(m.height - 2)

	return box.Render(
		"To:\n" + m.to.View() +
			"\n\nSubject:\n" + m.subject.View() +
			"\n\nBody:\n" + m.body.View(),
	)
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
