package drafts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Draft struct {
	ID      string
	To      string
	Subject string
	Body    string
}

type Model struct {
	drafts   []Draft
	selected int
	width    int
	height   int
	err      error
}

func InitialModel() Model {
	drafts := loadDrafts()
	return Model{
		drafts:   drafts,
		selected: 0,
		width:    80,
		height:   24,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+q":
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
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if len(m.drafts) == 0 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(m.width - 2).
			Height(m.height - 6)

		content := box.Render("No drafts found")

		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 1).
			Width(m.width - 2).
			Render("q quit")

		return lipgloss.JoinVertical(lipgloss.Left, content, footer)
	}

	var draftLines string
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
			draftLines += "\n"
		}
		draftLines += line
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(m.height - 6)

	content := box.Render(draftLines)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("↑/k up • ↓/j down • enter select • ctrl+q quit")

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m Model) GetSelectedDraft() *Draft {
	if m.selected >= 0 && m.selected < len(m.drafts) {
		return &m.drafts[m.selected]
	}
	return nil
}

func loadDrafts() []Draft {
	draftDir := getDraftDir()
	var drafts []Draft

	entries, err := os.ReadDir(draftDir)
	if err != nil {
		return drafts
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		draftPath := filepath.Join(draftDir, entry.Name())
		data, err := os.ReadFile(draftPath)
		if err != nil {
			continue
		}

		var draftData map[string]string
		if err := json.Unmarshal(data, &draftData); err != nil {
			continue
		}

		draftID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		drafts = append(drafts, Draft{
			ID:      draftID,
			To:      draftData["to"],
			Subject: draftData["subject"],
			Body:    draftData["body"],
		})
	}

	// Sort by ID (most recent first)
	sort.Slice(drafts, func(i, j int) bool {
		return drafts[i].ID > drafts[j].ID
	})

	return drafts
}

func getDraftDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cunc_drafts"
	}
	return filepath.Join(home, ".local", "share", "cunc", "drafts")
}
