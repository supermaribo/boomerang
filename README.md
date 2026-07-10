# 🪃 Boomerang

**Your backups come back.** Boomerang is a small, self-hosted backup appliance you run on your own Linux box — pull website files and MySQL databases on a schedule, keep versions, restore when things go wrong, and get emailed when a job fails.

One Go binary, built-in web UI, no cloud subscription. Data stays on **your** server under `/var/lib/boomerang`.

---

## 🏠 LXC / Proxmox — fastest way to try it

Perfect for a home lab: a dedicated Debian container that only does backups.

| Suggested CT | Value |
|--------------|-------|
| **Template** | Debian 12 or Ubuntu 22.04 |
| **CPU / RAM** | 1 vCPU, 512 MB–1 GB RAM |
| **Disk** | 20 GB+ (more if you keep lots of versions) |
| **Network** | Outbound SSH/FTP/MySQL to your web servers |

**On the container as root:**

```bash
apt-get update && apt-get install -y git
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
chmod +x install.sh
./install.sh
```

The installer runs a **system check** first (OS, disk, RAM, systemd, port 8080), then installs dependencies and starts the service.

Open **`http://YOUR_CT_IP:8080`** → set your admin password → add targets → run **Backup now**.

> 🔒 Keep port **8080** on your LAN only. Boomerang is HTTP + single password — not meant for the public internet without a reverse proxy.

---

## 🤔 What is this for?

Boomerang is the **backup box** — not the website server. It lives on your network (LXC, VPS, or spare machine) and **pulls** copies from your real servers:

- **File servers** — SFTP, RSYNC, or FTP/FTPS (WordPress files, configs, uploads, …)
- **Databases** — MySQL/MariaDB dumps (full or selected tables)

You get a timeline of versions, browse files inside a backup, restore selected paths back to the live server, download a zip, or roll back database tables — without paying for a hosted backup SaaS.

### Example setups

**Home lab (Proxmox)**  
- CT `192.168.1.50` runs Boomerang  
- Another CT or Pi hosts your site over SFTP + MariaDB  
- Firewall on the site: allow SSH/MySQL **only** from `192.168.1.50`

**Small business site**  
- Boomerang on a internal VM  
- Nightly SFTP backup of `/var/www/html`  
- Nightly `mysqldump` of the WordPress DB via SSH tunnel  
- Email alert if a backup fails overnight

**Manual backup cleanup**  
- You ran **Backup now** before a risky deploy  
- After you're happy, delete that version in **Explore backups** to free disk

**After a bad deploy**  
- Open **Explore backups** → pick yesterday's version → restore `wp-content/uploads` only  
- Or restore selected MySQL tables (e.g. `wp_posts`, `wp_options`) without touching the rest

---

## ✨ Features

- 📁 **File backups** — SFTP (recommended), RSYNC, FTP/FTPS; include/exclude paths; optional incremental chains
- 🗄️ **Database backups** — MySQL with direct or SSH-tunnel connection; per-table dumps for safer restores
- 🔍 **Explore & restore** — browse backup trees, search, selective restore, download zip, verify backup integrity
- 📅 **Schedules & retention** — friendly “every N hours” UI or cron; hourly/daily/weekly/monthly/yearly keep counts
- 📧 **Alerts** — local mail (postfix) or custom SMTP; toggles for backup/restore success and failure
- 🔐 **Encrypted at rest** — credentials and backup blobs encrypted with a local master key
- ☁️ **Off-site mirror** — optional automatic copy to **Cloudflare R2** after each backup (3-2-1)
- 🔄 **Restore from R2** — import a previous appliance on first install (before admin password is set)
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

`install.sh` checks these automatically before installing (see [Install script reference](#-install-script-reference)).

---

## 🚀 Quick install (Debian / Ubuntu / LXC)

On the backup appliance as **root**:

```bash
git clone https://github.com/supermaribo/boomerang.git
cd boomerang
chmod +x install.sh
./install.sh
```

What happens:

1. ✅ **System check** — OS, architecture, disk, memory, systemd, port 8080
2. 📦 Install system packages (SSH, rsync, MySQL client, postfix for local mail)
3. 🔨 Build web UI + Go binary (unless you pass `--no-build`)
4. 👤 Create `boomerang` user and `/var/lib/boomerang`
5. ⚙️ Enable and start the systemd service

### Upgrade

```bash
cd boomerang
git pull
sudo ./install.sh
```

### Pre-built binary (e.g. build on Mac, install on LXC)

```bash
# On your dev machine
cd web && npm ci && npm run build && cd ..
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/boomerang ./cmd/boomerang
scp dist/boomerang root@YOUR_SERVER:/tmp/

# On the server
cd boomerang   # or clone repo for install scripts
sudo ./install.sh --no-build /tmp/boomerang
```

---

## 🛠 Install script reference

```text
sudo ./install.sh [options] [path/to/boomerang-binary]

Options:
  --no-build     Use an existing binary (skip compile)
  --binary PATH  Same as passing PATH as the last argument
  -h, --help     Show help
```

**System check** (runs automatically before install):

- Linux on **amd64** or **arm64**
- **root** for native install
- **apt-get** + **systemd**
- At least **1 GB** free disk (warns below **20 GB**)
- Warns if RAM &lt; **512 MB** or port **8080** is busy

**Environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `BOOMERANG_DATA_DIR` | `/var/lib/boomerang` | Database, secrets, backup blobs |
| `PREFIX` | `/usr/local` | Where `boomerang` binary is installed |
| `GOOS` / `GOARCH` | `linux` / host arch | Cross-compile when building on the server |

Low-level install (binary + systemd only, includes the same system check):

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

```yaml
environment:
  BOOMERANG_LISTEN: "0.0.0.0:8080"
  BOOMERANG_DATA_DIR: "/var/lib/boomerang"
  # BOOMERANG_MASTER_KEY: "64-char-hex..."  # optional; for restores
```

---

## 👋 First-time setup

1. Open the UI and create the admin password (minimum 8 characters).
2. **Settings → Notifications** — your email, which alerts to send, send a test email.
3. Add a **file server** (SFTP/RSYNC/FTP) and/or **database** target.
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

### Master key — decrypts everything

`secrets/master.key` is a 32-byte random key. Boomerang uses it to:

- Encrypt and decrypt **backup files** (the `.enc` blobs under `backups/`)
- Encrypt **stored passwords** in `app.db` (SSH, MySQL, SMTP, R2 API keys, etc.)

Without `master.key`, your backups are ciphertext — they cannot be read or restored. The off-site R2 mirror includes this file so you can rebuild the appliance; that also means **anyone with your R2 bucket and credentials can download the full key**.

> **Never share `secrets/master.key` — anywhere, ever.**  
> Do not email it, paste it in chat, upload it to a ticket, commit it to git, or screenshot it. Treat it like the master password to your entire backup system. Keep one encrypted offline copy (password manager, safe) separate from the appliance.

**Copy this entire tree off the appliance** (rsync, snapshots, NAS, or R2 mirror). Without `master.key`, encrypted backups cannot be recovered.

### Off-site mirror (Cloudflare R2)

Boomerang can mirror `/var/lib/boomerang` to **Cloudflare R2** after each successful backup — local copy first, then off-site. Configure in **Settings → Off-site** in the UI.

R2 has a generous free tier (10 GB storage, no egress fees for typical backup use). Backup blobs are already encrypted; the mirror also includes `app.db` and `master.key` so you can rebuild the appliance from R2 alone.

#### 1. Create a bucket

1. Log in to the [Cloudflare dashboard](https://dash.cloudflare.com/).
2. Go to **Storage & databases** → **R2** → **Overview**.
3. If prompted, enable R2 for your account (billing may apply above the free tier).
4. Click **Create bucket**, choose a name (e.g. `boomerang-dr`), leave it **private**.

#### 2. Find your Account ID

On the same **R2 → Overview** page, in **Account details**, copy your **Account ID** (32-character hex string). Paste this into Boomerang’s **Cloudflare account ID** field.

#### 3. Create API credentials (Access Key ID + Secret Access Key)

These are **R2 S3 API tokens**, not your main Cloudflare login or global API key.

1. On **R2 → Overview**, under **Account details**, click **Manage** next to **R2 API tokens** (or **Manage R2 API tokens**).
2. Click **Create API token** (or **Create account API token** / **Create user API token**).
3. Set permissions:
   - **Object Read & Write**
   - **Apply to specific buckets only** → select the bucket you created
4. Click **Create API token**.
5. Cloudflare shows two values — copy both **immediately** (the secret is shown only once):
   - **Access Key ID** (sometimes labeled Client ID)
   - **Secret Access Key** (sometimes labeled Client Secret)

Paste them into **Settings → Off-site** in Boomerang. Leave the fields blank on later saves to keep the existing keys.

#### 4. Enable mirroring in Boomerang

1. Open **Settings → Off-site**.
2. Enable **off-site mirror after backups**.
3. Enter **Account ID**, **bucket name**, **Access Key ID**, and **Secret Access Key**.
4. Optional: change **Object prefix** (default `boomerang`) — folder name inside the bucket.
5. Click **Test connection**, then **Save**.
6. Use **Mirror now** for an immediate full sync, or wait for the next backup.

After each file or database backup (and when you delete a version), Boomerang syncs changed files to R2 and removes objects that no longer exist locally.

#### 5. Keep the bucket secure

| Practice | Why |
|----------|-----|
| **Private bucket** | R2 buckets are private by default — do not enable public access or a public custom domain for DR storage. |
| **Scoped API token** | Create an R2 token with **Object Read & Write** on **one bucket only** — not Account Admin or all-buckets access. |
| **Separate tokens** (optional) | Use a **read-only** token on a second appliance if you only need restore; use read/write on the live backup box. |
| **Store keys only in Boomerang** | Access keys are saved encrypted in `app.db` when you use Settings → Off-site. Do not commit them to git or paste into chat. |
| **Rotate tokens** | If a key may have leaked, delete the old R2 API token in Cloudflare and create a new one. |
| **Keep `master.key` separately** | R2 holds an encrypted copy of your data tree, but keep a second offline copy of `secrets/master.key` (password manager, safe) in case both appliance and Cloudflare account are lost. |
| **Enable versioning** (optional) | In the bucket settings, turn on **object versioning** for protection against accidental overwrites. |
| **TLS in transit** | Boomerang talks to R2 over HTTPS (`https://<ACCOUNT_ID>.r2.cloudflarestorage.com`). |

Backup **blobs** (`.enc` files) are ciphertext without `master.key`. The mirror still includes `app.db` and `master.key`, so anyone with **both** your R2 credentials and bucket access can download the full appliance — treat R2 credentials like root passwords.

#### 6. Restore on a new appliance (first flight)

On a **fresh install** before you create an admin password:

1. Open the Boomerang UI → **First flight**.
2. Choose **Restore from R2** (instead of New appliance).
3. Enter the same **Account ID**, **bucket**, **prefix**, and API keys used for mirroring.
4. Click **Test connection**, then **Restore appliance**.
5. The service restarts automatically. Sign in with your **previous** admin password, targets, and backups.

This only works on first install (before an admin password is set). To rebuild manually without the UI, download the mirrored tree from R2 (e.g. [rclone](https://rclone.org/)) into `/var/lib/boomerang`, then restart the service.

To restore on a new host (local copy or manual R2 download):

1. Install Boomerang (or Docker with the same volume data).
2. Stop the service; restore the data directory.
3. Ensure `master.key` is present (or set `BOOMERANG_MASTER_KEY`).
4. `chown -R boomerang:boomerang /var/lib/boomerang` (native install).
5. Start the service and update remote firewall rules for the new IP.

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

You may use and modify it at no cost. If you distribute or host a modified version, you must provide the corresponding source under the same license. Proprietary redistribution, relicensing, or selling copies without complying with AGPL-3.0 is not permitted.

Contributions welcome on GitHub.
