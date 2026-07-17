package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/agentstats"
	"github.com/boomerang-backup/boomerang/internal/metrics"
	"github.com/boomerang-backup/boomerang/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "daemon":
		runDaemon()
	case "collect":
		runCollect()
	case "ssh-export":
		runSSHExport(os.Args[2:])
	case "ssh-logs":
		runSSHLogs(os.Args[2:])
	case "ssh-log-sources":
		runSSHLogSources()
	case "ssh-forced":
		// Invoked via authorized_keys command=…; only honors SSH_ORIGINAL_COMMAND.
		runSSHForced()
	case "version", "--version", "-V":
		fmt.Println(version.Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `boomerang-monitor %s

Usage:
  boomerang-monitor daemon              Collect metrics every minute into the spool
  boomerang-monitor collect             Collect one sample (for testing)
  boomerang-monitor ssh-export [--since=RFC3339]
  boomerang-monitor ssh-logs [--lines=N] [--source=source-id]
  boomerang-monitor ssh-log-sources
  boomerang-monitor ssh-forced          Restricted SSH entrypoint (forced command)
  boomerang-monitor version
`, version.Version)
}

func spoolDir() string {
	if d := os.Getenv("BOOMERANG_MONITOR_SPOOL"); d != "" {
		return d
	}
	return agentstats.DefaultSpoolDir
}

func runCollect() {
	s, err := agentstats.Collect(version.Version)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(s)
}

func runDaemon() {
	dir := spoolDir()
	interval := time.Minute
	if v := os.Getenv("BOOMERANG_MONITOR_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 10*time.Second {
			interval = d
		}
	}
	log.Printf("boomerang-monitor %s collecting every %s into %s", version.Version, interval, dir)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	collectOnce(dir)
	for range ticker.C {
		collectOnce(dir)
	}
}

func collectOnce(dir string) {
	s, err := agentstats.Collect(version.Version)
	if err != nil {
		log.Printf("collect: %v", err)
		return
	}
	if err := agentstats.AppendSample(dir, s); err != nil {
		log.Printf("spool: %v", err)
	}
}

func runSSHForced() {
	cmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	action, err := agentstats.ParseSSHCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "boomerang-monitor: %v\n", err)
		os.Exit(1)
	}
	switch action.Kind {
	case agentstats.SSHActionExport:
		exportSince(action.Since)
	case agentstats.SSHActionLogs:
		printLogs(action.Lines, action.Source)
	case agentstats.SSHActionLogSources:
		runSSHLogSources()
	default:
		fmt.Fprintf(os.Stderr, "boomerang-monitor: forbidden SSH command\n")
		os.Exit(1)
	}
}

func runSSHExport(args []string) {
	since := time.Time{}
	for _, a := range args {
		if strings.HasPrefix(a, "--since=") {
			raw := strings.TrimPrefix(a, "--since=")
			if raw == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				t, err = time.Parse(time.RFC3339, raw)
			}
			if err != nil {
				log.Fatalf("invalid --since: %v", err)
			}
			since = t.UTC()
		}
	}
	exportSince(since)
}

func exportSince(since time.Time) {
	samples, err := agentstats.ReadSince(spoolDir(), since)
	if err != nil {
		log.Fatal(err)
	}
	batch := metrics.ExportBatch{
		SchemaVersion: metrics.SchemaVersion,
		ClientVersion: version.Version,
		Samples:       samples,
	}
	if batch.Samples == nil {
		batch.Samples = []metrics.Sample{}
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(batch); err != nil {
		log.Fatal(err)
	}
}

func runSSHLogs(args []string) {
	cmd := "boomerang-monitor ssh-logs"
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	action, err := agentstats.ParseSSHCommand(cmd)
	if err != nil {
		log.Fatal(err)
	}
	printLogs(action.Lines, action.Source)
}

func runSSHLogSources() {
	sources := agentstats.AvailableLogSources()
	if sources == nil {
		sources = []metrics.LogSource{}
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"sources": sources}); err != nil {
		log.Fatal(err)
	}
}

func printLogs(lines int, source string) {
	out, err := agentstats.ReadLogSource(lines, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "boomerang-monitor: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}
