package drafts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"cunc/src/title"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Draft struct {
	ID          string
	To          string
	Subject     string
	Body        string
	Attachments []string
}

type Model struct {
	drafts            []Draft
	selected          int
	width             int
	height            int
	err               error
	Action            string // "select" or "quit"
	confirming_delete bool
	title_animator    title.Animator
}

func InitialModel() Model {
	drafts_list := load_drafts()
	return Model{
		drafts:         drafts_list,
		selected:       0,
		width:          80,
		height:         24,
		title_animator: title.New(),
	}
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

		case "ctrl+q":
			fmt.Print("\033[2J")
			m.Action = "quit"
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.drafts)-1 {
				m.selected++
			}

		case "enter":
			if len(m.drafts) > 0 {
				m.Action = "select"
				return m, tea.Quit
			}

		case "d":
			if len(m.drafts) > 0 {
				m.confirming_delete = true
			}

		case "y":
			if m.confirming_delete {
				if draft := m.GetSelectedDraft(); draft != nil {
					delete_draft_file(draft.ID)
					m.drafts = load_drafts()
					if m.selected >= len(m.drafts) && m.selected > 0 {
						m.selected--
					}
				}
				m.confirming_delete = false
			}

		case "n":
			m.confirming_delete = false
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.confirming_delete {
		if draft := m.GetSelectedDraft(); draft != nil {
			confirm_text := fmt.Sprintf("Delete draft '%s'? (y/n)", draft.Subject)
			return lipgloss.NewStyle().
				Padding(2).
				Render(confirm_text)
		}
	}

	// Render animated title
	titleView := m.title_animator.Render()

	if len(m.drafts) == 0 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(m.width - 2).
			Height(max(m.height-6, 3))

		content := box.Render("No drafts found")

		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 1).
			Width(m.width - 2).
			Render("ctrl+q quit")

		return lipgloss.JoinVertical(lipgloss.Left, titleView, content, footer)
	}

	var draft_lines string
	for i, draft := range m.drafts {
		line := fmt.Sprintf("  To: %s | Subject: %s", draft.To, draft.Subject)

		if i == m.selected {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4")).
				Render(line)
		} else {
			line = lipgloss.NewStyle().Render(line)
		}

		if i > 0 {
			draft_lines += "\n"
		}
		draft_lines += line
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(max(m.height-6, 3))

	content := box.Render(draft_lines)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("↑/k up • ↓/j down • enter select • d delete • ctrl+q quit")

	return lipgloss.JoinVertical(lipgloss.Left, titleView, content, footer)
}

func (m Model) GetSelectedDraft() *Draft {
	if m.selected >= 0 && m.selected < len(m.drafts) {
		return &m.drafts[m.selected]
	}
	return nil
}

func load_drafts() []Draft {
	draft_dir := get_draft_dir()
	var drafts []Draft

	entries, err := os.ReadDir(draft_dir)
	if err != nil {
		return drafts
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		draft_path := filepath.Join(draft_dir, entry.Name())
		data, err := os.ReadFile(draft_path)
		if err != nil {
			continue
		}

		var draft_data map[string]interface{}
		if err := json.Unmarshal(data, &draft_data); err != nil {
			continue
		}

		draft_id := entry.Name()[:len(entry.Name())-5] // Remove .json extension

		// Extract attachments if present
		var attachments []string
		if attachmentsData, ok := draft_data["attachments"].([]interface{}); ok {
			for _, a := range attachmentsData {
				if str, ok := a.(string); ok {
					attachments = append(attachments, str)
				}
			}
		}

		drafts = append(drafts, Draft{
			ID:          draft_id,
			To:          getStringField(draft_data, "to"),
			Subject:     getStringField(draft_data, "subject"),
			Body:        getStringField(draft_data, "body"),
			Attachments: attachments,
		})
	}

	// Sort by ID (most recent first)
	sort.Slice(drafts, func(i, j int) bool {
		return drafts[i].ID > drafts[j].ID
	})

	return drafts
}

func get_draft_dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_drafts"
	}
	return filepath.Join(home, ".local", "share", "cunc", "drafts")
}

func delete_draft_file(draft_id string) error {
	draft_dir := get_draft_dir()
	draft_path := filepath.Join(draft_dir, draft_id+".json")
	return os.Remove(draft_path)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getStringField(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}
