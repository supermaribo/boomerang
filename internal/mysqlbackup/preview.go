package mysqlbackup

import (
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

type RestoreTableRow struct {
	Name     string `json:"name"`
	InBackup bool   `json:"inBackup"`
	InLive   bool   `json:"inLive"`
}

type RestorePreviewResult struct {
	Tables      []RestoreTableRow `json:"tables"`
	BackupCount int               `json:"backupCount"`
	LiveCount   int               `json:"liveCount"`
	OnlyBackup  []string          `json:"onlyBackup"`
	OnlyLive    []string          `json:"onlyLive"`
	Message     string            `json:"message"`
}

// BuildRestorePreview compares backup manifest tables with the live database.
func BuildRestorePreview(box *crypto.Box, t Target, versionDir string, selected []string) (RestorePreviewResult, error) {
	_ = box
	backupTables, err := ReadManifestTables(versionDir)
	if err != nil {
		return RestorePreviewResult{}, err
	}
	want := map[string]bool{}
	if len(selected) > 0 {
		for _, tbl := range selected {
			want[tbl] = true
		}
	} else {
		for _, tbl := range backupTables {
			want[tbl] = true
		}
	}

	liveTables, liveErr := ListTables(t, nil)
	liveSet := map[string]bool{}
	for _, tbl := range liveTables {
		liveSet[tbl] = true
	}

	var rows []RestoreTableRow
	seen := map[string]bool{}
	for _, tbl := range backupTables {
		if len(want) > 0 && !want[tbl] {
			continue
		}
		seen[tbl] = true
		rows = append(rows, RestoreTableRow{Name: tbl, InBackup: true, InLive: liveSet[tbl]})
	}
	if liveErr == nil {
		for _, tbl := range liveTables {
			if seen[tbl] || (len(want) > 0 && !want[tbl]) {
				continue
			}
			rows = append(rows, RestoreTableRow{Name: tbl, InBackup: false, InLive: true})
		}
	}

	var onlyBackup, onlyLive []string
	for _, r := range rows {
		if r.InBackup && !r.InLive {
			onlyBackup = append(onlyBackup, r.Name)
		}
		if !r.InBackup && r.InLive {
			onlyLive = append(onlyLive, r.Name)
		}
	}

	msg := "Selected tables will be imported into the live database, overwriting existing data."
	if liveErr != nil {
		msg = "Could not reach live database — showing backup contents only. Restore will overwrite matching tables."
	} else if len(onlyBackup) > 0 || len(onlyLive) > 0 {
		msg = "Review differences before restore. Tables only in the backup will be created; tables only on live are unchanged unless selected."
	}

	return RestorePreviewResult{
		Tables:      rows,
		BackupCount: len(backupTables),
		LiveCount:   len(liveTables),
		OnlyBackup:  onlyBackup,
		OnlyLive:    onlyLive,
		Message:     msg,
	}, nil
}
