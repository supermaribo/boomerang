package backup

import "testing"

func TestDefaultBrowsePrefix(t *testing.T) {
	rsync := &FileManifest{
		Paths: []string{"/home/flexmedboomerangssh/htdocs"},
		Entries: []ManifestEntry{
			{Path: "home/flexmedboomerangssh/htdocs/index.php"},
		},
	}
	if got := DefaultBrowsePrefix(rsync); got != "home/flexmedboomerangssh/htdocs" {
		t.Fatalf("rsync prefix = %q, want home/flexmedboomerangssh/htdocs", got)
	}

	sftp := &FileManifest{
		Paths: []string{"/home/nhspaybackup/htdocs"},
		Entries: []ManifestEntry{
			{Path: ".gitignore"},
		},
	}
	if got := DefaultBrowsePrefix(sftp); got != "" {
		t.Fatalf("sftp prefix = %q, want empty", got)
	}
}

func TestRemoteBrowsePath(t *testing.T) {
	m := &FileManifest{Paths: []string{"/home/user/htdocs"}}
	if got := RemoteBrowsePath(m); got != "/home/user/htdocs" {
		t.Fatalf("remote path = %q", got)
	}
}
