package rke2

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/michael/device_grid/internal/transport"
)

var (
	helmNameRe  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/]*$`)
	helmValueRe = regexp.MustCompile(`^[a-zA-Z0-9._\-/=]+$`)
)

func sanitizeForShell(s string) string {
	for _, ch := range s {
		switch ch {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\n', '\r', '\\', '"', '\'', '!', '#':
			return ""
		}
	}
	return s
}

// HelmInstall installs a Helm chart on the RKE2 cluster
func (m *Manager) HelmInstall(ctx context.Context, serverNodeID string, repoName, repoURL, chartName, namespace string, values map[string]string) (<-chan transport.StreamChunk, error) {
	// Validate all inputs
	if !helmNameRe.MatchString(chartName) {
		return nil, fmt.Errorf("invalid chart name")
	}
	if repoName != "" && !helmNameRe.MatchString(repoName) {
		return nil, fmt.Errorf("invalid repo name")
	}
	if namespace != "" && !helmNameRe.MatchString(namespace) {
		return nil, fmt.Errorf("invalid namespace")
	}
	for k, v := range values {
		if sanitizeForShell(k) == "" || sanitizeForShell(v) == "" {
			return nil, fmt.Errorf("invalid helm value: %s", k)
		}
	}

	var cmd strings.Builder
	cmd.WriteString("export PATH=$PATH:/var/lib/rancher/rke2/bin\n")
	cmd.WriteString("export KUBECONFIG=/etc/rancher/rke2/rke2.yaml\n\n")

	if repoName != "" && repoURL != "" {
		cmd.WriteString(fmt.Sprintf("helm repo add %s %s 2>/dev/null || true\n", repoName, repoURL))
		cmd.WriteString("helm repo update\n")
	}

	cmd.WriteString(fmt.Sprintf("helm install %s %s", chartName, chartName))
	if namespace != "" {
		cmd.WriteString(fmt.Sprintf(" -n %s --create-namespace", namespace))
	}
	for k, v := range values {
		cmd.WriteString(fmt.Sprintf(" --set %s=%s", k, v))
	}
	cmd.WriteString(" || helm upgrade --install")
	if namespace != "" {
		cmd.WriteString(fmt.Sprintf(" -n %s", namespace))
	}
	cmd.WriteString(fmt.Sprintf(" %s %s", chartName, chartName))
	for k, v := range values {
		cmd.WriteString(fmt.Sprintf(" --set %s=%s", k, v))
	}
	cmd.WriteString("\n")

	return m.tm.ExecStream(ctx, serverNodeID, cmd.String())
}

// HelmList lists installed Helm releases
func (m *Manager) HelmList(ctx context.Context, serverNodeID string) (string, error) {
	result, err := m.tm.Exec(ctx, serverNodeID,
		`export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; helm list -A 2>/dev/null`)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// HelmUninstall removes a Helm release
func (m *Manager) HelmUninstall(ctx context.Context, serverNodeID, releaseName, namespace string) (<-chan transport.StreamChunk, error) {
	nsFlag := ""
	if namespace != "" {
		nsFlag = fmt.Sprintf("-n %s", namespace)
	}
	script := fmt.Sprintf(`export PATH=$PATH:/var/lib/rancher/rke2/bin
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
helm uninstall %s %s || true
echo "Release %s uninstalled"`, releaseName, nsFlag, releaseName)
	return m.tm.ExecStream(ctx, serverNodeID, script)
}

// InstallRancher installs Rancher Manager on the cluster via Helm
func (m *Manager) InstallRancher(ctx context.Context, serverNodeID, hostname, password string, useMirror bool) (<-chan transport.StreamChunk, error) {
	// Validate inputs
	if sanitizeForShell(hostname) == "" {
		return nil, fmt.Errorf("invalid hostname")
	}
	if sanitizeForShell(password) == "" {
		return nil, fmt.Errorf("invalid password")
	}
	var cmd strings.Builder
	cmd.WriteString("export PATH=$PATH:/var/lib/rancher/rke2/bin\n")
	cmd.WriteString("export KUBECONFIG=/etc/rancher/rke2/rke2.yaml\n\n")

	cmd.WriteString("helm repo add rancher-stable https://releases.rancher.com/server-charts/stable 2>/dev/null || true\n")
	cmd.WriteString("helm repo update\n\n")

	cmd.WriteString("helm install rancher rancher-stable/rancher \\\n")
	cmd.WriteString("  --namespace cattle-system \\\n")
	cmd.WriteString("  --create-namespace \\\n")
	cmd.WriteString(fmt.Sprintf("  --set hostname=%s \\\n", hostname))
	cmd.WriteString("  --set replicas=1 \\\n")
	cmd.WriteString(fmt.Sprintf("  --set bootstrapPassword=%s \\\n", password))
	if useMirror {
		cmd.WriteString("  --set systemDefaultRegistry=registry.cn-hangzhou.aliyuncs.com \\\n")
	}
	cmd.WriteString("  --set ingress.tls.source=secret \\\n")
	cmd.WriteString("  || helm upgrade --install rancher rancher-stable/rancher \\\n")
	cmd.WriteString("    --namespace cattle-system \\\n")
	cmd.WriteString(fmt.Sprintf("    --set hostname=%s \\\n", hostname))
	cmd.WriteString("    --set replicas=1 \\\n")
	cmd.WriteString(fmt.Sprintf("    --set bootstrapPassword=%s\n", password))

	return m.tm.ExecStream(ctx, serverNodeID, cmd.String())
}

// GetRancherStatus checks if Rancher is installed
func (m *Manager) GetRancherStatus(ctx context.Context, serverNodeID string) (installed bool, version string, err error) {
	result, err := m.tm.Exec(ctx, serverNodeID,
		`export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; helm list -n cattle-system 2>/dev/null | grep rancher || echo "not installed"`)
	if err != nil {
		return false, "", err
	}
	output := strings.TrimSpace(result.Stdout)
	if strings.Contains(output, "not installed") || output == "" {
		return false, "", nil
	}
	// Parse version from helm list output
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "rancher") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return true, fields[1], nil
			}
		}
	}
	return true, "unknown", nil
}
