package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/transport"
)

type Manager struct {
	transport *transport.Manager
}

func NewManager(tm *transport.Manager) *Manager {
	return &Manager{transport: tm}
}

// dockerBin resolves the docker binary path on the remote node.
// Non-login SSH sessions often lack PATH entries like /usr/local/bin.
const dockerBin = `DPATH=""; for p in /usr/bin/docker /usr/local/bin/docker /snap/bin/docker; do [ -x "$p" ] && DPATH="$p" && break; done; [ -z "$DPATH" ] && DPATH=docker; `

func (m *Manager) IsInstalled(ctx context.Context, nodeID string) (bool, string, error) {
	result, err := m.transport.Exec(ctx, nodeID, dockerBin+`$DPATH --version 2>/dev/null`)
	if err != nil {
		return false, "", err
	}
	if result.ExitCode != 0 {
		return false, "", nil
	}
	version := strings.TrimSpace(result.Stdout)
	parts := strings.Fields(version)
	if len(parts) >= 3 {
		return true, strings.TrimSuffix(parts[2], ","), nil
	}
	return true, version, nil
}

func (m *Manager) Install(ctx context.Context, nodeID string, opts InstallOptions) (<-chan transport.StreamChunk, error) {
	script := m.buildInstallScript(opts)
	return m.transport.ExecStream(ctx, nodeID, script)
}

type InstallOptions struct {
	Mirror       string
	DataRoot     string
	InsecureReg  []string
}

func (m *Manager) buildInstallScript(opts InstallOptions) string {
	osCheck := `
OS_TYPE=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')
if [ "$OS_TYPE" = "ubuntu" ] || [ "$OS_TYPE" = "debian" ]; then
`
	aptScript := `
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg lsb-release
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/$(. /etc/os-release && echo "$ID")/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$(. /etc/os-release && echo "$ID") $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
`
	yumScript := `
  yum install -y -q yum-utils
  yum-config-manager -y --add-repo https://download.docker.com/linux/centos/docker-ce.repo
  yum install -y -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
`
	endCheck := `
else
  echo "Unsupported OS: $OS_TYPE" >&2
  exit 1
fi

systemctl enable docker
systemctl start docker
docker --version
`

	mirrorConfig := ""
	if opts.Mirror != "" {
		mirrorConfig = fmt.Sprintf(`
mkdir -p /etc/docker
cat > /etc/docker/daemon.json << 'EOF'
{
  "registry-mirrors": ["%s"],
  "data-root": "%s"
}
EOF
systemctl daemon-reload
systemctl restart docker
`, opts.Mirror, defaultStr(opts.DataRoot, "/var/lib/docker"))
	}

	return "set -e\n" + osCheck + aptScript + "elif [ \"$OS_TYPE\" = \"centos\" ] || [ \"$OS_TYPE\" = \"rhel\" ] || [ \"$OS_TYPE\" = \"rocky\" ]; then" + yumScript + endCheck + mirrorConfig
}

func (m *Manager) Uninstall(ctx context.Context, nodeID string) (<-chan transport.StreamChunk, error) {
	script := `set -e
OS_TYPE=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')
systemctl stop docker || true
systemctl disable docker || true
if [ "$OS_TYPE" = "ubuntu" ] || [ "$OS_TYPE" = "debian" ]; then
  apt-get purge -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras
  apt-get autoremove -y --purge
elif [ "$OS_TYPE" = "centos" ] || [ "$OS_TYPE" = "rhel" ] || [ "$OS_TYPE" = "rocky" ]; then
  yum remove -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
rm -rf /var/lib/docker /var/lib/containerd /etc/docker
echo "Docker uninstalled successfully"
`
	return m.transport.ExecStream(ctx, nodeID, script)
}

type ContainerInfo struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	Status string            `json:"status"`
	State  string            `json:"state"`
	Ports  []PortInfo        `json:"ports"`
}

type PortInfo struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (m *Manager) ListContainers(ctx context.Context, nodeID string, all bool) ([]ContainerInfo, error) {
	flag := ""
	if all {
		flag = "-a"
	}
	cmd := dockerBin + fmt.Sprintf(`$DPATH ps %s --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.Ports}}'`, flag)
	result, err := m.transport.Exec(ctx, nodeID, cmd)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	var containers []ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 5 {
			continue
		}
	c := ContainerInfo{
		ID:     parts[0],
		Name:   parts[1],
		Image:  parts[2],
		Status: parts[3],
		State:  parts[4],
	}
	if len(parts) > 5 && parts[5] != "" {
		for _, p := range strings.Split(parts[5], ", ") {
			p = strings.TrimSpace(p)
			if p != "" {
				c.Ports = append(c.Ports, PortInfo{HostPort: p, ContainerPort: p})
			}
		}
	}
	containers = append(containers, c)
	}
	return containers, nil
}

func (m *Manager) ContainerStats(ctx context.Context, nodeID, containerID string) (map[string]string, error) {
	cmd := dockerBin + fmt.Sprintf(`$DPATH stats %s --no-stream --format '{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.NetIO}}|{{.BlockIO}}|{{.PIDs}}'`, containerID)
	result, err := m.transport.Exec(ctx, nodeID, cmd)
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(result.Stdout)
	parts := strings.SplitN(line, "|", 6)
	stats := make(map[string]string)
	labels := []string{"cpu", "mem", "mem_pct", "net_io", "block_io", "pids"}
	for i, l := range labels {
		if i < len(parts) {
			stats[l] = strings.TrimSpace(parts[i])
		}
	}
	return stats, nil
}

func (m *Manager) ContainerAction(ctx context.Context, nodeID string, containerID string, action model.ContainerAction) (string, error) {
	result, err := m.transport.Exec(ctx, nodeID, fmt.Sprintf(dockerBin+`$DPATH %s %s`, action, containerID))
	if err != nil {
		return "", fmt.Errorf("container %s: %w", action, err)
	}
	if result.ExitCode != 0 {
		return result.Stderr, fmt.Errorf("container %s failed: %s", action, result.Stderr)
	}
	return result.Stdout, nil
}

func (m *Manager) CreateContainer(ctx context.Context, nodeID string, req CreateContainerRequest) (string, error) {
	cmd := dockerBin + fmt.Sprintf(`$DPATH run -d --name %s`, req.Name)
	for _, p := range req.Ports {
		cmd += fmt.Sprintf(" -p %s", p)
	}
	for k, v := range req.Env {
		cmd += fmt.Sprintf(" -e %s=%s", k, v)
	}
	for k, v := range req.Labels {
		cmd += fmt.Sprintf(" --label %s=%s", k, v)
	}
	if req.RestartPolicy != "" {
		cmd += fmt.Sprintf(" --restart %s", req.RestartPolicy)
	}
	cmd += fmt.Sprintf(" %s", req.Image)
	if len(req.Cmd) > 0 {
		cmd += " " + strings.Join(req.Cmd, " ")
	}

	result, err := m.transport.Exec(ctx, nodeID, cmd)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("create container failed: %s", result.Stderr)
	}
	return strings.TrimSpace(result.Stdout), nil
}

type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Ports         []string          `json:"ports"`
	Env           map[string]string `json:"env"`
	Labels        map[string]string `json:"labels"`
	RestartPolicy string            `json:"restart_policy"`
	Cmd           []string          `json:"cmd"`
}

type ImageInfo struct {
	ID      string `json:"id"`
	Tags    string `json:"tags"`
	Size    string `json:"size"`
	Created string `json:"created"`
}

func (m *Manager) ListImages(ctx context.Context, nodeID string) ([]ImageInfo, error) {
	result, err := m.transport.Exec(ctx, nodeID,
		dockerBin+`$DPATH images --format '{{.ID}}|{{.Repository}}:{{.Tag}}|{{.Size}}|{{.CreatedAt}}'`)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	var images []ImageInfo
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		images = append(images, ImageInfo{
			ID:      parts[0],
			Tags:    parts[1],
			Size:    parts[2],
			Created: parts[3],
		})
	}
	return images, nil
}

func (m *Manager) PullImage(ctx context.Context, nodeID, image string) (<-chan transport.StreamChunk, error) {
	return m.transport.ExecStream(ctx, nodeID, dockerBin+fmt.Sprintf(`$DPATH pull %s`, image))
}

func (m *Manager) RemoveImage(ctx context.Context, nodeID, imageID string, force bool) error {
	cmd := dockerBin + fmt.Sprintf(`$DPATH rmi %s`, imageID)
	if force {
		cmd += " -f"
	}
	_, err := m.transport.Exec(ctx, nodeID, cmd)
	return err
}

func (m *Manager) ComposeUp(ctx context.Context, nodeID, projectName, composeContent string) (<-chan transport.StreamChunk, error) {
	remotePath := fmt.Sprintf("/tmp/dg-compose-%s.yml", projectName)
	script := fmt.Sprintf(`cat > %s << 'COMPOSEEOF'
%s
COMPOSEEOF
`+dockerBin+`$DPATH compose -p %s -f %s up -d`, remotePath, composeContent, projectName, remotePath)
	return m.transport.ExecStream(ctx, nodeID, script)
}

func (m *Manager) ComposeDown(ctx context.Context, nodeID, projectName string) (<-chan transport.StreamChunk, error) {
	script := dockerBin + fmt.Sprintf(`$DPATH compose -p %s down --remove-orphans`, projectName)
	return m.transport.ExecStream(ctx, nodeID, script)
}

type NetworkInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Scope    string `json:"scope"`
	Subnet   string `json:"subnet"`
}

func (m *Manager) ListNetworks(ctx context.Context, nodeID string) ([]NetworkInfo, error) {
	result, err := m.transport.Exec(ctx, nodeID,
		dockerBin+`$DPATH network ls --format '{{.ID}}|{{.Name}}|{{.Driver}}|{{.Scope}}'`)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	var networks []NetworkInfo
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		networks = append(networks, NetworkInfo{
			ID: parts[0], Name: parts[1], Driver: parts[2], Scope: parts[3],
		})
	}
	return networks, nil
}

type VolumeInfo struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
}

func (m *Manager) ListVolumes(ctx context.Context, nodeID string) ([]VolumeInfo, error) {
	result, err := m.transport.Exec(ctx, nodeID,
		dockerBin+`$DPATH volume ls --format '{{.Name}}|{{.Driver}}|{{.Mountpoint}}'`)
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}

	var volumes []VolumeInfo
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		volumes = append(volumes, VolumeInfo{
			Name: parts[0], Driver: parts[1], Mountpoint: parts[2],
		})
	}
	return volumes, nil
}

func (m *Manager) Logs(ctx context.Context, nodeID, containerID string, follow bool, tail int) (<-chan transport.StreamChunk, error) {
	cmd := dockerBin + fmt.Sprintf(`$DPATH logs %s`, containerID)
	if follow {
		cmd += " -f"
	}
	if tail > 0 {
		cmd += fmt.Sprintf(" --tail %d", tail)
	}
	return m.transport.ExecStream(ctx, nodeID, cmd)
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
