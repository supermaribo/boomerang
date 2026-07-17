package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/metrics"
	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
)

type Poller struct {
	store      *store.Store
	box        *crypto.Box
	notifyLoad func() (notify.MailConfig, error)
	mu         sync.Mutex
	stop       chan struct{}
	running    bool
}

func NewPoller(st *store.Store, box *crypto.Box) *Poller {
	return &Poller{store: st, box: box, stop: make(chan struct{})}
}

func (p *Poller) SetNotifier(load func() (notify.MailConfig, error)) {
	p.notifyLoad = load
}

func (p *Poller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stop = make(chan struct{})
	p.mu.Unlock()
	go p.loop()
	log.Printf("monitoring poller started")
}

func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	close(p.stop)
	p.running = false
}

func (p *Poller) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	p.pollAll()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

func (p *Poller) pollAll() {
	servers, err := p.store.ListMonitoredServers()
	if err != nil {
		log.Printf("monitor list: %v", err)
		return
	}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}
		interval := time.Duration(srv.PollIntervalSec) * time.Second
		if interval < 30*time.Second {
			interval = time.Minute
		}
		if srv.LastPollAt.Valid {
			if t, err := store.ParseStoreTime(srv.LastPollAt.String); err == nil {
				if time.Since(t) < interval-5*time.Second {
					// Still evaluate offline/alerts even if we skip SSH this tick.
					p.evaluateAlerts(srv)
					continue
				}
			}
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(m store.MonitoredServer) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := p.PollOne(m.ID); err != nil {
				log.Printf("monitor poll %s: %v", m.Name, err)
			}
		}(srv)
	}
	wg.Wait()
	_ = p.store.PruneMonitorData(30, 365)
}

// PollOne fetches metrics for a single server over SSH.
func (p *Poller) PollOne(id string) error {
	m, err := p.store.GetMonitoredServer(id)
	if err != nil || m == nil {
		return fmt.Errorf("server not found")
	}
	secret, err := p.openSecret(m)
	if err != nil {
		_ = p.store.UpdateMonitoredServerPoll(id, time.Now().UTC(), err.Error(), "", "", nil)
		p.evaluateAlerts(*m)
		return err
	}

	since := time.Time{}
	if m.LastSampleAt.Valid {
		if t, err := store.ParseStoreTime(m.LastSampleAt.String); err == nil {
			since = t
		}
	}
	cmd := "boomerang-monitor ssh-export"
	if !since.IsZero() {
		cmd = fmt.Sprintf("boomerang-monitor ssh-export --since=%s", since.UTC().Format(time.RFC3339Nano))
	}

	out, err := p.runExport(*m, secret, cmd)
	now := time.Now().UTC()
	if err != nil {
		_ = p.store.UpdateMonitoredServerPoll(id, now, err.Error(), "", "", nil)
		updated, _ := p.store.GetMonitoredServer(id)
		if updated != nil {
			p.evaluateAlerts(*updated)
		}
		return err
	}

	var batch metrics.ExportBatch
	if err := json.Unmarshal(out, &batch); err != nil {
		_ = p.store.UpdateMonitoredServerPoll(id, now, "invalid metrics payload: "+err.Error(), "", "", nil)
		return err
	}
	if batch.SchemaVersion != 0 && batch.SchemaVersion != metrics.SchemaVersion {
		_ = p.store.UpdateMonitoredServerPoll(id, now, fmt.Sprintf("unsupported schema version %d", batch.SchemaVersion), batch.ClientVersion, "", nil)
		return fmt.Errorf("unsupported schema version %d", batch.SchemaVersion)
	}

	var latest *time.Time
	var bootID string
	for _, sample := range batch.Samples {
		if sample.SampledAt.IsZero() {
			continue
		}
		// Reject samples more than 24h in the future (clock skew).
		if sample.SampledAt.After(now.Add(24 * time.Hour)) {
			continue
		}
		inserted, err := p.store.InsertMonitorSample(id, sample)
		if err != nil {
			log.Printf("monitor ingest %s: %v", m.Name, err)
			continue
		}
		if inserted {
			_ = p.store.RollupMonitorHour(id, sample.SampledAt)
		}
		t := sample.SampledAt.UTC()
		if latest == nil || t.After(*latest) {
			latest = &t
			bootID = sample.BootID
		}
	}
	_ = p.store.UpdateMonitoredServerPoll(id, now, "", batch.ClientVersion, bootID, latest)
	updated, _ := p.store.GetMonitoredServer(id)
	if updated != nil {
		p.evaluateAlerts(*updated)
	}
	return nil
}

func (p *Poller) openSecret(m *store.MonitoredServer) (remote.AuthSecret, error) {
	if len(m.EncSecret) == 0 {
		return remote.AuthSecret{}, fmt.Errorf("monitoring SSH key not configured")
	}
	plain, err := p.box.Open(m.EncSecret)
	if err != nil {
		return remote.AuthSecret{}, fmt.Errorf("decrypt secret: %w", err)
	}
	return remote.UnmarshalSecret(plain)
}

func (p *Poller) runExport(m store.MonitoredServer, secret remote.AuthSecret, cmd string) ([]byte, error) {
	client, err := remote.DialSSH(m.Host, m.Port, m.Username, "key", secret, remote.HostKeyTrust{
		KnownFingerprint: m.SSHHostKey,
		Pin: func(fp string) error {
			return p.store.PinMonitoredHostKey(m.ID, fp)
		},
	})
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()
	select {
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return nil, fmt.Errorf("ssh export: %s", msg)
		}
	case <-time.After(45 * time.Second):
		_ = session.Close()
		return nil, fmt.Errorf("ssh export timed out")
	}
	return stdout.Bytes(), nil
}

// TestConnection verifies SSH + export works and returns a short status string.
func (p *Poller) TestConnection(id string) (string, error) {
	m, err := p.store.GetMonitoredServer(id)
	if err != nil || m == nil {
		return "", fmt.Errorf("server not found")
	}
	secret, err := p.openSecret(m)
	if err != nil {
		return "", err
	}
	out, err := p.runExport(*m, secret, "boomerang-monitor ssh-export")
	if err != nil {
		return "", err
	}
	var batch metrics.ExportBatch
	if err := json.Unmarshal(out, &batch); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := p.PollOne(id); err != nil {
		return "", err
	}
	ver := batch.ClientVersion
	if ver == "" {
		ver = "unknown"
	}
	return fmt.Sprintf("OK — client %s, %d sample(s) in spool", ver, len(batch.Samples)), nil
}
