# AGENTS.md

High-signal instructions for AI agents (opencode, Claude, Copilot, etc.) working in this codebase.
Pair this with `SPEC.md` (feature/API contract), `ARCHITECTURE.md` (component/data-flow diagrams),
and `PLAN.md` (delivery status) when adding features.

## Commands

```bash
make build           # build server + agent into bin/
make dev-server      # go run ./cmd/server  (NOT hot-reload; despite the help text there is no `air`)
make dev-web         # vite dev server from web/

make test            # go test -race ./...  (120s timeout)
make test-web        # vitest run  (from web/)
make lint            # go vet + gofmt + goimports (REQUIRED). golangci-lint/staticcheck run only if installed.
make lint-web        # eslint .  (from web/)
make typecheck-web   # vue-tsc --noEmit  (from web/)

make proto           # regenerate internal/agent/proto/*.pb.go from agent.proto
make package         # full release build: web -> embed into internal/web/dist -> server + cross-compiled agents
make electron        # build Electron desktop app (see electron/)
```

Run a single Go test: `go test -run TestName ./internal/store/sqlite/...`
Run a single frontend test: `cd web && npx vitest run path/to/file`

Validation order before considering work done: `make lint && make typecheck-web && make test && make test-web`.

## Tech Stack (verified from go.mod / web/package.json)

- **Backend**: Go 1.25, Gin, gorilla/websocket, gRPC, golang-jwt, modernc.org/sqlite (pure-Go, no CGO), go.mongodb.org/mongo-driver. Viper for config.
- **Frontend**: Vue 3 + Element Plus + Vite + Pinia + Vue Router + vue-i18n + xterm.js. TypeScript via `vue-tsc`. Tests: Vitest + @vue/test-utils + jsdom.
- **Databases**: SQLite (default) and MongoDB, selected by `database.driver` in config.
- **Agent**: independent Go binary (`cmd/agent`), connects to server via gRPC reverse tunnel with mTLS.
- No `air`, no Asynq, no Redis in the current dependency tree despite older docs implying them.

## Architecture Rules

### Storage Layer
- ALL database access goes through `internal/store/repo.Repositories` (interface in `repo.go`).
- NEVER import `internal/store/sqlite` or `internal/store/mongodb` in business logic. Drivers self-register via `repo.Register()` in `init()` and are activated by blank import in `cmd/server/main.go`.
- Obtain a `Repositories` via `repo.New(ctx, cfg.Database)` (factory by driver name).

### Schema / Migrations (IMPORTANT — differs from typical Go projects)
- The `migrations/*.sql` files and the `make migrate-up` / `migrate-down` / `migrate-create` targets are **stale**: `cmd/server/main.go` does not parse the `--migrate` flag, so those targets no-op the server.
- SQLite schema lives in `internal/store/sqlite/store.go` as embedded `schemaSQL` (`CREATE TABLE IF NOT EXISTS ...`), plus a hand-maintained `ALTER TABLE` backfill list in `migrate()`.
- **To add a column/table**: edit `schemaSQL` and append a matching idempotent `ALTER TABLE` to the `migrations` slice in `internal/store/sqlite/store.go`. Add the MongoDB equivalent in `internal/store/mongodb/`.
- Do NOT rely on `make migrate-up`.

### Transport Layer
- ALL remote node operations go through `internal/transport.Manager` (implements `Transporter`). Business code must not branch on transport type.
- Selection order in `Manager.getTransport` (`internal/transport/transport.go:107`):
  1. **Tunnel** (`agent.NewTunnelTransport`) if the agent is currently connected (checked via `agentReg.IsConnected`)
  2. else `node.TransportMode == TransportAgent` -> agent gRPC transport
  3. else SSH
- So an "agent mode" node that is offline still falls back to SSH automatically.

### Security
- SSH private keys / passwords stored AES-256-GCM encrypted in DB. Master key from `crypto.master_key` in config or `DG_CRYPTO_MASTER_KEY` env var. If both are empty in debug mode, a key is generated and written to `configs/.master_key` (chmod 600).
- In `server.mode: release`, both `auth.jwt_secret` and `crypto.master_key` are **required** or startup fails.
- Agent uses mTLS; certs configured under `agent.*_cert` / `*_key` (default `configs/certs/`).
- NEVER log secrets, passwords, private keys, or tokens.
- NEVER commit `configs/certs/`, `data/`, `configs/.master_key`, or `.env`.

### Configuration
- Config file path: `DG_CONFIG_PATH` env var, default `configs/config.yaml`.
- Viper with env prefix `DG_` and `.` -> `_` replacer. e.g. `database.sqlite.path` -> `DG_DATABASE_SQLITE_PATH`.
- Two special-cased env vars read directly: `DG_CRYPTO_MASTER_KEY`, `DG_AUTH_JWT_SECRET`.
- `network.environment: internal` disables public-network features (geo lookup, streaming/AI checks, connectivity/route tests).

## Frontend Conventions

- `<script setup lang="ts">` Composition API only. Props typed with TS interfaces.
- Layout: `src/api/` (axios calls), `src/components/`, `src/views/`, `src/composables/`, `src/stores/` (Pinia), `src/layouts/`, `src/utils/`, `src/router/`.
- i18n is centralized in `src/i18n/index.ts` as one inline `messages` object with `zh-CN` and `en-US` locales. The UI default follows `navigator.language` and persists to `localStorage` under `dg_locale`. **When adding any user-visible string, add keys to BOTH locales in that file** — do not hardcode Chinese or English in components.
- Element Plus is auto-imported via `unplugin-vue-components` (see `auto-imports.d.ts`, `components.d.ts`); do not manually import components.

## Go Conventions

- `gofmt` + `goimports` mandatory. Error wrapping: `fmt.Errorf("do X: %w", err)`.
- Exported identifiers need doc comments. Interfaces live in the consumer package.
- No `init()` side effects except driver self-registration in `store/sqlite` and `store/mongodb`.
- NEVER add comments unless they explain *why* (not *what*).

## Git Conventions

- Branches: `feat/<topic>`, `fix/<topic>`, `chore/<topic>`.
- Conventional Commits: `feat(docker): add container restart endpoint`.
- NEVER commit unless explicitly asked.

## When Adding a Feature

1. Read `SPEC.md` for the contract and `PLAN.md` for phase status.
2. Define/update the repo interface in `internal/store/repo/` first.
3. Implement SQLite in `internal/store/sqlite/` (update `schemaSQL` + `ALTER` backfills), then MongoDB in `internal/store/mongodb/`.
4. Add the transport-agnostic handler under `internal/api/`.
5. Add frontend view/component + both i18n locale entries.
6. Write tests (table-driven; mock `repo.Repositories` and `transport.Transporter`, never the concrete implementations).
7. Run `make lint && make typecheck-web && make test && make test-web`.
8. Update `PLAN.md` status checkboxes.
