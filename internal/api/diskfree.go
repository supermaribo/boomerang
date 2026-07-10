package api

import "github.com/boomerang-backup/boomerang/internal/diskfree"

func diskFree(path string) (uint64, bool) {
	return diskfree.Bytes(path)
}
