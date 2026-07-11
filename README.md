# DeviceGrid

**A practical control plane for the servers you already run.**

DeviceGrid brings node inventory, live health data, remote access, Docker operations,
batch deployment, and RKE2 workflows into one self-hosted web application. Manage a
small lab or a growing fleet through SSH, then deploy lightweight agents where a
persistent mTLS connection is a better fit.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-42B883?logo=vuedotjs&logoColor=white)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-MIT-2563EB)](LICENSE)

## What You Can Do

- See fleet health, capacity, and connectivity from a single dashboard.
- Access servers with a browser terminal and built-in SFTP file manager.
- Manage Docker containers, images, networks, volumes, Compose projects, and logs.
- Run scripts and package installs across selected nodes with live output.
- Create and operate RKE2 clusters, including pre-flight checks and rolling upgrades.
- Automate recurring work, alerts, and audit trails with role-based access control.

## Get Started

### Run a release package

```bash
sudo dpkg -i devicegrid-*.deb
sudo systemctl enable --now devicegrid
```

Then open `http://<server-ip>:3000` and sign in with `admin` / `admin123`.
Change the default password immediately.

### Build from source

```bash
git clone https://github.com/ColorDanio/DeviceGrid.git
cd DeviceGrid
make package
./bin/devicegrid-server
```

`make package` builds the Vue application, embeds it in the server binary, and builds
the agent binaries. The default configuration listens on port `3000`.

## Choose a Connection Mode

| Mode | Best for |
| --- | --- |
| SSH | Getting started quickly or managing hosts without installed software |
| Agent | Persistent, mTLS-protected connections and lower SSH overhead |

Start with SSH. Once a node is trusted, deploy an agent from the node view to keep it
connected through DeviceGrid's gRPC tunnel.

## Documentation

- [Operations guide](docs/OPERATIONS.md): installation, configuration, development, and agent deployment.
- [Architecture](ARCHITECTURE.md): system components, transports, data flow, and security model.
- [Technical specification](SPEC.md): complete feature and API contract.
- [Project plan](PLAN.md): delivery roadmap.
- [Contributor guidance](AGENTS.md): repository conventions and validation commands.

## License

MIT. See [LICENSE](LICENSE).
