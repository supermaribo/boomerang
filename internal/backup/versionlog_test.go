package backup

import "testing"

func TestSkippedLogLines(t *testing.T) {
	lines := SkippedLogLines([]string{".env", "storage/app/private/foo/"}, 0)
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %v", len(lines), lines)
	}
	if lines[0] != "--- missed paths (2) ---" {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if lines[1] != "missed: .env" {
		t.Fatalf("unexpected line: %q", lines[1])
	}
}

func TestLogHasMissedPaths(t *testing.T) {
	if LogHasMissedPaths([]string{"summary: ok"}) {
		t.Fatal("expected false")
	}
	if !LogHasMissedPaths([]string{"missed: .env"}) {
		t.Fatal("expected true")
	}
}
