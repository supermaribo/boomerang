package jobs

import (
	"log"
	"time"

	"github.com/boomerang-backup/boomerang/internal/store"
)

const cleanupInterval = 24 * time.Hour
const jobLogKeepDays = 90

// StartCleanupLoop runs stale version and job-log cleanup daily.
func StartCleanupLoop(st *store.Store) {
	if st == nil {
		return
	}
	go func() {
		runCleanup(st)
		tick := time.NewTicker(cleanupInterval)
		defer tick.Stop()
		for range tick.C {
			runCleanup(st)
		}
	}()
}

func runCleanup(st *store.Store) {
	if err := st.CleanupStaleVersions(); err != nil {
		log.Printf("cleanup stale versions: %v", err)
	}
	if err := st.PruneJobLogs(jobLogKeepDays); err != nil {
		log.Printf("cleanup job logs: %v", err)
	}
}
