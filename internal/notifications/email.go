package notifications

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// EmailConfig holds email service configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// EmailService handles sending emails
type EmailService struct {
	config EmailConfig
}

// NewEmailService creates a new email service
func NewEmailService(config EmailConfig) *EmailService {
	return &EmailService{config: config}
}

// NewEmailServiceFromEnv creates an email service from environment variables
func NewEmailServiceFromEnv() (*EmailService, error) {
	config := EmailConfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
		FromName:     os.Getenv("EMAIL_FROM_NAME"),
	}

	if config.SMTPHost == "" {
		config.SMTPHost = "localhost"
	}
	if config.SMTPPort == "" {
		config.SMTPPort = "1025" // Default to local dev SMTP
	}
	if config.FromAddress == "" {
		config.FromAddress = "noreply@orzbob.cloud"
	}
	if config.FromName == "" {
		config.FromName = "Orzbob Cloud"
	}

	return &EmailService{config: config}, nil
}

// Email represents an email to be sent
type Email struct {
	To      []string
	Subject string
	Body    string
	HTML    bool
}

// Send sends an email
func (s *EmailService) Send(ctx context.Context, email Email) error {
	if len(email.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// Prepare message
	from := fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromAddress)
	to := strings.Join(email.To, ", ")

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))

	if email.HTML {
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	}

	msg.WriteString("\r\n")
	msg.WriteString(email.Body)

	// Set up authentication
	var auth smtp.Auth
	if s.config.SMTPUsername != "" && s.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	// Send email
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	err := smtp.SendMail(addr, auth, s.config.FromAddress, email.To, msg.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// BudgetAlertData contains data for budget alert emails
type BudgetAlertData struct {
	OrgName        string
	HoursUsed      float64
	HoursIncluded  float64
	PercentageUsed int
	ResetDate      time.Time
	ManagePlanURL  string
}

const budgetAlertTemplate = `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #2563eb; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f8f9fa; }
        .alert-box { background-color: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; margin: 20px 0; }
        .usage-bar { background-color: #e0e0e0; height: 20px; border-radius: 10px; margin: 20px 0; }
        .usage-fill { background-color: #2563eb; height: 100%; border-radius: 10px; width: {{.PercentageUsed}}%; }
        .button { display: inline-block; padding: 10px 20px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Orzbob Cloud Usage Alert</h1>
        </div>
        <div class="content">
            <div class="alert-box">
                <h2>⚠️ {{.PercentageUsed}}% of included hours used</h2>
            </div>
            
            <p>Hello {{.OrgName}},</p>
            
            <p>Your organization has used <strong>{{.HoursUsed}} hours</strong> of your included <strong>{{.HoursIncluded}} hours</strong> this billing period.</p>
            
            <div class="usage-bar">
                <div class="usage-fill"></div>
            </div>
            
            <p>Your usage will reset on <strong>{{.ResetDate.Format "January 2, 2006"}}</strong>.</p>
            
            <p>Once you exceed your included hours, additional usage will be charged at standard rates:</p>
            <ul>
                <li>Small instances: $0.083/hour</li>
                <li>Medium instances: $0.167/hour</li>
                <li>Large instances: $0.333/hour</li>
                <li>GPU instances: $2.08/hour</li>
            </ul>
            
            <p style="text-align: center; margin-top: 30px;">
                <a href="{{.ManagePlanURL}}" class="button">Manage Your Plan</a>
            </p>
        </div>
    </div>
</body>
</html>
`

// SendBudgetAlert sends a budget threshold alert email
func (s *EmailService) SendBudgetAlert(ctx context.Context, to []string, data BudgetAlertData) error {
	tmpl, err := template.New("budget-alert").Parse(budgetAlertTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	subject := fmt.Sprintf("Orzbob Cloud: %d%% of included hours used", data.PercentageUsed)

	return s.Send(ctx, Email{
		To:      to,
		Subject: subject,
		Body:    body.String(),
		HTML:    true,
	})
}
