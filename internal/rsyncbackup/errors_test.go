package rsyncbackup

import (
	"strings"
	"testing"
)

func TestSummarizeRsyncWarnings(t *testing.T) {
	out := `rsync: [sender] send_files failed to open "/home/user/htdocs/.env": Permission denied (13)
rsync: [sender] opendir "/home/user/htdocs/storage/app/private/avatars": Permission denied (13)
rsync: [sender] send_files failed to open "/home/user/htdocs/vendor/foo 2.svg": Permission denied (13)`
	msg := summarizeRsyncWarnings(out, "/home/user/htdocs")
	if !strings.Contains(msg, "2 file") || !strings.Contains(msg, "1 folder") {
		t.Fatalf("unexpected summary: %q", msg)
	}
	if !strings.Contains(msg, ".env") {
		t.Fatalf("expected .env in samples: %q", msg)
	}
}
