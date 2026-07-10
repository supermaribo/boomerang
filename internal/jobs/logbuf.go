package jobs

import (
	"github.com/boomerang-backup/boomerang/internal/store"
)

const jobLogBatchSize = 25

type jobLogSink struct {
	store *store.Store
	jobID string
	lines []string
}

func newJobLogSink(st *store.Store, jobID string) *jobLogSink {
	return &jobLogSink{store: st, jobID: jobID}
}

func (l *jobLogSink) log(line string) {
	l.lines = append(l.lines, line)
	if len(l.lines) >= jobLogBatchSize {
		l.flush()
	}
}

func (l *jobLogSink) flush() {
	if len(l.lines) == 0 {
		return
	}
	_ = l.store.AppendJobLogs(l.jobID, l.lines)
	l.lines = l.lines[:0]
}
