package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	DataDir           string
	ListenAddr        string
	MasterKey         []byte
	DBPath            string
	MaxConcurrentJobs int
}

func Load() (*Config, error) {
	dataDir := envOr("BOOMERANG_DATA_DIR", "/var/lib/boomerang")
	listen := envOr("BOOMERANG_LISTEN", "127.0.0.1:8080")

	if err := os.MkdirAll(filepath.Join(dataDir, "secrets"), 0o700); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "backups"), 0o700); err != nil {
		return nil, fmt.Errorf("backups dir: %w", err)
	}

	key, err := loadOrCreateMasterKey(dataDir)
	if err != nil {
		return nil, err
	}

	return &Config{
		DataDir:           dataDir,
		ListenAddr:        listen,
		MasterKey:         key,
		DBPath:            filepath.Join(dataDir, "app.db"),
		MaxConcurrentJobs: envIntOr("BOOMERANG_MAX_JOBS", defaultMaxJobs()),
	}, nil
}

func defaultMaxJobs() int {
	n := runtime.NumCPU() * 2
	if n < 4 {
		return 4
	}
	if n > 16 {
		return 16
	}
	return n
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func loadOrCreateMasterKey(dataDir string) ([]byte, error) {
	if env := os.Getenv("BOOMERANG_MASTER_KEY"); env != "" {
		b, err := hex.DecodeString(env)
		if err != nil {
			return nil, fmt.Errorf("BOOMERANG_MASTER_KEY must be hex: %w", err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("BOOMERANG_MASTER_KEY must be 32 bytes (64 hex chars)")
		}
		return b, nil
	}

	path := filepath.Join(dataDir, "secrets", "master.key")
	if raw, err := os.ReadFile(path); err == nil {
		b, err := hex.DecodeString(string(bytesTrimSpace(raw)))
		if err != nil {
			return nil, fmt.Errorf("read master key: %w", err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("master key file must be 32 bytes")
		}
		return b, nil
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(b)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}
	return b, nil
}

func bytesTrimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
