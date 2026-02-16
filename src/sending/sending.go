package sending

import (
	"fmt"
	"net/smtp"
	"strings"

	"cunc/src/settings"
)

// Send sends an email to the specified recipient with subject and body.
// Supports Gmail, Outlook/Hotmail, and generic SMTP hosts.
func Send(to, subject, body string) error {
	sett := settings.InitialModel()

	senderEmail := sett.GetSetting("email")
	senderPassword := sett.GetSetting("password")
	appPassword := sett.GetSetting("2fa-app-password")

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

	// Compose the message
	message := []byte("From: " + senderEmail + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body)

	// Set up authentication
	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	// Send the email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, senderEmail, []string{to}, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}
