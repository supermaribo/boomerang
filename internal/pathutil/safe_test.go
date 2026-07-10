package pathutil

import (
	"path/filepath"
	"testing"
)

func TestSafeDataPath(t *testing.T) {
	base := t.TempDir()
	cases := []struct {
		rel   string
		ok    bool
		check string
	}{
		{"app.db", true, "app.db"},
		{"secrets/master.key", true, filepath.Join("secrets", "master.key")},
		{"../etc/passwd", false, ""},
		{"foo/../../etc/passwd", false, ""},
		{"/etc/passwd", false, ""},
		{"backups/files/id/v1/data.tar.zst.enc", true, filepath.Join("backups", "files", "id", "v1", "data.tar.zst.enc")},
	}
	for _, tc := range cases {
		got, err := SafeDataPath(base, tc.rel)
		if tc.ok {
			if err != nil {
				t.Fatalf("rel %q: unexpected error: %v", tc.rel, err)
			}
			want := filepath.Join(base, tc.check)
			if got != want {
				t.Fatalf("rel %q: got %q want %q", tc.rel, got, want)
			}
		} else if err == nil {
			t.Fatalf("rel %q: expected error, got %q", tc.rel, got)
		}
	}
}
