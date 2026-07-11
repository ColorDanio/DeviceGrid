# DeviceGrid Task Tracker

Last reviewed: 2026-07-11

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
- [ ] The login screen advertises and pre-fills `admin/admin123`, but that credential
  is only valid when a new debug database is seeded without an override.
- [ ] Fleet views duplicate polling: the layout fetches nodes every 15s, Kanban
  fetches nodes every 30s plus every online node's metrics every 20s, and Nodes
  fetches nodes and metrics every 10s.
- [ ] Health checks launch one goroutine per managed node without a concurrency cap.
- [ ] The current production build has a 1.2 MB shared JavaScript chunk (365 KB gzip)
  and a 524 KB terminal codec chunk (139 KB gzip).
- [ ] Playwright UX audit: at a 390px viewport, the fixed desktop sidebar leaves too
  little workspace; page titles wrap vertically, header status pills crowd the title,
  and empty-state command buttons become narrow vertical labels.
- [ ] Playwright UX audit: the empty dashboard gives no route to "添加节点"; Docker
  and deployment identify their node prerequisite but do not link users to node setup.
- [ ] Playwright UX audit: icon-only controls, including the sidebar collapse and
  dashboard refresh buttons, have no accessible name in the browser accessibility tree.

## Next Priorities

1. [ ] Deliver a mobile-first application shell.
   - At narrow widths, replace the persistent sidebar with an accessible overlay drawer.
   - Keep the title, status summary, and primary commands on one readable line or
     explicitly stack them without clipping.
   - Verify dashboard, Nodes, Deploy, Docker, and Terminal at 390px and 768px.
2. [ ] Build an actionable first-fleet onboarding path.
   - Replace zero-value dashboard KPIs with a concise empty state and "添加节点" CTA.
   - Link Docker, Deploy, Terminal, and SFTP prerequisite states to node setup.
   - Split node onboarding into connect, verify, trust, and save steps with clear errors.
3. [ ] Add accessible names and tooltips to icon-only controls.
   - Cover navigation collapse, refresh, theme, close, and destructive icon actions.
   - Use Playwright accessibility snapshots to prevent regressions.
4. [ ] Make initial login guidance truthful and secure.
   - Do not pre-fill or advertise a default password after the first seeded account.
   - Display one-time setup guidance only when the server explicitly reports it.
   - Preserve a smooth local-development path without weakening production onboarding.
5. [ ] Complete the i18n rollout across feature views.
   - Migrate Nodes, Docker, Deploy, RKE2, Terminal, SFTP, Settings, Automation,
     SSH keys, Compare, and Audit to the locale catalog.
   - Cover both locales in frontend tests and keep API payloads locale-neutral.
6. [ ] Replace fleet polling with a shared, visibility-aware state source.
   - Subscribe Kanban and node status views to the existing WebSocket topics.
   - Share node and metric data between layout, Kanban, and Nodes.
   - Fall back to bounded polling only while the socket is disconnected or the tab is visible.
7. [ ] Bound health-check concurrency for larger fleets.
   - Add a configurable worker limit analogous to `node.metrics_concurrency`.
   - Ensure cancellation and shutdown do not leave blocked goroutines.
8. [ ] Establish a frontend performance budget and reduce initial payload.
   - Stop globally registering every Element Plus icon.
   - Measure route-level loading before changing terminal/xterm chunking.
   - Fail CI when first-load JavaScript or CSS exceeds the agreed budget.
9. [ ] Reconcile `PLAN.md` with the implemented code and this tracker.
   - Mark completed phases accurately.
   - Keep only verified remaining work in Phase 8.
10. [ ] Add frontend testing with Vitest and Vue Test Utils.
   - Start with auth store, API error handling, and a high-traffic view.
   - Add the test command to `web/package.json`, `Makefile`, and CI.
11. [ ] Add integration coverage for transport boundaries.
   - Exercise SSH execution and SFTP against an ephemeral target.
   - Exercise Agent gRPC tunnel request/response behavior.
12. [ ] Close remaining production features from `PLAN.md` Phase 8.
   - Agent-backed PTY terminal.
   - Docker operations through the Agent local API.
   - Configurable Docker and RKE2 mirror URLs.
13. [ ] Add release preflight checks.
   - Verify package startup, embedded frontend serving, and the Debian artifact.

## Working Rules

- Do not mark a feature complete without implementation and a relevant automated check.
- Update this tracker in the same change that completes, re-prioritizes, or removes a task.
- Keep secrets, certificates, runtime data, and build artifacts out of version control.
