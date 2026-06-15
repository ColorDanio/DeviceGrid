package rke2

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type Manager struct {
	repos repo.Repositories
	tm    *transport.Manager
}

func NewManager(repos repo.Repositories, tm *transport.Manager) *Manager {
	return &Manager{repos: repos, tm: tm}
}

type ClusterConfig struct {
	ServerURL    string
	Token        string
	NodeName     string
	NodeLabels   []string
	NodeTaints   []string
	CNI          string
	ClusterCIDR  string
	ServiceCIDR  string
	Disable      []string
	ETCDSnapshots bool
}

func DefaultServerConfig(cfg ClusterConfig) string {
	if cfg.CNI == "" {
		cfg.CNI = "canal"
	}
	if cfg.ClusterCIDR == "" {
		cfg.ClusterCIDR = "10.42.0.0/16"
	}
	if cfg.ServiceCIDR == "" {
		cfg.ServiceCIDR = "10.43.0.0/16"
	}

	tmpl := `server: {{.ServerURL}}
token: {{.Token}}
cni: {{.CNI}}
cluster-cidr: {{.ClusterCIDR}}
service-cidr: {{.ServiceCIDR}}
{{- if .ETCDSnapshots}}
etcd-snapshot-schedule-cron: "0 */12 * * *"
etcd-snapshot-retention: 14
{{- end}}
{{- range .Disable}}
disable:
  - {{.}}
{{- end}}
{{- if .NodeName}}
node-name: {{.NodeName}}
{{- end}}
{{- range .NodeLabels}}
node-label:
  - "{{.}}"
{{- end}}
`
	t, _ := template.New("config").Parse(tmpl)
	var sb strings.Builder
	t.Execute(&sb, cfg)
	return sb.String()
}

func DefaultAgentConfig(serverURL, token string, cfg ClusterConfig) string {
	tmpl := `server: {{.ServerURL}}
token: {{.Token}}
{{- if .NodeName}}
node-name: {{.NodeName}}
{{- end}}
{{- range .NodeLabels}}
node-label:
  - "{{.}}"
{{- end}}
`
	data := ClusterConfig{
		ServerURL:  serverURL,
		Token:      token,
		NodeName:   cfg.NodeName,
		NodeLabels: cfg.NodeLabels,
	}
	t, _ := template.New("agent").Parse(tmpl)
	var sb strings.Builder
	t.Execute(&sb, data)
	return sb.String()
}

func (m *Manager) InstallServer(ctx context.Context, nodeID, configContent string) (<-chan transport.StreamChunk, error) {
	script := fmt.Sprintf(`set -e
mkdir -p /etc/rancher/rke2
cat > /etc/rancher/rke2/config.yaml << 'CONFIGEOF'
%s
CONFIGEOF

curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE=server sh -
systemctl enable rke2-server
systemctl start rke2-server

sleep 5
export PATH=$PATH:/var/lib/rancher/rke2/bin
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
kubectl get nodes
`, configContent)

	return m.tm.ExecStream(ctx, nodeID, script)
}

func (m *Manager) InstallAgent(ctx context.Context, nodeID, configContent string) (<-chan transport.StreamChunk, error) {
	script := fmt.Sprintf(`set -e
mkdir -p /etc/rancher/rke2
cat > /etc/rancher/rke2/config.yaml << 'CONFIGEOF'
%s
CONFIGEOF

curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE=agent sh -
systemctl enable rke2-agent
systemctl start rke2-agent

echo "RKE2 agent installed and started"
`, configContent)

	return m.tm.ExecStream(ctx, nodeID, script)
}

func (m *Manager) GetServerToken(ctx context.Context, nodeID string) (string, error) {
	result, err := m.tm.Exec(ctx, nodeID, "cat /var/lib/rancher/rke2/server/node-token 2>/dev/null || echo ''")
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (m *Manager) GetServerURL(ctx context.Context, nodeID string) (string, error) {
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s:9345", node.Host), nil
}

type NodeStatus struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Version string `json:"version"`
	Ready   bool   `json:"ready"`
	Status  string `json:"status"`
}

func (m *Manager) GetClusterStatus(ctx context.Context, nodeID string) ([]NodeStatus, error) {
	result, err := m.tm.Exec(ctx, nodeID,
		`export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get nodes -o wide 2>/dev/null | tail -n +2`)
	if err != nil {
		return nil, fmt.Errorf("get cluster status: %w", err)
	}

	var nodes []NodeStatus
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ns := NodeStatus{
			Name:    fields[0],
			Status:  fields[1],
			Version: fields[4],
			Ready:   fields[1] == "Ready",
		}
		for _, f := range fields {
			if strings.Contains(f, "control-plane") || strings.Contains(f, "master") {
				ns.Role = "server"
			} else if f == "<none>" || f == "" {
				ns.Role = "agent"
			}
		}
		if ns.Role == "" {
			ns.Role = "agent"
		}
		nodes = append(nodes, ns)
	}
	return nodes, nil
}

func (m *Manager) GetCertInfo(ctx context.Context, nodeID string) (map[string]string, error) {
	result, err := m.tm.Exec(ctx, nodeID,
		`export PATH=$PATH:/var/lib/rancher/rke2/bin; kubectl get --kubeconfig=/etc/rancher/rke2/rke2.yaml -n kube-system secrets 2>/dev/null | head -20`)
	if err != nil {
		return nil, err
	}
	info := map[string]string{
		"raw": result.Stdout,
	}
	return info, nil
}

func (m *Manager) Upgrade(ctx context.Context, nodeID, version string) (<-chan transport.StreamChunk, error) {
	script := fmt.Sprintf(`set -e
export PATH=$PATH:/var/lib/rancher/rke2/bin
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml

NODE_NAME=$(hostname)
kubectl cordon $NODE_NAME 2>/dev/null || true
kubectl drain $NODE_NAME --ignore-daemonsets --delete-emptydir-data --force 2>/dev/null || true

curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=%s sh -
systemctl restart rke2-server

sleep 10
kubectl uncordon $NODE_NAME
echo "Node upgraded to %s"
`, version, version)

	return m.tm.ExecStream(ctx, nodeID, script)
}

func (m *Manager) Uninstall(ctx context.Context, nodeID string, role model.ClusterNodeRole) (<-chan transport.StreamChunk, error) {
	var script string
	if role == model.RoleServer {
		script = `/usr/bin/rke2-uninstall.sh || true
rm -rf /etc/rancher/rke2 /var/lib/rancher/rke2
echo "RKE2 server uninstalled"`
	} else {
		script = `/usr/bin/rke2-uninstall.sh agent || true
rm -rf /etc/rancher/rke2 /var/lib/rancher/rke2
echo "RKE2 agent uninstalled"`
	}

	return m.tm.ExecStream(ctx, nodeID, script)
}
