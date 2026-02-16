package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cunc/src/contacts"
	"cunc/src/settings"
	"cunc/src/title"

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
	emails              []Email
	selected            int
	page                int
	emails_per_page     int
	width               int
	height              int
	viewing_email       bool
	viewing_attachments bool
	selected_attachment int
	Action              string // "select" or "quit"

	// async fetch control
	Loading    bool
	fetch_user string
	fetch_pass string
	fetch_max  int

	// contact history
	contact_history *contacts.ContactHistory

	// download status
	download_message string
	showing_download bool

	// title animation
	title_animator title.Animator

	// spinner animation
	spinner_frame int
}

func InitialModel(emails []Email, emails_per_page int, loading bool, fetch_user, fetch_pass string, fetch_max int) Model {
	// Sort emails by ID descending (most recent first)
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].ID > emails[j].ID
	})

	return Model{
		emails:              emails,
		selected:            0,
		page:                0,
		emails_per_page:     emails_per_page,
		width:               80,
		height:              24,
		viewing_email:       false,
		viewing_attachments: false,
		selected_attachment: 0,
		Loading:             loading,
		fetch_user:          fetch_user,
		fetch_pass:          fetch_pass,
		fetch_max:           fetch_max,
		contact_history:     contacts.Load(),
		download_message:    "",
		showing_download:    false,
		title_animator:      title.New(),
		spinner_frame:       0,
	}
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.Loading && m.fetch_user != "" && m.fetch_pass != "" {
		cmds = append(cmds, FetchEmailsCmd(m.fetch_user, m.fetch_pass, m.fetch_max))
	}
	cmds = append(cmds, title.TickCmd())
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle title animation updates and spinner
	if cmd := m.title_animator.Update(msg); cmd != nil {
		// Also update spinner frame on each tick
		if _, ok := msg.(title.TickMsg); ok {
			m.spinner_frame++
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Dismiss download message
		if m.showing_download {
			m.showing_download = false
			m.download_message = ""
			return m, nil
		}

		switch msg.String() {
		case "ctrl+q":
			fmt.Print("\033[2J")
			m.Action = "quit"
			return m, tea.Quit

		case "ctrl+r":
			// Refresh inbox
			if m.fetch_user != "" && m.fetch_pass != "" {
				m.Loading = true
				return m, FetchEmailsCmd(m.fetch_user, m.fetch_pass, m.fetch_max)
			}

		case "up", "k":
			if m.viewing_attachments {
				if m.selected_attachment > 0 {
					m.selected_attachment--
				}
			} else if m.selected > 0 {
				m.selected--
			} else if m.page > 0 {
				m.page--
				m.selected = m.emails_per_page - 1
			}

		case "down", "j":
			if m.viewing_attachments {
				email := m.get_selected_email()
				if email != nil && m.selected_attachment < len(email.Attachments)-1 {
					m.selected_attachment++
				}
			} else if m.selected < m.current_page_count()-1 {
				m.selected++
			} else if (m.page+1)*m.emails_per_page < len(m.emails) {
				m.page++
				m.selected = 0
			}

		case "left", "h":
			if m.page > 0 {
				m.page--
				m.selected = 0
			}

		case "right", "l":
			maxPage := (len(m.emails) - 1) / m.emails_per_page
			if m.page < maxPage {
				m.page++
				m.selected = 0
			}

		case "enter":
			if !m.viewing_email && len(m.emails) > 0 {
				m.viewing_email = true
			} else if m.viewing_email && !m.viewing_attachments {
				m.viewing_email = false
			} else if m.viewing_attachments {
				m.viewing_attachments = false
			}

		case "a":
			// Show attachments list when viewing an email
			if m.viewing_email && !m.viewing_attachments {
				email := m.get_selected_email()
				if email != nil && len(email.Attachments) > 0 {
					m.viewing_attachments = true
					m.selected_attachment = 0
				}
			}

		case "d":
			// Download selected attachment
			if m.viewing_attachments {
				email := m.get_selected_email()
				if email != nil && m.selected_attachment >= 0 && m.selected_attachment < len(email.Attachments) {
					att := email.Attachments[m.selected_attachment]
					filePath, err := m.download_attachment(att)
					if err != nil {
						m.download_message = fmt.Sprintf("Failed to download: %v", err)
						m.showing_download = true
					} else {
						m.download_message = fmt.Sprintf("Downloaded successfully!\n\nFile: %s\nLocation: %s", att.Filename, filepath.Dir(filePath))
						m.showing_download = true
					}
				}
			}

		case "esc":
			if m.viewing_attachments {
				m.viewing_attachments = false
			} else if m.viewing_email {
				m.viewing_email = false
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
			m.contact_history = contacts.Load()
			// Reset to first page
			m.page = 0
			m.selected = 0
		}
	}

	return m, nil
}

func (m Model) View() string {
	// Show download message if active
	if m.showing_download {
		messageBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(60).
			BorderForeground(lipgloss.Color("2")).
			Render(m.download_message + "\n\n(press any key to dismiss)")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, messageBox)
	}

	// Animated title with box
	title := m.title_animator.Render()

	var body string
	if m.Loading {
		body = m.render_loading_spinner()
	} else if m.viewing_attachments {
		body = m.render_attachment_list()
	} else if m.viewing_email {
		email := m.get_selected_email()
		if email != nil {
			body = m.render_email_view(email)
		} else {
			body = "No email selected."
		}
	} else {
		body = m.render_email_list()
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
		Render(m.get_footer_text())

	return lipgloss.JoinVertical(lipgloss.Left, title, box, footer)
}

func (m Model) get_footer_text() string {
	if m.viewing_attachments {
		return "↑/k up • ↓/j down • d download • esc back • ctrl+q quit"
	} else if m.viewing_email {
		email := m.get_selected_email()
		if email != nil && len(email.Attachments) > 0 {
			return "enter back • a view attachments • esc back • ctrl+q quit"
		}
		return "enter back • esc back • ctrl+q quit"
	}
	return "↑/k up • ↓/j down • h/l prev/next page • enter view • ctrl+r refresh • ctrl+q quit"
}

// render_email_list shows the current page of emails
func (m Model) render_email_list() string {
	start := m.page * m.emails_per_page
	end := start + m.emails_per_page
	if end > len(m.emails) {
		end = len(m.emails)
	}

	emails := m.emails[start:end]
	var lines string
	for i, email := range emails {
		line := fmt.Sprintf("  From: %s | Subject: %s", email.From, email.Subject)

		// Check if this is from a known contact
		isKnown := m.contact_history != nil && m.contact_history.IsKnown(email.From)

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
	totalPages := (len(m.emails) + m.emails_per_page - 1) / m.emails_per_page
	if totalPages > 1 {
		lines += fmt.Sprintf("\n\nPage %d/%d", m.page+1, totalPages)
	}

	return lines
}

func (m Model) get_selected_email() *Email {
	idx := m.page*m.emails_per_page + m.selected
	if idx >= 0 && idx < len(m.emails) {
		return &m.emails[idx]
	}
	return nil
}

func (m Model) current_page_count() int {
	remaining := len(m.emails) - m.page*m.emails_per_page
	if remaining > m.emails_per_page {
		return m.emails_per_page
	}
	return remaining
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// render_email_view renders a single email with attachment indicators
func (m Model) render_email_view(email *Email) string {
	view := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", email.From, email.Subject, email.Body)

	if len(email.Attachments) > 0 {
		// Check unicode support
		sett := settings.InitialModel()
		unicode_support := sett.GetSetting("unicode support") == "y"
		attachment_icon := "[A]"
		if unicode_support {
			attachment_icon = "📎"
		}

		attachStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

		view += "\n\n" + attachStyle.Render(fmt.Sprintf("%s %d attachment(s)", attachment_icon, len(email.Attachments)))
		view += lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render("\n(press 'a' to view/download)")
	}

	return view
}

// render_attachment_list renders the list of attachments for download
func (m Model) render_attachment_list() string {
	email := m.get_selected_email()
	if email == nil || len(email.Attachments) == 0 {
		return "No attachments available."
	}

	// Check unicode support
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"
	attachment_icon := "[A]"
	if unicode_support {
		attachment_icon = "📎"
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
		size := format_size(len(att.Data))
		line := fmt.Sprintf("  %s %s (%s)", attachment_icon, att.Filename, size)

		if i == m.selected_attachment {
			view.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			view.WriteString(normalStyle.Render(line) + "\n")
		}
	}

	return view.String()
}

// download_attachment saves an attachment to the configured download folder
func (m *Model) download_attachment(att Attachment) (string, error) {
	// Get download path from settings
	sett := settings.InitialModel()
	download_path := sett.GetSetting("default attachment download path")

	// If not set, use Downloads folder as default
	if download_path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		download_path = filepath.Join(homeDir, "Downloads")
	}

	// Expand tilde in path
	if strings.HasPrefix(download_path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		download_path = filepath.Join(homeDir, download_path[2:])
	} else if download_path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		download_path = homeDir
	}

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(download_path, 0755); err != nil {
		return "", fmt.Errorf("could not create download directory: %v", err)
	}

	// Check if file exists and add number suffix if needed
	filePath := filepath.Join(download_path, att.Filename)
	if _, err := os.Stat(filePath); err == nil {
		// File exists, add number suffix
		ext := filepath.Ext(att.Filename)
		nameWithoutExt := strings.TrimSuffix(att.Filename, ext)
		counter := 1
		for {
			filePath = filepath.Join(download_path, fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext))
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

// format_size formats bytes into human-readable format
func format_size(bytes int) string {
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

// render_loading_spinner renders a spinning wheel animation for loading state
func (m Model) render_loading_spinner() string {
	// Check unicode support
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"

	var frame string
	if unicode_support {
		// Spinner frames using Unicode characters
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame = spinners[m.spinner_frame%len(spinners)]
	} else {
		// ASCII spinner frames
		spinners := []string{"-", "\\", "|", "/"}
		frame = spinners[m.spinner_frame%len(spinners)]
	}

	// Style the spinner with cyan color
	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("51")).
		Bold(true)

	message := spinnerStyle.Render(frame) + " Fetching emails..."

	// Center the message
	centerStyle := lipgloss.NewStyle().
		Padding(2, 0)

	return centerStyle.Render(message)
}
