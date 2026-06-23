# DeviceGrid

> Modern server fleet management platform — SSH + Agent dual-mode, real-time monitoring, Docker lifecycle, batch deployment, RKE2 orchestration, and network diagnostics.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://go.dev/)
[![Vue Version](https://img.shields.io/badge/Vue-3-42b883)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue)](LICENSE)

## Features

### Core
- **Dual Communication Mode** — SSH (agentless) or gRPC Agent (persistent reverse tunnel with mTLS)
- **Real-time Dashboard** — CPU model/cores, memory, disk, network traffic, GPU, virtualization type with auto-refresh
- **Web Terminal** — GPU-accelerated xterm.js (WebGL renderer, Ghostty theme), multi-tab, split-pane, auto-reconnect
- **SFTP File Manager** — Integrated file browser with drag-and-drop upload, embedded in terminal page
- **Multi-user** — JWT auth with admin/operator/viewer roles, CSV import/export
- **Single Binary** — Frontend embedded via `go:embed`, zero external dependencies

### Docker Management
- **Full Container Lifecycle** — Create, start, stop, restart, remove, pause
- **Container Logs** — Real-time WebSocket streaming (`docker logs -f`)
- **Container Terminal** — `docker exec -it` via WebSocket PTY
- **Container Stats** — Per-container CPU/memory monitoring
- **Image Management** — Pull, list, remove images
- **Docker Compose** — YAML editor with deploy/teardown
- **Networks & Volumes** — Full management

### Deployment
- **Batch Script Deployment** — Concurrent execution across multiple nodes with live output
- **Package Installation** — apt/yum auto-detection
- **Scheduled Tasks (Cron)** — Interval-based recurring script execution
- **Node CSV Import** — Bulk node creation with template download

### RKE2 Kubernetes
- **Cluster Creation Wizard** — Pre-flight checks (CPU/RAM/Disk/Swap/modules/ports) with auto-fix
- **Auto China Mirror** — Detects CN nodes, injects Aliyun registry automatically
- **Behind Proxy Support** — Three-layer proxy injection (env + systemd + config.yaml)
- **Helm Management** — Install/uninstall/list Helm charts
- **Rancher UI** — One-click Helm install with auto mirror detection
- **Pod Monitoring** — `kubectl get pods` integration
- **Rolling Upgrades** — Drain → upgrade → uncordon per node

### Network Diagnostics
- **Streaming Unlock** — Netflix, Disney+, YouTube, TikTok, HBO Max, Amazon Prime, Apple TV+, Crunchyroll, TVB, AbemaTV, DAZN, Peacock, Discovery+, Canal+, Hotstar, Viu, Now TV, Bilibili, 爱奇艺国际版, Spotify, KKBOX (22+ services)
- **AI Service Availability** — ChatGPT, Claude, Gemini, Grok, Copilot, DeepSeek, Midjourney, Suno AI, Cohere, Together AI, Perplexity, Stability AI, OpenRouter, Groq, Mistral AI, Poe, GitHub Copilot, Hugging Face, Runway, You.com (20+ services)
- **Global Connectivity Test** — Latency/loss to China/HK/Japan/Korea/Singapore/US/Europe
- **China ISP Return Route** — Telecom/Unicom/Mobile traceroute with AS-number line type detection (CN2 GIA/GT, 4837/9929, CMI, etc.)

### Automation
- **Alert System** — Threshold-based rules (CPU/mem/disk/node_offline) with Webhook notifications (Slack/DingTalk/WeChat)
- **Scheduled Tasks** — Cron-style interval execution with node targeting
- **Audit Logging** — All POST/PUT/DELETE operations recorded with user/IP/duration

### Security
- **AES-256-GCM** credential encryption (SSH passwords, private keys)
- **API Rate Limiting** — 10 req/min for login, 200 req/min for authenticated API
- **JWT Authentication** — Configurable expiry, bcrypt password hashing
- **Agent mTLS** — CA certificate verification (--ca-cert flag)

### UI/UX
- **Theme System** — Dark/light/system mode with 6 accent colors
- **Responsive Design** — Split-pane layouts, modern card design
- **Version Tracking** — `YYYY.M.D_commitSHA` format in footer

## Quick Start

### Option 1: Download Release

```bash
# Download from GitHub Releases
# https://github.com/ColorDanio/DeviceGrid/releases

# Ubuntu/Debian
sudo dpkg -i devicegrid-*.deb
sudo systemctl enable --now devicegrid

# Or standalone binary
chmod +x devicegrid-server-amd64
./devicegrid-server-amd64
```

Open `http://<server-ip>:8080` — Login: `admin` / `admin123`

### Option 2: Build from Source

```bash
git clone https://github.com/ColorDanio/DeviceGrid.git
cd DeviceGrid

# Build everything (server + agent + frontend)
make build

# Or full package with embedded frontend
make package

# Run
./bin/devicegrid-server
```

### Option 3: Desktop App (Electron)

```bash
make build-server
cd electron && npm install
npx electron . --no-sandbox
```

### Option 4: Development Mode

```bash
make dev-server    # Backend at :8080 (Go)
make dev-web       # Frontend at :5173 (Vite hot reload)
```

## Deploy Agent (Persistent Connection)

After establishing SSH trust on a node, deploy the Agent for zero-SSH-overhead operations:

```bash
# Automatic deployment (from UI: click "Agent" button on trusted node)
# The server uploads the agent binary and registers a systemd service

# Or manual deployment:
./devicegrid-agent -server <server-ip>:9090 -node-id <unique-id> -node-name <display-name>

# With mTLS:
./devicegrid-agent -server <server-ip>:9090 -ca-cert /path/to/ca.crt -node-id <id>

# Resource limits (auto-configured by systemd):
# MemoryMax=64M, CPUQuota=5%, Nice=10
```

## Configuration

All settings in `configs/config.yaml`. Override via environment variables with `DG_` prefix:

```bash
DG_SERVER_PORT=9090
DG_DATABASE_DRIVER=mongodb
DG_CRYPTO_MASTER_KEY=<32-byte-hex>
DG_AUTH_JWT_SECRET=<random-string>
DG_NETWORK_ENVIRONMENT=internal  # Disable public-network features
```

### Key Configuration

| Section | Key | Default | Description |
|---|---|---|---|
| `server.mode` | `debug` / `release` | `debug` | Release mode requires explicit secrets |
| `network.environment` | `public` / `internal` | `public` | Internal mode disables geo/streaming/AI/route checks |
| `database.driver` | `sqlite` / `mongodb` | `sqlite` | Database backend |
| `agent.grpc_port` | int | `9090` | gRPC tunnel listener for agents |
| `deploy.max_concurrent` | int | `20` | Max parallel deployment workers |
| `ssh.max_connections` | int | `50` | SSH connection pool size per node |

### Network Environment Modes

| Mode | Geo Lookup | Streaming Check | AI Check | Connectivity | Return Route |
|---|---|---|---|---|---|
| `public` (default) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `internal` | ❌ | ❌ | ❌ | ❌ | ❌ |

Individual features can be overridden:
```yaml
network:
  environment: "public"
  enable_route: false  # Disable only return route test
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed system design.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22+ / Gin / gorilla/websocket / gRPC |
| Frontend | Vue 3 / Element Plus / Vite / xterm.js (WebGL) / Pinia |
| Database | SQLite (default) / MongoDB (optional) |
| Agent | Go binary / gRPC / mTLS / systemd |
| Desktop | Electron (all-in-one, no external dependencies) |
| CI/CD | GitHub Actions (test, build, release, arm64 cross-compile) |
| Build | Make / go:embed / Vite / electron-builder |

## Documentation

- [SPEC.md](SPEC.md) — Technical specification
- [PLAN.md](PLAN.md) — Implementation roadmap
- [ARCHITECTURE.md](ARCHITECTURE.md) — System architecture
- [AGENTS.md](AGENTS.md) — AI agent coding guidelines

## License

MIT
