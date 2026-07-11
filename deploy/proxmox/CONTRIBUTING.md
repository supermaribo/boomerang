# Proxmox community-scripts

Boomerang ships Proxmox VE helper scripts compatible with [community-scripts/ProxmoxVE](https://github.com/community-scripts/ProxmoxVE).

## One-liner (works today)

Run on the **Proxmox host** shell (not inside an existing CT):

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/proxmox/ct-boomerang.sh)"
```

This uses community-scripts `build.func` for the interactive LXC wizard, but pulls the Boomerang installer from this repository until the script is listed on [community-scripts.org](https://community-scripts.org).

## Files in this repo

| File here | Upstream path (after acceptance) |
|-----------|----------------------------------|
| `deploy/proxmox/ct-boomerang.sh` | `ct/boomerang.sh` |
| `deploy/proxmox/boomerang-install.sh` | `install/boomerang-install.sh` |

## Adding to community-scripts.org

New scripts are **not** merged directly into `ProxmoxVE`. Follow the official workflow:

1. Read [CONTRIBUTING.md](https://github.com/community-scripts/ProxmoxVE/blob/main/CONTRIBUTING.md) and [contribution docs](https://community-scripts.org/docs/contribution).
2. Fork [ProxmoxVED](https://github.com/community-scripts/ProxmoxVED) (the testing repo).
3. Copy `ct-boomerang.sh` → `ct/boomerang.sh` and `boomerang-install.sh` → `install/boomerang-install.sh`.
4. Remove the `curl` redirect hack from `ct/boomerang.sh` (upstream `build.func` will fetch `install/boomerang-install.sh` automatically).
5. Test on a real Proxmox node; open a PR against **ProxmoxVED**.
6. After maintainer review, the script is promoted to `ProxmoxVE` and listed on the website.

You can also [request the script](https://github.com/community-scripts/ProxmoxVE/discussions) or ask in their [Discord](https://discord.gg/3AnUqsXnmK).

## Suggested LXC defaults

| Setting | Value |
|---------|-------|
| OS | Debian 12 |
| CPU | 1 |
| RAM | 512 MB–1 GB |
| Disk | 20 GB+ |
| Network | DHCP on your backup VLAN |

After creation, open `http://<ct-ip>:8080` and enter the setup token from the container MOTD.
