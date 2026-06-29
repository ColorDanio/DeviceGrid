# PLAN.md — DeviceGrid Implementation Roadmap

> Status legend: `[ ]` not started · `[~]` in progress · `[x]` complete

---

## Phase 1 — Foundation & Skeleton

**Goal**: Working Go server + Vue frontend scaffolding with dual database support,
config system, and embedded static assets.

- [x] **1.1 Project init**
  - [x] `go.mod` with module path `github.com/michael/device_grid`
  - [x] Directory structure per SPEC §1.3
  - [x] `.gitignore` (data/, configs/certs/, web/dist/, .env, *.db)
  - [x] `Makefile` with build/dev/test/lint/proto targets

- [x] **1.2 Config system** (`internal/config/`)
  - [x] Config struct matching SPEC §15
  - [x] Viper loader: reads `configs/config.yaml` + env overrides (`DG_` prefix)
  - [x] Auto-generate master key if empty (write warning to stderr)
  - [x] Config validation on startup

- [x] **1.3 Storage abstraction** (`internal/store/`)
  - [x] Model definitions (`internal/model/`): Node, DeployTask, DeployResult, Container, Cluster, User
  - [x] Repository interfaces (`internal/store/repo/`): Node, DeployTask, DeployResult, Container, Cluster, User
  - [x] `repo.Repositories` aggregate interface
  - [x] `store.Factory` — selects SQLite or MongoDB by config

- [x] **1.4 SQLite implementation** (`internal/store/sqlite/`)
  - [x] Connection setup (modernc.org/sqlite — pure Go, no CGO)
  - [x] Schema migrations (`migrations/0001_init.up.sql` / `.down.sql`)
  - [x] Implement all repository interfaces
  - [x] Indexes on frequently queried columns

- [x] **1.5 MongoDB implementation** (`internal/store/mongodb/`)
  - [x] Connection setup (go.mongodb.org/mongo-driver)
  - [x] Collection mapping + index creation on startup
  - [x] Implement all repository interfaces
  - [x] BSON tag mapping on models

- [x] **1.6 Crypto module** (`internal/crypto/`)
  - [x] AES-256-GCM encrypt/decrypt helpers
  - [x] Key derivation from config master key
  - [x] Unit tests (round-trip encrypt/decrypt)

- [x] **1.7 Auth module** (`internal/auth/`)
  - [x] JWT token generation + validation
  - [x] Gin middleware: `AuthRequired()` + `RoleRequired(roles...)`
  - [x] bcrypt password hashing helpers
  - [x] Seed default admin user on first run

- [x] **1.8 Gin server skeleton** (`internal/api/`, `cmd/server/main.go`)
  - [x] Router setup with route groups (auth, nodes, docker, deploy, rke2)
  - [x] Health check endpoint `/healthz`
  - [x] Graceful shutdown (signal handling)
  - [x] Structured logging (slog)
  - [x] CORS middleware for dev mode

- [x] **1.9 Frontend scaffold** (`web/`)
  - [x] Vite + Vue 3 + TypeScript project init
  - [x] Element Plus + Pinia + Vue Router setup
  - [x] API client (`src/api/client.ts`) with JWT interceptor
  - [x] Login page + auth store
  - [x] Layout (sidebar nav + header + router-view)
  - [x] Placeholder views for all sections

- [x] **1.10 Frontend embed** (`internal/web/`)
  - [x] `go:embed web/dist` for static assets
  - [x] Fallback to `index.html` for SPA routing
  - [x] Serve embedded assets in production mode

- [x] **1.11 Wire it together**
  - [x] `cmd/server/main.go`: load config → init store → init router → serve
  - [x] Verify `make dev-server` boots successfully
  - [x] Verify `make dev-web` shows login page

**Phase 1 Exit Criteria**: Server boots, connects to SQLite/MongoDB, serves
login page, JWT auth works, `make build` produces single binary.

---

## Phase 2 — Transport & Communication Layer

**Goal**: Both SSH and Agent transports functional, abstracted behind
Transporter interface.

- [ ] **2.1 Transport interface** (`internal/transport/`)
  - [ ] Define `Transporter` interface per SPEC §3.1
  - [ ] `TransportManager` with node-based selection
  - [ ] Fallback logic: agent → SSH on connection failure (configurable)

- [ ] **2.2 SSH core** (`internal/ssh/`, `internal/transport/ssh/`)
  - [ ] Connection manager with per-node pool
  - [ ] Key-based auth (decrypt private key from DB)
  - [ ] Password-based auth (for initial trust establishment)
  - [ ] `Exec`, `ExecStream`, `Upload`, `Download` implementations
  - [ ] Keepalive + stale connection reaping
  - [ ] Connect timeout handling

- [ ] **2.3 SSH trust establishment** (`internal/ssh/trust.go`)
  - [ ] Generate Ed25519 keypair
  - [ ] Connect with password, append public key to authorized_keys
  - [ ] Verify key-based login
  - [ ] Store encrypted private key, clear password
  - [ ] Idempotent: detect if key already present

- [ ] **2.4 SSH PTY** (`internal/ssh/pty.go`)
  - [ ] `RequestPty` session
  - [ ] Stdin/stdout pipe to interface
  - [ ] Window resize forwarding

- [ ] **2.5 Agent proto definition** (`internal/agent/proto/`)
  - [ ] Define `agent.proto` gRPC service
  - [ ] `make proto` generates Go code
  - [ ] Messages: Ping, Exec, Upload, Download, SystemInfo, Docker ops, PTY

- [ ] **2.6 Agent binary** (`cmd/agent/`, `internal/agent/`)
  - [ ] gRPC server with mTLS
  - [ ] Command execution (exec + stream)
  - [ ] File upload/download
  - [ ] System info collection
  - [ ] PTY session handling
  - [ ] systemd service file generation

- [ ] **2.7 Agent gRPC client** (`internal/transport/agent/`)
  - [ ] gRPC client with mTLS dial
  - [ ] Connection pool per agent
  - [ ] Implement all `Transporter` methods via gRPC
  - [ ] Health check / reconnection

- [ ] **2.8 Agent deployment** (`internal/agent/installer.go`)
  - [ ] Cross-compile agent for linux/amd64 + linux/arm64
  - [ ] SSH-based deployment: upload binary → systemd setup → start
  - [ ] Wait for gRPC health check
  - [ | Flip node.TransportMode to "agent"

- [ ] **2.9 mTLS cert management** (`internal/agent/certs.go`)
  - [ ] Generate CA on first server run
  - [ ] Sign agent certs during deployment
  - [ ] Server cert for gRPC listener
  - [ ] Store certs in `configs/certs/`

- [ ] **2.10 Transport tests**
  - [ ] Mock Transporter for unit tests
  - [ ] Integration test: SSH exec against local container
  - [ ] Integration test: Agent gRPC round-trip

**Phase 2 Exit Criteria**: Can SSH to a node, establish trust, execute commands,
deploy agent, and execute commands via agent gRPC — all behind the same
Transporter interface.

---

## Phase 3 — Node Management & Kanban

**Goal**: Full node lifecycle management with real-time dashboard.

- [ ] **3.1 Node CRUD API** (`internal/api/node.go`)
  - [ ] Create node (with initial password — stored encrypted temporarily)
  - [ ] List nodes (with filters: tag, status, transport)
  - [ ] Get node detail
  - [ ] Update node
  - [ ] Delete node (optionally clean up agent)
  - [ ] Trust establishment endpoint
  - [ ] Agent deployment endpoint

- [ ] **3.2 Node health checker** (`internal/node/health.go`)
  - [ ] Background goroutine: periodic health check per node
  - [ ] SSH mode: `transport.Ping()` every 30s
  - [ ] Agent mode: gRPC health check every 30s
  - [ ] Update node.Status + node.LastSeenAt in DB
  - [ ] Emit status change events to WS hub

- [ ] **3.3 Facts collector** (`internal/node/facts.go`)
  - [ ] OS detection (parse /etc/os-release)
  - [ ] Architecture detection
  - [ ] Docker version detection
  - [ ] Resource metrics: CPU %, memory, disk usage
  - [ ] Run on node creation + periodic refresh

- [ ] **3.4 WebSocket hub** (`internal/ws/`)
  - [ ] Client connection registry
  - [ ] Topic-based subscriptions (kanban, terminal, deploy, logs)
  - [ ] Broadcast metrics to kanban subscribers
  - [ ] Connection lifecycle management

- [ ] **3.5 Kanban frontend**
  - [ ] Node card grid with real-time metrics
  - [ ] WebSocket connection to `/ws/kanban`
  - [ ] Group/filter controls
  - [ ] Status color indicators
  - [ ] Quick action buttons (terminal, containers, detail)
  - [ ] Node detail page with full facts

- [ ] **3.6 Node management frontend**
  - [ ] Node list table with inline status
  - [ ] Create/edit node dialog (with password field for trust)
  - [ ] Trust establishment flow with progress feedback
  - [ ] Agent deployment button with status

**Phase 3 Exit Criteria**: Can add nodes, establish SSH trust, see real-time
metrics on Kanban, view node details, deploy agents.

---

## Phase 4 — Docker Lifecycle Management

**Goal**: Full Docker engine + container/image/compose/network/volume management.

- [ ] **4.1 Docker manager abstraction** (`internal/docker/`)
  - [ ] `DockerManager` that delegates to transport
  - [ ] Engine install/uninstall scripts (Ubuntu, CentOS)
  - [ ] Container operations (list, create, action, inspect)
  - [ ] Image operations (list, pull, remove, build)
  - [ ] Network operations (list, create, remove)
  - [ ] Volume operations (list, create, remove)
  - [ ] Compose operations (up, down, ps)

- [ ] **4.2 Docker API handlers** (`internal/api/docker.go`)
  - [ ] All REST endpoints per SPEC §13.1
  - [ ] Container log streaming via WebSocket
  - [ ] Container action (start/stop/restart/remove/pause)
  - [ ] Image pull with progress feedback

- [ ] **4.3 Docker in Agent** (gRPC)
  - [ ] Agent calls local Docker API directly (go SDK)
  - [ ] More efficient than CLI parsing
  - [ ] Streaming stats via gRPC server stream

- [ ] **4.4 Docker frontend**
  - [ ] Container list table (per node): status, actions, logs viewer
  - [ ] Container detail drawer: inspect, env, ports, logs
  - [ ] Image management page
  - [ ] Compose editor (YAML textarea + validate + up/down)
  - [ ] Network and volume management
  - [ ] Docker engine install/uninstall flow

- [ ] **4.5 Container log streaming**
  - [ ] WebSocket endpoint `/ws/containers/:nodeID/:cid/logs`
  - [ ] SSH mode: `docker logs -f`
  - [ ] Agent mode: Docker API log stream

**Phase 4 Exit Criteria**: Can install Docker, manage full container lifecycle,
pull images, run compose projects, view streaming logs — all from the UI.

---

## Phase 5 — Batch Deployment

**Goal**: Asynq-powered batch task engine with real-time output.

- [ ] **5.1 Asynq integration** (`internal/deploy/`)
  - [ ] Redis connection setup
  - [ ] Task type definitions (script, file, package)
  - [ ] Worker pool with configurable concurrency
  - [ ] Task queue priority

- [ ] **5.2 Deploy handlers** (`internal/api/deploy.go`)
  - [ ] Create deploy task → enqueue Asynq job
  - [ ] List/get deploy tasks + results
  - [ ] Cancel deploy task (Asynq deletion)

- [ ] **5.3 Deploy worker** (`internal/deploy/worker.go`)
  - [ ] Process task: iterate nodeIDs respecting concurrency
  - [ ] Execute via transport (Exec for scripts, Upload for files)
  - [ ] Stream output to WS hub
  - [ ] Update DeployResult incrementally
  - [ ] Handle timeout + cancellation

- [ ] **5.4 Deploy frontend**
  - [ ] Task creation wizard (select nodes, type, payload)
  - [ ] Task list with status badges
  - [ ] Task detail: per-node results with streaming output
  - [ ] Live terminal-style output viewer

**Phase 5 Exit Criteria**: Can create batch deploy tasks targeting multiple
nodes, see real-time per-node output, view aggregated results.

---

## Phase 6 — RKE2 Orchestration (Optional)

**Goal**: Full RKE2 cluster lifecycle from the UI.

- [ ] **6.1 RKE2 manager** (`internal/rke2/`)
  - [ ] Config.yaml generator (template + form fields)
  - [ ] Server installation script
  - [ ] Agent installation script (with token + server URL)
  - [ ] Token extraction from server node
  - [ ] Node drain/cordon/uncordon
  - [ ] Cluster status collection (kubectl get nodes)

- [ ] **6.2 RKE2 API handlers** (`internal/api/rke2.go`)
  - [ ] Cluster CRUD
  - [ ] Config update endpoint
  - [ ] Upgrade endpoint (rolling)
  - [ ] Status monitoring

- [ ] **6.3 RKE2 frontend**
  - [ ] Cluster creation wizard (select nodes, assign roles, config form)
  - [ ] Config.yaml editor (form + raw YAML toggle)
  - [ ] Cluster detail: node table, health, cert expiry
  - [ ] Upgrade flow with progress

**Phase 6 Exit Criteria**: Can create RKE2 clusters, edit config, monitor
health, and perform rolling upgrades.

---

## Phase 7 — Polish & Packaging

**Goal**: Production-ready single binary with embedded frontend.

- [ ] **7.1 Web Terminal**
  - [ ] xterm.js integration in Vue
  - [ ] WebSocket binary protocol for PTY
  - [ ] Resize handling
  - [ ] Multi-tab support

- [ ] **7.2 Settings & User Management**
  - [ ] User list/create/delete (admin)
  - [ ] Change password
  - [ ] System settings page

- [ ] **7.3 Embed & Package**
  - [ ] `make package` → builds frontend + embeds + cross-compiles
  - [ ] Single binary serves API + frontend
  - [ ] Agent binary cross-compiled (amd64 + arm64)

- [ ] **7.4 Testing**
  - [ ] Unit test coverage > 70% for business logic
  - [ ] Integration tests with testcontainers (SSH, Docker)
  - [ ] Frontend component tests (Vitest)

- [ ] **7.5 Documentation**
  - [ ] README.md with quickstart
  - [ ] API documentation (OpenAPI/Swagger)
  - [ ] Deployment guide

**Phase 7 Exit Criteria**: `make package` produces distributable binary that
runs the complete application.

---

## Dependency Summary

| Dependency | Purpose | Import Path |
|---|---|---|
| Gin | HTTP framework | `github.com/gin-gonic/gin` |
| gorilla/websocket | WebSocket | `github.com/gorilla/websocket` |
| Viper | Configuration | `github.com/spf13/viper` |
| golang-jwt | JWT auth | `github.com/golang-jwt/jwt/v5` |
| golang.org/x/crypto | SSH + bcrypt | `golang.org/x/crypto` |
| modernc.org/sqlite | SQLite (pure Go) | `modernc.org/sqlite` |
| jmoiron/sqlx | SQL helper | `github.com/jmoiron/sqlx` |
| mongo-driver | MongoDB | `go.mongodb.org/mongo-driver` |
| asynq | Task queue | `github.com/hibiken/asynq` |
| grpc-go | gRPC | `google.golang.org/grpc` |
| protobuf | gRPC codegen | `google.golang.org/protobuf` |
| zap/slog | Logging | `log/slog` (stdlib) |
| testify | Test assertions | `github.com/stretchr/testify` |
| docker SDK | Docker API (agent) | `github.com/docker/docker/client` |
| xterm.js | Web terminal | npm: `@xterm/xterm` |
| Element Plus | UI components | npm: `element-plus` |

---

## Milestone Tracking

| Phase | Target | Status |
|---|---|---|
| Phase 1 — Foundation | Base skeleton + dual DB | `[x]` Complete |
| Phase 2 — Transport | SSH + Agent dual mode | `[x]` Complete |
| Phase 3 — Nodes + Kanban | Dashboard + node mgmt | `[x]` Complete |
| Phase 4 — Docker | Container lifecycle | `[x]` Complete |
| Phase 5 — Deploy | Batch deployment | `[x]` Complete |
| Phase 6 — RKE2 | K8s orchestration | `[x]` Complete |
| Phase 7 — Polish | Terminal/Web/SFTP/Themes | `[x]` Complete |

## Phase 8 — Production Hardening (Post-Release)

- [x] **8.1 Security**
  - [x] Agent mTLS certificate verification (remove InsecureSkipVerify)
  - [x] Configurable default admin password (env var)
  - [x] Rate limiting on auth endpoints
  - [x] CORS origin from config

- [x] **8.2 Scalability**
  - [x] Configurable health check / metrics intervals
  - [x] Deploy engine uses config max_concurrent
  - [x] MongoDB init() bypass fix (use config URI)
  - [x] Configurable HTTP server timeouts

- [~] **8.3 Features**
  - [ ] Agent PTY via gRPC tunnel (eliminate SSH for terminal)
  - [ ] Docker management via Agent (local Docker API)
  - [ ] Configurable Docker/RKE2 install mirror URLs
  - [x] Terminal search (Ctrl+F) with SearchAddon
  - [x] Batch deployment with file distribution

- [~] **8.4 Testing**
  - [x] Unit tests: auth, crypto, SSH pool, SQLite repos
  - [ ] Coverage > 70% (current: ~25%, focused on critical paths)
  - [ ] Integration tests with testcontainers
  - [ ] Frontend component tests (Vitest)

- [x] **8.5 Distribution**
  - [x] Docker image (multi-stage, distroless)
  - [x] systemd service file (hardened)
  - [ ] Auto-update mechanism for agent
| Phase 7 — Polish | Packaging + release | `[x]` Complete |
