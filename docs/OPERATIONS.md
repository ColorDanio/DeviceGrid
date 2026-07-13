# DeviceGrid Operations Guide

This guide covers installation, configuration, development, and agent deployment.
For system design, see [Architecture](../ARCHITECTURE.md). For the full product
contract, see [Technical Specification](../SPEC.md).

## Requirements

- Go 1.25 or later
- Node.js 22 or later
- npm with the lockfile-supported version
- Redis for queued batch tasks when that capability is enabled
- MongoDB only when `database.driver` is set to `mongodb`

## Installation

### Debian or Ubuntu package

```bash
sudo dpkg -i devicegrid-*.deb
sudo systemctl enable --now devicegrid
sudo systemctl status devicegrid
```

The package stores its configuration at `/etc/devicegrid/config.yaml` and its runtime
data under `/var/lib/devicegrid/data`. The server listens on port `3000` in the bundled
configuration.

### Build from source

```bash
git clone https://github.com/ColorDanio/DeviceGrid.git
cd DeviceGrid
make package
./bin/devicegrid-server
```

`make package` builds and embeds the frontend, builds the server, and cross-compiles
Linux agent binaries for amd64 and arm64.

## Configuration

Copy or edit `configs/config.yaml` for local deployments. Environment variables use
the `DG_` prefix, for example:

```bash
export DG_SERVER_PORT=3000
export DG_AUTH_JWT_SECRET='replace-with-a-random-secret'
export DG_CRYPTO_MASTER_KEY='replace-with-a-32-byte-hex-key'
./bin/devicegrid-server
```

Set `server.mode` to `release` for production. Release mode requires both
`DG_AUTH_JWT_SECRET` and `DG_CRYPTO_MASTER_KEY`.

| Setting | Default | Purpose |
| --- | --- | --- |
| `server.port` | `3000` | HTTP listener port in the bundled configuration |
| `database.driver` | `sqlite` | Select `sqlite` or `mongodb` |
| `database.sqlite.path` | `./data/device_grid.db` | SQLite data file |
| `agent.grpc_port` | `9090` | Agent tunnel listener |
| `ssh.max_connections` | `50` | SSH pool limit per node |
| `node.health_check_concurrency` | `3` | Maximum simultaneous node health checks |
| `deploy.max_concurrent` | `20` | Parallel deployment worker limit |
| `network.environment` | `public` | Use `internal` to disable public-network diagnostics |

Never commit production secrets, certificates, or database files. See
[`configs/config.yaml`](../configs/config.yaml) for all configuration keys.

## Agent Deployment

After SSH access is working, use the **Agent** action from a node view to upload the
matching binary and register its systemd service. For a manual deployment:

```bash
./devicegrid-agent -server <server-ip>:9090 \
  -node-id <unique-id> \
  -node-name <display-name>
```

To verify the server certificate, add `-ca-cert /path/to/ca.crt`. The agent establishes
a persistent gRPC connection protected by mTLS when certificates are configured.

## Development

Run the backend and frontend in separate terminals:

```bash
make dev-server
make dev-web
```

The frontend development server runs on port `5173`. The backend uses the port from
`configs/config.yaml` unless overridden.

Useful validation commands:

```bash
make test
make lint
npm run build --prefix web
```

## Packaging

```bash
make package
```

Output files:

- `bin/devicegrid-server`
- `bin/devicegrid-agent`
- `dist/agent-linux-amd64`
- `dist/agent-linux-arm64`
