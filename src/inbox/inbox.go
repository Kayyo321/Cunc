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
	HTMLBody    string // html version of email body, because plain text is too easy
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
	Action              string // action flag, try not to panic

	// async fetch stuff, try not to trip
	Loading    bool
	fetch_user string
	fetch_pass string
	fetch_max  int

	// paging and lazy loading, because waiting is a crime
	total_available int  // total emails available on server (approx, maybe)
	emails_offset   int  // offset for next batch fetch
	fetching_more   bool // true while loading the next batch

	// contact history, aka people we emailed
	contact_history *contacts.ContactHistory

	// download status, also known as "did it work?"
	download_message string
	showing_download bool

	// title animation, because shiny
	title_animator title.Animator

	spinner_frame int

	// search mode, for when you forgot who you emailed
	search_mode      bool
	search_query     string
	search_results   []Email
	search_input_pos int

	// email view scrolling, for long-winded messages
	email_scroll_offset int // vertical scroll offset when viewing an email

	// sent emails view, because you also talk
	view_mode    string  // "inbox" or "sent", pick your poison
	sent_emails  []Email // sent emails list, because you did send them
	sent_loading bool    // loading sent emails, brace yourself
}

func InitialModel(emails []Email, emails_per_page int, loading bool, fetch_user, fetch_pass string, fetch_max int) Model {
	// sort by id desc, because newest should show up first, shocker
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
		email_scroll_offset: 0,
		view_mode:           "inbox",
		sent_emails:         []Email{},
		sent_loading:        false,
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
	// handle title animation updates and spinner, because sparkle therapy
	if cmd := m.title_animator.Update(msg); cmd != nil {
		// also update spinner frame on each tick, obviously
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
		// dismiss download message, because it got the memo
		if m.showing_download {
			m.showing_download = false
			m.download_message = ""
			return m, nil
		}

		// handle search mode input, for your wandering brain
		if m.search_mode {
			switch msg.String() {
			case "esc":
				m.search_mode = false
				m.search_query = ""
				m.search_results = []Email{}
				m.page = 0
				m.selected = 0

			case "enter":
				// exit search input mode if we have results
				if len(m.search_results) > 0 {
					m.search_mode = false
					m.page = 0
					m.selected = 0
				}

			case "backspace":
				if m.search_input_pos > 0 {
					m.search_query = m.search_query[:m.search_input_pos-1] + m.search_query[m.search_input_pos:]
					m.search_input_pos--
					// perform live search as we edit, because instant gratification
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
					// perform live search as we type, because typing alone is too easy
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
			// refresh current view (inbox or sent), like shaking the mailbox
			if m.fetch_user != "" && m.fetch_pass != "" {
				if m.view_mode == "sent" {
					m.sent_loading = true
					return m, FetchSentEmailsCmd(m.fetch_user, m.fetch_pass, m.fetch_max)
				} else {
					m.Loading = true
					return m, FetchEmailsCmd(m.fetch_user, m.fetch_pass, m.fetch_max)
				}
			}

		case "ctrl+f":
			// enter search mode, for when scrolling is too much work
			m.search_mode = true
			m.search_query = ""
			m.search_input_pos = 0
			return m, nil

		case "tab":
			// toggle between inbox and sent views, because options
			if !m.viewing_email && !m.viewing_attachments && !m.search_mode {
				if m.view_mode == "inbox" {
					m.view_mode = "sent"
					// load sent emails if not loaded yet
					if len(m.sent_emails) == 0 && !m.sent_loading && m.fetch_user != "" && m.fetch_pass != "" {
						m.sent_loading = true
						return m, FetchSentEmailsCmd(m.fetch_user, m.fetch_pass, m.fetch_max)
					}
				} else {
					m.view_mode = "inbox"
				}
				// reset to first page when switching views, because consistency
				m.page = 0
				m.selected = 0
			}
			return m, nil

		case "up", "k":
			if m.viewing_attachments {
				if m.selected_attachment > 0 {
					m.selected_attachment--
				}
			} else if m.viewing_email {
				// scroll up in email content, because walls of text
				if m.email_scroll_offset > 0 {
					m.email_scroll_offset--
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
			} else if m.viewing_email {
				// scroll down in email content, because walls of text
				email := m.get_selected_email()
				if email != nil {
					wrappedLines := m.wrap_body_lines(email.Body)
					availableHeight := m.height - 15
					if availableHeight < 5 {
						availableHeight = 5
					}
					maxScroll := len(wrappedLines) - availableHeight
					if maxScroll < 0 {
						maxScroll = 0
					}
					if m.email_scroll_offset < maxScroll {
						m.email_scroll_offset++
					}
				}
			} else {
				current_emails := m.get_current_emails()
				// calculate how many emails are on the current page
				start := m.page * m.emails_per_page
				end := start + m.emails_per_page
				if end > len(current_emails) {
					end = len(current_emails)
				}
				emails_on_page := end - start

				// check if we can move down within the current page
				if m.selected < emails_on_page-1 {
					m.selected++
				} else if (m.page+1)*m.emails_per_page < len(current_emails) {
					// move to next page
					m.page++
					m.selected = 0
				} else if m.should_load_more() {
					// try to load more emails
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
				// try to load more emails
				m.fetching_more = true
				return m, FetchMoreEmailsCmd(m.fetch_user, m.fetch_pass, m.emails_offset, m.fetch_max)
			}

		case "enter":
			if !m.viewing_email && len(m.get_current_emails()) > 0 {
				m.viewing_email = true
				m.email_scroll_offset = 0 // reset scroll when opening email
			} else if m.viewing_email && !m.viewing_attachments {
				m.viewing_email = false
				m.email_scroll_offset = 0 // reset scroll when closing email
			} else if m.viewing_attachments {
				m.viewing_attachments = false
			}

		case "a":
			// show attachments list when viewing an email
			if m.viewing_email && !m.viewing_attachments {
				email := m.get_selected_email()
				if email != nil && len(email.Attachments) > 0 {
					m.viewing_attachments = true
					m.selected_attachment = 0
				}
			}

		case "d":
			// download selected attachment, if you can find it
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
				m.email_scroll_offset = 0 // reset scroll when exiting email
			} else if len(m.search_results) > 0 {
				// clear search results and return to full inbox
				m.search_results = []Email{}
				m.search_query = ""
				m.page = 0
				m.selected = 0
			}
		}
	}

	// handle async fetch completion, because networking is a thing
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
			// reset to first page
			m.page = 0
			m.selected = 0
			// update offset for next fetch
			m.emails_offset = len(m.emails)
		}

	case EmailsFetchedWithOffsetMsg:
		m.Loading = false
		m.fetching_more = false
		if v.Err == nil && len(v.Emails) > 0 {
			// append new emails to existing list
			m.emails = append(m.emails, v.Emails...)
			// re-sort to maintain order
			sort.Slice(m.emails, func(i, j int) bool { return m.emails[i].ID > m.emails[j].ID })
			// update offset for next fetch
			m.emails_offset += len(v.Emails)
		}

	case SentEmailsFetchedMsg:
		m.sent_loading = false
		if v.Err == nil && len(v.Emails) > 0 {
			m.sent_emails = v.Emails
			// ensure sorted newest-first
			sort.Slice(m.sent_emails, func(i, j int) bool { return m.sent_emails[i].ID > m.sent_emails[j].ID })
		}
	}

	return m, nil
}

func (m Model) View() string {
	// show download message if active
	if m.showing_download {
		messageBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Width(60).
			BorderForeground(lipgloss.Color("2")).
			Render(m.download_message + "\n\n(press any key to dismiss)")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, messageBox)
	}

	// show fetching overlay if loading more emails
	if m.fetching_more {
		return m.render_with_overlay(m.render_fetching_overlay())
	}

	// show search mode
	if m.search_mode {
		return m.render_search_mode()
	}

	// animated title with box
	title := m.title_animator.Render()

	// view mode indicator
	modeText := "INBOX"
	modeColor := lipgloss.Color("4")
	if m.view_mode == "sent" {
		modeText = "SENT"
		modeColor = lipgloss.Color("6")
	}
	modeIndicator := lipgloss.NewStyle().
		Foreground(modeColor).
		Bold(true).
		Padding(0, 1).
		Render("[ " + modeText + " ]")

	var body string
	if m.Loading || m.sent_loading {
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

	return lipgloss.JoinVertical(lipgloss.Left, title, modeIndicator, box, footer)
}

func (m Model) get_footer_text() string {
	if m.viewing_attachments {
		return "↑/k up • ↓/j down • d download • esc back • ctrl+q quit"
	} else if m.viewing_email {
		email := m.get_selected_email()
		if email != nil && len(email.Attachments) > 0 {
			return "↑/k ↓/j scroll • enter/esc back • a view attachments • ctrl+q quit"
		}
		return "↑/k ↓/j scroll • enter/esc back • ctrl+q quit"
	}
	return "↑/k up • ↓/j down • h/l prev/next page • enter view • tab inbox/sent • ctrl+f search • ctrl+r refresh • ctrl+q quit"
}

// render_email_list shows the current page of emails, nothing fancy
func (m Model) render_email_list() string {
	current_emails := m.get_current_emails()

	start := m.page * m.emails_per_page
	end := start + m.emails_per_page
	if end > len(current_emails) {
		end = len(current_emails)
	}

	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"

	emails := current_emails[start:end]
	var lines string
	for i, email := range emails {
		// check if this is from a known contact
		isKnown := m.contact_history != nil && m.contact_history.IsKnown(email.From)

		// calculate available width for email line
		// account for labels, star, and padding, because math again
		availableWidth := m.width - 25
		if isKnown {
			availableWidth -= 2 // account for star
		}
		if availableWidth < 20 {
			availableWidth = 20
		}

		// split available width: 40% for from, 60% for subject
		fromWidth := availableWidth * 40 / 100
		subjectWidth := availableWidth - fromWidth

		// truncate from field
		fromField := email.From
		if len(fromField) > fromWidth {
			if fromWidth > 3 {
				fromField = fromField[:fromWidth-3] + "..."
			} else {
				fromField = fromField[:fromWidth]
			}
		}

		// truncate subject field
		subjectField := email.Subject
		if len(subjectField) > subjectWidth {
			if subjectWidth > 3 {
				subjectField = subjectField[:subjectWidth-3] + "..."
			} else {
				subjectField = subjectField[:subjectWidth]
			}
		}

		line := fmt.Sprintf("  From: %s | Subject: %s", fromField, subjectField)
		star := "★ "
		if !unicode_support {
			star = "* "
		}

		if i == m.selected {
			// selected email style, fancy highlight
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4"))

			if isKnown {
				// add an indicator for known contacts when selected
				line = star + line
			}
			line = style.Render(line)
		} else if isKnown {
			// known contact, highlight in yellow
			line = star + line
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

	// add page indicator
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

// wrap_body_lines wraps email body lines to fit within the display width, mostly
func (m Model) wrap_body_lines(body string) []string {
	contentWidth := m.width - 14 // width - 10 for box, - 4 for border and padding
	if contentWidth < 20 {
		contentWidth = 20
	}

	bodyLines := strings.Split(body, "\n")
	wrappedLines := []string{}

	for _, line := range bodyLines {
		if len(line) <= contentWidth {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// word-based wrapping to preserve urls and words
		words := strings.Fields(line) // split on whitespace
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		currentLine := ""
		for _, word := range words {
			// check if this word looks like a url (with or without wrapping characters)
			isURL := strings.HasPrefix(word, "http://") ||
				strings.HasPrefix(word, "https://") ||
				strings.HasPrefix(word, "<http://") ||
				strings.HasPrefix(word, "<https://") ||
				strings.HasPrefix(word, "[http://") ||
				strings.HasPrefix(word, "[https://") ||
				strings.HasPrefix(word, "(http://") ||
				strings.HasPrefix(word, "(https://") ||
				strings.Contains(word, "://")

			// if word is too long and is a url, put it on its own line
			if isURL && len(word) > contentWidth {
				if currentLine != "" {
					wrappedLines = append(wrappedLines, currentLine)
					currentLine = ""
				}
				wrappedLines = append(wrappedLines, word)
				continue
			}

			// try to add word to current line
			testLine := currentLine
			if testLine != "" {
				testLine += " " + word
			} else {
				testLine = word
			}

			if len(testLine) <= contentWidth {
				currentLine = testLine
			} else {
				// current line is full, start a new line
				if currentLine != "" {
					wrappedLines = append(wrappedLines, currentLine)
				}
				// if the word itself is too long and not a url, break it
				if len(word) > contentWidth && !isURL {
					for len(word) > contentWidth {
						wrappedLines = append(wrappedLines, word[:contentWidth])
						word = word[contentWidth:]
					}
					currentLine = word
				} else {
					currentLine = word
				}
			}
		}

		// add any remaining text
		if currentLine != "" {
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return wrappedLines
}

// get_current_emails returns search_results if in search mode, otherwise full email list
func (m Model) get_current_emails() []Email {
	if len(m.search_results) > 0 {
		return m.search_results
	}
	if m.view_mode == "sent" {
		return m.sent_emails
	}
	return m.emails
}

// should_load_more checks if we should load the next batch of emails
func (m Model) should_load_more() bool {
	// don't load more in sent mode or search mode
	if len(m.search_results) > 0 || m.view_mode == "sent" || m.fetch_user == "" || m.fetch_pass == "" {
		return false
	}
	// load more if we're at the end of our current emails
	total_loaded := len(m.emails)
	return total_loaded == m.emails_offset && total_loaded > 0
}

// perform_search filters emails by subject and body, because search is life
func (m Model) perform_search(query string) []Email {
	if query == "" {
		return []Email{}
	}

	query_lower := strings.ToLower(query)
	var results []Email

	// search in the correct email list based on view mode
	emailsToSearch := m.emails
	if m.view_mode == "sent" {
		emailsToSearch = m.sent_emails
	}

	for _, email := range emailsToSearch {
		if strings.Contains(strings.ToLower(email.Subject), query_lower) ||
			strings.Contains(strings.ToLower(email.Body), query_lower) {
			results = append(results, email)
		}
	}

	return results
}

// render_search_mode renders the search input ui, try not to blink
func (m Model) render_search_mode() string {
	title := m.title_animator.Render()

	// build search input with cursor
	input := m.search_query
	cursor_pos := m.search_input_pos

	// split input into parts: before cursor and after cursor
	before := input[:cursor_pos]
	after := ""
	if cursor_pos < len(input) {
		after = input[cursor_pos:]
	}

	// create cursor style
	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).
		Foreground(lipgloss.Color("0"))

	// show cursor as next character or space
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

	// build result display
	var resultText string
	if m.search_query == "" {
		resultText = "Start typing to search emails by subject or body..."
	} else if len(m.search_results) == 0 {
		resultText = fmt.Sprintf("No results found for: %q", m.search_query)
	} else {
		resultText = fmt.Sprintf("Found %d email(s) - press Enter to view results, Esc to cancel", len(m.search_results))

		// show first few results, because paging is effort
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
	// calculate the content width (accounting for border and padding)
	contentWidth := m.width - 14 // width - 10 for box, - 4 for border and padding
	if contentWidth < 20 {
		contentWidth = 20
	}

	// styles for different sections
	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(0, 1).
		MaxWidth(m.width - 10)

	subjectStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		MaxWidth(m.width - 10)

	bodyStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")).
		Padding(0, 1).
		MaxWidth(m.width - 10)

	// from header - truncate if needed
	fromText := "From: " + email.From
	if len(fromText) > contentWidth {
		fromText = fromText[:contentWidth-3] + "..."
	}
	fromBox := headerStyle.Render(fromText)

	// subject header - truncate if needed
	subjectText := "Subject: " + email.Subject
	if len(subjectText) > contentWidth {
		subjectText = subjectText[:contentWidth-3] + "..."
	}
	subjectBox := subjectStyle.Render(subjectText)

	// body - handle scrolling with wrapped lines
	wrappedLines := m.wrap_body_lines(email.Body)

	// calculate available height for body (total height - title - from - subject - footer - borders)
	availableHeight := m.height - 15 // adjust based on ui layout
	if availableHeight < 5 {
		availableHeight = 5
	}

	// apply scroll offset
	startLine := m.email_scroll_offset
	endLine := startLine + availableHeight

	// clamp to valid range
	if startLine < 0 {
		startLine = 0
	}
	if startLine > len(wrappedLines) {
		startLine = len(wrappedLines)
	}
	if endLine > len(wrappedLines) {
		endLine = len(wrappedLines)
	}
	if startLine > endLine {
		startLine = endLine
	}

	visibleBody := strings.Join(wrappedLines[startLine:endLine], "\n")

	// add scroll indicators on a separate line
	scrollInfo := ""
	if len(wrappedLines) > availableHeight {
		scrollInfo = fmt.Sprintf("\n\n[%d-%d of %d lines]", startLine+1, endLine, len(wrappedLines))
	}

	bodyBox := bodyStyle.Render(visibleBody + scrollInfo)

	// attachment indicator
	attachmentInfo := ""
	if len(email.Attachments) > 0 {
		// check unicode support
		sett := settings.InitialModel()
		unicode_support := sett.GetSetting("unicode support") == "y"
		attachment_icon := "[A]"
		if unicode_support {
			attachment_icon = "📎"
		}

		attachStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

		attachmentInfo = "\n\n" + attachStyle.Render(fmt.Sprintf("%s %d attachment(s)", attachment_icon, len(email.Attachments)))
		attachmentInfo += lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render("\n(press 'a' to view/download)")
	}

	return fromBox + "\n\n" + subjectBox + "\n\n" + bodyBox + attachmentInfo
}

// render_attachment_list renders the list of attachments for download
func (m Model) render_attachment_list() string {
	email := m.get_selected_email()
	if email == nil || len(email.Attachments) == 0 {
		return "No attachments available."
	}

	// check unicode support
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
		// format size
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
	// get download path from settings
	sett := settings.InitialModel()
	download_path := sett.GetSetting("default attachment download path")

	// if not set, use Downloads folder as default
	if download_path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}
		download_path = filepath.Join(homeDir, "Downloads")
	}

	// expand tilde in path
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

	// create download directory if it doesn't exist
	if err := os.MkdirAll(download_path, 0755); err != nil {
		return "", fmt.Errorf("could not create download directory: %v", err)
	}

	// check if file exists and add number suffix if needed
	filePath := filepath.Join(download_path, att.Filename)
	if _, err := os.Stat(filePath); err == nil {
		// file exists, add number suffix
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

	// write file
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
	// check unicode support
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"

	var frame string
	if unicode_support {
		// spinner frames using unicode characters
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame = spinners[m.spinner_frame%len(spinners)]
	} else {
		// ascii spinner frames
		spinners := []string{"-", "\\", "|", "/"}
		frame = spinners[m.spinner_frame%len(spinners)]
	}

	// style the spinner with cyan color
	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("51")).
		Bold(true)

	message := spinnerStyle.Render(frame) + " Fetching emails..."

	// center the message
	centerStyle := lipgloss.NewStyle().
		Padding(2, 0)

	return centerStyle.Render(message)
}

// render_fetching_overlay creates an overlay box for loading more emails
func (m Model) render_fetching_overlay() string {
	// check unicode support
	sett := settings.InitialModel()
	unicode_support := sett.GetSetting("unicode support") == "y"

	var frame string
	if unicode_support {
		// spinner frames using unicode characters
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame = spinners[m.spinner_frame%len(spinners)]
	} else {
		// ascii spinner frames
		spinners := []string{"-", "\\", "|", "/"}
		frame = spinners[m.spinner_frame%len(spinners)]
	}

	// style the spinner with cyan color
	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("51")).
		Bold(true)

	message := spinnerStyle.Render(frame) + " Loading more emails..."

	return message
}

// render_with_overlay renders the normal view with an overlay box
func (m Model) render_with_overlay(overlay string) string {
	// create overlay box
	overlayBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		BorderForeground(lipgloss.Color("51")).
		Render(overlay)

	// place overlay on top of main view
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		overlayBox,
	)
}
