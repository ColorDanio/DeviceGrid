# Architecture — DeviceGrid

## System Overview

```
                          ┌──────────────────────────────────────────┐
                          │           DeviceGrid Server               │
                          │                                          │
                          │  ┌─────────┐ ┌───────┐ ┌─────────────┐  │
                          │  │ Gin API │ │ WS Hub│ │ gRPC Tunnel │  │
                          │  └────┬────┘ └───┬───┘ └──────┬──────┘  │
                          │       │          │             │         │
                          │  ┌────▼──────────▼─────────────▼──────┐ │
                          │  │       Transport Manager             │ │
                          │  │  SSH Pool | Agent Tunnel | Fallback │ │
                          │  └────────────────┬───────────────────┘ │
                          │                   │                      │
                          │  ┌───────┬────────┼────────┬───────┐   │
                          │  │Health │Metrics │ Alert  │ Cron  │   │
                          │  │Checker│ Cache  │ Manager│Sched. │   │
                          │  └───────┴────────┴────────┴───────┘   │
                          │                                          │
                          │  ┌──────────────────────────────────┐   │
                          │  │  Store Factory (SQLite/MongoDB)   │   │
                          │  └──────────────────────────────────┘   │
                          │                                          │
                          │  ┌──────────────────────────────────┐   │
                          │  │  Embedded Frontend (go:embed)    │   │
                          │  └──────────────────────────────────┘   │
                          └──────────────────┬───────────────────────┘
                                             │
                    ┌────────────────────────┼────────────────────┐
                    │                        │                    │
               ┌────▼─────┐           ┌─────▼─────┐        ┌────▼─────┐
               │  Node A   │           │  Node B   │        │  Node C   │
               │  (SSH)    │           │ (Agent)   │        │ (Agent)   │
               │           │           │ ┌───────┐ │        │ ┌───────┐ │
               │           │           │ │ Agent │ │        │ │ Agent │ │
               │           │           │ │(PTY + │ │        │ │(PTY + │ │
               │           │           │ │Metrics│ │        │ │Metrics│ │
               │           │           │ └───────┘ │        │ └───────┘ │
               └───────────┘           └──────────┘        └──────────┘
```

## Component Breakdown

### 1. Control Plane (`cmd/server/main.go`)

Single Go binary that bundles:
- **Gin HTTP API** — REST endpoints for all CRUD operations
- **WebSocket Hub** — Real-time terminal, container logs, metrics push, deployment output
- **gRPC Tunnel Server** (:9090) — Accepts reverse connections from deployed agents
- **Health Checker** — Background goroutine: pings all nodes every 30s (with 2-strike grace period)
- **Metrics Cache** — Background goroutine: pre-fetches metrics every 15s (max 3 parallel SSH)
- **Alert Manager** — Evaluates threshold rules every 30s, sends webhook notifications
- **Cron Scheduler** — Executes scheduled tasks at defined intervals
- **Embedded Frontend** — Vue 3 build artifacts via `go:embed`

### 2. Transport Layer (`internal/transport/`)

All remote operations abstracted behind `Transporter` interface:

```go
type Transporter interface {
    Exec(ctx, nodeID, cmd) (ExecResult, error)
    ExecStream(ctx, nodeID, cmd) (<-chan StreamChunk, error)
    Upload / Download / PTY / ContainerPTY / Ping / Facts / Metrics
}
```

**Transport Selection Logic (per request):**
1. Agent connected via gRPC tunnel? → `TunnelTransport` (zero SSH overhead, PTY via gRPC)
2. Node configured as agent mode? → Direct gRPC `AgentTransport`
3. Default → `SSHTransport` with connection pool

### 3. SSH Layer (`internal/ssh/`)

- **Connection Pool** — Per-node pooling with keepalive (15s), stale detection, thundering-herd prevention
- **Auto Retry** — Stale connection detected → close → dial fresh → retry (max 2)
- **Auth Methods** — Private key → password → keyboard-interactive (auto-fallback)
- **Trust Establishment** — Ed25519 keypair generation → public key push → verification
- **PTY** — Full interactive terminal with resize, UTF-8 locale injection
- **SFTP** — File browser, upload, download, delete, mkdir, rename

### 4. Agent (`cmd/agent/main.go`)

Standalone Go binary deployed to managed nodes:
- **Reverse Connection** — Connects to server's gRPC port, maintains persistent bidirectional stream
- **PTY Support** — Opens `/dev/ptmx` with `setsid` + `setctty`, full interactive shell
- **Metrics Reporter** — CPU/memory/disk/network/GPU every 5s
- **Heartbeat** — Every 10s to keep tunnel alive
- **Auto-Reconnect** — Exponential backoff on disconnection
- **mTLS** — CA certificate verification (`--ca-cert` flag)
- **Resource Limits** — systemd cgroups: MemoryMax=64M, CPUQuota=5%

### 5. Storage Layer (`internal/store/`)

Repository pattern with swappable backends:
- **SQLite** — Default, zero-config, pure Go (`modernc.org/sqlite`)
- **MongoDB** — Optional, for larger fleets
- **Auto-migration** — Schema changes handled via `ALTER TABLE ADD COLUMN`

### 6. Security

| Layer | Mechanism |
|---|---|
| User Auth | JWT (HS256), bcrypt password hashing |
| Credential Storage | AES-256-GCM encryption, master key from env |
| SSH Keys | Ed25519, stored encrypted in DB |
| Agent Tunnel | gRPC with mTLS (CA cert verification) |
| WebSocket | JWT validated on connection upgrade |
| API Rate Limiting | 10/min login, 200/min authenticated |
| Audit Logging | All mutations logged with user/IP/duration |
| Release Mode | Requires explicit JWT secret + master key |

### 7. Frontend (`web/`)

Vue 3 SPA with Element Plus:
- **Theme System** — CSS variables, 3 modes (dark/light/system) × 6 accent colors
- **Terminal** — xterm.js with WebGL renderer, Ghostty color scheme, Unicode 11
- **Lazy Metrics** — Background parallel fetch, memory cache, no per-request SSH
- **Auto-reconnect** — Terminal sessions with exponential backoff
- **Split-pane** — Multi-tab terminal with horizontal/vertical split

### 8. Docker Management

All operations through transport layer:
- SSH mode: `$DPATH` PATH resolution for docker binary
- Agent mode: PTY via gRPC tunnel → `docker exec -it`
- Container logs: WebSocket streaming with 30min keepalive
- Batch operations: Cross-node start/stop/restart

### 9. RKE2 Orchestration

- **Pre-flight Checks** — CPU/RAM/Disk/Swap/Modules/Ports with auto-fix
- **Auto Mirror Detection** — CN nodes get Aliyun registry automatically
- **Proxy Support** — Three-layer injection (env + systemd override + config.yaml)
- **Helm** — Install/uninstall/list via RKE2 built-in helm
- **Rancher** — One-click install with auto mirror

## Data Flow Examples

### Web Terminal (Agent Online)
```
Browser (xterm.js)
  → WebSocket (/ws/terminal/:nodeID)
  → Transport Manager → TunnelTransport (gRPC)
  → Agent: PtyStart message
  → Agent: openPty() + setsid + shell
  ← PtyOutput stream via gRPC
```

### Web Terminal (Agent Offline, SSH Fallback)
```
Browser (xterm.js)
  → WebSocket (/ws/terminal/:nodeID)
  → Transport Manager → SSHTransport
  → SSH Pool → NewSession + RequestPty
  → Remote shell
```

### Metrics Collection
```
MetricsCache goroutine (every 15s, max 3 parallel)
  → For each online node:
    → Transport Manager
    → SSH Exec (or Agent cached data)
    → Parse output
    → Store in memory cache
  → Frontend GET /api/nodes/:id/metrics
    → Return from cache (instant, no SSH)
```

### Agent Auto-Deployment
```
UI: Click "Agent" button
  → Detect architecture (uname -m)
  → Upload binary via SFTP (fallback: base64 chunks)
  → Create systemd service (with resource limits)
  → systemctl enable + restart
  → Node switches to Agent mode
  → Agent connects via gRPC tunnel
  → All subsequent operations via tunnel
```

## Deployment Modes

### Single Binary (Default)
```bash
./devicegrid-server  # Serves API + embedded frontend on :8080
```

### Electron Desktop App
```bash
# All-in-one: Electron spawns Go server internally
# Data stored in ~/.config/DeviceGrid/
# No external dependencies needed
```

### Systemd Service (Production)
```bash
sudo dpkg -i devicegrid-*.deb
sudo systemctl enable --now devicegrid
```
