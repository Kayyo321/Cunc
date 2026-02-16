package sending

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"cunc/src/settings"
)

func Send(to, subject, body string) error {
	sett := settings.InitialModel()

	sender_email := sett.GetSetting("email")
	sender_password := sett.GetSetting("password")

	if sender_email == "" {
		return fmt.Errorf("sender email not configured in settings")
	}

	if sender_password == "" {
		return fmt.Errorf("sender password not configured in settings")
	}

	// Parse the email to get the SMTP server domain
	email_parts := strings.Split(sender_email, "@")
	if len(email_parts) != 2 {
		return fmt.Errorf("invalid email format")
	}

	domain := email_parts[1]

	// Determine SMTP server based on domain
	var smtp_host string
	switch domain {
	case "gmail.com":
		smtp_host = "smtp.gmail.com"
	case "outlook.com", "hotmail.com":
		smtp_host = "smtp-mail.outlook.com"
	default:
		smtp_host = "smtp." + domain
	}

	smtp_server := smtp_host + ":587"

	// Compose the email message
	message := "From: " + sender_email + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body

	// Connect to SMTP server
	conn, err := smtp.Dial(smtp_server)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	// Start TLS
	tlsconfig := &tls.Config{
		ServerName: smtp_host,
	}
	if err := conn.StartTLS(tlsconfig); err != nil {
		return fmt.Errorf("failed to start TLS: %v", err)
	}

	// Authenticate
	auth := smtp.PlainAuth("", sender_email, sender_password, smtp_host)
	if err := conn.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	// Send the email
	if err := conn.Mail(sender_email); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	if err := conn.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	wc, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to open message writer: %v", err)
	}

	_, err = wc.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = wc.Close()
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	conn.Quit()
	return nil
}
