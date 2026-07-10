package offsite

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/store"
)

// Syncer mirrors the local data directory to off-site storage after backups complete.
type Syncer struct {
	Store   *store.Store
	Box     *crypto.Box
	DataDir string

	mu      sync.Mutex
	running bool
	pending bool

	notifyLoad func() (notify.MailConfig, error)
}

func NewSyncer(st *store.Store, box *crypto.Box, dataDir string) *Syncer {
	return &Syncer{Store: st, Box: box, DataDir: dataDir}
}

func (s *Syncer) SetNotifier(load func() (notify.MailConfig, error)) {
	s.notifyLoad = load
}

// Schedule queues a mirror run. Multiple calls while a sync is running coalesce into one follow-up.
func (s *Syncer) Schedule() {
	s.mu.Lock()
	if s.running {
		s.pending = true
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.runLoop()
}

// RunNow performs a mirror immediately (used by manual sync API).
func (s *Syncer) RunNow(ctx context.Context) (Result, error) {
	cfg, err := LoadConfig(s.Store, s.Box)
	if err != nil {
		return Result{}, err
	}
	if !cfg.Ready() {
		return Result{}, fmt.Errorf("off-site mirror is disabled or incomplete")
	}
	return s.mirror(ctx, cfg)
}

func (s *Syncer) IsSyncing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Syncer) runLoop() {
	for {
		cfg, err := LoadConfig(s.Store, s.Box)
		if err != nil || !cfg.Ready() {
			s.finishLoop()
			return
		}
		_, err = s.mirror(context.Background(), cfg)
		if err != nil {
			log.Printf("off-site mirror: %v", err)
		}

		s.mu.Lock()
		if !s.pending {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.pending = false
		s.mu.Unlock()
	}
}

func (s *Syncer) finishLoop() {
	s.mu.Lock()
	s.running = false
	s.pending = false
	s.mu.Unlock()
}

func (s *Syncer) mirror(ctx context.Context, cfg Config) (Result, error) {
	_ = s.Store.SetMeta("offsite_sync_running", "1")
	_ = s.Store.SetMeta("offsite_last_error", "")
	defer func() { _ = s.Store.SetMeta("offsite_sync_running", "0") }()

	if s.Store != nil && s.Store.DB != nil {
		_, _ = s.Store.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}

	logFn := func(line string) { log.Printf("%s", line) }
	res, err := Mirror(ctx, s.DataDir, cfg, logFn)
	now := formatSyncTime(time.Now())
	_ = s.Store.SetMeta("offsite_last_sync", now)
	_ = s.Store.SetMeta("offsite_last_files", fmt.Sprintf("%d", res.Files))
	_ = s.Store.SetMeta("offsite_last_bytes", fmt.Sprintf("%d", res.Bytes))
	if err != nil {
		_ = s.Store.SetMeta("offsite_last_error", err.Error())
		s.alertFailure(err.Error())
		return res, err
	}
	_ = s.Store.SetMeta("offsite_last_error", "")
	_ = s.Store.SetMeta("offsite_last_alert_error", "")
	return res, nil
}

func (s *Syncer) alertFailure(errMsg string) {
	if s.notifyLoad == nil || s.Store == nil {
		return
	}
	last, _, _ := s.Store.GetMeta("offsite_last_alert_error")
	if last == errMsg {
		return
	}
	cfg, err := s.notifyLoad()
	if err != nil || !cfg.Ready() || !cfg.Alerts.OffsiteFailure {
		return
	}
	if err := notify.OffsiteMirrorEmail(cfg, errMsg); err != nil {
		log.Printf("off-site alert email: %v", err)
		return
	}
	_ = s.Store.SetMeta("offsite_last_alert_error", errMsg)
}
