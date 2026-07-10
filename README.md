# Boomerang

Self-hosted backup appliance for websites and databases. Pull files over **SFTP**, **RSYNC**, or **FTP**; dump **MySQL** over direct TCP or SSH tunnel; browse versions, restore selectively, and get email alerts when jobs fail or succeed.

Single Go binary + embedded web UI. Data stays on your server under `/var/lib/boomerang` by default.

---

## Requirements

| Component | Purpose |
|-----------|---------|
| **Linux** | Debian 12+, Ubuntu 22.04+, or similar (systemd for native install) |
| **openssh-client** | SFTP / RSYNC / SSH tunnels |
| **rsync** | RSYNC backups |
| **default-mysql-client** | `mysqldump` / `mysql` for database backup & restore |
| **postfix** (optional) | Local email alerts (Settings → Local mail) |

**Disk:** Plan for the full size of retained file + database backups. **RAM:** 512 MB+ is fine for small sites.

---

## Quick install (Debian / Ubuntu / LXC)

On the backup appliance as **root**:

```bash
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
chmod +x install.sh
./install.sh
```

The installer will:

1. Install system dependencies (including postfix for local mail)
2. Build the web UI and Go binary
3. Create the `boomerang` user and `/var/lib/boomerang`
4. Enable the systemd service

Open **http://YOUR_SERVER_IP:8080** and set the admin password on first visit.

### Upgrade

```bash
cd boomerang
git pull
sudo ./install.sh
```

Or copy a pre-built binary only:

```bash
# On your dev machine (example: Mac → Linux amd64)
cd web && npm ci && npm run build && cd ..
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
scp dist/boomerang root@YOUR_SERVER:/tmp/

# On the server
sudo ./install.sh --no-build /tmp/boomerang
```

---

## Install script reference

```text
sudo ./install.sh [options] [binary]

Options:
  --no-build     Use an existing binary (skip compile)
  --binary PATH  Same as passing PATH as the last argument
  -h, --help     Show help
```

**Environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `BOOMERANG_DATA_DIR` | `/var/lib/boomerang` | Database, secrets, backup blobs |
| `PREFIX` | `/usr/local` | Where `boomerang` binary is installed |
| `GOOS` / `GOARCH` | `linux` / host arch | Cross-compile when building on the server |

Low-level install (binary + systemd only):

```bash
sudo bash deploy/install.sh /path/to/boomerang
```

---

## Docker

Docker is suitable for testing or if you prefer containers over systemd. **Use custom SMTP** for email in Docker (no local postfix in the image).

```bash
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
docker compose up -d --build
```

UI: **http://localhost:8080**

Data is stored in the `boomerang-data` volume. Back it up regularly (see [Disaster recovery](#disaster-recovery)).

### Docker environment

```yaml
environment:
  BOOMERANG_LISTEN: "0.0.0.0:8080"
  BOOMERANG_DATA_DIR: "/var/lib/boomerang"
  # BOOMERANG_MASTER_KEY: "64-char-hex..."  # optional; for restores
```

---

## Manual build (developers)

```bash
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
./dist/boomerang
```

Runs on **http://127.0.0.1:8080** with data in `./var/lib/boomerang` if you set `BOOMERANG_DATA_DIR`.

---

## First-time setup

1. Open the UI and create the admin password (minimum 8 characters).
2. **Settings → Notifications** — enter your email, choose which alerts to send, send a test email.
3. Add a **file server** (SFTP/RSYNC/FTP) and/or **database** target.
4. On remote hosts, allow **only this appliance’s IP** through the firewall (LAN and public IP shown in the setup wizard).
5. Run **Backup now** and confirm a version appears.

### Security — internal network only

Boomerang is designed as a **private backup appliance**, not a public web app.

- Run it on a **LAN, LXC, or VPN** — not directly on the open internet.
- The UI listens on **HTTP port 8080** with **no TLS** and a **single shared password**.
- Do **not** port-forward 8080 on your router or expose it via a public IP without a reverse proxy and proper auth.
- Remote servers should allow backup traffic **only from Boomerang’s IP**, never from `0.0.0.0/0`.

---

## Features

- **File backups:** SFTP, RSYNC, FTP/FTPS — dotfiles, exclude globs, incremental chains, encrypted blobs at rest
- **Database backups:** MySQL with optional SSH tunnel; pick tables to include; per-table restore
- **Explore & restore:** Browse backup trees, search, selective restore, download zip
- **Retention:** GFS-style hourly / daily / weekly / yearly counts per target
- **Schedules:** Friendly “every N hours” UI or raw cron
- **Alerts:** Local mail (postfix) or custom SMTP; backup/restore success & failure toggles
- **Security:** Encrypted credentials and backup files (master key); single-user login

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BOOMERANG_DATA_DIR` | `/var/lib/boomerang` | SQLite DB, `secrets/`, `backups/` |
| `BOOMERANG_LISTEN` | `127.0.0.1:8080` | HTTP listen address (use `0.0.0.0:8080` on a dedicated appliance; put TLS in front for remote access) |
| `BOOMERANG_MASTER_KEY` | auto-generated | 64 hex chars (32 bytes). If set, used instead of `secrets/master.key` |
| `BOOMERANG_MAX_JOBS` | `4`–`16` (CPU-based) | Max backups/restores running at once across different targets |

---

## TLS and reverse proxy

Boomerang serves plain HTTP. For a dedicated appliance, `deploy/boomerang.service` sets `BOOMERANG_LISTEN=0.0.0.0:8080` on your LAN only.

For HTTPS, put **nginx**, **Caddy**, or another reverse proxy in front and terminate TLS there. Example Caddy:

```text
boomerang.lan {
  reverse_proxy 127.0.0.1:8080
}
```

Do not expose the UI directly to the internet without TLS and network restrictions.

---

## Service management (systemd)

```bash
sudo systemctl status boomerang
sudo systemctl restart boomerang
sudo journalctl -u boomerang -f
```

Unit file: `deploy/boomerang.service`

---

## Disaster recovery

Everything important lives under **`BOOMERANG_DATA_DIR`**:

```text
/var/lib/boomerang/
  app.db                 # targets, schedules, encrypted secrets
  secrets/master.key     # required to decrypt backups & passwords
  backups/               # all file and database versions
```

**Copy this entire tree off the appliance** (rsync, snapshots, NAS). Without `master.key`, encrypted backups cannot be recovered.

To restore on a new host:

1. Install Boomerang (or Docker with the same volume data).
2. Stop the service; restore the data directory.
3. Ensure `master.key` is present (or set `BOOMERANG_MASTER_KEY`).
4. `chown -R boomerang:boomerang /var/lib/boomerang` (native install).
5. Start the service and update remote firewall rules for the new IP.

Details are also in **Settings → Recovery** in the UI.

---

## LXC (Proxmox) notes

- Debian or Ubuntu template, 1 CPU, 512 MB–1 GB RAM, 20 GB+ disk (more for large backups).
- Install on the CT as root with `./install.sh`.
- Allow outbound TCP (SSH, FTP, MySQL) to your web servers.
- Access UI via the CT IP, e.g. `http://192.168.x.x:8080`.

---

## Firewall (remote servers)

Boomerang connects **outbound** to your sites. On each **remote** server, allow:

- **SFTP/RSYNC:** TCP 22 from the Boomerang IP only  
- **FTP:** TCP 21 (or FTPS 990) from the Boomerang IP only  
- **MySQL (direct):** TCP 3306 from the Boomerang IP only  

Do **not** open these ports to `0.0.0.0/0`. The setup wizards show this appliance’s IP.

---

## License

[Boomerang](https://github.com/supermaribo/boomerang) is free and open source under the **[GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE)**.

You may use and modify it at no cost. If you distribute or host a modified version, you must provide the corresponding source under the same license. Proprietary redistribution, relicensing, or selling copies without complying with AGPL-3.0 is not permitted.

The full license text is in [`LICENSE`](LICENSE) and may be refined on GitHub. Contributions welcome.
