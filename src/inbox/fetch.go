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

// fetchlatest connects to gmail imap and grabs the newest emails. fancy.
// username should be the full address and password is either normal or app-only.
func FetchLatest(username, password string, max int) ([]Email, error) {
	return FetchWithOffset(username, password, 0, max)
}

// fetchwithoffset pulls emails starting at the offset, because paging is a thing.
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

	// compute sequence range from the newest set, because math.
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

		// start empty, as usual
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
						// grab attachment
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

		// no plain text, so we fake it
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

	// newest first, because obviously
	sort.Slice(results, func(i, j int) bool { return results[i].ID > results[j].ID })

	return results, nil
}

// fetchfromfolder grabs emails from a specific imap folder, no surprises
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

	// compute sequence range from the newest emails, because more math
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

		// start empty, again
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

		// no plain text, so we fake it
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

	// newest first, because obviously
	sort.Slice(results, func(i, j int) bool { return results[i].ID > results[j].ID })

	return results, nil
}

// fetchsentemails fetches emails from the sent folder (gmail uses "[gmail]/sent mail")
func FetchSentEmails(username, password string, max int) ([]Email, error) {
	return FetchFromFolder(username, password, "[Gmail]/Sent Mail", 0, max)
}

// emailsfetchedmsg is sent back to bubble tea when fetching is done
type EmailsFetchedMsg struct {
	Emails []Email
	Err    error
}

// emailsfetchedwithoffsetmsg is sent when fetching more emails with an offset
type EmailsFetchedWithOffsetMsg struct {
	Emails []Email
	Offset int
	Err    error
}

// sentemailsfetchedmsg is sent when fetching sent emails
type SentEmailsFetchedMsg struct {
	Emails []Email
	Err    error
}

// fetcheemailscmd returns a tea.cmd that fetches recent emails
func FetchEmailsCmd(username, password string, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchLatest(username, password, max)
		return EmailsFetchedMsg{Emails: emails, Err: err}
	}
}

// fetchmoreemailscmd returns a tea.cmd that fetches more emails from an offset
func FetchMoreEmailsCmd(username, password string, offset, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchWithOffset(username, password, offset, max)
		return EmailsFetchedWithOffsetMsg{Emails: emails, Offset: offset, Err: err}
	}
}

// fetchsentemailscmd returns a tea.cmd that fetches sent emails
func FetchSentEmailsCmd(username, password string, max int) tea.Cmd {
	return func() tea.Msg {
		emails, err := FetchSentEmails(username, password, max)
		return SentEmailsFetchedMsg{Emails: emails, Err: err}
	}
}

// htmltotext converts html to plain text, because tags are annoying here
func htmlToText(html string) string {
	// remove script and style tags, nobody wants that in email
	reScript := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	html = reScript.ReplaceAllString(html, "")
	reStyle := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = reStyle.ReplaceAllString(html, "")

	// convert line breaks, because html can't help itself
	reBr := regexp.MustCompile(`(?i)<br\s*/?>`)
	html = reBr.ReplaceAllString(html, "\n")
	reP := regexp.MustCompile(`(?i)</p>`)
	html = reP.ReplaceAllString(html, "\n\n")
	reDiv := regexp.MustCompile(`(?i)</div>`)
	html = reDiv.ReplaceAllString(html, "\n")

	// convert links to show url, so you know where you're going
	reLink := regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	html = reLink.ReplaceAllString(html, "$2 [$1]")

	// remove all other html tags, because chaos
	reTag := regexp.MustCompile(`<[^>]+>`)
	text := reTag.ReplaceAllString(html, "")

	// decode common html entities, because ampersands are not that cute
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")

	// clean up excessive whitespace, because no one asked for it
	reWhitespace := regexp.MustCompile(`\n\s*\n\s*\n`)
	text = reWhitespace.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
