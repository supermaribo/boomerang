package filebackup

import (
	"fmt"

	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
)

// VerifyCompleteness checks local archive integrity and that rsync did not skip paths.
func VerifyCompleteness(st *store.Store, versionID, versionDir string, box *crypto.Box) error {
	if err := VerifyVersionChain(st, versionID, versionDir, box); err != nil {
		return err
	}
	return backup.VerifyTransferComplete(versionDir)
}

// VerifyAgainstSource compares the snapshot to files currently readable on the host.
func VerifyAgainstSource(st *store.Store, versionID, versionDir string, box *crypto.Box, target remote.FileTarget, excludes []string, log Logger) error {
	if log == nil {
		log = func(string) {}
	}
	if err := VerifyCompleteness(st, versionID, versionDir, box); err != nil {
		return err
	}
	log("archive integrity OK")

	merged, err := LoadMergedManifest(st, "file", "", versionID, versionDir)
	if err != nil {
		return err
	}
	inBackup := map[string]backup.ManifestEntry{}
	for p, e := range backup.EntryIndex(merged) {
		inBackup[backup.NormalizeRelPath(p)] = e
	}
	log(fmt.Sprintf("snapshot has %d file(s); listing remote", len(inBackup)))

	live, err := remote.ListRegularFiles(target, excludes)
	if err != nil {
		return fmt.Errorf("list remote files: %w", err)
	}
	log(fmt.Sprintf("remote listing: %d readable file(s), %d unreadable folder(s)", len(live.Files), len(live.Unreadable)))

	if len(live.Unreadable) > 0 {
		return fmt.Errorf("backup user cannot read %d remote folder(s), e.g. %s — files there were not transferred", len(live.Unreadable), backup.SampleJoin(live.Unreadable, 8))
	}

	var missing []string
	var changed int
	for _, f := range live.Files {
		rel := backup.NormalizeRelPath(f.Rel)
		ent, ok := inBackup[rel]
		if !ok {
			missing = append(missing, f.Rel)
			continue
		}
		if ent.Size != f.Size {
			changed++
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d readable remote file(s) are not in this backup, e.g. %s", len(missing), backup.SampleJoin(missing, 8))
	}
	if changed > 0 {
		log(fmt.Sprintf("note: %d file(s) differ in size from live (changed since this backup)", changed))
	} else {
		log("live listing matches snapshot (all readable files are present)")
	}
	return nil
}
