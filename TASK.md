# DeviceGrid Task Tracker

Last reviewed: 2026-07-20

This is the active development tracker. `PLAN.md` records the original delivery plan,
but its phase-level checkboxes are no longer in sync with the implemented code; use
this file for current priorities until that plan is reconciled.

## Ready To Land

- [x] Restore GitHub Actions compatibility with the current toolchain.
  - GitHub Actions reads the Go version from `go.mod`.
  - CI uses Node.js 22, which satisfies Vite 8.
  - Frontend dependency installs use `npm ci`.
  - Releases run only from a manual dispatch on `main`.
  - MongoDB repository source is `gofmt` compliant.
  - Generated protobuf files are excluded from authored-source `goimports` checks.
- [x] Refresh project entry-point documentation.
  - README is concise and product-focused.
  - Operations and configuration detail lives in `docs/OPERATIONS.md`.
- [x] Restore local frontend-to-backend development routing.
  - Vite proxies `/api` and `/ws` to the configured default server port `3000`.

## Current Baseline

- [x] Core SSH and agent transport, node management, Docker, deployment, RKE2,
  terminal, SFTP, user management, alerts, and audit APIs are present.
- [x] Production packaging and GitHub release workflow are present.
- [x] Backend unit tests cover authentication, crypto, SSH pooling, and SQLite repos.
- [ ] Frontend has no unit-test runner or component tests.
- [x] Frontend linting and type-checking commands run through `package.json`,
  `Makefile`, and CI.
- [x] Chinese and English locale infrastructure covers login, application shell,
  navigation, dashboard labels, statuses, and accessible control names.
- [ ] Feature-specific views still contain hard-coded Chinese strings and need to
  migrate to the shared locale catalog.
- [ ] No integration suite validates SSH, Agent gRPC, Docker, or RKE2 workflows.
- [ ] Backend coverage has not been measured in the current toolchain because its
  `covdata` tool is unavailable; rerun with the Go version declared in `go.mod`.
- [x] Initial login guidance is server-reported and does not pre-fill or advertise a password.
- [ ] Fleet views duplicate polling: the layout fetches nodes every 15s, Kanban
  fetches nodes every 30s plus every online node's metrics every 20s, and Nodes
  fetches nodes and metrics every 10s.
- [x] Health checks use a configurable bounded worker pool.
- [ ] The current production build has a 1.09 MB shared JavaScript chunk (357 KB gzip)
  and a 524 KB terminal codec chunk (139 KB gzip); route-level loading remains unmeasured.
- [x] Playwright UX audit mobile-shell findings were resolved and verified at 390px and 768px.
- [x] Empty dashboard and dependent feature views route operators to node setup.
- [x] Shared and destructive icon-only controls have accessible names and tooltips.

## Next Priorities

1. [x] Deliver a mobile-first application shell.
   - Replaced the persistent narrow-screen sidebar with an accessible overlay drawer.
   - Stacked page actions and contained wide tables within their cards on mobile.
   - Verified Dashboard, Nodes, Deploy, Docker, and Terminal at 390px; verified
     the shared desktop/tablet shell across the same routes at 768px with Playwright.
2. [x] Build an actionable first-fleet onboarding path.
   - [x] Replaced zero-value dashboard KPIs with a concise empty state and "添加节点" CTA.
   - [x] Linked Docker, Deploy, Terminal, and SFTP prerequisite states to node setup.
   - [x] Split node onboarding into connect, verify, trust, and save steps with clear errors.
3. [x] Add accessible names and tooltips to icon-only controls.
   - [x] Covered navigation collapse, refresh, theme, close, and destructive icon actions.
   - [x] Verified shared-control names through Playwright accessibility snapshots; the
     remaining terminal close and Docker image-delete controls now provide labels and titles.
4. [x] Make initial login guidance truthful and secure.
   - [x] Removed pre-filled and advertised default credentials from the login page.
   - [x] Added the server-reported, one-time `/api/auth/setup` status; it clears after
     the first successful sign-in or on the next process start.
   - [x] Kept local development smooth with neutral setup guidance while keeping
     deployment credentials out of the browser UI.
5. [x] Complete the i18n rollout across feature views.
   - [x] Migrated Terminal, SFTP, SSH keys, Compare, Audit, Settings, Automation, Deploy, Docker, RKE2, Nodes, and Node Detail to the locale catalog.
   - [x] All feature-view UI strings now use the locale catalog; backend payloads and protocol values remain locale-neutral.
   - [x] Vitest verifies representative Docker, RKE2, Nodes, and Node Detail labels in both locales.
6. [x] Replace fleet polling with a shared, visibility-aware state source.
   - [x] Shared node and metric data between the layout, Kanban, and Nodes through a Pinia fleet store.
   - [x] Subscribed to the authenticated `kanban` and per-node `metrics-*` WebSocket topics.
   - [x] Uses visibility-aware 20-second polling only while the socket is disconnected; Vitest covers auth framing, subscription, and metric messages.
7. [x] Bound health-check concurrency for larger fleets.
   - [x] Added `node.health_check_concurrency`, defaulting to three workers.
   - [x] Health-check workers stop accepting jobs when the checker context is cancelled,
     then the scheduler waits for the bounded pool to return.
8. [x] Establish a frontend performance budget and reduce initial payload.
   - [x] Stopped globally registering every Element Plus icon; the shared client chunk
     fell from 1.19 MB / 377 KB gzip to 1.04 MB / 338 KB gzip.
   - [x] `make route-report` measures manifest-based gzip dependency closures; Docker is 486 KB and Terminal is 510 KB because both include the 139 KB codec chunk.
   - [x] CI enforces initial-load budgets of 370 KB gzip JavaScript and 60 KB gzip CSS.
9. [x] Reconcile `PLAN.md` with the implemented code and this tracker.
   - Clarified that Phases 1–7 granular lists are historical; verified milestone
     status, Phase 8, and this tracker are the maintained sources of truth.
   - Updated Phase 8 frontend component-test status; remaining Phase 8 items
     match the production-feature, integration, coverage, and release-preflight work below.
10. [x] Add frontend testing with Vitest and Vue Test Utils.
   - [x] Added focused coverage for the auth store, API error-message handling, and
     the dashboard's first-fleet empty state.
   - [x] Added `npm run test`, `make test-web`, and the frontend CI test step.
11. [x] Add integration coverage for transport boundaries.
    - Exercise SSH execution and SFTP against an ephemeral target.
      (`internal/ssh/integration_test.go`: in-process SSH + SFTP server backed
      by a temp dir; covers Exec, non-zero exit, pool reuse, Ping, Upload
      via SCP, Download via `cat`, Facts parsing, and SFTPListDir.)
    - Exercise Agent gRPC tunnel request/response behavior.
      (`internal/agent/tunnel_integration_test.go`: real gRPC TunnelServer +
      fake agent client stream; covers Exec round trip, non-zero exit,
      not-connected error, Upload ack, FileList round trip, and disconnect
      unregistering. Surfaced and fixed a `Registry.LastSeen` data race by
      moving the field to `atomic.Int64`.)
12. [x] Close remaining production features from `PLAN.md` Phase 8.
    - [x] Agent-backed PTY terminal is implemented through the connected tunnel transport and selected by terminal WebSocket handlers.
    - Docker operations through the Agent local API.
      - Added `DockerListRequest`/`DockerListResponse` messages to `agent.proto`
        and regenerated the gRPC bindings; agent handles
        `DockerListRequest` by HTTP-GETting `/var/run/docker.sock` (no Docker
        SDK dependency, no CLI parsing — agent is a thin Engine REST proxy).
      - Server side: `transport.Manager.DockerList` exposes the tunnel path,
        `docker.Manager.ListContainers` / `ListImages` opportunistically use
        it via `transport.DockerLister` and fall back to the existing CLI on
        `ErrDockerViaTransportUnavailable` or parse failure.
      - Covers containers + images today; networks/volumes endpoints are
        wired in the agent's `dockerListPath` so follow-up work is one
        helper per kind.
      - Tests: `cmd/agent/docker_test.go` (fake Engine over unix socket +
        capture stream) and `internal/docker/manager_test.go` (JSON parsers,
        transport wiring, opportunistic path, fallback sentinel).
    - [x] Configurable Docker registry and RKE2 installer mirror URLs, with RKE2 installer-script coverage.
13. [x] Add release preflight checks.
    - [x] `make release-preflight` starts the release server binary on an isolated port
      and verifies `/healthz` plus embedded SPA fallback serving.
    - [x] Release CI verifies the Debian package metadata and expected server, agent,
      config, and systemd paths before publishing artifacts.

## Future Priorities (creative direction, added 2026-07-20)

Scope note: these are larger feature tracks. Each is broken into a thin MVP slice
suitable for landing in one PR, followed by iteration steps. Pick one at a time;
do not block 11/12 on these.

14. [ ] Add a fleet metrics time-series store and graph view.
    - Persist per-node CPU / memory / disk / container-count samples at a
      configurable cadence (`metrics.storage_interval`, default 30s).
    - Add `MetricRepository` to `internal/store/repo/` with SQLite (rolled
      bucket table) and MongoDB implementations; backfill `ALTER TABLE` per
      AGENTS.md schema rules.
    - Add `GET /api/nodes/:id/metrics?range=1h|24h|7d` returning downsampled
      series for charts.
    - Frontend: sparkline on the Kanban card and a history chart on Node
      Detail (lightweight inline SVG first; defer chart libs until budgeted).
    - Migrate `internal/api/alerts.go` threshold evaluation from the latest
      gauge sample to the stored series (avg-over-window, sustained-for).

15. [ ] Add a reusable blueprint / topology library.
    - New `Blueprint` model: name, version, kind (compose | script | rke2 |
      systemd), body (parameterized text), variable schema (JSON-Schema-lite).
    - Repository + CRUD endpoints under `/api/blueprints`.
    - "Apply blueprint" endpoint: pick nodes, fill variables, compile to a
      `DeployTask` or `Cluster` create payload.
    - Frontend: library browser, variable form generated from schema,
      apply-to-nodes wizard reusing the existing `NodeSelector`.
    - Optional later: Git-backed import (`blueprints.git.url` config) with
      pull-on-startup and a manual refresh button.

16. [ ] Add runbook / playbook automation.
    - `Playbook` model: ordered list of steps (`exec` | `upload` | `wait` |
      `assert`), per-step `on_fail` (abort | continue | retry(n)), target
      scope (node set or tag query).
    - Engine reuses the deploy worker with a new task type, keeps per-node,
      per-step state, and streams output over `/ws/playbook/:runID`.
    - Frontend: playbook editor with step palette, run view with per-node
      step progress and live output.
    - Ship 2-3 starter playbooks in `examples/playbooks/` (collect diagnostics,
      rotate SSH key fleet-wide, drain + restart docker).

17. [ ] Add systemd / cron fleet management.
    - New transport surface in `internal/transport/transport.go`:
      `SystemdList`, `SystemdAction`, `TimerList`, `CronList`, `CronSet`.
    - Implement for SSH (parse `systemctl list-units --output=json`,
      `systemctl list-timers`, edit `/etc/cron.d` files) and for Agent
      (call `systemd` D-Bus or shell out; cron via file distribution).
    - API under `/api/nodes/:id/systemd` and `/api/nodes/:id/cron`.
    - Frontend: per-node systemd unit table with start/stop/enable/disable,
      timers card, cron editor with diff preview.
    - Reuses existing audit log; every state-changing call records an entry.

18. [ ] Add OpenTelemetry tracing across transports.
    - Add `otel.go` in `internal/config`: exporter endpoint from
      `DG_OTEL_EXPORTER_OTLP_ENDPOINT`, sample ratio, service name; disabled
      when endpoint is empty.
    - Wrap `transport.Manager` with an otel-aware decorator that opens a span
      per `Exec`/`Upload`/`Ping`/`PTY`/`Metrics`, including node ID and
      transport kind (`ssh` | `agent-tunnel` | `agent-grpc`) as attributes.
    - Propagate trace context over the Agent gRPC tunnel via metadata so a
      single user action maps to a server-side and agent-side span tree.
    - For SSH, fall back to `traceparent` env var injection on the command
      line where reasonable; otherwise correlate by request ID in logs.
    - Optional: `/debug/trace/last/:taskID` summarizes the trace for the
      most recent deploy or playbook run.

## Working Rules

- Do not mark a feature complete without implementation and a relevant automated check.
- Update this tracker in the same change that completes, re-prioritizes, or removes a task.
- Keep secrets, certificates, runtime data, and build artifacts out of version control.
