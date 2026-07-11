# Changelog

## v0.1.2

- Fix blank screen when opening `/app` directly (nested React Router routes + SPA index fallback).
- Add UI error boundary so crashes show a recovery screen instead of a blank page.

## v0.1.1

- Fix in-app updates on systemd and Proxmox installs (remove `NoNewPrivileges` from the service unit).
- Add `deploy/upgrade.sh` for appliances without a git clone (Proxmox one-liner installs).
- Add Proxmox VE LXC one-liner installer (`deploy/proxmox/`).
- Restructure README with Debian, Ubuntu, LXC, and Docker install guides.
- Clearer update UI when one-click install is unavailable.

## v0.1.0

- Skip-if-unchanged for file and database backups.
- Full global backup from the dashboard.
- In-app updates via Settings → Updates (GitHub releases).
- GitHub release binaries for linux amd64/arm64.
