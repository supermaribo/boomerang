# AGENTS.md

## Cursor Cloud specific instructions

Boomerang is a single self-hosted backup appliance: a **Go backend** (`cmd/boomerang`,
`internal/...`) that serves both a JSON API and a **React + Vite web UI** (`web/`) on one
HTTP port (default `8080`). The web UI is built into `cmd/boomerang/webdist/` and embedded
into the Go binary via `//go:embed`. In production there is only ONE process — the
`boomerang` binary.

Dependency install (`go mod download`, `npm ci` in `web/`) is handled by the startup update
script; the notes below are the non-obvious things that dependency install does not cover.

### Building and running

Standard commands are documented in `README.md` (see "Manual build (developers)"):

```bash
cd web && npm run build && cd ..          # builds UI into cmd/boomerang/webdist/
CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
./dist/boomerang
```

Non-obvious caveats:

- `cmd/boomerang/webdist/` is **committed to the repo**, so `go build ./cmd/boomerang`
  works without first running the Vite build. Run `npm run build` in `web/` only when you
  change frontend code (it overwrites `webdist/`). `go:embed` requires `webdist/` to exist
  and contain `index.html`.
- For local dev, always set `BOOMERANG_DATA_DIR` to a writable path in the repo, e.g.
  `export BOOMERANG_DATA_DIR=/workspace/var/lib/boomerang`. The default is
  `/var/lib/boomerang` (not writable here). This dir holds `app.db` (SQLite),
  `secrets/master.key`, and `backups/`. It is not gitignored — do not commit it.
- Default listen is `127.0.0.1:8080` (`BOOMERANG_LISTEN`). SQLite is pure-Go
  (`modernc.org/sqlite`), so `CGO_ENABLED=0` is fine and there is no separate DB service.
- **First-run setup token:** on first boot (empty data dir) the app prints a one-time setup
  token to the log and writes it to `<DATA_DIR>/secrets/setup.token`. The UI's "First flight"
  screen requires this token to create the admin password. To re-run first-flight, stop the
  app and delete the data dir. Token is consumed/cleared after setup.

### Frontend hot-reload dev server (optional)

`cd web && npm run dev` runs Vite on port `5173` and proxies `/api` to `127.0.0.1:8080`, so
the Go binary must also be running for API calls to work.

### Lint / test

- Go: `go vet ./...` (clean) and `go test ./...` (all packages pass). `gofmt -l .` currently
  reports several pre-existing unformatted files — this is the repo's existing state, not
  something to "fix" as part of unrelated work.
- The web UI has **no** `lint` or `test` npm scripts defined.
- CI (`.github/workflows/release.yml`) only builds release binaries on `v*` tags; it does not
  run tests or linters.

### Running real backups end-to-end

Actually running a backup requires a reachable **SFTP/SSH (22)**, **FTP (21)**, or
**MySQL/MariaDB (3306)** target, plus the `openssh-client`, `rsync`, and `mysql`/`mysqldump`
CLIs installed on the host (invoked as subprocesses). None of these are present by default in
the cloud VM, and the "Add website/database" wizard requires a successful connection to the
target before a target can be saved. Setup + login + dashboard + the target wizard can all be
exercised without a target, but a live backup/restore needs one of the above services.
