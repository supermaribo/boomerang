# Proxmox community-scripts

Boomerang ships Proxmox VE helper scripts compatible with [community-scripts/ProxmoxVE](https://github.com/community-scripts/ProxmoxVE).

## One-liner (works today)

Run on the **Proxmox host** shell (not inside an existing CT):

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/proxmox/ct-boomerang.sh)"
```

This uses community-scripts `build.func` for the interactive LXC wizard, but pulls the Boomerang installer from this repository until the script is listed on [community-scripts.org](https://community-scripts.org).

After upstream acceptance, users will install with:

```bash
bash -c "$(curl -fsSL https://github.com/community-scripts/ProxmoxVE/raw/main/ct/boomerang.sh)"
```

## Files in this repo

| File here | Upstream path (after acceptance) |
|-----------|----------------------------------|
| `deploy/proxmox/ct-boomerang.sh` | `ct/boomerang.sh` (with curl redirect removed) |
| `deploy/proxmox/boomerang-install.sh` | `install/boomerang-install.sh` |
| `deploy/proxmox/upstream/json/boomerang.json` | `json/boomerang.json` |

**PR-ready copies** (no curl redirect hack): [`deploy/proxmox/upstream/`](upstream/).

Full submission steps: [`SUBMIT.md`](SUBMIT.md).

## Suggested LXC defaults

| Setting | Value |
|---------|-------|
| OS | Debian 12 |
| CPU | 1 |
| RAM | 512 MB–1 GB |
| Disk | 20 GB+ |
| Network | DHCP on your backup VLAN |

After creation, open `http://<ct-ip>:8080` and set your admin password.
