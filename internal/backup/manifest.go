package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileManifest is stored cleartext beside encrypted blobs for fast browse.
type FileManifest struct {
	Root          string         `json:"root"`
	Paths         []string       `json:"paths,omitempty"`
	Kind          string         `json:"kind"` // full | incremental
	BaseVersionID string         `json:"baseVersionId,omitempty"`
	Encrypted     bool           `json:"encrypted,omitempty"`
	Entries       []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
	Mtime string `json:"mtime"`
	IsDir bool   `json:"isDir"`
}

func ReadFileManifest(versionDir string) (*FileManifest, error) {
	b, err := os.ReadFile(filepath.Join(versionDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m FileManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func WriteFileManifest(versionDir string, m *FileManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(versionDir, "manifest.json"), b, 0o600)
}

// EntryIndex builds path -> entry for incremental comparisons.
func EntryIndex(m *FileManifest) map[string]ManifestEntry {
	out := map[string]ManifestEntry{}
	if m == nil {
		return out
	}
	for _, e := range m.Entries {
		if e.IsDir {
			continue
		}
		out[e.Path] = e
	}
	return out
}

// Changed returns true if file differs from the base snapshot.
func Changed(e ManifestEntry, base map[string]ManifestEntry) bool {
	prev, ok := base[e.Path]
	if !ok {
		return true
	}
	return prev.Size != e.Size || prev.Mtime != e.Mtime
}
