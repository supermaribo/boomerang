package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/boomerang-backup/boomerang/internal/api"
	"github.com/boomerang-backup/boomerang/internal/config"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/jobs"
	"github.com/boomerang-backup/boomerang/internal/store"
)

//go:embed all:webdist
var webEmbed embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	if n, err := st.CleanupOrphans(cfg.DataDir); err != nil {
		log.Printf("orphan cleanup: %v", err)
	} else if n > 0 {
		log.Printf("cleaned up %d orphaned backup target(s)", n)
	}

	box, err := crypto.NewBox(cfg.MasterKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	runner := jobs.NewRunner(st, box, cfg.DataDir)
	sched := jobs.NewScheduler(runner, st)

	var webFS fs.FS
	if sub, err := fs.Sub(webEmbed, "webdist"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			webFS = sub
		}
	}

	srv := api.New(cfg, st, box, webFS, runner)
	srv.SetScheduler(sched)
	runner.SetNotifier(srv.LoadMail, func(targetType, targetID string) string {
		switch targetType {
		case "file":
			if f, _ := st.GetFileServer(targetID); f != nil {
				return f.Name
			}
		case "db":
			if d, _ := st.GetDatabase(targetID); d != nil {
				return d.Name
			}
		}
		return targetID
	})

	sched.Start()
	defer sched.Stop()

	log.Printf("Boomerang listening on http://%s (data: %s)", cfg.ListenAddr, cfg.DataDir)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Handler()); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
