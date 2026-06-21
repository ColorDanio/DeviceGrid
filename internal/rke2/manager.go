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
	ServerURL            string
	Token                string
	Version              string
	NodeName             string
	NodeLabels           []string
	NodeTaints           []string
	CNI                  string
	ClusterCIDR          string
	ServiceCIDR          string
	Disable              []string
	ETCDSnapshots       bool
	// Infrastructure
	SystemDefaultRegistry string   // mirror registry, auto-detected
	Proxy                 string   // HTTP_PROXY
	NoProxy               string   // NO_PROXY
	InstallMirror         string   // mirror for get.rke2.io script
}

// DefaultServerConfig generates RKE2 server config.yaml
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

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("token: %s\n", cfg.Token))
	sb.WriteString(fmt.Sprintf("cni: %s\n", cfg.CNI))
	sb.WriteString(fmt.Sprintf("cluster-cidr: %s\n", cfg.ClusterCIDR))
	sb.WriteString(fmt.Sprintf("service-cidr: %s\n", cfg.ServiceCIDR))

	if cfg.SystemDefaultRegistry != "" {
		sb.WriteString(fmt.Sprintf("system-default-registry: %s\n", cfg.SystemDefaultRegistry))
	}
	if cfg.ETCDSnapshots {
		sb.WriteString("etcd-snapshot-schedule-cron: \"0 */12 * * *\"\n")
		sb.WriteString("etcd-snapshot-retention: 14\n")
	}
	for _, d := range cfg.Disable {
		sb.WriteString(fmt.Sprintf("disable:\n  - %s\n", d))
	}
	if cfg.NodeName != "" {
		sb.WriteString(fmt.Sprintf("node-name: %s\n", cfg.NodeName))
	}
	for _, l := range cfg.NodeLabels {
		sb.WriteString(fmt.Sprintf("node-label:\n  - \"%s\"\n", l))
	}
	for _, t := range cfg.NodeTaints {
		sb.WriteString(fmt.Sprintf("node-taint:\n  - \"%s\"\n", t))
	}
	return sb.String()
}

// DefaultAgentConfig generates RKE2 agent config.yaml
func DefaultAgentConfig(serverURL, token string, cfg ClusterConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("server: %s\n", serverURL))
	sb.WriteString(fmt.Sprintf("token: %s\n", token))
	if cfg.SystemDefaultRegistry != "" {
		sb.WriteString(fmt.Sprintf("system-default-registry: %s\n", cfg.SystemDefaultRegistry))
	}
	if cfg.NodeName != "" {
		sb.WriteString(fmt.Sprintf("node-name: %s\n", cfg.NodeName))
	}
	for _, l := range cfg.NodeLabels {
		sb.WriteString(fmt.Sprintf("node-label:\n  - \"%s\"\n", l))
	}
	for _, t := range cfg.NodeTaints {
		sb.WriteString(fmt.Sprintf("node-taint:\n  - \"%s\"\n", t))
	}
	return sb.String()
}

// DetectAndApplyMirror checks node's geo data and auto-configures mirrors
func (m *Manager) DetectAndApplyMirror(ctx context.Context, nodeID string, cfg *ClusterConfig) {
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return
	}

	// Only auto-detect if not explicitly configured
	if cfg.SystemDefaultRegistry == "" {
		cfg.SystemDefaultRegistry = GetMirrorForCountry(node.CountryCode)
	}
}

// GetMirrorForCountry returns appropriate registry mirror based on country
func GetMirrorForCountry(countryCode string) string {
	switch strings.ToUpper(countryCode) {
	case "CN":
		// Mainland China — use Aliyun RKE2 mirror
		return "registry.cn-hangzhou.aliyuncs.com/rancher"
	case "HK", "TW", "MO":
		// HK/TW/MO — may need mirror for pull-through cache
		return "" // Let user decide, these regions usually have decent connectivity
	default:
		return "" // No mirror needed for other regions
	}
}

// IsChinaRegion checks if a country code is in the China region
func IsChinaRegion(countryCode string) bool {
	cc := strings.ToUpper(countryCode)
	return cc == "CN"
}

// buildInstallScript generates the full RKE2 install script with proxy/mirror support
func (m *Manager) buildInstallScript(installType, configContent string, cfg ClusterConfig) string {
	var envExports []string
	if cfg.Proxy != "" {
		envExports = append(envExports, fmt.Sprintf("export HTTP_PROXY=%s", cfg.Proxy))
		envExports = append(envExports, fmt.Sprintf("export HTTPS_PROXY=%s", cfg.Proxy))
		if cfg.NoProxy != "" {
			envExports = append(envExports, fmt.Sprintf("export NO_PROXY=%s", cfg.NoProxy))
		} else {
			envExports = append(envExports, "export NO_PROXY=127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.cluster.local")
		}
	}

	envSection := strings.Join(envExports, "\n")

	// Install command with optional version + mirror
	installCmd := "curl -sfL https://get.rke2.io"
	if cfg.InstallMirror != "" {
		// Use mirror endpoint for the install script itself
		installCmd = fmt.Sprintf("curl -sfL %s/rke2/install.sh", cfg.InstallMirror)
	}
	versionFlag := ""
	if cfg.Version != "" {
		versionFlag = fmt.Sprintf("INSTALL_RKE2_VERSION=%s", cfg.Version)
	}
	installCmd = fmt.Sprintf("%s | %s INSTALL_RKE2_TYPE=%s %s sh -",
		installCmd, versionFlag, installType,
		func() string { if versionFlag != "" { return "" }; return "" }())

	// Fix: construct install command properly
	if versionFlag != "" {
		installCmd = fmt.Sprintf("curl -sfL https://get.rke2.io | %s INSTALL_RKE2_TYPE=%s sh -", versionFlag, installType)
	} else {
		installCmd = fmt.Sprintf("curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE=%s sh -", installType)
	}

	systemdProxyConf := ""
	if cfg.Proxy != "" {
		noProxy := cfg.NoProxy
		if noProxy == "" {
			noProxy = "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.cluster.local"
		}
		systemdProxyConf = fmt.Sprintf(`
mkdir -p /etc/systemd/system/rke2-%s.service.d
cat > /etc/systemd/system/rke2-%s.service.d/proxy.conf << 'PROXYEOF'
[Service]
Environment=HTTP_PROXY=%s
Environment=HTTPS_PROXY=%s
Environment=NO_PROXY=%s
PROXYEOF
systemctl daemon-reload
`, installType, installType, cfg.Proxy, cfg.Proxy, noProxy)
	}

	return fmt.Sprintf(`set -e
%s

# Pre-install: ensure prerequisites
# 1. Disable swap (K8s requirement)
swapoff -a 2>/dev/null || true
cp /etc/fstab /etc/fstab.bak.$(date +%%s) 2>/dev/null || true
sed -i '/swap/d' /etc/fstab 2>/dev/null || true

# 2. Load kernel modules
modprobe overlay 2>/dev/null || true
modprobe br_netfilter 2>/dev/null || true
echo 'overlay' > /etc/modules-load.d/rke2.conf 2>/dev/null || true
echo 'br_netfilter' >> /etc/modules-load.d/rke2.conf 2>/dev/null || true

# 3. Sysctl settings
cat > /etc/sysctl.d/99-rke2.conf << 'SYSCTLEOF'
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward = 1
SYSCTLEOF
sysctl --system 2>/dev/null || true

# Write RKE2 config
mkdir -p /etc/rancher/rke2
cat > /etc/rancher/rke2/config.yaml << 'CONFIGEOF'
%s
CONFIGEOF

# Install RKE2
%s

# Systemd proxy override (if proxy configured)
%s

# Enable and start
systemctl enable rke2-%s
systemctl start rke2-%s

# Wait for service to be active
sleep 8
systemctl is-active rke2-%s
echo "RKE2 %s installed and started"
`, envSection, configContent, installCmd, systemdProxyConf,
		installType, installType, installType, installType)
}

func (m *Manager) InstallServer(ctx context.Context, nodeID, configContent string) (<-chan transport.StreamChunk, error) {
	// Auto-detect mirror based on node geo
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	cfg := ClusterConfig{}
	if err == nil {
		cfg.SystemDefaultRegistry = GetMirrorForCountry(node.CountryCode)
	}

	// If config already has mirror, respect it
	if strings.Contains(configContent, "system-default-registry") {
		cfg.SystemDefaultRegistry = ""
	}
	if cfg.SystemDefaultRegistry != "" {
		// Inject mirror into config
		configContent = fmt.Sprintf("system-default-registry: %s\n%s", cfg.SystemDefaultRegistry, configContent)
	}

	script := m.buildInstallScript("server", configContent, cfg)
	script += `
export PATH=$PATH:/var/lib/rancher/rke2/bin
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
kubectl get nodes 2>/dev/null || echo "Cluster still initializing..."
`
	return m.tm.ExecStream(ctx, nodeID, script)
}

func (m *Manager) InstallAgent(ctx context.Context, nodeID, configContent string) (<-chan transport.StreamChunk, error) {
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	cfg := ClusterConfig{}
	if err == nil {
		cfg.SystemDefaultRegistry = GetMirrorForCountry(node.CountryCode)
	}

	if strings.Contains(configContent, "system-default-registry") {
		cfg.SystemDefaultRegistry = ""
	}
	if cfg.SystemDefaultRegistry != "" {
		configContent = fmt.Sprintf("system-default-registry: %s\n%s", cfg.SystemDefaultRegistry, configContent)
	}

	script := m.buildInstallScript("agent", configContent, cfg)
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
			}
		}
		if ns.Role == "" {
			ns.Role = "agent"
		}
		nodes = append(nodes, ns)
	}
	return nodes, nil
}

func (m *Manager) GetClusterPods(ctx context.Context, nodeID, namespace string) ([]string, error) {
	nsFlag := "-A"
	if namespace != "" && namespace != "all" {
		nsFlag = "-n " + namespace
	}
	result, err := m.tm.Exec(ctx, nodeID,
		fmt.Sprintf(`export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get pods %s -o wide 2>/dev/null | head -50`, nsFlag))
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(result.Stdout), "\n"), nil
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
		script = `/usr/local/bin/rke2-uninstall.sh 2>/dev/null || /usr/bin/rke2-uninstall.sh 2>/dev/null || true
rm -rf /etc/rancher/rke2 /var/lib/rancher/rke2 /run/k3s/containerd
echo "RKE2 server uninstalled"`
	} else {
		script = `/usr/local/bin/rke2-uninstall.sh 2>/dev/null || /usr/bin/rke2-uninstall.sh 2>/dev/null || true
rm -rf /etc/rancher/rke2 /var/lib/rancher/rke2 /run/k3s/containerd
echo "RKE2 agent uninstalled"`
	}
	return m.tm.ExecStream(ctx, nodeID, script)
}

// Drift template import to avoid unused error
var _ = template.New
