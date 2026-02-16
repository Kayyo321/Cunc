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

	// pagination and lazy loading
	total_available int  // total emails available on server (approx)
	emails_offset   int  // offset for next batch fetch
	fetching_more   bool // true when loading the next batch

	// contact history
	contact_history *contacts.ContactHistory

	// download status
	download_message string
	showing_download bool

	// title animation
	title_animator title.Animator

	// spinner animation
	spinner_frame int

	// search mode
	search_mode      bool
	search_query     string
	search_results   []Email
	search_input_pos int
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
		total_available:     fetch_max,
		emails_offset:       0,
		fetching_more:       false,
		contact_history:     contacts.Load(),
		download_message:    "",
		showing_download:    false,
		title_animator:      title.New(),
		spinner_frame:       0,
		search_mode:         false,
		search_query:        "",
		search_results:      []Email{},
		search_input_pos:    0,
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

		// Handle search mode input
		if m.search_mode {
			switch msg.String() {
			case "esc":
				m.search_mode = false
				m.search_query = ""
				m.search_results = []Email{}
				m.page = 0
				m.selected = 0

			case "enter":
				// Exit search input mode if we have results
				if len(m.search_results) > 0 {
					m.search_mode = false
					m.page = 0
					m.selected = 0
				}

			case "backspace":
				if m.search_input_pos > 0 {
					m.search_query = m.search_query[:m.search_input_pos-1] + m.search_query[m.search_input_pos:]
					m.search_input_pos--
					// Perform live search as we edit
					m.search_results = m.perform_search(m.search_query)
				}

			case "left":
				if m.search_input_pos > 0 {
					m.search_input_pos--
				}

			case "right":
				if m.search_input_pos < len(m.search_query) {
					m.search_input_pos++
				}

			default:
				if len(msg.Runes) > 0 {
					for _, r := range msg.Runes {
						m.search_query = m.search_query[:m.search_input_pos] + string(r) + m.search_query[m.search_input_pos:]
						m.search_input_pos++
					}
					// Perform live search as we type
					m.search_results = m.perform_search(m.search_query)
				}
			}
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

		case "ctrl+f":
			// Enter search mode
			m.search_mode = true
			m.search_query = ""
			m.search_input_pos = 0
			return m, nil

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
			} else {
				current_emails := m.get_current_emails()
				// Calculate how many emails are on the current page
				start := m.page * m.emails_per_page
				end := start + m.emails_per_page
				if end > len(current_emails) {
					end = len(current_emails)
				}
				emails_on_page := end - start

				// Check if we can move down within the current page
				if m.selected < emails_on_page-1 {
					m.selected++
				} else if (m.page+1)*m.emails_per_page < len(current_emails) {
					// Move to next page
					m.page++
					m.selected = 0
				} else if m.should_load_more() {
					// Try to load more emails
					m.fetching_more = true
					return m, FetchMoreEmailsCmd(m.fetch_user, m.fetch_pass, m.emails_offset, m.fetch_max)
				}
			}

		case "left", "h":
			if m.page > 0 {
				m.page--
				m.selected = 0
			}

		case "right", "l":
			current_emails := m.get_current_emails()
			maxPage := (len(current_emails) - 1) / m.emails_per_page
			if m.page < maxPage {
				m.page++
				m.selected = 0
			} else if m.should_load_more() {
				// Try to load more emails
				m.fetching_more = true
				return m, FetchMoreEmailsCmd(m.fetch_user, m.fetch_pass, m.emails_offset, m.fetch_max)
			}

		case "enter":
			if !m.viewing_email && len(m.get_current_emails()) > 0 {
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
			} else if len(m.search_results) > 0 {
				// Clear search results and return to full inbox
				m.search_results = []Email{}
				m.search_query = ""
				m.page = 0
				m.selected = 0
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
			// Update offset for next fetch
			m.emails_offset = len(m.emails)
		}

	case EmailsFetchedWithOffsetMsg:
		m.Loading = false
		m.fetching_more = false
		if v.Err == nil && len(v.Emails) > 0 {
			// Append new emails to existing list
			m.emails = append(m.emails, v.Emails...)
			// Re-sort to maintain order
			sort.Slice(m.emails, func(i, j int) bool { return m.emails[i].ID > m.emails[j].ID })
			// Update offset for next fetch
			m.emails_offset += len(v.Emails)
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

	// Show fetching overlay if loading more emails
	if m.fetching_more {
		return m.render_with_overlay(m.render_fetching_overlay())
	}

	// Show search mode
	if m.search_mode {
		return m.render_search_mode()
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
	return "↑/k up • ↓/j down • h/l prev/next page • enter view • ctrl+f search • ctrl+r refresh • ctrl+q quit"
}

// render_email_list shows the current page of emails
func (m Model) render_email_list() string {
	current_emails := m.get_current_emails()

	start := m.page * m.emails_per_page
	end := start + m.emails_per_page
	if end > len(current_emails) {
		end = len(current_emails)
	}

	emails := current_emails[start:end]
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
	totalPages := (len(current_emails) + m.emails_per_page - 1) / m.emails_per_page
	if totalPages > 1 {
		lines += fmt.Sprintf("\n\nPage %d/%d", m.page+1, totalPages)
	}

	return lines
}

func (m Model) get_selected_email() *Email {
	idx := m.page*m.emails_per_page + m.selected
	current_emails := m.get_current_emails()
	if idx >= 0 && idx < len(current_emails) {
		return &current_emails[idx]
	}
	return nil
}

// get_current_emails returns search_results if in search mode, otherwise full email list
func (m Model) get_current_emails() []Email {
	if len(m.search_results) > 0 {
		return m.search_results
	}
	return m.emails
}

// should_load_more checks if we should load the next batch of emails
func (m Model) should_load_more() bool {
	// Only load more if we're in normal mode (not searching) and have credentials
	if len(m.search_results) > 0 || m.fetch_user == "" || m.fetch_pass == "" {
		return false
	}
	// Load more if we're at the end of our current emails
	total_loaded := len(m.emails)
	return total_loaded == m.emails_offset && total_loaded > 0
}

// perform_search filters emails by subject and body
func (m Model) perform_search(query string) []Email {
	if query == "" {
		return []Email{}
	}

	query_lower := strings.ToLower(query)
	var results []Email

	for _, email := range m.emails {
		if strings.Contains(strings.ToLower(email.Subject), query_lower) ||
			strings.Contains(strings.ToLower(email.Body), query_lower) {
			results = append(results, email)
		}
	}

	return results
}

// render_search_mode renders the search input UI
func (m Model) render_search_mode() string {
	title := m.title_animator.Render()

	// Build search input with cursor
	input := m.search_query
	cursor_pos := m.search_input_pos

	// Split input into parts: before cursor and after cursor
	before := input[:cursor_pos]
	after := ""
	if cursor_pos < len(input) {
		after = input[cursor_pos:]
	}

	// Create cursor style
	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).
		Foreground(lipgloss.Color("0"))

	// Show cursor as next character or space
	var cursorChar string
	if cursor_pos < len(input) {
		cursorChar = string([]rune(input)[cursor_pos])
	} else {
		cursorChar = " "
	}

	searchInput := before + cursorStyle.Render(cursorChar) + after
	if cursor_pos >= len(input) {
		searchInput = before + cursorStyle.Render(" ")
	}

	// Build result display
	var resultText string
	if m.search_query == "" {
		resultText = "Start typing to search emails by subject or body..."
	} else if len(m.search_results) == 0 {
		resultText = fmt.Sprintf("No results found for: %q", m.search_query)
	} else {
		resultText = fmt.Sprintf("Found %d email(s) - press Enter to view results, Esc to cancel", len(m.search_results))

		// Show first few results
		resultText += "\n\nPreview:"
		for i, email := range m.search_results {
			if i >= 5 {
				resultText += fmt.Sprintf("\n... and %d more", len(m.search_results)-5)
				break
			}
			resultText += fmt.Sprintf("\n  • %s: %s", email.From, email.Subject)
		}
	}

	content := fmt.Sprintf("Search Query:\n%s\n\n%s", searchInput, resultText)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 2).
		Height(max(m.height-6, 3)).
		Render(content)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 2).
		Render("enter view results • esc cancel • ← → move cursor • backspace delete")

	return lipgloss.JoinVertical(lipgloss.Left, title, box, footer)
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

// render_fetching_overlay creates an overlay box for loading more emails
func (m Model) render_fetching_overlay() string {
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

	message := spinnerStyle.Render(frame) + " Loading more emails..."

	return message
}

// render_with_overlay renders the normal view with an overlay box
func (m Model) render_with_overlay(overlay string) string {
	// Create overlay box
	overlayBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		BorderForeground(lipgloss.Color("51")).
		Render(overlay)

	// Place overlay on top of main view
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		overlayBox,
	)
}
