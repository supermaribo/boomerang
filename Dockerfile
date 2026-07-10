# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-bookworm AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/cmd/boomerang/webdist ./cmd/boomerang/webdist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /boomerang ./cmd/boomerang

FROM debian:bookworm-slim
RUN export DEBIAN_FRONTEND=noninteractive \
  && apt-get update -qq \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    openssh-client \
    rsync \
    default-mysql-client \
  && rm -rf /var/lib/apt/lists/*

RUN useradd --system --home /var/lib/boomerang --shell /usr/sbin/nologin boomerang \
  && install -d -m 700 -o boomerang -g boomerang /var/lib/boomerang

COPY --from=gobuild /boomerang /usr/local/bin/boomerang

ENV BOOMERANG_DATA_DIR=/var/lib/boomerang
ENV BOOMERANG_LISTEN=0.0.0.0:8080

VOLUME ["/var/lib/boomerang"]
EXPOSE 8080

USER boomerang
ENTRYPOINT ["/usr/local/bin/boomerang"]
