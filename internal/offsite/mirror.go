package offsite

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Files      int
	Bytes      int64
	Uploaded   int
	Deleted    int
	Skipped    int
}

// Mirror uploads the local data directory to R2 and removes remote objects that no longer exist locally.
func Mirror(ctx context.Context, dataDir string, cfg Config, log func(string)) (Result, error) {
	if !cfg.Ready() {
		return Result{}, fmt.Errorf("off-site mirror is not configured")
	}
	client, err := newClient(cfg)
	if err != nil {
		return Result{}, err
	}
	prefix := cfg.ObjectPrefix()
	bucket := cfg.Bucket
	localKeys := map[string]struct{}{}
	var res Result

	log(fmt.Sprintf("off-site mirror: bucket %s prefix %s/", bucket, prefix))

	err = filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkip(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		key := prefix + "/" + rel
		localKeys[key] = struct{}{}
		res.Files++

		remoteSize, exists, err := headObject(ctx, client, bucket, key)
		if err != nil {
			return fmt.Errorf("head %s: %w", rel, err)
		}
		if exists && remoteSize == info.Size() {
			res.Skipped++
			res.Bytes += info.Size()
			return nil
		}
		if err := uploadFile(ctx, client, bucket, key, path); err != nil {
			return fmt.Errorf("upload %s: %w", rel, err)
		}
		res.Uploaded++
		res.Bytes += info.Size()
		if res.Uploaded%25 == 0 {
			log(fmt.Sprintf("off-site: uploaded %d file(s)…", res.Uploaded))
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	remoteKeys, err := listRemoteKeys(ctx, client, bucket, prefix+"/")
	if err != nil {
		return res, fmt.Errorf("list remote: %w", err)
	}
	for _, key := range remoteKeys {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if _, ok := localKeys[key]; ok {
			continue
		}
		if err := deleteRemoteKey(ctx, client, bucket, key); err != nil {
			return res, fmt.Errorf("delete remote %s: %w", key, err)
		}
		res.Deleted++
	}

	log(fmt.Sprintf(
		"off-site mirror done: %d local file(s), %d uploaded, %d unchanged, %d removed remotely, %d bytes",
		res.Files, res.Uploaded, res.Skipped, res.Deleted, res.Bytes,
	))
	return res, nil
}

func shouldSkip(rel string) bool {
	base := filepath.Base(rel)
	if strings.HasPrefix(base, ".") {
		return true
	}
	switch base {
	case "lost+found":
		return true
	}
	return false
}

func formatSyncTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
