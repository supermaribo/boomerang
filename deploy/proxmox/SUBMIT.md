# Submit Boomerang to community-scripts.org

New scripts must go to **[ProxmoxVED](https://github.com/community-scripts/ProxmoxVED)** first (not ProxmoxVE). After review, maintainers promote them to [ProxmoxVE](https://github.com/community-scripts/ProxmoxVE) and list them on [community-scripts.org](https://community-scripts.org).

## Files to copy

Copy from this repository into your ProxmoxVED fork:

| This repo | ProxmoxVED path |
|-----------|-----------------|
| `deploy/proxmox/upstream/ct/boomerang.sh` | `ct/boomerang.sh` |
| `deploy/proxmox/upstream/install/boomerang-install.sh` | `install/boomerang-install.sh` |
| `deploy/proxmox/upstream/json/boomerang.json` | `json/boomerang.json` |

These upstream files **do not** include the temporary `curl` redirect hack used in `deploy/proxmox/ct-boomerang.sh`.

## Steps

1. Read [ProxmoxVE CONTRIBUTING](https://github.com/community-scripts/ProxmoxVE/blob/main/CONTRIBUTING.md) and [contribution docs](https://community-scripts.org/docs/contribution).

2. Fork and clone ProxmoxVED:
   ```bash
   git clone https://github.com/YOUR_USERNAME/ProxmoxVED.git
   cd ProxmoxVED
   git switch -c feat/boomerang
   ```

3. Copy the three files (paths in table above).

4. Optional: refine metadata at [community-scripts JSON generator](https://community-scripts.org/json-generator) (category **Backup & Recovery**, port **8080**).

5. Add a simple icon at `docs/icon.webp` in the Boomerang repo (or update `logo` in `json/boomerang.json` before copying).

6. Test on a **real Proxmox host**:
   ```bash
   dev_mode="keep,logs" bash -c "$(curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/ProxmoxVED/feat/boomerang/ct/boomerang.sh)"
   ```
   Open `http://<ct-ip>:8080`, set admin password, add a test target.

7. Open a PR against **community-scripts/ProxmoxVED** `main` with title: `Add Boomerang backup appliance`.

8. After merge to ProxmoxVE, users install with:
   ```bash
   bash -c "$(curl -fsSL https://github.com/community-scripts/ProxmoxVE/raw/main/ct/boomerang.sh)"
   ```

## Until the PR is merged

The one-liner in the main README still works (installer from this repo):

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/proxmox/ct-boomerang.sh)"
```

## Optional: script request

Open a [discussion](https://github.com/community-scripts/ProxmoxVE/discussions) or ask in [Discord](https://discord.gg/3AnUqsXnmK) before submitting the PR.
