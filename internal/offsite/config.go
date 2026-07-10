package offsite

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/store"
)

// Config holds Cloudflare R2 (S3-compatible) mirror settings.
type Config struct {
	Enabled   bool
	AccountID string
	Bucket    string
	Prefix    string
	AccessKey string
	SecretKey string
}

type Status struct {
	LastSync  string
	LastError string
	LastFiles int
	LastBytes int64
	Syncing   bool
}

func LoadConfig(st *store.Store, box *crypto.Box) (Config, error) {
	get := func(k string) string {
		v, ok, _ := st.GetMeta(k)
		if !ok {
			return ""
		}
		return v
	}
	boolMeta := func(k string) bool {
		v := get(k)
		return v == "1" || strings.EqualFold(v, "true")
	}
	cfg := Config{
		Enabled:   boolMeta("offsite_enabled"),
		AccountID: get("offsite_account_id"),
		Bucket:    get("offsite_bucket"),
		Prefix:    get("offsite_prefix"),
	}
	if hexEnc, ok, _ := st.GetMeta("offsite_access_key_sealed"); ok && hexEnc != "" {
		if raw, err := hex.DecodeString(hexEnc); err == nil {
			if plain, err := box.Open(raw); err == nil {
				cfg.AccessKey = string(plain)
			}
		}
	}
	if hexEnc, ok, _ := st.GetMeta("offsite_secret_key_sealed"); ok && hexEnc != "" {
		if raw, err := hex.DecodeString(hexEnc); err == nil {
			if plain, err := box.Open(raw); err == nil {
				cfg.SecretKey = string(plain)
			}
		}
	}
	return cfg, nil
}

func LoadStatus(st *store.Store) Status {
	get := func(k string) string {
		v, ok, _ := st.GetMeta(k)
		if !ok {
			return ""
		}
		return v
	}
	var files int
	fmt.Sscanf(get("offsite_last_files"), "%d", &files)
	var bytes int64
	fmt.Sscanf(get("offsite_last_bytes"), "%d", &bytes)
	return Status{
		LastSync:  get("offsite_last_sync"),
		LastError: get("offsite_last_error"),
		LastFiles: files,
		LastBytes: bytes,
		Syncing:   get("offsite_sync_running") == "1",
	}
}

func SaveConfig(st *store.Store, box *crypto.Box, cfg Config, accessKey, secretKey string) error {
	if err := st.SetMeta("offsite_enabled", boolStr(cfg.Enabled)); err != nil {
		return err
	}
	if err := st.SetMeta("offsite_account_id", strings.TrimSpace(cfg.AccountID)); err != nil {
		return err
	}
	if err := st.SetMeta("offsite_bucket", strings.TrimSpace(cfg.Bucket)); err != nil {
		return err
	}
	prefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if prefix == "" {
		prefix = "boomerang"
	}
	if err := st.SetMeta("offsite_prefix", prefix); err != nil {
		return err
	}
	if accessKey != "" {
		sealed, err := box.Seal([]byte(accessKey))
		if err != nil {
			return err
		}
		if err := st.SetMeta("offsite_access_key_sealed", hex.EncodeToString(sealed)); err != nil {
			return err
		}
	}
	if secretKey != "" {
		sealed, err := box.Seal([]byte(secretKey))
		if err != nil {
			return err
		}
		if err := st.SetMeta("offsite_secret_key_sealed", hex.EncodeToString(sealed)); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) Ready() bool {
	return c.Enabled && c.AccountID != "" && c.Bucket != "" && c.AccessKey != "" && c.SecretKey != ""
}

func (c Config) ObjectPrefix() string {
	p := strings.Trim(strings.TrimSpace(c.Prefix), "/")
	if p == "" {
		return "boomerang"
	}
	return p
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
