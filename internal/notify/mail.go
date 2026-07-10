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
	BackupSuccess   bool
	BackupFailure   bool
	RestoreSuccess  bool
	RestoreFailure  bool
	OffsiteFailure  bool
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

func OffsiteMirrorEmail(cfg MailConfig, errMsg string) error {
	subject := "[Boomerang] Off-site mirror failed"
	body := fmt.Sprintf(`Boomerang could not mirror backups to off-site storage.

Time:  %s
Error: %s

Check Settings → Off-site and the appliance logs. Local backups are unaffected.
`, time.Now().Format(time.RFC3339), errMsg)
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
	// Prefer SMTP to local postfix — works when the service runs as a non-root user
	// (sendmail often needs membership in the postdrop group).
	err := SendSMTP(SMTPConfig{
		Host: "127.0.0.1",
		Port: 25,
		From: from,
		To:   to,
	}, subject, body)
	if err == nil {
		return nil
	}
	if path, lookErr := exec.LookPath("sendmail"); lookErr == nil {
		cmd := exec.Command(path, "-t", "-oi")
		cmd.Stdin = strings.NewReader(msg)
		if out, sendErr := cmd.CombinedOutput(); sendErr == nil {
			return nil
		} else if sendErr != nil {
			return fmt.Errorf("local mail failed (smtp: %v; sendmail: %w %s)", err, sendErr, strings.TrimSpace(string(out)))
		}
	}
	return fmt.Errorf("local mail failed: %w (is postfix running?)", err)
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
