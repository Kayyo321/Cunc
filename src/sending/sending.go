package sending

import (
	"fmt"
	"net/smtp"
	"strings"

	"cunc/src/contacts"
	"cunc/src/settings"
)

// Send sends an email to the specified recipient with subject and body.
// Supports Gmail, Outlook/Hotmail, and generic SMTP hosts.
// The "to" parameter can include cc and bcc addresses in the format:
// "recipient@example.com cc: cc@example.com bcc: bcc@example.com"
func Send(to, subject, body string) error {
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
	messageBody := "From: " + senderEmail + "\r\n" +
		"To: " + toAddr + "\r\n"

	if ccAddrs != "" {
		messageBody += "Cc: " + ccAddrs + "\r\n"
	}

	messageBody += "Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body

	message := []byte(messageBody)

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
