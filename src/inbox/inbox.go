package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cunc/src/contacts"
	"cunc/src/settings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type Email struct {
	ID          string
	From        string
	Subject     string
	Body        string
	Attachments []Attachment
}

type Model struct {
	emails             []Email
	selected           int
	page               int
	emailsPerPage      int
	width              int
	height             int
	viewingEmail       bool
	viewingAttachments bool
	selectedAttachment int
	Action             string // "select" or "quit"

	// async fetch control
	Loading   bool
	fetchUser string
	fetchPass string
	fetchMax  int

	// contact history
	contactHistory *contacts.ContactHistory

	// download status
	downloadMessage string
	showingDownload bool
}

func InitialModel(emails []Email, emailsPerPage int, loading bool, fetchUser, fetchPass string, fetchMax int) Model {
	// Sort emails by ID descending (most recent first)
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].ID > emails[j].ID
	})

	return Model{
		emails:             emails,
		selected:           0,
		page:               0,
		emailsPerPage:      emailsPerPage,
		width:              80,
		height:             24,
		viewingEmail:       false,
		viewingAttachments: false,
		selectedAttachment: 0,
		Loading:            loading,
		fetchUser:          fetchUser,
		fetchPass:          fetchPass,
		fetchMax:           fetchMax,
		contactHistory:     contacts.Load(),
		downloadMessage:    "",
		showingDownload:    false,
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
		// Dismiss download message
		if m.showingDownload {
			m.showingDownload = false
			m.downloadMessage = ""
			return m, nil
		}

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
			if m.viewingAttachments {
				if m.selectedAttachment > 0 {
					m.selectedAttachment--
				}
			} else if m.selected > 0 {
				m.selected--
			} else if m.page > 0 {
				m.page--
				m.selected = m.emailsPerPage - 1
			}

		case "down", "j":
			if m.viewingAttachments {
				email := m.getSelectedEmail()
				if email != nil && m.selectedAttachment < len(email.Attachments)-1 {
					m.selectedAttachment++
				}
			} else if m.selected < m.currentPageCount()-1 {
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
			} else if m.viewingEmail && !m.viewingAttachments {
				m.viewingEmail = false
			} else if m.viewingAttachments {
				m.viewingAttachments = false
			}

		case "a":
			// Show attachments list when viewing an email
			if m.viewingEmail && !m.viewingAttachments {
				email := m.getSelectedEmail()
				if email != nil && len(email.Attachments) > 0 {
					m.viewingAttachments = true
					m.selectedAttachment = 0
				}
			}

		case "d":
			// Download selected attachment
			if m.viewingAttachments {
				email := m.getSelectedEmail()
				if email != nil && m.selectedAttachment >= 0 && m.selectedAttachment < len(email.Attachments) {
					att := email.Attachments[m.selectedAttachment]
					filePath, err := m.downloadAttachment(att)
					if err != nil {
						m.downloadMessage = fmt.Sprintf("Failed to download: %v", err)
						m.showingDownload = true
					} else {
						m.downloadMessage = fmt.Sprintf("Downloaded successfully!\n\nFile: %s\nLocation: %s", att.Filename, filepath.Dir(filePath))
						m.showingDownload = true
					}
				}
			}

		case "esc":
			if m.viewingAttachments {
				m.viewingAttachments = false
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
	// Show download message if active
	if m.showingDownload {
		messageBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(60).
			BorderForeground(lipgloss.Color("2")).
			Render(m.downloadMessage + "\n\n(press any key to dismiss)")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, messageBox)
	}

	// Stylized title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5")).
		Padding(0, 2).
		Render("  C U N C  ")

	var body string
	if m.Loading {
		body = "Fetching emails..."
	} else if m.viewingAttachments {
		body = m.renderAttachmentList()
	} else if m.viewingEmail {
		email := m.getSelectedEmail()
		if email != nil {
			body = m.renderEmailView(email)
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
		Render(m.getFooterText())

	return lipgloss.JoinVertical(lipgloss.Left, title, box, footer)
}

func (m Model) getFooterText() string {
	if m.viewingAttachments {
		return "↑/k up • ↓/j down • d download • esc back • ctrl+q quit"
	} else if m.viewingEmail {
		email := m.getSelectedEmail()
		if email != nil && len(email.Attachments) > 0 {
			return "enter back • a view attachments • esc back • ctrl+q quit"
		}
		return "enter back • esc back • ctrl+q quit"
	}
	return "↑/k up • ↓/j down • h/l prev/next page • enter view • ctrl+r refresh • ctrl+q quit"
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

// renderEmailView renders a single email with attachment indicators
func (m Model) renderEmailView(email *Email) string {
	view := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", email.From, email.Subject, email.Body)

	if len(email.Attachments) > 0 {
		attachStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

		view += "\n\n" + attachStyle.Render(fmt.Sprintf("📎 %d attachment(s)", len(email.Attachments)))
		view += lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render("\n(press 'a' to view/download)")
	}

	return view
}

// renderAttachmentList renders the list of attachments for download
func (m Model) renderAttachmentList() string {
	email := m.getSelectedEmail()
	if email == nil || len(email.Attachments) == 0 {
		return "No attachments available."
	}

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	var view strings.Builder
	view.WriteString(titleStyle.Render("Attachments") + "\n\n")

	for i, att := range email.Attachments {
		// Format size
		size := formatSize(len(att.Data))
		line := fmt.Sprintf("  📎 %s (%s)", att.Filename, size)

		if i == m.selectedAttachment {
			view.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			view.WriteString(normalStyle.Render(line) + "\n")
		}
	}

	return view.String()
}

// downloadAttachment saves an attachment to the configured download folder
func (m *Model) downloadAttachment(att Attachment) (string, error) {
	// Get download path from settings
	sett := settings.InitialModel()
	downloadPath := sett.GetSetting("default attachment download path")

	// If not set, use Downloads folder as default
	if downloadPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		downloadPath = filepath.Join(homeDir, "Downloads")
	}

	// Expand tilde in path
	if strings.HasPrefix(downloadPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		downloadPath = filepath.Join(homeDir, downloadPath[2:])
	} else if downloadPath == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		downloadPath = homeDir
	}

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return "", fmt.Errorf("could not create download directory: %v", err)
	}

	// Check if file exists and add number suffix if needed
	filePath := filepath.Join(downloadPath, att.Filename)
	if _, err := os.Stat(filePath); err == nil {
		// File exists, add number suffix
		ext := filepath.Ext(att.Filename)
		nameWithoutExt := strings.TrimSuffix(att.Filename, ext)
		counter := 1
		for {
			filePath = filepath.Join(downloadPath, fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext))
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				break
			}
			counter++
		}
	}

	// Write file
	if err := os.WriteFile(filePath, att.Data, 0644); err != nil {
		return "", fmt.Errorf("could not write file: %v", err)
	}

	return filePath, nil
}

// formatSize formats bytes into human-readable format
func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
