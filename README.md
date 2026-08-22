# VyNode Media

VyNode Media is a clean-room, local-first personal media server and client ecosystem.

This repository currently contains the Phase 0 foundation: a headless Go service,
a versioned HTTP API, an OpenAPI contract, and a small React administration shell
that reports real server health. It is not yet a media server suitable for daily use.

## Quick start

Requirements: Go 1.24+, Node.js 22+, and npm 10+.

```sh
cp .env.example .env
go run ./server/cmd/vynode-server
```

In a second terminal:

```sh
npm install
npm run dev --workspace @vynode/web
```

The server listens on `http://127.0.0.1:8096` by default and stores its embedded
database in `./data`. The web development
server proxies `/api` to it.

## Verification

```sh
go test ./...
npm test --workspace @vynode/web
npm run build --workspace @vynode/web
```

See [docs/architecture/0001-foundation.md](docs/architecture/0001-foundation.md)
and [docs/limitations.md](docs/limitations.md) for scope and decisions.

Docker and platform instructions live under `deploy/`. The canonical architecture,
roadmap, and support truth are in `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, and
`docs/PLATFORM_SUPPORT.md`.
