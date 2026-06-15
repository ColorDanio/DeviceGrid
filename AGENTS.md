# AGENTS.md

Instructions for AI agents (opencode, Claude, Copilot, etc.) working on this codebase.

## Project Overview

**DeviceGrid** is a server fleet management web application built in Go + Vue 3.
It manages multiple server nodes via dual communication modes (SSH agentless + gRPC
Agent), provides a Kanban dashboard, batch deployment, Docker lifecycle management,
and optional RKE2 Kubernetes orchestration.

## Tech Stack

- **Backend**: Go 1.22+ (Gin, gorilla/websocket, gRPC, Asynq)
- **Frontend**: Vue 3 + Element Plus + Vite + Pinia + Vue Router + xterm.js
- **Databases**: SQLite (default) and MongoDB (swappable via config)
- **Message Queue**: Redis + Asynq for batch task scheduling
- **Agent**: Independent Go binary, gRPC + mTLS

## Essential Commands

```bash
# Build everything
make build

# Run backend server (with hot reload via air)
make dev-server

# Run frontend dev server
make dev-web

# Run tests
make test

# Run linters
make lint          # go vet + golangci-lint
make lint-web      # eslint on web/

# Type check frontend
make typecheck-web

# Generate protobuf for agent gRPC
make proto

# Generate DB migrations
make migrate-create name=<description>
make migrate-up
make migrate-down

# Embed frontend into Go binary (release)
make package
```

## Code Style

- **Go**: Follow `gofmt` + `goimports`. Use `golangci-lint`. No unused code.
  - Error handling: always wrap with context: `fmt.Errorf("do X: %w", err)`
  - Naming: exported identifiers need doc comments
  - Package layout: one responsibility per package
  - Interfaces: define in the consumer package, not the implementer
  - No `init()` side effects — use explicit `New()` constructors
  - NEVER add comments unless the comment explains *why* (not *what*)

- **Vue/TypeScript**: Follow ESLint + Prettier config in `web/`
  - Use `<script setup lang="ts">` Composition API
  - Props typed with TypeScript interfaces
  - API calls go in `src/api/`, components in `src/components/`, views in `src/views/`
  - Use Pinia stores for shared state
  - Element Plus components for UI primitives

## Architecture Rules

### Storage Layer
- ALL database access goes through `internal/store/repo` interfaces
- NEVER import `internal/store/sqlite` or `internal/store/mongodb` directly in business logic
- Use `store.Factory` to obtain repository instances based on config
- Schema changes require a migration in `migrations/`

### Transport Layer
- ALL remote node operations go through `internal/transport.Transporter` interface
- Two implementations: `transport/ssh` and `transport/agent`
- Business code must not know which transport is in use
- Transport is chosen per-node based on `node.TransportMode`

### Security
- SSH private keys stored AES-256-GCM encrypted in DB (master key from env/config)
- Agent uses mTLS; certs in `configs/certs/`
- JWT for user auth, configurable expiry
- NEVER log secrets, passwords, private keys, or tokens
- NEVER commit `configs/certs/`, `data/`, or `.env`

## Testing
- Go tests colocated: `foo_test.go` next to `foo.go`
- Use table-driven tests
- Mock the repo + transport interfaces, not the implementations
- Frontend: Vitest for unit tests
- Run `make test` before considering work done

## Git Conventions
- Branch naming: `feat/<topic>`, `fix/<topic>`, `chore/<topic>`
- Commit message format (Conventional Commits):
  ```
  feat(docker): add container restart endpoint
  fix(ssh): handle connection timeout on slow networks
  docs: update PLAN.md phase 3 status
  ```
- NEVER commit unless explicitly asked

## When Adding a Feature
1. Read SPEC.md for the relevant section to understand the contract
2. Check PLAN.md to see if the phase is started/complete
3. Define or update the repo interface in `store/repo/` first
4. Implement SQLite version, then MongoDB version
5. Add the transport-agnostic handler in `internal/api/`
6. Add the frontend view/component
7. Write tests
8. Run `make lint && make test`
9. Update PLAN.md status checkboxes
