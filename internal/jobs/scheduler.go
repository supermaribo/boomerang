package jobs

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/boomerang-backup/boomerang/internal/tzutil"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	runner     *Runner
	store      *store.Store
	cron       *cron.Cron
	mu         sync.Mutex
	running    bool
	ids        map[string]cron.EntryID
	notifyLoad func() (notify.MailConfig, error)
	notifyName func(targetType, targetID string) string
}

func NewScheduler(r *Runner, st *store.Store) *Scheduler {
	return &Scheduler{
		runner: r,
		store:  st,
		cron:   cron.New(),
		ids:    map[string]cron.EntryID{},
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	s.Reload()
	s.cron.Start()
	s.startMissedLoop()
	log.Printf("scheduler started (timezone %s)", tzutil.Name(s.store))
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *Scheduler) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil && s.running {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}
	s.ids = map[string]cron.EntryID{}
	loc := tzutil.Load(s.store)
	s.cron = cron.New(cron.WithLocation(loc))
	if s.running {
		s.cron.Start()
	}

	files, err := s.store.ListFileServers()
	if err == nil {
		for _, f := range files {
			if !f.Enabled || f.ScheduleCron == "" {
				continue
			}
			fs := f
			eid, err := s.cron.AddFunc(fs.ScheduleCron, func() {
				if !scheduleDue(fs.ScheduleStart) {
					log.Printf("scheduled file backup %s skipped (before start %s)", fs.Name, fs.ScheduleStart)
					return
				}
				jobID, err := s.runner.StartFileBackup(fs.ID)
				if err != nil {
					log.Printf("scheduled file backup %s: %v", fs.Name, err)
					return
				}
				log.Printf("scheduled file backup %s job %s", fs.Name, jobID)
			})
			if err != nil {
				log.Printf("bad cron for file server %s (%s): %v", fs.Name, fs.ScheduleCron, err)
				continue
			}
			s.ids["file:"+fs.ID] = eid
		}
	}
	dbs, err := s.store.ListDatabases()
	if err == nil {
		for _, d := range dbs {
			if !d.Enabled || d.ScheduleCron == "" {
				continue
			}
			db := d
			eid, err := s.cron.AddFunc(db.ScheduleCron, func() {
				if !scheduleDue(db.ScheduleStart) {
					log.Printf("scheduled db backup %s skipped (before start %s)", db.Name, db.ScheduleStart)
					return
				}
				jobID, err := s.runner.StartDBBackup(db.ID)
				if err != nil {
					log.Printf("scheduled db backup %s: %v", db.Name, err)
					return
				}
				log.Printf("scheduled db backup %s job %s", db.Name, jobID)
			})
			if err != nil {
				log.Printf("bad cron for database %s (%s): %v", db.Name, db.ScheduleCron, err)
				continue
			}
			s.ids["db:"+db.ID] = eid
		}
	}
}

func scheduleDue(start string) bool {
	start = strings.TrimSpace(start)
	if start == "" {
		return true
	}
	t, ok := parseStart(start)
	if !ok {
		return true
	}
	return !time.Now().UTC().Before(t)
}

func parseStart(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
