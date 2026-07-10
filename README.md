# 🪃 Boomerang

**Your backups come back.** Boomerang is a small, self-hosted backup appliance you run on your own Linux box — pull website files and MySQL databases on a schedule, keep versions, restore when things go wrong, and get emailed when a job fails.

One Go binary, built-in web UI, no cloud subscription. Data stays on **your** server under `/var/lib/boomerang`.

---

## 🤔 What is this for?

Boomerang is the **backup box** — not the website server. It lives on your network (LXC, VPS, or spare machine) and **pulls** copies from your real servers:

- **Websites** — SFTP, RSYNC, or FTP/FTPS (WordPress files, configs, uploads, …)
- **Databases** — MySQL/MariaDB dumps (full or selected tables)

You get a timeline of versions, browse files inside a backup, restore selected paths back to the live server, download a zip or SQL dump, or roll back database tables — without paying for a hosted backup SaaS.

### Example setups

**Home lab (Proxmox)**  
- CT `192.168.1.50` runs Boomerang  
- Another CT or Pi hosts your site over SFTP + MariaDB  
- Firewall on the site: allow SSH/MySQL **only** from `192.168.1.50`

**Small business site**  
- Boomerang on an internal VM  
- Nightly SFTP backup of `/var/www/html`  
- Nightly `mysqldump` of the WordPress DB via SSH tunnel  
- Email alert if a backup fails overnight

**After a bad deploy**  
- Open **Explore backups** → pick yesterday's version → restore `wp-content/uploads` only  
- Or restore selected MySQL tables (e.g. `wp_posts`, `wp_options`) with a pre-restore table diff

---

## ✨ Features

- 📁 **Website backups** — SFTP (recommended), RSYNC (full snapshot each run), FTP/FTPS; include/exclude paths; optional incremental chains (SFTP/FTP)
- 🗄️ **Database backups** — MySQL with direct or SSH-tunnel connection; per-table dumps for safer restores
- 🔍 **Explore & restore** — browse backup trees, search, selective restore, download zip/SQL, verify integrity (files + databases)
- 📅 **Schedules & retention** — friendly “every N hours” UI or cron; hourly/daily/weekly/monthly/yearly keep counts
- ⚡ **Bulk backup** — backup all websites or all databases in one click
- 🛑 **Job cancel** — cancel queued or in-progress backup jobs from the UI
- 📧 **Alerts** — local mail (postfix) or custom SMTP; backup/restore/off-site mirror failure toggles
- 🔐 **Encrypted at rest** — credentials and backup blobs encrypted with a local master key
- ☁️ **Off-site mirror** — optional automatic copy to **Cloudflare R2** after each backup (3-2-1); dashboard banner when sync is stale or failed
- 🔄 **Restore from R2** — import a previous appliance on first install (before admin password is set)
- 📊 **Storage forecast** — retention-aware estimate of disk use over the next 30 days
- 🗑️ **Delete versions** — remove individual backups you no longer need

---

## 📋 Requirements

| Component | Purpose |
|-----------|---------|
| **Linux** | Debian 12+, Ubuntu 22.04+, or similar (**systemd** for native install) |
| **openssh-client** | SFTP / RSYNC / SSH tunnels |
| **rsync** | RSYNC backups |
| **default-mysql-client** | `mysqldump` / `mysql` |
| **postfix** (optional) | Local email alerts |

**Disk:** Plan for the full size of everything you retain (file + DB versions).  
**RAM:** 512 MB+ is fine for small sites.

`install.sh` runs a system check before installing (OS, disk, RAM, systemd, port 8080).

---

## 🚀 Install (Debian / Ubuntu / LXC)

Suggested Proxmox CT: **Debian 12+**, 1 vCPU, 512 MB–1 GB RAM, **20 GB+** disk, outbound SSH/MySQL to your sites.

On the backup appliance as **root**:

```bash
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
chmod +x install.sh
./install.sh
```

The installer installs dependencies, builds the UI + binary (unless `--no-build`), creates the `boomerang` user and `/var/lib/boomerang`, and starts systemd.

Open **`http://YOUR_SERVER_IP:8080`** → set your admin password → add targets → run **Backup now**.

> 🔒 Keep port **8080** on your LAN only. Boomerang is HTTP + single password — not meant for the public internet without a reverse proxy.

### Upgrade

```bash
cd boomerang
git pull
sudo ./install.sh
```

### Pre-built binary (build on Mac, install on LXC)

```bash
# Dev machine
cd web && npm ci && npm run build && cd ..
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
scp dist/boomerang root@YOUR_SERVER:/tmp/

# Server (repo needed for install scripts)
cd boomerang
sudo ./install.sh --no-build /tmp/boomerang
```

### Install script options

```text
sudo ./install.sh [options] [path/to/boomerang-binary]

  --no-build     Use an existing binary (skip compile)
  --binary PATH  Same as passing PATH as the last argument
  -h, --help     Show help
```

Low-level install (binary + systemd only):

```bash
sudo bash deploy/install.sh /path/to/boomerang
```

---

## 🐳 Docker

Good for testing or if you prefer containers over systemd. **Use custom SMTP** for email in Docker (no local postfix in the image).

```bash
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
docker compose up -d --build
```

UI: **http://localhost:8080**

---

## 👋 First-time setup

1. Open the UI and enter the **setup token** from `install.sh` output or `journalctl -u boomerang` (first boot only), then create the admin password (minimum 8 characters).
2. **Settings → Notifications** — your email, which alerts to send, send a test email.
3. Add a **website** (SFTP/RSYNC/FTP) and/or **database** target.
4. On remote hosts, allow **only this appliance's IP** in the firewall (shown in the setup wizard).
5. Run **Backup now** and confirm a version appears under **Explore backups**.

### Security — internal network only

- Run on a **LAN, LXC, or VPN** — not raw on the open internet.
- UI is **HTTP on 8080**, single shared password.
- Do **not** port-forward 8080 without TLS and proper access control.
- Remote servers: allow backup ports **only from Boomerang's IP**.

---

## 🧑‍💻 Manual build (developers)

```bash
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
./dist/boomerang
```

Runs on **http://127.0.0.1:8080** with data in `./var/lib/boomerang` if you set `BOOMERANG_DATA_DIR`.

---

## ⚙️ Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BOOMERANG_DATA_DIR` | `/var/lib/boomerang` | SQLite DB, `secrets/`, `backups/` |
| `BOOMERANG_LISTEN` | `127.0.0.1:8080` | HTTP listen (`0.0.0.0:8080` on appliance; use TLS in front for remote access) |
| `BOOMERANG_MASTER_KEY` | auto-generated | 64 hex chars (32 bytes). If set, used instead of `secrets/master.key` |
| `BOOMERANG_MAX_JOBS` | `4`–`16` (CPU-based) | Max backups/restores running at once |
| `PREFIX` | `/usr/local` | Where `boomerang` binary is installed (install script) |
| `GOOS` / `GOARCH` | `linux` / host arch | Cross-compile when building on the server |

---

## 🔒 TLS and reverse proxy

Boomerang serves plain HTTP. On a dedicated appliance, `deploy/boomerang.service` uses `BOOMERANG_LISTEN=0.0.0.0:8080` on your LAN.

For HTTPS, put **Caddy** or **nginx** in front:

```text
boomerang.lan {
  reverse_proxy 127.0.0.1:8080
}
```

---

## 📟 Service management (systemd)

```bash
sudo systemctl status boomerang
sudo systemctl restart boomerang
sudo journalctl -u boomerang -f
```

Unit file: `deploy/boomerang.service`

---

## 💾 Disaster recovery

Everything important lives under **`BOOMERANG_DATA_DIR`**:

```text
/var/lib/boomerang/
  app.db                 # targets, schedules, encrypted secrets
  secrets/master.key     # required to decrypt backups & passwords
  backups/               # all file and database versions
```

### Master key

`secrets/master.key` encrypts backup blobs and stored passwords (SSH, MySQL, SMTP, R2 keys). **Without it, backups cannot be restored.**

> **Never share `secrets/master.key`.** Keep one encrypted offline copy separate from the appliance.

Copy the entire data directory off the appliance (rsync, snapshots, NAS, or R2 mirror).

### Off-site mirror (Cloudflare R2)

Boomerang can mirror `/var/lib/boomerang` to **Cloudflare R2** after each successful backup. Configure in **Settings → Off-site**.

1. Create a **private** R2 bucket in the [Cloudflare dashboard](https://dash.cloudflare.com/) → **Storage & databases** → **R2**.
2. Copy your **Account ID** from R2 → Overview.
3. Create an **R2 API token** with **Object Read & Write** scoped to that bucket only. Copy **Access Key ID** and **Secret Access Key** (secret shown once).
4. In Boomerang **Settings → Off-site**: enable mirror, enter credentials, **Test connection**, **Save**.
5. Use **Mirror now** or wait for the next backup. Failed syncs can email you (Settings → Notifications).

The mirror includes `app.db` and `master.key` so you can rebuild from R2 alone — treat R2 credentials like root passwords.

#### Restore on a new appliance (first flight)

On a **fresh install** before you create an admin password:

1. Open the UI → **First flight** → **Restore from R2**.
2. Enter the same Account ID, bucket, prefix, and API keys.
3. **Test connection** → **Restore appliance** → service restarts.
4. Sign in with your **previous** admin password.

For manual restore, download the mirrored tree (e.g. [rclone](https://rclone.org/)) into `/var/lib/boomerang`, ensure `master.key` is present, `chown -R boomerang:boomerang /var/lib/boomerang`, and restart the service.

More detail in **Settings → Recovery** in the UI.

---

## 🧱 Firewall (remote servers)

Boomerang connects **outbound** to your sites. On each **remote** server, allow:

- **SFTP/RSYNC:** TCP 22 from the Boomerang IP only
- **FTP:** TCP 21 (or FTPS 990) from the Boomerang IP only
- **MySQL (direct):** TCP 3306 from the Boomerang IP only

Do **not** open these ports to `0.0.0.0/0`. The setup wizards show this appliance's IP.

---

## 📜 License

[Boomerang](https://github.com/supermaribo/boomerang) is free and open source under the **[GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE)**.

You may use and modify it at no cost. If you distribute or host a modified version, you must provide the corresponding source under the same license.

Contributions welcome on GitHub.
