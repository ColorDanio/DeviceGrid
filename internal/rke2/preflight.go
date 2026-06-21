package rke2

import (
	"context"
	"fmt"
	"strings"
)

// PreFlightCheckResult holds the result of a single pre-flight check
type PreFlightCheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Value    string `json:"value"`
	Required string `json:"required"`
	Warning  string `json:"warning,omitempty"`
	Fixed    bool   `json:"fixed,omitempty"`
}

// PreFlightResult is the complete pre-flight check result
type PreFlightResult struct {
	NodeID   string                `json:"node_id"`
	NodeName string                `json:"node_name"`
	AllPassed bool                 `json:"all_passed"`
	Checks   []PreFlightCheckResult `json:"checks"`
}

// PreFlightCheck runs hardware and system configuration checks on a node before RKE2 installation
func (m *Manager) PreFlightCheck(ctx context.Context, nodeID string, autoFix bool) (*PreFlightResult, error) {
	result := &PreFlightResult{NodeID: nodeID}

	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	result.NodeName = node.Name

	// Run all checks in one SSH command for efficiency
	script := `echo "===PREFLIGHT==="
# CPU cores
CPU_CORES=$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo)
echo "CPU_CORES=$CPU_CORES"

# Memory (MB)
MEM_MB=$(awk '/MemTotal/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null)
echo "MEM_MB=$MEM_MB"

# Disk space on / (GB)
DISK_GB=$(df -BG / 2>/dev/null | tail -1 | awk '{print $2}' | tr -d 'G')
echo "DISK_GB=$DISK_GB"

# Swap status
SWAP_TOTAL=$(awk '/SwapTotal/{print $2}' /proc/meminfo 2>/dev/null)
echo "SWAP_TOTAL=$SWAP_TOTAL"

# Swap enabled in fstab
SWAP_FSTAB=$(grep -c swap /etc/fstab 2>/dev/null || echo 0)
echo "SWAP_FSTAB=$SWAP_FSTAB"

# Architecture
ARCH=$(uname -m)
echo "ARCH=$ARCH"

# OS type
OS_ID=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')
echo "OS_ID=$OS_ID"

# Kernel version
KERNEL=$(uname -r)
echo "KERNEL=$KERNEL"

# Required kernel modules
MOD_OVERLAY=$(lsmod 2>/dev/null | grep -c overlay || echo 0)
echo "MOD_OVERLAY=$MOD_OVERLAY"
MOD_BRIDGE=$(lsmod 2>/dev/null | grep -c br_netfilter || echo 0)
echo "MOD_BRIDGE=$MOD_BRIDGE"

# sysctl settings
IP_FORWARD=$(cat /proc/sys/net/ipv4/ip_forward 2>/dev/null || echo 0)
echo "IP_FORWARD=$IP_FORWARD"
BRIDGE_NF=$(cat /proc/sys/net/bridge/bridge-nf-call-iptables 2>/dev/null || echo 0)
echo "BRIDGE_NF=$BRIDGE_NF"

# Existing RKE2 installation
RKE2_EXISTS=$(systemctl is-enabled rke2-server rke2-agent 2>/dev/null | head -1 || echo "none")
echo "RKE2_EXISTS=$RKE2_EXISTS"

# Port availability (6443, 9345, 8080, 8443)
PORT_6443=$(ss -tlnp 2>/dev/null | grep -c ':6443 ' || echo 0)
echo "PORT_6443=$PORT_6443"
PORT_9345=$(ss -tlnp 2>/dev/null | grep -c ':9345 ' || echo 0)
echo "PORT_9345=$PORT_9345"

echo "===END==="
`

	execResult, err := m.tm.Exec(ctx, nodeID, script)
	if err != nil {
		return nil, fmt.Errorf("preflight exec: %w", err)
	}

	// Parse results
	values := parsePreFlightOutput(execResult.Stdout)

	// Evaluate checks
	checks := []PreFlightCheckResult{}

	// 1. CPU cores (minimum 2, recommended 4)
	cpuCores := parseInt(values["CPU_CORES"])
	cpuCheck := PreFlightCheckResult{
		Name:     "CPU 核心数",
		Value:    fmt.Sprintf("%d 核", cpuCores),
		Required: "最少 2 核（推荐 4 核以上）",
		Passed:   cpuCores >= 2,
	}
	if cpuCores < 2 {
		cpuCheck.Warning = "CPU 核心数不足，可能导致集群不稳定"
	}
	checks = append(checks, cpuCheck)

	// 2. Memory (minimum 4GB / 3840MB, recommended 8GB)
	memMB := parseInt(values["MEM_MB"])
	memCheck := PreFlightCheckResult{
		Name:     "内存大小",
		Value:    fmt.Sprintf("%.1f GB", float64(memMB)/1024),
		Required: "最少 4GB（推荐 8GB 以上）",
		Passed:   memMB >= 3840,
	}
	if memMB < 3840 {
		memCheck.Warning = "内存不足，可能导致 Pod 无法调度或 OOM"
	} else if memMB < 7680 {
		memCheck.Warning = "内存偏少，仅适合测试环境"
	}
	checks = append(checks, memCheck)

	// 3. Disk space (minimum 20GB)
	diskGB := parseInt(values["DISK_GB"])
	diskCheck := PreFlightCheckResult{
		Name:     "磁盘空间",
		Value:    fmt.Sprintf("%d GB", diskGB),
		Required: "最少 20GB",
		Passed:   diskGB >= 20,
	}
	if diskGB < 20 {
		diskCheck.Warning = "磁盘空间不足，etcd 和镜像存储需要充足空间"
	}
	checks = append(checks, diskCheck)

	// 4. Swap — must be disabled for K8s
	swapTotal := parseInt(values["SWAP_TOTAL"])
	swapFstab := parseInt(values["SWAP_FSTAB"])
	swapCheck := PreFlightCheckResult{
		Name:     "Swap 分区",
		Value:    "已禁用",
		Required: "K8s 要求禁用 swap",
		Passed:   swapTotal == 0,
	}
	if swapTotal > 0 || swapFstab > 0 {
		swapCheck.Passed = false
		swapCheck.Value = fmt.Sprintf("已启用 (%d KB)", swapTotal)
		swapCheck.Warning = "Swap 需要禁用，K8s kubelet 在 swap 启用时拒绝启动"

		if autoFix {
			// Auto-disable swap
			fixScript := `# Disable swap immediately
swapoff -a 2>/dev/null || true
# Remove swap from fstab (backup first)
cp /etc/fstab /etc/fstab.bak.$(date +%s) 2>/dev/null
sed -i '/swap/d' /etc/fstab 2>/dev/null
# Verify
SWAP_CHECK=$(awk '/SwapTotal/{print $2}' /proc/meminfo)
echo "SWAP_AFTER=$SWAP_CHECK"
`
			fixResult, err := m.tm.Exec(ctx, nodeID, fixScript)
			if err == nil {
				fixValues := parsePreFlightOutput(fixResult.Stdout)
				if parseInt(fixValues["SWAP_AFTER"]) == 0 {
					swapCheck.Passed = true
					swapCheck.Value = "已自动禁用"
					swapCheck.Fixed = true
				} else {
					swapCheck.Warning = "自动禁用失败，请手动执行: swapoff -a && sed -i '/swap/d' /etc/fstab"
				}
			}
		}
	}
	checks = append(checks, swapCheck)

	// 5. Kernel modules
	modOverlay := parseInt(values["MOD_OVERLAY"])
	modBridge := parseInt(values["MOD_BRIDGE"])
	modCheck := PreFlightCheckResult{
		Name:     "内核模块 (overlay/br_netfilter)",
		Value:    fmt.Sprintf("overlay=%s br_netfilter=%s", boolStr(modOverlay > 0), boolStr(modBridge > 0)),
		Required: "需要加载 overlay 和 br_netfilter",
		Passed:   modOverlay > 0 && modBridge > 0,
	}
	if !(modOverlay > 0 && modBridge > 0) {
		if autoFix {
			m.tm.Exec(ctx, nodeID, "modprobe overlay 2>/dev/null; modprobe br_netfilter 2>/dev/null; echo 'overlay' >> /etc/modules-load.d/rke2.conf; echo 'br_netfilter' >> /etc/modules-load.d/rke2.conf")
			modCheck.Passed = true
			modCheck.Value = "已自动加载"
			modCheck.Fixed = true
		} else {
			modCheck.Warning = "缺少内核模块，安装时会自动加载"
		}
	}
	checks = append(checks, modCheck)

	// 6. Sysctl settings
	ipForward := parseInt(values["IP_FORWARD"])
	bridgeNF := parseInt(values["BRIDGE_NF"])
	sysctlCheck := PreFlightCheckResult{
		Name:     "内核网络参数",
		Value:    fmt.Sprintf("ip_forward=%d bridge-nf=%d", ipForward, bridgeNF),
		Required: "ip_forward=1, bridge-nf-call-iptables=1",
		Passed:   ipForward == 1 && bridgeNF == 1,
	}
	if !(ipForward == 1 && bridgeNF == 1) {
		if autoFix {
			m.tm.Exec(ctx, nodeID, `echo 'net.ipv4.ip_forward = 1' > /etc/sysctl.d/99-rke2.conf; echo 'net.bridge.bridge-nf-call-iptables = 1' >> /etc/sysctl.d/99-rke2.conf; sysctl --system 2>/dev/null`)
			sysctlCheck.Passed = true
			sysctlCheck.Value = "已自动配置"
			sysctlCheck.Fixed = true
		} else {
			sysctlCheck.Warning = "参数未配置，安装时会自动设置"
		}
	}
	checks = append(checks, sysctlCheck)

	// 7. Existing RKE2
	rke2Exists := values["RKE2_EXISTS"]
	rke2Check := PreFlightCheckResult{
		Name:     "RKE2 已安装",
		Value:    rke2Exists,
		Required: "不应已安装",
		Passed:   rke2Exists == "none" || rke2Exists == "disabled" || rke2Exists == "",
	}
	if !rke2Check.Passed {
		rke2Check.Warning = "节点上已安装 RKE2，重复安装可能导致冲突"
	}
	checks = append(checks, rke2Check)

	// 8. Port availability
	port6443 := parseInt(values["PORT_6443"])
	port9345 := parseInt(values["PORT_9345"])
	portCheck := PreFlightCheckResult{
		Name:     "端口可用性 (6443/9345)",
		Value:    fmt.Sprintf("6443=%s 9345=%s", boolStr(port6443 == 0), boolStr(port9345 == 0)),
		Required: "6443 和 9345 端口需空闲",
		Passed:   port6443 == 0 && port9345 == 0,
	}
	if port6443 > 0 || port9345 > 0 {
		portCheck.Warning = "端口被占用，可能已有其他 K8s 服务运行"
	}
	checks = append(checks, portCheck)

	// Overall
	result.AllPassed = true
	for _, c := range checks {
		if !c.Passed {
			result.AllPassed = false
			break
		}
	}
	result.Checks = checks
	return result, nil
}

func parsePreFlightOutput(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func boolStr(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}
