package notify

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MailMode string

const (
	MailLocal MailMode = "local"
	MailSMTP  MailMode = "smtp"
)

// AlertPrefs controls which job outcomes trigger email.
type AlertPrefs struct {
	BackupSuccess  bool
	BackupFailure  bool
	RestoreSuccess bool
	RestoreFailure bool
}

type MailConfig struct {
	Mode   MailMode
	To     string
	From   string
	SMTP   SMTPConfig
	Alerts AlertPrefs
}

func (c MailConfig) Ready() bool {
	return strings.TrimSpace(c.To) != ""
}

func (c MailConfig) Send(subject, body string) error {
	if !c.Ready() {
		return fmt.Errorf("notify email address not configured")
	}
	from := strings.TrimSpace(c.From)
	if from == "" {
		from = defaultFrom()
	}
	switch c.Mode {
	case MailSMTP:
		cfg := c.SMTP
		if cfg.From == "" {
			cfg.From = from
		}
		if cfg.To == "" {
			cfg.To = c.To
		}
		if cfg.Host == "" || cfg.From == "" {
			return fmt.Errorf("smtp host and from address are required")
		}
		return SendSMTP(cfg, subject, body)
	default:
		return sendLocal(from, c.To, subject, body)
	}
}

func JobEmail(cfg MailConfig, targetName, jobID, kind string, failed bool, errMsg string) error {
	var subject, body string
	when := time.Now().Format(time.RFC3339)
	if failed {
		subject = fmt.Sprintf("[Boomerang] %s failed: %s", titleKind(kind), targetName)
		body = fmt.Sprintf(`Boomerang reported a failed job.

Target: %s
Kind:   %s
Job ID: %s
Time:   %s
Error:  %s

Open Boomerang to inspect the job log.
`, targetName, kind, jobID, when, errMsg)
	} else {
		subject = fmt.Sprintf("[Boomerang] %s succeeded: %s", titleKind(kind), targetName)
		body = fmt.Sprintf(`Boomerang completed a job successfully.

Target: %s
Kind:   %s
Job ID: %s
Time:   %s

Open Boomerang for details.
`, targetName, kind, jobID, when)
	}
	return cfg.Send(subject, body)
}

func titleKind(kind string) string {
	switch kind {
	case "backup":
		return "Backup"
	case "restore":
		return "Restore"
	default:
		if kind == "" {
			return "Job"
		}
		return strings.ToUpper(kind[:1]) + kind[1:]
	}
}

func defaultFrom() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "boomerang"
	}
	return "boomerang@" + host
}

func sendLocal(from, to, subject, body string) error {
	msg := buildMessage(from, to, subject, body)
	if path, err := exec.LookPath("sendmail"); err == nil {
		cmd := exec.Command(path, "-t", "-oi")
		cmd.Stdin = strings.NewReader(msg)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sendmail: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return SendSMTP(SMTPConfig{
		Host: "127.0.0.1",
		Port: 25,
		From: from,
		To:   to,
	}, subject, body)
}

func buildMessage(from, to, subject, body string) string {
	return strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Date: " + time.Now().Format(time.RFC1123Z),
		"",
		body,
	}, "\r\n")
}
