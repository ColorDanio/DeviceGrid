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
- [x] Refresh project entry-point documentation.
  - README is concise and product-focused.
  - Operations and configuration detail lives in `docs/OPERATIONS.md`.

## Current Baseline

- [x] Core SSH and agent transport, node management, Docker, deployment, RKE2,
  terminal, SFTP, user management, alerts, and audit APIs are present.
- [x] Production packaging and GitHub release workflow are present.
- [x] Backend unit tests cover authentication, crypto, SSH pooling, and SQLite repos.
- [ ] Frontend has no unit-test runner or component tests.
- [ ] `make lint-web` and `make typecheck-web` reference missing npm scripts.
- [ ] No integration suite validates SSH, Agent gRPC, Docker, or RKE2 workflows.
- [ ] Backend coverage has not been measured in the current toolchain because its
  `covdata` tool is unavailable; rerun with the Go version declared in `go.mod`.

## Next Priorities

1. [ ] Restore the documented frontend validation commands.
   - Add and validate `lint` and `typecheck` npm scripts, or remove the Makefile targets.
   - Make the CI frontend validation command call the package script directly.
2. [ ] Reconcile `PLAN.md` with the implemented code and this tracker.
   - Mark completed phases accurately.
   - Keep only verified remaining work in Phase 8.
3. [ ] Add frontend testing with Vitest and Vue Test Utils.
   - Start with auth store, API error handling, and a high-traffic view.
   - Add the test command to `web/package.json`, `Makefile`, and CI.
4. [ ] Add integration coverage for transport boundaries.
   - Exercise SSH execution and SFTP against an ephemeral target.
   - Exercise Agent gRPC tunnel request/response behavior.
5. [ ] Close remaining production features from `PLAN.md` Phase 8.
   - Agent-backed PTY terminal.
   - Docker operations through the Agent local API.
   - Configurable Docker and RKE2 mirror URLs.
6. [ ] Add release preflight checks.
   - Verify package startup, embedded frontend serving, and the Debian artifact.

## Working Rules

- Do not mark a feature complete without implementation and a relevant automated check.
- Update this tracker in the same change that completes, re-prioritizes, or removes a task.
- Keep secrets, certificates, runtime data, and build artifacts out of version control.
