# Architecture — DeviceGrid

## System Overview

```
                          ┌──────────────────────────────────┐
                          │         DeviceGrid Server         │
                          │  ┌────────┐  ┌──────┐  ┌───────┐ │
                          │  │ Gin API│  │WS Hub │  │Asynq  │ │
                          │  └───┬────┘  └──┬───┘  └───┬───┘ │
                          │      │          │          │     │
                          │  ┌───▼──────────▼──────────▼───┐ │
                          │  │     Business Logic Layer     │ │
                          │  │ node · docker · deploy · rke2│ │
                          │  └───┬──────────────────────┬───┘ │
                          │      │                      │     │
                          │  ┌───▼──────┐    ┌─────────▼───┐ │
                          │  │store.Factory│  │transport.Mgr│ │
                          │  │sqlite/mongo│  │ ssh / tunnel│ │
                          │  └──────────┘    └────────┬────┘ │
                          │                           │      │
                          └───────────────────────────┼──────┘
                                                      │
                                   ┌──────────────────┼──────────────┐
                                   │                  │              │
                              ┌────▼─────┐     ┌─────▼────┐   ┌────▼─────┐
                              │  Node A   │     │  Node B   │   │  Node C   │
                              │  (SSH)    │     │ (Agent)   │   │ (Agent)   │
                              │           │     │ ┌───────┐│   │ ┌───────┐ │
                              │           │     │ │ Agent ││   │ │ Agent │ │
                              └───────────┘     │ └───────┘│   │ └───────┘ │
                                                └──────────┘   └──────────┘
```

## Component Breakdown

### 1. Control Plane (`cmd/server/main.go`)

The server is a single Go binary that bundles:
- **Gin HTTP API** — REST endpoints for all CRUD operations
- **WebSocket Hub** — Real-time terminal, metrics push, deployment output
- **gRPC Tunnel Server** — Accepts reverse connections from deployed agents
- **Health Checker** — Background goroutine that pings all nodes every 10s
- **Metrics Cache** — Background goroutine that pre-fetches metrics every 15s
- **Embedded Frontend** — Vue 3 build artifacts via `go:embed`

### 2. Transport Layer (`internal/transport/`)

All remote operations are abstracted behind `Transporter` interface:

```go
type Transporter interface {
    Exec(ctx, nodeID, cmd) (ExecResult, error)
    ExecStream(ctx, nodeID, cmd) (<-chan StreamChunk, error)
    Upload(ctx, nodeID, path, reader, mode) error
    Download(ctx, nodeID, path) (io.ReadCloser, error)
    PTY(ctx, nodeID, cols, rows) (PTYSession, error)
    ContainerPTY(ctx, nodeID, containerID, cols, rows) (PTYSession, error)
    Ping(ctx, nodeID) error
    Facts(ctx, nodeID) (NodeFacts, error)
    Metrics(ctx, nodeID) (NodeMetrics, error)
}
```

**Transport Selection Logic:**
1. If agent is connected via gRPC tunnel → use `TunnelTransport` (zero SSH overhead)
2. Else if `node.TransportMode == "agent"` → use `AgentTransport` (direct gRPC)
3. Else → use `SSHTransport` (traditional SSH)

### 3. SSH Layer (`internal/ssh/`)

- **Connection Pool** — Per-node connection pooling with keepalive and stale reaping
- **Auth Methods** — Tries private key first, then password + keyboard-interactive (for servers with `PasswordAuthentication no`)
- **Trust Establishment** — Generates Ed25519 keypair, pushes public key via password-authed SSH session, verifies key-based login
- **PTY** — Full interactive terminal with resize support, UTF-8 locale injection
- **SFTP** — File browser, upload, download, delete, mkdir

### 4. Agent (`cmd/agent/main.go`)

Standalone Go binary deployed to managed nodes:
- **Reverse Connection** — Connects to server's gRPC port, maintains persistent bidirectional stream
- **Metrics Reporter** — Collects CPU/memory/disk/network/GPU every 5s and pushes to server
- **Heartbeat** — Every 10s to keep tunnel alive
- **Auto-Reconnect** — Exponential backoff on disconnection
- **Command Execution** — Receives exec/file operations via tunnel, executes locally

### 5. Storage Layer (`internal/store/`)

Repository pattern with swappable backends:

```
repo.Repositories (interface)
├── NodeRepository
├── DeployTaskRepository
├── DeployResultRepository
├── ContainerRepository
├── ClusterRepository
└── UserRepository
```

- **SQLite** — Default, zero-config, file-based. Uses `modernc.org/sqlite` (pure Go, no CGO)
- **MongoDB** — Optional, for larger fleets or document-oriented use cases

### 6. Frontend (`web/`)

Vue 3 SPA with Element Plus:
- **Theme System** — CSS variables driven, 3 modes (dark/light/system) × 6 accent colors
- **Terminal** — xterm.js with WebGL renderer, Ghostty-inspired color scheme, Unicode 11
- **Lazy Metrics** — Background parallel fetch with memory cache, no per-request SSH overhead
- **WebSocket** — Terminal I/O, real-time metrics, deployment output

### 7. Docker Management (`internal/docker/`)

All Docker operations go through the transport layer:
- SSH mode: executes `docker` CLI commands (with `$DPATH` PATH resolution)
- Agent mode: calls local Docker API directly (planned)
- Supports: container CRUD, image management, Compose, networks, volumes, logs

### 8. Network Diagnostics (`internal/api/netcheck.go`)

Runs on the remote node via SSH/gRPC:
- **Streaming Unlock** — Tests 22+ services (Netflix, Disney+, YouTube, etc.)
- **AI Availability** — Tests 20+ services (ChatGPT, Claude, Gemini, etc.)
- **Connectivity** — Ping latency to global regions
- **Return Route** — Traceroute to China ISP nodes (CU/CT/CM) with AS-number-based line type detection (CN2 GIA/GT, 4837, etc.)

## Data Flow Examples

### Web Terminal Connection
```
Browser (xterm.js)
  → WebSocket (/ws/terminal/:nodeID)
  → Go WS Handler
  → Transport Manager → SSH PTY (or Agent Tunnel)
  → Remote shell
```

### Metrics Collection
```
Background MetricsCache goroutine (every 15s)
  → For each online node:
    → Transport Manager → SSH Exec (or Agent cached data)
    → Parse metrics output
    → Store in memory cache
  → Frontend GET /api/nodes/:id/metrics
    → Return from cache (instant, no SSH)
```

### Agent Tunnel Connection
```
Agent binary on node
  → gRPC Connect() stream to Server:9090
  → Server TunnelServer.Connect()
  → Register in AgentRegistry
  → Metrics reported via stream every 5s
  → Server caches metrics (zero SSH for data)
  → Commands sent via stream (no SSH needed)
```

## Security Model

| Layer | Mechanism |
|---|---|
| User Auth | JWT (HS256), bcrypt password hashing |
| Credential Storage | AES-256-GCM encryption, master key from env |
| SSH Keys | Ed25519, stored encrypted in DB |
| Agent Tunnel | gRPC with mTLS (planned), currently InsecureSkipVerify |
| WebSocket | JWT validated on connection upgrade |
| Release Mode | Requires explicit JWT secret + master key |

## Deployment Modes

### Single Binary (Default)
```bash
./devicegrid-server  # Serves API + embedded frontend on :8080
```

### Electron Desktop App
```bash
make electron  # Builds AppImage/.deb/.exe with bundled server
```

### Docker (Future)
```bash
docker run -p 8080:8080 -v ./data:/data devicegrid/server
```
