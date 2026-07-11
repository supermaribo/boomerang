# Changelog

## v0.1.6

- Buffer file ZIP and database SQL downloads before sending headers (no more corrupt downloads on error).
- Pin upgrade.sh deploy scripts to the release tag; verify SHA256SUMS when available.
- Narrow sudoers for in-app updates to `${DATA_DIR}/.update/*`; improve `boomerang-update --check`.
- Rate-limit password changes; clear sessions and require re-login after password change.
- Trust `X-Forwarded-For` only when `BOOMERANG_TRUST_PROXY=1`; origin check on mutating API requests.
- Setup token in fallback HTML; constant-time token compare; stop logging token value to journal.
- Secure session cookie when HTTPS is detected; runner nil guards on DB preview paths.

## v0.1.5

- Harden UI against null JSON arrays from the API (backup version lists, target health, job lists).
- Fix boot flow when `/api/status` fails — show connection error instead of setup screen.
- Redirect to login on expired session (401 handling).
- Fix job API timestamps (`startedAt`/`finishedAt` no longer serialize as `{String, Valid}`).
- Add runner nil guards on database backup and restore handlers.
- Dashboard loads partial data when one endpoint fails; global backup button respects disabled targets.

## v0.1.4

- Fix dashboard crash on fresh install (`Cannot read properties of null (reading 'some')`) when no backup targets exist yet.

## v0.1.3

- Settings → Updates now recommends `deploy/upgrade.sh` instead of `install.sh --no-build` (fixes misleading copy on Proxmox installs).

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
