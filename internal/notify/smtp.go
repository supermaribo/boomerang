package notify

import (
	"fmt"
	"net"
	"net/smtp"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       string
}

func (c SMTPConfig) Enabled() bool {
	return c.Host != "" && c.To != "" && c.From != "" && c.Port > 0
}

func SendSMTP(cfg SMTPConfig, subject, body string) error {
	if !cfg.Enabled() {
		return fmt.Errorf("smtp not fully configured")
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	msg := buildMessage(cfg.From, cfg.To, subject, body)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, []byte(msg))
}

// Send is kept for compatibility; prefers SMTP config as-is.
func Send(cfg SMTPConfig, subject, body string) error {
	return SendSMTP(cfg, subject, body)
}

// FailureEmail sends a backup failure notice via SMTP config (legacy).
func FailureEmail(cfg SMTPConfig, targetName, jobID, errMsg string) error {
	subject := fmt.Sprintf("[Boomerang] Backup failed: %s", targetName)
	body := fmt.Sprintf(`Boomerang reported a failed job.

Target: %s
Job ID: %s
Error:  %s

Open Boomerang to inspect the job log.
`, targetName, jobID, errMsg)
	return SendSMTP(cfg, subject, body)
}
