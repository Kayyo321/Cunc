package inbox

import (
	"crypto/tls"
	"fmt"
	"io"
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

	// Compute sequence range for the last `max` messages
	var from uint32 = 1
	if mbox.Messages > uint32(max) {
		from = mbox.Messages - uint32(max) + 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

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
						} else if body_str == "" {
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

// EmailsFetchedMsg is sent back to a Bubble Tea program when FetchEmailsCmd completes.
type EmailsFetchedMsg struct {
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
