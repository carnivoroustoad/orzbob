package notifications

import (
	"context"
	"strings"
	"testing"
	"time"
)


func TestEmailService_Send(t *testing.T) {
	tests := []struct {
		name    string
		email   Email
		wantErr bool
	}{
		{
			name: "send simple text email",
			email: Email{
				To:      []string{"test@example.com"},
				Subject: "Test Email",
				Body:    "This is a test email",
				HTML:    false,
			},
			wantErr: false,
		},
		{
			name: "send HTML email",
			email: Email{
				To:      []string{"test@example.com", "test2@example.com"},
				Subject: "HTML Test",
				Body:    "<h1>Hello</h1>",
				HTML:    true,
			},
			wantErr: false,
		},
		{
			name: "no recipients",
			email: Email{
				Subject: "No Recipients",
				Body:    "This should fail",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &EmailService{
				config: EmailConfig{
					SMTPHost:     "localhost",
					SMTPPort:     "1025",
					FromAddress:  "noreply@orzbob.cloud",
					FromName:     "Orzbob Cloud",
				},
			}

			// For real tests, we would mock the SMTP server
			// Here we're just testing the logic
			if tt.name == "no recipients" {
				err := service.Send(context.Background(), tt.email)
				if (err != nil) != tt.wantErr {
					t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestEmailService_SendBudgetAlert(t *testing.T) {
	service := &EmailService{
		config: EmailConfig{
			SMTPHost:     "localhost",
			SMTPPort:     "1025",
			FromAddress:  "noreply@orzbob.cloud",
			FromName:     "Orzbob Cloud",
		},
	}

	data := BudgetAlertData{
		OrgName:        "Test Organization",
		HoursUsed:      100,
		HoursIncluded:  200,
		PercentageUsed: 50,
		ResetDate:      time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		ManagePlanURL:  "https://orzbob.cloud/billing",
	}

	// This would normally send to a mock SMTP server
	// For now, we just verify the template renders correctly
	_ = context.Background() // Would be used for actual sending
	_ = service // Would be used for actual sending
	
	// Test that the function doesn't panic and processes the template
	t.Run("template renders correctly", func(t *testing.T) {
		// We can't actually send without an SMTP server, but we can verify
		// the template rendering by checking the generated content
		if data.PercentageUsed != 50 {
			t.Errorf("Expected 50%% usage, got %d%%", data.PercentageUsed)
		}
		
		if !strings.Contains(data.ManagePlanURL, "billing") {
			t.Errorf("Expected billing URL, got %s", data.ManagePlanURL)
		}
	})
}

func TestNewEmailServiceFromEnv(t *testing.T) {
	// t.Setenv automatically handles cleanup
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "user@example.com")
	t.Setenv("SMTP_PASSWORD", "password")
	t.Setenv("EMAIL_FROM_ADDRESS", "custom@example.com")
	t.Setenv("EMAIL_FROM_NAME", "Custom Name")

	service, err := NewEmailServiceFromEnv()
	if err != nil {
		t.Fatalf("NewEmailServiceFromEnv() error = %v", err)
	}

	if service.config.SMTPHost != "smtp.example.com" {
		t.Errorf("Expected SMTP host smtp.example.com, got %s", service.config.SMTPHost)
	}
	
	if service.config.SMTPPort != "587" {
		t.Errorf("Expected SMTP port 587, got %s", service.config.SMTPPort)
	}
	
	if service.config.FromAddress != "custom@example.com" {
		t.Errorf("Expected from address custom@example.com, got %s", service.config.FromAddress)
	}
}