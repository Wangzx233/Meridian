package control

import "testing"

func TestNormalizeEmailNotificationConfig(t *testing.T) {
	cfg, err := normalizeEmailNotificationConfig(EmailNotificationConfig{
		Name:         " team ",
		Enabled:      true,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		FromAddress:  "Workbench <workbench@example.com>",
		ToAddresses:  []string{"dev@example.com", " dev@example.com ", "ops@example.com"},
		TLSMode:      "",
		SMTPUsername: "user",
		SMTPPassword: "secret",
	})
	if err != nil {
		t.Fatalf("normalize config: %v", err)
	}
	if cfg.Name != "team" {
		t.Fatalf("name = %q, want team", cfg.Name)
	}
	if cfg.TLSMode != "starttls" {
		t.Fatalf("tls mode = %q, want starttls", cfg.TLSMode)
	}
	if cfg.SubjectPrefix != "[Meridian]" {
		t.Fatalf("subject prefix = %q", cfg.SubjectPrefix)
	}
	if len(cfg.ToAddresses) != 2 {
		t.Fatalf("to addresses = %#v, want deduped 2 addresses", cfg.ToAddresses)
	}
}

func TestNormalizeEmailNotificationConfigRejectsInvalidEmail(t *testing.T) {
	_, err := normalizeEmailNotificationConfig(EmailNotificationConfig{
		Name:        "team",
		Enabled:     true,
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "workbench@example.com",
		ToAddresses: []string{"not an email"},
		TLSMode:     "starttls",
	})
	if err == nil {
		t.Fatalf("expected invalid recipient to fail validation")
	}
}
