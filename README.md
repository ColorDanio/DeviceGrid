# DeviceGrid

> Modern server fleet management platform — SSH + Agent dual-mode, real-time monitoring, Docker lifecycle, batch deployment, and RKE2 orchestration.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://go.dev/)
[![Vue Version](https://img.shields.io/badge/Vue-3-42b883)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue)](LICENSE)

## Features

- **Dual Communication Mode** — SSH (agentless) or gRPC Agent (persistent reverse tunnel)
- **Real-time Dashboard** — CPU / memory / disk / network metrics with auto-refresh
- **Web Terminal** — GPU-accelerated xterm.js terminal with multi-tab, split-pane, and auto-reconnect
- **SFTP File Manager** — Integrated file browser with drag-and-drop upload
- **Docker Management** — Full container lifecycle (CRUD, logs, exec, Compose, networks, volumes)
- **Batch Deployment** — Concurrent script/package deployment with live output streaming
- **RKE2 Orchestration** — Cluster install, config management, monitoring, rolling upgrade
- **Network Diagnostics** — Streaming unlock, AI service availability, China ISP return route testing
- **Theme System** — Dark/light/system mode with 6 customizable accent colors
- **Multi-user** — JWT auth with admin/operator/viewer roles
- **Single Binary** — Frontend embedded via `go:embed`, zero external dependencies

## Quick Start

### Build from Source

```bash
# Install dependencies
make deps

# Build server + agent
make build

# Run in development
make dev-server    # Backend at :8080
make dev-web       # Frontend at :5173 (with hot reload)

# Package single binary
make package       # Embeds frontend into Go binary
```

### Run

```bash
# Start the server
./bin/devicegrid-server

# Default login: admin / admin123 (change immediately in production)
# Open http://localhost:8080
```

### Deploy Agent (Optional — for persistent connections)

```bash
# On the managed node:
./bin/devicegrid-agent -server <server-ip>:9090 -node-id <unique-id> -node-name <display-name>
```

## Configuration

All settings in `configs/config.yaml`. Override via environment variables with `DG_` prefix:

```bash
DG_SERVER_PORT=9090                    # Change listen port
DG_DATABASE_DRIVER=mongodb             # Use MongoDB instead of SQLite
DG_CRYPTO_MASTER_KEY=<32-byte-hex>     # AES-256-GCM master key for credential encryption
DG_AUTH_JWT_SECRET=<random-string>     # JWT signing secret (required in release mode)
```

### Key Configuration

| Section | Key | Default | Description |
|---|---|---|---|
| `server.mode` | `debug` / `release` | `debug` | Release mode requires explicit secrets |
| `database.driver` | `sqlite` / `mongodb` | `sqlite` | Database backend |
| `agent.grpc_port` | int | `9090` | gRPC tunnel listener for agents |
| `deploy.max_concurrent` | int | `20` | Max parallel deployment workers |

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed system design.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22+ / Gin / gorilla/websocket |
| Frontend | Vue 3 / Element Plus / Vite / xterm.js (WebGL) |
| Database | SQLite (default) / MongoDB (optional) |
| Agent | Go binary / gRPC / mTLS |
| Build | Make / go:embed / Vite |

## License

MIT
