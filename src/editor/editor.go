package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	to        textinput.Model
	subject   textinput.Model
	body      textarea.Model
	focus     int
	width     int
	height    int
	draft_id  string
	last_save time.Time
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

	return Model{
		to:       to,
		subject:  subject,
		body:     body,
		focus:    0,
		width:    80,
		height:   24,
		draft_id: fmt.Sprintf("%d", time.Now().UnixNano()),
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

	return Model{
		to:       to_input,
		subject:  subject_input,
		body:     body_input,
		focus:    0,
		width:    80,
		height:   24,
		draft_id: draft_id,
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

func (m *Model) SendEmail() {

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

		case "ctrl+q":
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "tab":
			m.focus = (m.focus + 1) % 3
			m.Refocus()

		case "shift+tab":
			m.focus = (m.focus - 1 + 3) % 3
			m.Refocus()

		case "ctrl+s":
			m.SaveAsDraft()
			fmt.Print("\033[2J")
			return m, tea.Quit

		case "ctrl+enter":
			m.SendEmail()
			fmt.Print("\033[2J")
			return m, tea.Quit
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
		Height(m.height - 6)

	content := box.Render(
		"To:\n" + m.to.View() +
			"\n\nSubject:\n" + m.subject.View() +
			"\n\nBody:\n" + m.body.View(),
	)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("ctrl+q quit • tab/shift+tab navigate • ctrl+s save as draft • ctrl+enter send email")

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
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
