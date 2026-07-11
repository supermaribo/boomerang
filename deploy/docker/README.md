# Docker Hub publishing

Release tags automatically build and push multi-arch images to Docker Hub when GitHub secrets are configured.

## One-time setup

1. Create a [Docker Hub](https://hub.docker.com/) account (e.g. `supermaribo`).

2. Create an access token: **Account Settings → Security → New Access Token** (read/write).

3. Add GitHub repository secrets (**Settings → Secrets and variables → Actions**):

   | Secret | Value |
   |--------|--------|
   | `DOCKERHUB_USERNAME` | Your Docker Hub username |
   | `DOCKERHUB_TOKEN` | The access token (not your password) |

4. Create the repository `boomerang` on Docker Hub (public), or the first push will create it if your token allows.

## What happens on tag push

Pushing `v0.1.x` runs `.github/workflows/release.yml` job **docker**, which publishes:

- `supermaribo/boomerang:latest`
- `supermaribo/boomerang:0.1.x` (without `v` prefix)
- `supermaribo/boomerang:v0.1.x`

Platforms: **linux/amd64**, **linux/arm64**.

## Run the published image

```bash
docker pull supermaribo/boomerang:latest
docker run -d \
  --name boomerang \
  -p 8080:8080 \
  -v boomerang-data:/var/lib/boomerang \
  --restart unless-stopped \
  supermaribo/boomerang:latest
```

Or use `docker-compose.hub.yml` from the repo root.

**Note:** In-app updates (Settings → Updates) are for systemd appliances only. Upgrade Docker by pulling a new tag and recreating the container.

## Local build (development)

```bash
BOOMERANG_VERSION=dev docker compose up -d --build
```
