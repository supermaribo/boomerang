package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/filebackup"
	"github.com/boomerang-backup/boomerang/internal/mysqlbackup"
	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/google/uuid"
)

type Runner struct {
	Store   *store.Store
	Box     *crypto.Box
	DataDir string
	mu      sync.Mutex
	active  int
	max     int

	notifyLoad func() (notify.MailConfig, error)
	notifyName func(targetType, targetID string) string
}

func NewRunner(st *store.Store, box *crypto.Box, dataDir string) *Runner {
	return &Runner{Store: st, Box: box, DataDir: dataDir, max: 2}
}

func (r *Runner) acquire() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active >= r.max {
		return fmt.Errorf("too many jobs running")
	}
	r.active++
	return nil
}

func (r *Runner) dec() {
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
}

func (r *Runner) StartFileBackup(fileServerID string) (string, error) {
	if err := r.acquire(); err != nil {
		return "", err
	}
	jobID := uuid.NewString()
	versionID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "file", fileServerID, "backup"); err != nil {
		r.dec()
		return "", err
	}
	go r.runFileBackup(jobID, versionID, fileServerID)
	return jobID, nil
}

func (r *Runner) StartFileRestore(fileServerID, versionID string, paths []string) (string, error) {
	if err := r.acquire(); err != nil {
		return "", err
	}
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "file", fileServerID, "restore"); err != nil {
		r.dec()
		return "", err
	}
	go r.runFileRestore(jobID, fileServerID, versionID, paths)
	return jobID, nil
}

func (r *Runner) StartDBBackup(databaseID string) (string, error) {
	if err := r.acquire(); err != nil {
		return "", err
	}
	jobID := uuid.NewString()
	versionID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "db", databaseID, "backup"); err != nil {
		r.dec()
		return "", err
	}
	go r.runDBBackup(jobID, versionID, databaseID)
	return jobID, nil
}

func (r *Runner) StartDBRestore(databaseID, versionID string, tables []string) (string, error) {
	if err := r.acquire(); err != nil {
		return "", err
	}
	jobID := uuid.NewString()
	if err := r.Store.CreateJob(jobID, "db", databaseID, "restore"); err != nil {
		r.dec()
		return "", err
	}
	go r.runDBRestore(jobID, databaseID, versionID, tables)
	return jobID, nil
}

func (r *Runner) runFileBackup(jobID, versionID, fileServerID string) {
	defer r.dec()
	_ = r.Store.UpdateJob(jobID, "running", "", time.Now().UTC(), nil)
	_ = r.Store.AppendJobLog(jobID, "starting file backup")

	fs, err := r.Store.GetFileServer(fileServerID)
	if err != nil || fs == nil {
		r.fail(jobID, "file server not found")
		return
	}
	target, err := r.fileTarget(fs)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}

	opt := filebackup.Options{Box: r.Box, ExcludePaths: fs.ExcludePaths}
	if prev, _ := r.Store.LastSucceededVersion("file", fileServerID); prev != nil {
		if m, err := backup.ReadFileManifest(prev.PathOnDisk); err == nil {
			opt.BaseManifest = m
			opt.BaseVersionID = prev.ID
		}
	}

	outDir := filepath.Join(r.DataDir, "backups", "files", fileServerID, versionID)
	_ = r.Store.CreateVersion(versionID, "file", fileServerID, outDir)
	res, err := filebackup.Backup(target, outDir, opt, func(line string) {
		_ = r.Store.AppendJobLog(jobID, line)
	})
	if err != nil {
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	_ = r.Store.UpdateVersion(versionID, "succeeded", res.Bytes)
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	_ = r.Store.AppendJobLog(jobID, fmt.Sprintf("version %s ready (%s)", versionID, res.Manifest.Kind))
	r.notifyJob(jobID, false, "")
	_ = r.Store.PruneVersions("file", fileServerID, store.Retention{
		Hourly: fs.RetainHourly, Daily: fs.RetainDaily, Weekly: fs.RetainWeekly, Yearly: fs.RetainYearly,
		Count: fs.RetainCount, Days: fs.RetainDays,
	})
}

func (r *Runner) runFileRestore(jobID, fileServerID, versionID string, paths []string) {
	defer r.dec()
	_ = r.Store.UpdateJob(jobID, "running", "", time.Now().UTC(), nil)
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
	defer r.dec()
	_ = r.Store.UpdateJob(jobID, "running", "", time.Now().UTC(), nil)
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
	defer r.dec()
	_ = r.Store.UpdateJob(jobID, "running", "", time.Now().UTC(), nil)
	_ = r.Store.AppendJobLog(jobID, "starting database backup")

	db, err := r.Store.GetDatabase(databaseID)
	if err != nil || db == nil {
		r.fail(jobID, "database not found")
		return
	}
	t, err := r.mysqlTarget(db)
	if err != nil {
		r.fail(jobID, err.Error())
		return
	}

	outDir := filepath.Join(r.DataDir, "backups", "db", databaseID, versionID)
	_ = r.Store.CreateVersion(versionID, "db", databaseID, outDir)
	res, err := mysqlbackup.Backup(t, outDir, func(line string) {
		_ = r.Store.AppendJobLog(jobID, line)
	})
	if err != nil {
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	if err := r.encryptSQL(outDir); err != nil {
		_ = r.Store.UpdateVersion(versionID, "failed", 0)
		r.fail(jobID, err.Error())
		return
	}
	_ = r.Store.UpdateVersion(versionID, "succeeded", res.Bytes)
	now := time.Now().UTC()
	_ = r.Store.UpdateJob(jobID, "succeeded", "", time.Time{}, &now)
	_ = r.Store.AppendJobLog(jobID, fmt.Sprintf("version %s ready (%d tables)", versionID, len(res.Tables)))
	r.notifyJob(jobID, false, "")
	_ = r.Store.PruneVersions("db", databaseID, store.Retention{
		Hourly: db.RetainHourly, Daily: db.RetainDaily, Weekly: db.RetainWeekly, Yearly: db.RetainYearly,
		Count: db.RetainCount, Days: db.RetainDays,
	})
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
	return remote.FileTarget{
		Protocol: fs.Protocol, Host: fs.Host, Port: fs.Port, Username: fs.Username,
		RemoteRoot: fs.RemoteRoot, IncludePaths: fs.IncludePaths, AuthMode: fs.AuthMode, Secret: secret,
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
	case "backup":
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
	now := time.Now().UTC()
	_ = r.Store.AppendJobLog(jobID, "error: "+msg)
	_ = r.Store.UpdateJob(jobID, "failed", msg, time.Time{}, &now)
	r.notifyJob(jobID, true, msg)
}
