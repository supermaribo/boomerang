package offsite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Restore downloads a mirrored data directory from R2 into dataDir (overwriting files).
func Restore(ctx context.Context, dataDir string, cfg Config, log func(string)) (Result, error) {
	if cfg.AccountID == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return Result{}, fmt.Errorf("incomplete R2 credentials")
	}
	client, err := newClient(cfg)
	if err != nil {
		return Result{}, err
	}
	prefix := cfg.ObjectPrefix()
	keys, err := listRemoteKeys(ctx, client, cfg.Bucket, prefix+"/")
	if err != nil {
		return Result{}, err
	}
	if len(keys) == 0 {
		return Result{}, fmt.Errorf("no objects found at s3://%s/%s/ — check bucket and prefix", cfg.Bucket, prefix)
	}

	log(fmt.Sprintf("off-site restore: bucket %s prefix %s/ (%d object(s))", cfg.Bucket, prefix, len(keys)))

	var res Result
	for _, key := range keys {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		rel := strings.TrimPrefix(key, prefix+"/")
		if rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		dest := filepath.Join(dataDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return res, fmt.Errorf("mkdir %s: %w", rel, err)
		}
		n, err := downloadFile(ctx, client, cfg.Bucket, key, dest)
		if err != nil {
			return res, fmt.Errorf("download %s: %w", rel, err)
		}
		res.Files++
		res.Bytes += n
		res.Uploaded++
		if res.Uploaded%25 == 0 {
			log(fmt.Sprintf("off-site restore: downloaded %d file(s)…", res.Uploaded))
		}
	}

	masterKey := filepath.Join(dataDir, "secrets", "master.key")
	if _, err := os.Stat(masterKey); err != nil {
		return res, fmt.Errorf("restore incomplete: secrets/master.key not found in mirror")
	}
	dbPath := filepath.Join(dataDir, "app.db")
	if _, err := os.Stat(dbPath); err != nil {
		return res, fmt.Errorf("restore incomplete: app.db not found in mirror")
	}

	log(fmt.Sprintf("off-site restore done: %d file(s), %d bytes", res.Files, res.Bytes))
	return res, nil
}
