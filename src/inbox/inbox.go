package inbox

import (
	"fmt"
	"sort"

	"cunc/src/contacts"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Email struct {
	ID      string
	From    string
	Subject string
	Body    string
}

type Model struct {
	emails        []Email
	selected      int
	page          int
	emailsPerPage int
	width         int
	height        int
	viewingEmail  bool
	Action        string // "select" or "quit"

	// async fetch control
	Loading   bool
	fetchUser string
	fetchPass string
	fetchMax  int

	// contact history
	contactHistory *contacts.ContactHistory
}

func InitialModel(emails []Email, emailsPerPage int, loading bool, fetchUser, fetchPass string, fetchMax int) Model {
	// Sort emails by ID descending (most recent first)
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].ID > emails[j].ID
	})

	return Model{
		emails:         emails,
		selected:       0,
		page:           0,
		emailsPerPage:  emailsPerPage,
		width:          80,
		height:         24,
		viewingEmail:   false,
		Loading:        loading,
		fetchUser:      fetchUser,
		fetchPass:      fetchPass,
		fetchMax:       fetchMax,
		contactHistory: contacts.Load(),
	}
}

func (m Model) Init() tea.Cmd {
	if m.Loading && m.fetchUser != "" && m.fetchPass != "" {
		return FetchEmailsCmd(m.fetchUser, m.fetchPass, m.fetchMax)
	}
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
			fmt.Print("\033[2J")
			m.Action = "quit"
			return m, tea.Quit

		case "ctrl+r":
			// Refresh inbox
			if m.fetchUser != "" && m.fetchPass != "" {
				m.Loading = true
				return m, FetchEmailsCmd(m.fetchUser, m.fetchPass, m.fetchMax)
			}

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			} else if m.page > 0 {
				m.page--
				m.selected = m.emailsPerPage - 1
			}

		case "down", "j":
			if m.selected < m.currentPageCount()-1 {
				m.selected++
			} else if (m.page+1)*m.emailsPerPage < len(m.emails) {
				m.page++
				m.selected = 0
			}

		case "left", "h":
			if m.page > 0 {
				m.page--
				m.selected = 0
			}

		case "right", "l":
			maxPage := (len(m.emails) - 1) / m.emailsPerPage
			if m.page < maxPage {
				m.page++
				m.selected = 0
			}

		case "enter":
			if !m.viewingEmail && len(m.emails) > 0 {
				m.viewingEmail = true
			} else if m.viewingEmail {
				m.viewingEmail = false
			}
		}
	}

	// Handle async fetch completion
	switch v := msg.(type) {
	case EmailsFetchedMsg:
		m.Loading = false
		if v.Err != nil || len(v.Emails) == 0 {
			// leave emails as-is (caller may have provided fallback)
		} else {
			// replace emails with fetched ones and reload contact history
			m.emails = v.Emails
			// ensure sorted newest-first
			sort.Slice(m.emails, func(i, j int) bool { return m.emails[i].ID > m.emails[j].ID })
			m.contactHistory = contacts.Load()
			// Reset to first page
			m.page = 0
			m.selected = 0
		}
	}

	return m, nil
}

func (m Model) View() string {
	// Stylized title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5")).
		Padding(0, 2).
		Render("  C U N C  ")

	var body string
	if m.Loading {
		body = "Fetching emails..."
	} else if m.viewingEmail {
		email := m.getSelectedEmail()
		if email != nil {
			body = fmt.Sprintf("From: %s\nSubject: %s\n\n%s",
				email.From, email.Subject, email.Body)
		} else {
			body = "No email selected."
		}
	} else {
		body = m.renderEmailList()
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(max(m.height-6, 3)).
		Render(body)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("↑/k up • ↓/j down • h/l prev/next page • enter view • ctrl+r refresh • ctrl+q quit")

	return lipgloss.JoinVertical(lipgloss.Left, title, box, footer)
}

// renderEmailList shows the current page of emails
func (m Model) renderEmailList() string {
	start := m.page * m.emailsPerPage
	end := start + m.emailsPerPage
	if end > len(m.emails) {
		end = len(m.emails)
	}

	emails := m.emails[start:end]
	var lines string
	for i, email := range emails {
		line := fmt.Sprintf("  From: %s | Subject: %s", email.From, email.Subject)
		
		// Check if this is from a known contact
		isKnown := m.contactHistory != nil && m.contactHistory.IsKnown(email.From)
		
		if i == m.selected {
			// Selected email style
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4"))
			
			if isKnown {
				// Add an indicator for known contacts when selected
				line = "★ " + line
			}
			line = style.Render(line)
		} else if isKnown {
			// Known contact - highlight in yellow
			line = "★ " + line
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")) // yellow
			line = style.Render(line)
		} else {
			line = lipgloss.NewStyle().Render(line)
		}

		if i > 0 {
			lines += "\n"
		}
		lines += line
	}

	// Add page indicator
	totalPages := (len(m.emails) + m.emailsPerPage - 1) / m.emailsPerPage
	if totalPages > 1 {
		lines += fmt.Sprintf("\n\nPage %d/%d", m.page+1, totalPages)
	}

	return lines
}

func (m Model) getSelectedEmail() *Email {
	idx := m.page*m.emailsPerPage + m.selected
	if idx >= 0 && idx < len(m.emails) {
		return &m.emails[idx]
	}
	return nil
}

func (m Model) currentPageCount() int {
	remaining := len(m.emails) - m.page*m.emailsPerPage
	if remaining > m.emailsPerPage {
		return m.emailsPerPage
	}
	return remaining
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
