package jobs

import (
	"testing"
	"time"

	"github.com/boomerang-backup/boomerang/internal/store"
)

func TestLastBackupCheckReturnsSkipped(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.CreateJob("j1", "file", "fs1", "backup"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.UpdateJob("j1", "skipped", "", time.Time{}, &now); err != nil {
		t.Fatal(err)
	}
	j, err := st.LastBackupCheck("file", "fs1")
	if err != nil || j == nil {
		t.Fatalf("LastBackupCheck: %#v %v", j, err)
	}
	if j.Status != "skipped" {
		t.Fatalf("status = %s", j.Status)
	}
	if !j.FinishedAt.Valid {
		t.Fatal("expected finished_at")
	}
}

func TestLastBackupCheckIgnoresFailed(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.CreateJob("j1", "db", "d1", "backup"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.UpdateJob("j1", "failed", "boom", time.Time{}, &now); err != nil {
		t.Fatal(err)
	}
	j, err := st.LastBackupCheck("db", "d1")
	if err != nil {
		t.Fatal(err)
	}
	if j != nil {
		t.Fatalf("expected nil, got %#v", j)
	}
}
