package mysqlbackup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type dbManifest struct {
	Database  string            `json:"database"`
	Tables    []string          `json:"tables"`
	Bytes     int64             `json:"bytes"`
	Checksums map[string]uint32 `json:"checksums,omitempty"`
	Finished  string            `json:"finished"`
}

func readDBManifest(versionDir string) (*dbManifest, error) {
	b, err := os.ReadFile(filepath.Join(versionDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m dbManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func writeDBManifest(outDir string, database string, tables []string, bytes int64, checksums map[string]uint32, finished string) error {
	m := dbManifest{
		Database:  database,
		Tables:    tables,
		Bytes:     bytes,
		Checksums: checksums,
		Finished:  finished,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "manifest.json"), b, 0o600)
}
