package sending

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"cunc/src/contacts"
	"cunc/src/settings"
)

// Send sends an email to the specified recipient with subject and body.
// Supports Gmail, Outlook/Hotmail, and generic SMTP hosts.
// The "to" parameter can include cc and bcc addresses in the format:
// "recipient@example.com cc: cc@example.com bcc: bcc@example.com"
// Attachments are file paths to attach to the email.
func Send(to, subject, body string, attachments []string) error {
	sett := settings.InitialModel()

	sender_email := sett.GetSetting("email")
	sender_password := sett.GetSetting("password")
	app_password := sett.GetSetting("2fa app-password")

	if sender_email == "" {
		return fmt.Errorf("sender email not configured in settings")
	}

	// Prefer 2FA app password if it exists
	if app_password != "" {
		sender_password = app_password
	}

	if sender_password == "" {
		return fmt.Errorf("sender password not configured in settings")
	}

	// Parse the to field to extract to, cc, and bcc
	to_addr, cc_addrs, bcc_addrs, err := ParseToSection(to)
	if err != nil {
		return err
	}

	// Parse the email to get the domain
	parts := strings.Split(sender_email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid sender email format")
	}
	domain := parts[1]

	// Determine SMTP host and port based on domain
	var smtp_host, smtp_port string
	switch domain {
	case "gmail.com":
		smtp_host = "smtp.gmail.com"
		smtp_port = "587"
	case "outlook.com", "hotmail.com":
		smtp_host = "smtp-mail.outlook.com"
		smtp_port = "587"
	default:
		smtp_host = "smtp." + domain
		smtp_port = "587"
	}

	// Build the recipient list (to + cc + bcc all get the email, but only to/cc appear in headers)
	recipients := []string{to_addr}

	var cc_list []string
	if cc_addrs != "" {
		cc_list = strings.Fields(cc_addrs)
		recipients = append(recipients, cc_list...)
	}

	var bcc_list []string
	if bcc_addrs != "" {
		bcc_list = strings.Fields(bcc_addrs)
		recipients = append(recipients, bcc_list...)
	}

	// Compose the message
	var message []byte
	if len(attachments) > 0 {
		message, err = build_mime_message_with_attachments(sender_email, to_addr, cc_addrs, subject, body, attachments)
		if err != nil {
			return fmt.Errorf("failed to build message with attachments: %v", err)
		}
	} else {
		message_body := "From: " + sender_email + "\r\n" +
			"To: " + to_addr + "\r\n"

		if cc_addrs != "" {
			message_body += "Cc: " + cc_addrs + "\r\n"
		}

		message_body += "Subject: " + subject + "\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body

		message = []byte(message_body)
	}

	// Set up authentication
	auth := smtp.PlainAuth("", sender_email, sender_password, smtp_host)

	// Send the email
	err = smtp.SendMail(smtp_host+":"+smtp_port, auth, sender_email, recipients, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	// Save the recipient to contact history
	history := contacts.Load()
	history.AddContact(to_addr)
	history.Save() // Ignore error, this is not critical

	return nil
}

// build_mime_message_with_attachments creates a MIME multipart message with attachments
func build_mime_message_with_attachments(from, to, cc, subject, body string, attachments []string) ([]byte, error) {
	var buf bytes.Buffer
	boundary := "----=_NextPart_000_0000_01DA1234.5678ABCD"

	// Write headers
	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + to + "\r\n")
	if cc != "" {
		buf.WriteString("Cc: " + cc + "\r\n")
	}
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n")
	buf.WriteString("\r\n")

	// Write body part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body + "\r\n")
	buf.WriteString("\r\n")

	// Write attachment parts
	for _, filePath := range attachments {
		err := add_attachment_to_part(&buf, boundary, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to attach %s: %v", filePath, err)
		}
	}

	// Write final boundary
	buf.WriteString("--" + boundary + "--\r\n")

	return buf.Bytes(), nil
}

// add_attachment_to_part adds a single file attachment to the MIME message
func add_attachment_to_part(buf *bytes.Buffer, boundary, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	file_content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	file_name := filepath.Base(filePath)
	// Encode filename for MIME header
	encoded_file_name := mime.QEncoding.Encode("UTF-8", file_name)

	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/octet-stream; name=\"" + encoded_file_name + "\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("Content-Disposition: attachment; filename=\"" + encoded_file_name + "\"\r\n")
	buf.WriteString("\r\n")

	// Encode file content as base64
	encoded := base64.StdEncoding.EncodeToString(file_content)
	// Split into 76-character lines as per RFC 2045
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buf.WriteString(encoded[i:end] + "\r\n")
	}

	return nil
}
