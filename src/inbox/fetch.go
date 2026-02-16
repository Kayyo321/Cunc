package inbox

import (
	"crypto/tls"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// FetchLatest connects to Gmail IMAP and returns up to `max` most recent emails.
// username should be the user's full gmail address and password should be their
// account password or an app password when 2FA is enabled.
func FetchLatest(username, password string, max int) ([]Email, error) {
	return FetchWithOffset(username, password, 0, max)
}

// FetchWithOffset connects to Gmail IMAP and returns emails starting from offset.
// offset = 0 gets the newest emails, offset > 0 skips that many of the newest emails.
func FetchWithOffset(username, password string, offset, max int) ([]Email, error) {
	c, err := imapclient.DialTLS("imap.gmail.com:993", &tls.Config{ServerName: "imap.gmail.com"})
	if err != nil {
		return nil, fmt.Errorf("dial imap: %w", err)
	}
	defer c.Logout()

	if err := c.Login(username, password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return nil, fmt.Errorf("select INBOX: %w", err)
	}

	if mbox.Messages == 0 {
		return []Email{}, nil
	}

	// Compute sequence range starting from offset into the newest emails
	// mbox.Messages is the highest (newest) message number
	// offset = 0 starts at the newest, offset = 1 skips the newest, etc.
	var from uint32 = 1
	var to = mbox.Messages - uint32(offset)

	if to <= 0 {
		// offset is beyond the available emails
		return []Email{}, nil
	}

	// calc the from position
	if to-uint32(max)+1 > 1 {
		from = to - uint32(max) + 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, max)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	var results []Email
	for msg := range messages {
		if msg == nil {
			continue
		}

		env := msg.Envelope

		// Default body
		body_str := ""
		html_body := ""
		var attachments []Attachment

		if r := msg.GetBody(section); r != nil {
			mr, err := mail.CreateReader(r)
			if err == nil {
				for {
					p, err := mr.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}

					switch h := p.Header.(type) {
					case *mail.InlineHeader:
						b, _ := io.ReadAll(p.Body)
						ctype, _, _ := h.ContentType()
						if strings.HasPrefix(ctype, "text/plain") && body_str == "" {
							body_str = string(b)
						} else if strings.HasPrefix(ctype, "text/html") && html_body == "" {
							html_body = string(b)
						} else if body_str == "" && html_body == "" {
							body_str = string(b)
						}
					case *mail.AttachmentHeader:
						// Extract attachment
						filename, _ := h.Filename()
						ctype, _, _ := h.ContentType()
						data, err := io.ReadAll(p.Body)
						if err == nil && filename != "" {
							attachments = append(attachments, Attachment{
								Filename:    filename,
								ContentType: ctype,
								Data:        data,
							})
						}
					}
				}
			}
		}

		// If we don't have text/plain but have HTML, convert HTML to text
		if body_str == "" && html_body != "" {
			body_str = htmlToText(html_body)
		}

		id := fmt.Sprintf("%d", msg.SeqNum)
		from_addr := ""
		if len(env.From) > 0 {
			addr := env.From[0]
			if addr.MailboxName != "" && addr.HostName != "" {
				from_addr = addr.MailboxName + "@" + addr.HostName
			} else {
				from_addr = addr.Address()
			}
		}

		subj := env.Subject
		results = append(results, Email{
			ID:          id,
			From:        from_addr,
			Subject:     subj,
			Body:        body_str,
			HTMLBody:    html_body,
			Attachments: attachments,
		})
	}

	if err := <-done; err != nil {
		return results, fmt.Errorf("fetch: %w", err)
	}

	// Sort newest first (higher seqnum = newer)
	sort.Slice(results, func(i, j int) bool { return results[i].ID > results[j].ID })

	return results, nil
}

// FetchFromFolder fetches emails from a specific IMAP folder
func FetchFromFolder(username, password, folder string, offset, max int) ([]Email, error) {
	c, err := imapclient.DialTLS("imap.gmail.com:993", &tls.Config{ServerName: "imap.gmail.com"})
	if err != nil {
		return nil, fmt.Errorf("dial imap: %w", err)
	}
	defer c.Logout()

	if err := c.Login(username, password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	mbox, err := c.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("select %s: %w", folder, err)
	}

	if mbox.Messages == 0 {
		return []Email{}, nil
	}

	// Compute sequence range starting from offset into the newest emails
	var from uint32 = 1
	var to = mbox.Messages - uint32(offset)

	if to <= 0 {
		return []Email{}, nil
	}

	if to-uint32(max)+1 > 1 {
		from = to - uint32(max) + 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, max)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	var results []Email
	for msg := range messages {
		if msg == nil {
			continue
		}

		env := msg.Envelope

		// Default body
		body_str := ""
		html_body := ""
		var attachments []Attachment

		if r := msg.GetBody(section); r != nil {
			mr, err := mail.CreateReader(r)
			if err == nil {
				for {
					p, err := mr.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}

					switch h := p.Header.(type) {
					case *mail.InlineHeader:
						b, _ := io.ReadAll(p.Body)
						ctype, _, _ := h.ContentType()
						if strings.HasPrefix(ctype, "text/plain") && body_str == "" {
							body_str = string(b)
						} else if strings.HasPrefix(ctype, "text/html") && html_body == "" {
							html_body = string(b)
						} else if body_str == "" && html_body == "" {
							body_str = string(b)
						}
					case *mail.AttachmentHeader:
						filename, _ := h.Filename()
						ctype, _, _ := h.ContentType()
						data, err := io.ReadAll(p.Body)
						if err == nil && filename != "" {
							attachments = append(attachments, Attachment{
								Filename:    filename,
								ContentType: ctype,
								Data:        data,
							})
						}
					}
				}
			}
		}

		// If we don't have text/plain but have HTML, convert HTML to text
		if body_str == "" && html_body != "" {
			body_str = htmlToText(html_body)
		}

		id := fmt.Sprintf("%d", msg.SeqNum)
		from_addr := ""
		if len(env.From) > 0 {
			addr := env.From[0]
			if addr.MailboxName != "" && addr.HostName != "" {
				from_addr = addr.MailboxName + "@" + addr.HostName
			} else {
				from_addr = addr.Address()
			}
		}

		subj := env.Subject
		results = append(results, Email{
			ID:          id,
			From:        from_addr,
			Subject:     subj,
			Body:        body_str,
			HTMLBody:    html_body,
			Attachments: attachments,
		})
	}

	if err := <-done; err != nil {
		return results, fmt.Errorf("fetch: %w", err)
	}

	// Sort newest first (higher seqnum = newer)
	sort.Slice(results, func(i, j int) bool { return results[i].ID > results[j].ID })

	return results, nil
}

// FetchSentEmails fetches emails from the Sent folder (Gmail uses "[Gmail]/Sent Mail")
func FetchSentEmails(username, password string, max int) ([]Email, error) {
	return FetchFromFolder(username, password, "[Gmail]/Sent Mail", 0, max)
}

// EmailsFetchedMsg is sent back to a Bubble Tea program when FetchEmailsCmd completes.
type EmailsFetchedMsg struct {
	Emails []Email
	Err    error
}

// EmailsFetchedWithOffsetMsg is sent when fetching more emails with an offset
type EmailsFetchedWithOffsetMsg struct {
	Emails []Email
	Offset int
	Err    error
}

// SentEmailsFetchedMsg is sent when fetching sent emails
type SentEmailsFetchedMsg struct {
	Emails []Email
	Err    error
}

// FetchEmailsCmd returns a tea.Cmd that fetches recent emails and returns an EmailsFetchedMsg.
func FetchEmailsCmd(username, password string, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchLatest(username, password, max)
		return EmailsFetchedMsg{Emails: emails, Err: err}
	}
}

// FetchMoreEmailsCmd returns a tea.Cmd that fetches more emails from a given offset.
func FetchMoreEmailsCmd(username, password string, offset, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchWithOffset(username, password, offset, max)
		return EmailsFetchedWithOffsetMsg{Emails: emails, Offset: offset, Err: err}
	}
}

// FetchSentEmailsCmd returns a tea.Cmd that fetches sent emails
func FetchSentEmailsCmd(username, password string, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchSentEmails(username, password, max)
		return SentEmailsFetchedMsg{Emails: emails, Err: err}
	}
}

// htmlToText converts HTML to plain text by stripping tags and converting common elements
func htmlToText(html string) string {
	// Remove script and style tags and their content
	reScript := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	html = reScript.ReplaceAllString(html, "")
	reStyle := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = reStyle.ReplaceAllString(html, "")

	// Convert line breaks
	reBr := regexp.MustCompile(`(?i)<br\s*/?>`)
	html = reBr.ReplaceAllString(html, "\n")
	reP := regexp.MustCompile(`(?i)</p>`)
	html = reP.ReplaceAllString(html, "\n\n")
	reDiv := regexp.MustCompile(`(?i)</div>`)
	html = reDiv.ReplaceAllString(html, "\n")

	// Convert links to show URL
	reLink := regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	html = reLink.ReplaceAllString(html, "$2 [$1]")

	// Remove all other HTML tags
	reTag := regexp.MustCompile(`<[^>]+>`)
	text := reTag.ReplaceAllString(html, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")

	// Clean up excessive whitespace
	reWhitespace := regexp.MustCompile(`\n\s*\n\s*\n`)
	text = reWhitespace.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
