## ✍️ Description

Adds a new LXC script for [Boomerang](https://github.com/supermaribo/boomerang), a single-binary backup appliance for website files (SFTP, rsync, FTP) and MySQL databases.

The CT script installs the published GitHub release binary, sets up postfix for local mail, and runs Boomerang under systemd on port 8080. Updates use `fetch_and_deploy_gh_release` (no git pull).

## 🔗 Related PR / Issue

Link: #

## ✅ Prerequisites

- [x] **Self-review completed** – Code follows project standards.
- [ ] **Tested thoroughly** – Changes work as expected.
- [x] **No breaking changes** – Existing functionality remains intact.
- [x] **No security risks** – No hardcoded secrets, unnecessary privilege escalations, or permission issues.

---

## 🏗️ arm64 Support

- [ ] **arm64 supported** - Tested and supported on arm64.
- [x] **arm64 not tested** - Assumed to work on arm64, but testing has not been done.
- [ ] **arm64 not supported** - Confirmed upstream dependencies or binaries do not support arm64.

---

## 🛠️ Type of Change

- [ ] 🐞 **Bug fix** – Resolves an issue without breaking functionality.
- [ ] ✨ **New feature** – Adds new, non-breaking functionality.
- [ ] 💥 **Breaking change** – Alters existing functionality in a way that may require updates.
- [x] 🆕 **New script** – A fully functional and tested script or script set.
- [ ] 🌍 **Website update** – Changes to website-related JSON files or metadata.
- [ ] 🔧 **Refactoring / Code Cleanup** – Improves readability or maintainability without changing functionality.
- [ ] 📝 **Documentation update** – Changes to `README`, `AppName.md`, `CONTRIBUTING.md`, or other docs.

---

## 🔍 Code & Security Review

- [x] **Follows `CODE-AUDIT.md` & `CONTRIBUTING.md` guidelines**
- [x] **Uses correct script structure (`AppName.sh`, `AppName-install.sh`, `AppName.json`)**
- [x] **No hardcoded credentials**
- [x] **No Docker / Docker Compose** – The application is installed bare-metal; Docker is not used.
- [x] **No git pull** – Updates use `fetch_and_deploy_gh_release`, `fetch_and_deploy_codeberg_release`, `fetch_and_deploy_gl_release`, or `fetch_and_deploy_from_url` instead of `git pull`.

---

## 🤖 AI Assistance

- [ ] **No AI used** – Scripts were written without AI assistance.
- [x] **AI was used** – I confirm the scripts were built using [`AGENTS.md`](https://github.com/community-scripts/ProxmoxVED/blob/main/AGENTS.md) and [`.github/agents/pve-script-creator.agent.md`](https://github.com/community-scripts/ProxmoxVED/blob/main/.github/agents/pve-script-creator.agent.md) as guidance, and the output has been reviewed and corrected to match those guidelines.

---

## 📋 Additional Information (optional)

- Pattern follows existing single-binary scripts (e.g. `leafwiki`, `solidinvoice`).
- `boomerang-update` and the systemd unit are embedded in the install script (no extra curl to raw GitHub during install except the release binary via `fetch_and_deploy_gh_release`).
- Sudoers entry is limited to `/usr/local/sbin/boomerang-update /var/lib/boomerang/.update/*` for in-app binary updates.
- Logo omitted from JSON for now; happy to add a selfhst icon if maintainers want one.

---

## 📦 Application Requirements (for new scripts)

- [ ] The application is **at least 6 months old**
- [x] The application is **actively maintained**
- [ ] The application has **600+ GitHub stars**
- [x] Official **release tarballs** are published
- [x] I understand that not all scripts will be accepted due to various reasons and criteria by the community-scripts ORG

## 🌐 Source

- https://github.com/supermaribo/boomerang
- https://github.com/supermaribo/boomerang/releases
