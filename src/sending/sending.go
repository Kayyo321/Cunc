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

	senderEmail := sett.GetSetting("email")
	senderPassword := sett.GetSetting("password")
	appPassword := sett.GetSetting("2fa app-password")

	if senderEmail == "" {
		return fmt.Errorf("sender email not configured in settings")
	}

	// Prefer 2FA app password if it exists
	if appPassword != "" {
		senderPassword = appPassword
	}

	if senderPassword == "" {
		return fmt.Errorf("sender password not configured in settings")
	}

	// Parse the to field to extract to, cc, and bcc
	toAddr, ccAddrs, bccAddrs, err := ParseToSection(to)
	if err != nil {
		return err
	}

	// Parse the email to get the domain
	parts := strings.Split(senderEmail, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid sender email format")
	}
	domain := parts[1]

	// Determine SMTP host and port based on domain
	var smtpHost, smtpPort string
	switch domain {
	case "gmail.com":
		smtpHost = "smtp.gmail.com"
		smtpPort = "587"
	case "outlook.com", "hotmail.com":
		smtpHost = "smtp-mail.outlook.com"
		smtpPort = "587"
	default:
		smtpHost = "smtp." + domain
		smtpPort = "587"
	}

	// Build the recipient list (to + cc + bcc all get the email, but only to/cc appear in headers)
	recipients := []string{toAddr}

	var ccList []string
	if ccAddrs != "" {
		ccList = strings.Fields(ccAddrs)
		recipients = append(recipients, ccList...)
	}

	var bccList []string
	if bccAddrs != "" {
		bccList = strings.Fields(bccAddrs)
		recipients = append(recipients, bccList...)
	}

	// Compose the message
	var message []byte
	if len(attachments) > 0 {
		message, err = buildMIMEMessageWithAttachments(senderEmail, toAddr, ccAddrs, subject, body, attachments)
		if err != nil {
			return fmt.Errorf("failed to build message with attachments: %v", err)
		}
	} else {
		messageBody := "From: " + senderEmail + "\r\n" +
			"To: " + toAddr + "\r\n"

		if ccAddrs != "" {
			messageBody += "Cc: " + ccAddrs + "\r\n"
		}

		messageBody += "Subject: " + subject + "\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body

		message = []byte(messageBody)
	}

	// Set up authentication
	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	// Send the email
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, senderEmail, recipients, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	// Save the recipient to contact history
	history := contacts.Load()
	history.AddContact(toAddr)
	history.Save() // Ignore error, this is not critical

	return nil
}

// buildMIMEMessageWithAttachments creates a MIME multipart message with attachments
func buildMIMEMessageWithAttachments(from, to, cc, subject, body string, attachments []string) ([]byte, error) {
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
		err := addAttachmentToPart(&buf, boundary, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to attach %s: %v", filePath, err)
		}
	}

	// Write final boundary
	buf.WriteString("--" + boundary + "--\r\n")

	return buf.Bytes(), nil
}

// addAttachmentToPart adds a single file attachment to the MIME message
func addAttachmentToPart(buf *bytes.Buffer, boundary, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)
	// Encode filename for MIME header
	encodedFileName := mime.QEncoding.Encode("UTF-8", fileName)

	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/octet-stream; name=\"" + encodedFileName + "\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("Content-Disposition: attachment; filename=\"" + encodedFileName + "\"\r\n")
	buf.WriteString("\r\n")

	// Encode file content as base64
	encoded := base64.StdEncoding.EncodeToString(fileContent)
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
