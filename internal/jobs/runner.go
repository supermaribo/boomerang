package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/filebackup"
	"github.com/boomerang-backup/boomerang/internal/mysqlbackup"
	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/offsite"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/google/uuid"
)

type Runner struct {
	Store   *store.Store
	Box     *crypto.Box
	DataDir string
	Offsite *offsite.Syncer
	mu      sync.Mutex
	running int
	max     int
	busy    map[string]bool
	cancel  map[string]bool
	queue   []queuedWork

	notifyLoad func() (notify.MailConfig, error)
	notifyName func(targetType, targetID string) string
}

type queuedWork struct {
	jobID     string
	targetKey string
	run       func()
}

func NewRunner(st *store.Store, box *crypto.Box, dataDir string, maxConcurrent int) *Runner {
	if maxConcurrent < 1 {
		maxConcurrent = defaultMaxJobs()
	}
	return &Runner{
		Store:   st,
		Box:     box,
		DataDir: dataDir,
		max:     maxConcurrent,
		busy:    map[string]bool{},
		cancel:  map[string]bool{},
	}
}

func defaultMaxJobs() int {
	if v := os.Getenv("BOOMERANG_MAX_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := runtime.NumCPU() * 2
	if n < 4 {
		return 4
	}
	if n > 16 {
		return 16
	}
	return n
}

func targetKey(targetType, targetID string) string {
	return targetType + ":" + targetID
}

func (r *Runner) submit(targetType, targetID, jobID string, run func()) {
	target := targetKey(targetType, targetID)
	work := queuedWork{jobID: jobID, targetKey: target, run: run}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = append(r.queue, work)
	r.pumpLocked()
}

func (r *Runner) pumpLocked() {
	for {
		progressed := false
		for i := 0; i < len(r.queue); i++ {
			w := r.queue[i]
			if r.running >= r.max || r.busy[w.targetKey] {
				continue
			}
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
			r.running++
			r.busy[w.targetKey] = true
			progressed = true
			go r.execute(w)
			i--
		}
		if !progressed {
			return
		}
	}
}

func (r *Runner) execute(w queuedWork) {
	if r.Store != nil {
		if j, _ := r.Store.GetJob(w.jobID); j != nil && j.Status == "cancelled" {
			r.finishExecute(w)
			return
		}
		_ = r.Store.UpdateJob(w.jobID, "running", "", time.Now().UTC(), nil)
	}
	if r.cancelled(w.jobID) {
		r.cancelledDone(w.jobID)
		r.finishExecute(w)
		return
	}
	w.run()
	r.finishExecute(w)
}

func (r *Runner) finishExecute(w queuedWork) {
	r.mu.Lock()
	r.running--
	delete(r.busy, w.targetKey)
	delete(r.cancel, w.jobID)
	r.pumpLocked()
	r.mu.Unlock()
}

func (r *Runner) cancelled(jobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel[jobID]
}

func (r *Runner) CancelJob(jobID string) error {
	if r.Store == nil {
		return fmt.Errorf("store unavailable")
	}
	j, err := r.Store.GetJob(jobID)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	switch j.Status {
	case "succeeded", "failed", "cancelled":
		return fmt.Errorf("job already finished")
	case "running":
		r.mu.Lock()
		r.cancel[jobID] = true
		r.mu.Unlock()
		_ = r.Store.AppendJobLog(jobID, "cancel requested")
		return nil
	}
	r.mu.Lock()
	for i := 0; i < len(r.queue); i++ {
		if r.queue[i].jobID == jobID {
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
			break
		}
	}
	r.mu.Unlock()
	now := time.Now().UTC()
	_ = r.Store.AppendJobLog(jobID, "cancelled before start")
	_ = r.Store.UpdateJob(jobID, "cancelled", "cancelled by user", time.Time{}, &now)
	return nil
}

func (r *Runner) cancelledDone(jobID string) {
	now := time.Now().UTC()
	_ = r.Store.AppendJobLog(jobID, "cancelled")
	_ = r.Store.UpdateJob(jobID, "cancelled", "cancelled by user", time.Time{}, &now)
}

func (r *Runner) checkCancelled(jobID string) bool {
	if !r.cancelled(jobID) {
		return false
	}
	r.cancelledDone(jobID)
	return true
}

func (r *Runner) Stats() (running, queued int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running, len(r.queue)
}

func (r *Runner) StartDBVerify(databaseID, versionID string) (string, error) {
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "db", databaseID, "verify"); err != nil {
		return "", err
	}
	r.submit("db", databaseID, jobID, func() {
		r.runDBVerify(jobID, databaseID, versionID)
	})
	return jobID, nil
}

func (r *Runner) runDBVerify(jobID, databaseID, versionID string) {
	_ = r.Store.AppendJobLog(jobID, "starting database verify")
	if r.checkCancelled(jobID) {
		return
	}
	ver, err := r.Store.GetVersion(versionID)
	if err != nil || ver == nil || ver.TargetType != "db" || ver.TargetID != databaseID || ver.Status != "succeeded" {
		r.fail(jobID, "version not found or not successful")
		return
	}
	if err := mysqlbackup.VerifyDBBackup(ver.PathOnDisk, r.Box); err != nil {
		r.fail(jobID, err.Error())
		return
	}
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	_ = r.Store.AppendJobLog(jobID, "database backup verified OK")
	r.notifyJob(jobID, false, "")
}

func (r *Runner) StartFileVerify(fileServerID, versionID string) (string, error) {
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "file", fileServerID, "verify"); err != nil {
		return "", err
	}
	r.submit("file", fileServerID, jobID, func() {
		r.runFileVerify(jobID, fileServerID, versionID)
	})
	return jobID, nil
}

func (r *Runner) runFileVerify(jobID, fileServerID, versionID string) {
	_ = r.Store.AppendJobLog(jobID, "starting backup verify")
	if r.checkCancelled(jobID) {
		return
	}
	ver, err := r.Store.GetVersion(versionID)
	if err != nil || ver == nil || ver.TargetID != fileServerID || ver.Status != "succeeded" {
		r.fail(jobID, "version not found or not successful")
		return
	}
	if err := backup.VerifyFileBackup(ver.PathOnDisk, r.Box); err != nil {
		r.fail(jobID, err.Error())
		return
	}
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	_ = r.Store.AppendJobLog(jobID, "backup verified OK")
	r.notifyJob(jobID, false, "")
}

func (r *Runner) StartFileBackup(fileServerID string) (string, error) {
	jobID := uuid.NewString()
	versionID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "file", fileServerID, "backup"); err != nil {
		return "", err
	}
	r.submit("file", fileServerID, jobID, func() {
		r.runFileBackup(jobID, versionID, fileServerID)
	})
	return jobID, nil
}

func (r *Runner) StartFileRestore(fileServerID, versionID string, paths []string) (string, error) {
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "file", fileServerID, "restore"); err != nil {
		return "", err
	}
	r.submit("file", fileServerID, jobID, func() {
		r.runFileRestore(jobID, fileServerID, versionID, paths)
	})
	return jobID, nil
}

func (r *Runner) StartDBBackup(databaseID string) (string, error) {
	jobID := uuid.NewString()
	versionID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "db", databaseID, "backup"); err != nil {
		return "", err
	}
	r.submit("db", databaseID, jobID, func() {
		r.runDBBackup(jobID, versionID, databaseID)
	})
	return jobID, nil
}

func (r *Runner) StartDBRestore(databaseID, versionID string, tables []string) (string, error) {
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "db", databaseID, "restore"); err != nil {
		return "", err
	}
	r.submit("db", databaseID, jobID, func() {
		r.runDBRestore(jobID, databaseID, versionID, tables)
	})
	return jobID, nil
}

func (r *Runner) runFileBackup(jobID, versionID, fileServerID string) {
	sink := newJobLogSink(r.Store, jobID)
	defer sink.flush()
	sink.log("starting file backup")

	fs, err := r.Store.GetFileServer(fileServerID)
	if err != nil || fs == nil {
		r.fail(jobID, "file server not found")
		return
	}
	est := int64(0)
	if prev, _ := r.Store.LastSucceededVersion("file", fileServerID); prev != nil {
		est = prev.Bytes
	}
	if err := checkDiskForBackup(r.DataDir, est, fs.Protocol); err != nil {
		r.fail(jobID, err.Error())
		return
	}
	target, err := r.fileTarget(fs)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}

	opt := filebackup.Options{Box: r.Box, ExcludePaths: fs.ExcludePaths}
	if fs.IncrementalEnabled && fs.Protocol != "rsync" {
		if prev, _ := r.Store.LastSucceededVersion("file", fileServerID); prev != nil {
			if m, err := backup.ReadFileManifest(prev.PathOnDisk); err == nil {
				opt.BaseManifest = m
				opt.BaseVersionID = prev.ID
			}
		}
	}

	outDir := filepath.Join(r.DataDir, "backups", "files", fileServerID, versionID)
	_ = r.Store.CreateVersion(versionID, "file", fileServerID, outDir)
	vlog, err := backup.NewVersionLogger(outDir)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}
	defer vlog.Close()
	log := func(line string) {
		if r.checkCancelled(jobID) {
			return
		}
		sink.log(line)
		vlog.Log(line)
	}
	log("--- file backup ---")
	log(fmt.Sprintf("job: %s", jobID))
	log(fmt.Sprintf("version: %s", versionID))
	log(fmt.Sprintf("target: %s (%s)", fs.Name, fs.Protocol))
	log(fmt.Sprintf("started: %s", time.Now().UTC().Format(time.RFC3339)))

	res, err := filebackup.Backup(target, outDir, opt, log)
	if r.checkCancelled(jobID) {
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		return
	}
	if err != nil {
		log("error: " + err.Error())
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	log(fmt.Sprintf("summary: %d files backed up, %d bytes, kind=%s", res.Files, res.Bytes, res.Manifest.Kind))
	if skipped, _ := backup.ReadSkippedLog(outDir); len(skipped) > 0 {
		log(fmt.Sprintf("summary: %d path(s) could not be read on the remote", len(skipped)))
	} else {
		log("summary: no missed paths recorded")
	}
	log(fmt.Sprintf("finished: %s", time.Now().UTC().Format(time.RFC3339)))
	_ = r.Store.UpdateVersion(versionID, "succeeded", res.Bytes)
	if m, err := backup.ReadFileManifest(outDir); err == nil {
		_ = r.Store.ReplaceManifestIndex(versionID, m.Entries)
	}
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	sink.log(fmt.Sprintf("version %s ready (%s)", versionID, res.Manifest.Kind))
	r.notifyJob(jobID, false, "")
	_ = r.Store.PruneVersions("file", fileServerID, store.Retention{
		Hourly: fs.RetainHourly, Daily: fs.RetainDaily, Weekly: fs.RetainWeekly,
		Monthly: fs.RetainMonthly, Yearly: fs.RetainYearly,
		Count: fs.RetainCount, Days: fs.RetainDays,
	})
	r.scheduleOffsite()
}

func (r *Runner) runFileRestore(jobID, fileServerID, versionID string, paths []string) {
	_ = r.Store.AppendJobLog(jobID, fmt.Sprintf("starting restore of %d path(s)", len(paths)))

	fs, err := r.Store.GetFileServer(fileServerID)
	if err != nil || fs == nil {
		r.fail(jobID, "file server not found")
		return
	}
	ver, err := r.Store.GetVersion(versionID)
	if err != nil || ver == nil || ver.TargetID != fileServerID {
		r.fail(jobID, "version not found")
		return
	}
	if ver.Status != "succeeded" {
		r.fail(jobID, "version is not a successful backup")
		return
	}
	target, err := r.fileTarget(fs)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}
	n, err := filebackup.RestoreSelected(r.Store, r.Box, target, versionID, ver.PathOnDisk, paths, func(line string) {
		_ = r.Store.AppendJobLog(jobID, line)
	})
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	_ = r.Store.AppendJobLog(jobID, fmt.Sprintf("restored %d entries", n))
	r.notifyJob(jobID, false, "")
}

func (r *Runner) runDBRestore(jobID, databaseID, versionID string, tables []string) {
	_ = r.Store.AppendJobLog(jobID, "starting database restore")

	db, err := r.Store.GetDatabase(databaseID)
	if err != nil || db == nil {
		r.fail(jobID, "database not found")
		return
	}
	ver, err := r.Store.GetVersion(versionID)
	if err != nil || ver == nil || ver.TargetType != "db" || ver.TargetID != databaseID {
		r.fail(jobID, "version not found")
		return
	}
	if ver.Status != "succeeded" {
		r.fail(jobID, "version is not a successful backup")
		return
	}
	t, err := r.mysqlTarget(db)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}
	log := func(line string) { _ = r.Store.AppendJobLog(jobID, line) }
	var restoreErr error
	if len(tables) > 0 {
		restoreErr = mysqlbackup.RestoreTables(r.Box, t, ver.PathOnDisk, tables, log)
	} else {
		restoreErr = mysqlbackup.RestoreFull(r.Box, t, ver.PathOnDisk, log)
	}
	if restoreErr != nil {
		r.fail(jobID, restoreErr.Error())
		return
	}
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	r.notifyJob(jobID, false, "")
}

func (r *Runner) MySQLTarget(db *store.Database) (mysqlbackup.Target, error) {
	return r.mysqlTarget(db)
}

func (r *Runner) mysqlTarget(db *store.Database) (mysqlbackup.Target, error) {
	passPlain, err := r.Box.Open(db.EncMysqlPassword)
	if err != nil {
		return mysqlbackup.Target{}, fmt.Errorf("decrypt mysql password failed")
	}
	t := mysqlbackup.Target{
		MysqlHost: db.MysqlHost,
		MysqlPort: db.MysqlPort,
		MysqlDB:   db.MysqlDB,
		MysqlUser: db.MysqlUser,
		MysqlPass: string(passPlain),
		IncludeTables: db.IncludeTables,
	}
	switch db.TunnelMode {
	case "fileserver":
		if !db.FileServerID.Valid {
			return t, fmt.Errorf("tunnel via file server but none selected")
		}
		fs, err := r.Store.GetFileServer(db.FileServerID.String)
		if err != nil || fs == nil {
			return t, fmt.Errorf("linked file server not found")
		}
		secret, err := r.sshSecret(fs.EncSecret)
		if err != nil {
			return t, err
		}
		t.UseTunnel = true
		t.SSHHost = fs.Host
		t.SSHPort = fs.Port
		t.SSHUser = fs.Username
		t.SSHAuth = fs.AuthMode
		t.SSHSecret = secret
		t.SSHHostKey = fs.SSHHostKey
		t.MysqlHost = "127.0.0.1"
	case "inline":
		secret, err := r.sshSecret(db.EncSSHSecret)
		if err != nil {
			return t, err
		}
		t.UseTunnel = true
		t.SSHHost = db.SSHHost.String
		t.SSHPort = db.SSHPort
		t.SSHUser = db.SSHUsername.String
		t.SSHAuth = db.AuthMode
		t.SSHSecret = secret
	}
	return t, nil
}

func (r *Runner) runDBBackup(jobID, versionID, databaseID string) {
	sink := newJobLogSink(r.Store, jobID)
	defer sink.flush()
	sink.log("starting database backup")

	db, err := r.Store.GetDatabase(databaseID)
	if err != nil || db == nil {
		r.fail(jobID, "database not found")
		return
	}
	est := int64(0)
	if prev, _ := r.Store.LastSucceededVersion("db", databaseID); prev != nil {
		est = prev.Bytes
	}
	if err := checkDiskForBackup(r.DataDir, est, ""); err != nil {
		r.fail(jobID, err.Error())
		return
	}
	t, err := r.mysqlTarget(db)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}

	outDir := filepath.Join(r.DataDir, "backups", "db", databaseID, versionID)
	_ = r.Store.CreateVersion(versionID, "db", databaseID, outDir)
	vlog, err := backup.NewVersionLogger(outDir)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}
	defer vlog.Close()
	log := func(line string) {
		if r.checkCancelled(jobID) {
			return
		}
		sink.log(line)
		vlog.Log(line)
	}
	log("--- database backup ---")
	log(fmt.Sprintf("job: %s", jobID))
	log(fmt.Sprintf("version: %s", versionID))
	log(fmt.Sprintf("target: %s (%s@%s/%s)", db.Name, db.MysqlUser, db.MysqlHost, db.MysqlDB))
	log(fmt.Sprintf("started: %s", time.Now().UTC().Format(time.RFC3339)))

	res, err := mysqlbackup.Backup(t, outDir, log)
	if r.checkCancelled(jobID) {
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		return
	}
	if err != nil {
		log("error: " + err.Error())
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	if err := r.encryptSQL(outDir); err != nil {
		log("error: encrypt: " + err.Error())
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	log(fmt.Sprintf("summary: %d table(s) backed up, %d bytes", len(res.Tables), res.Bytes))
	if len(res.Tables) > 0 {
		log("tables: " + strings.Join(res.Tables, ", "))
	}
	log(fmt.Sprintf("finished: %s", time.Now().UTC().Format(time.RFC3339)))
	_ = r.Store.UpdateVersion(versionID, "succeeded", res.Bytes)
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	sink.log(fmt.Sprintf("version %s ready (%d tables)", versionID, len(res.Tables)))
	r.notifyJob(jobID, false, "")
	_ = r.Store.PruneVersions("db", databaseID, store.Retention{
		Hourly: db.RetainHourly, Daily: db.RetainDaily, Weekly: db.RetainWeekly,
		Monthly: db.RetainMonthly, Yearly: db.RetainYearly,
		Count: db.RetainCount, Days: db.RetainDays,
	})
	r.scheduleOffsite()
}

func (r *Runner) encryptSQL(outDir string) error {
	plain := archive.SQLBlobPath(outDir)
	if r.Box == nil {
		return nil
	}
	if err := r.Box.EncryptFile(plain, crypto.EncryptedPath(plain)); err != nil {
		return err
	}
	return os.Remove(plain)
}

func (r *Runner) fileTarget(fs *store.FileServer) (remote.FileTarget, error) {
	secret, err := r.sshSecret(fs.EncSecret)
	if err != nil {
		return remote.FileTarget{}, err
	}
	var pin func(string) error
	if fs.SSHHostKey == "" && (fs.Protocol == "sftp" || fs.Protocol == "rsync") {
		id := fs.ID
		pin = func(fp string) error {
			return r.Store.SetFileServerSSHHostKey(id, fp)
		}
	}
	return remote.FileTarget{
		Protocol: fs.Protocol, Host: fs.Host, Port: fs.Port, Username: fs.Username,
		RemoteRoot: fs.RemoteRoot, IncludePaths: fs.IncludePaths, AuthMode: fs.AuthMode, Secret: secret,
		SSHHostKey: fs.SSHHostKey, PinHostKey: pin,
	}, nil
}

func (r *Runner) sshSecret(enc []byte) (remote.AuthSecret, error) {
	plain, err := r.Box.Open(enc)
	if err != nil {
		return remote.AuthSecret{}, fmt.Errorf("decrypt credentials failed")
	}
	return remote.UnmarshalSecret(plain)
}

func (r *Runner) SetNotifier(load func() (notify.MailConfig, error), nameFor func(targetType, targetID string) string) {
	r.notifyLoad = load
	r.notifyName = nameFor
}

func (r *Runner) notifyJob(jobID string, failed bool, errMsg string) {
	j, _ := r.Store.GetJob(jobID)
	if j == nil || r.notifyLoad == nil {
		return
	}
	cfg, err := r.notifyLoad()
	if err != nil || !cfg.Ready() {
		return
	}
	want := false
	switch j.Kind {
	case "backup", "verify":
		want = failed && cfg.Alerts.BackupFailure || !failed && cfg.Alerts.BackupSuccess
	case "restore":
		want = failed && cfg.Alerts.RestoreFailure || !failed && cfg.Alerts.RestoreSuccess
	}
	if !want {
		return
	}
	name := j.TargetID
	if r.notifyName != nil {
		name = r.notifyName(j.TargetType, j.TargetID)
	}
	if err := notify.JobEmail(cfg, name, jobID, j.Kind, failed, errMsg); err != nil {
		_ = r.Store.AppendJobLog(jobID, "notify failed: "+err.Error())
	}
}

func (r *Runner) fail(jobID, msg string) {
	if r.cancelled(jobID) {
		r.cancelledDone(jobID)
		return
	}
	now := time.Now().UTC()
	msg = truncateJobMessage(msg)
	_ = r.Store.AppendJobLog(jobID, "error: "+msg)
	_ = r.Store.UpdateJob(jobID, "failed", msg, time.Time{}, &now)
	r.notifyJob(jobID, true, msg)
}

func truncateJobMessage(msg string) string {
	const max = 4000
	msg = strings.TrimSpace(msg)
	if len(msg) <= max {
		return msg
	}
	return msg[:max] + "… (truncated)"
}

func (r *Runner) scheduleOffsite() {
	if r.Offsite != nil {
		r.Offsite.Schedule()
	}
}
