package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (m *Manager) Exec(ctx context.Context, nodeID string, cmd string) (ExecResult, error) {
	result, err := m.execWithRetry(ctx, nodeID, cmd, 2)
	if err != nil {
		return ExecResult{}, err
	}
	return result, nil
}

// execWithRetry tries to exec, and on connection failure, discards the
// stale connection and retries with a fresh one
func (m *Manager) execWithRetry(ctx context.Context, nodeID string, cmd string, maxRetries int) (ExecResult, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		client, err := m.getClient(ctx, nodeID)
		if err != nil {
			lastErr = err
			continue
		}

		session, err := client.NewSession()
		if err != nil {
			// Stale connection — close it (don't release to pool)
			client.Close()
			lastErr = fmt.Errorf("new session (stale?): %w", err)
			continue
		}

		var stdout, stderr bytes.Buffer
		session.Stdout = &stdout
		session.Stderr = &stderr

		err = session.Run(cmd)
		session.Close()

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				// Command exited with non-zero — still a good connection
				m.releaseClient(nodeID, client)
				return ExecResult{
					Stdout:   stdout.String(),
					Stderr:   stderr.String(),
					ExitCode: exitErr.ExitStatus(),
				}, nil
			}
			// Connection-level error — don't return to pool
			client.Close()
			lastErr = fmt.Errorf("exec: %w", err)
			continue
		}

		// Success — return client to pool
		m.releaseClient(nodeID, client)
		return ExecResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: 0,
		}, nil
	}
	return ExecResult{}, lastErr
}

type StreamChunk struct {
	Type     string
	Data     string
	ExitCode int
}

func (m *Manager) ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan StreamChunk, error) {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		// Stale connection — close, don't release to pool
		client.Close()
		return nil, fmt.Errorf("new session: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, err
	}

	if err := session.Start(cmd); err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, fmt.Errorf("start command: %w", err)
	}

	ch := make(chan StreamChunk, 100)

	go func() {
		defer session.Close()
		defer close(ch)

		// Read stdout/stderr until the command finishes
		done := make(chan struct{})
		go func() {
			buf := make([]byte, 8192)
			for {
				n, err := stdoutPipe.Read(buf)
				if n > 0 {
					ch <- StreamChunk{Type: "stdout", Data: string(buf[:n])}
				}
				if err != nil {
					break
				}
			}
			// Read stderr
			for {
				n, err := stderrPipe.Read(buf)
				if n > 0 {
					ch <- StreamChunk{Type: "stderr", Data: string(buf[:n])}
				}
				if err != nil {
					break
				}
			}
			close(done)
		}()

		// Wait for completion OR context cancellation
		select {
		case <-done:
		case <-ctx.Done():
			// Context cancelled — kill the remote process via signal
			_ = session.Signal(ssh.SIGTERM)
			// Brief wait for graceful exit
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = session.Signal(ssh.SIGKILL)
			}
		}

		exitCode := 0
		err := session.Wait()
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			}
		}
		ch <- StreamChunk{Type: "exit", ExitCode: exitCode}

		// Release client back to pool ONLY if the connection is still good
		if ctx.Err() == nil {
			m.releaseClient(nodeID, client)
		} else {
			// Context was cancelled — connection may be in bad state
			client.Close()
		}
	}()

	return ch, nil
}

func (m *Manager) Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	w, err := session.StdinPipe()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("scp -t %q", remotePath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("start scp: %w", err)
	}

	header := fmt.Sprintf("C%04o %d %s\n", mode, 0, remotePath[strings.LastIndex(remotePath, "/")+1:])
	if _, err := fmt.Fprint(w, header); err != nil {
		return err
	}

	if _, err := io.Copy(w, content); err != nil {
		return err
	}

	fmt.Fprint(w, "\x00")
	w.Close()

	return session.Wait()
}

func (m *Manager) Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error) {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer m.releaseClient(nodeID, client)

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	var buf bytes.Buffer
	session.Stdout = &buf
	if err := session.Run(fmt.Sprintf("cat %q", remotePath)); err != nil {
		session.Close()
		return nil, fmt.Errorf("download: %w", err)
	}

	session.Close()
	return io.NopCloser(&buf), nil
}

func (m *Manager) Ping(ctx context.Context, nodeID string) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	return session.Run("echo ok")
}

func (m *Manager) Facts(ctx context.Context, nodeID string) (map[string]string, error) {
	result, err := m.Exec(ctx, nodeID, `echo "OS=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')"
echo "OS_VERSION=$(grep '^VERSION_ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')"
echo "ARCH=$(uname -m)"
echo "KERNEL=$(uname -r)"
DOCKER_PATH=$(which docker 2>/dev/null || command -v docker 2>/dev/null)
if [ -n "$DOCKER_PATH" ]; then
  echo "DOCKER=$($DOCKER_PATH --version 2>/dev/null | sed 's/Docker version //;s/,.*//')"
else
  echo "DOCKER="
fi
echo "RKE2=$(/usr/local/bin/rke2 --version 2>/dev/null | awk '{print $3}' | head -1)"`)
	if err != nil {
		return nil, err
	}

	facts := make(map[string]string)
	for _, line := range strings.Split(result.Stdout, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			facts[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	return facts, nil
}

func (m *Manager) Metrics(ctx context.Context, nodeID string) (*MetricsResult, error) {
	result, err := m.Exec(ctx, nodeID, metricsScript)
	if err != nil {
		return nil, err
	}
	return parseMetrics(result.Stdout), nil
}

type MetricsResult struct {
	CPUUsage   float64   `json:"cpu_usage"`
	CPUModel   string    `json:"cpu_model"`
	CPUCores   int       `json:"cpu_cores"`
	CPUSockets int       `json:"cpu_sockets"`
	CPUThreads int       `json:"cpu_threads"`
	LoadAvg1   float64   `json:"load_avg_1"`
	LoadAvg5   float64   `json:"load_avg_5"`
	LoadAvg15  float64   `json:"load_avg_15"`
	MemTotal   uint64    `json:"mem_total"`
	MemUsed    uint64    `json:"mem_used"`
	SwapTotal  uint64    `json:"swap_total"`
	SwapUsed   uint64    `json:"swap_used"`
	DiskTotal  uint64    `json:"disk_total"`
	DiskUsed   uint64    `json:"disk_used"`
	VirtType   string    `json:"virt_type"`
	NetIface   string    `json:"net_iface"`
	NetRx      uint64    `json:"net_rx"`
	NetTx      uint64    `json:"net_tx"`
	Uptime     uint64    `json:"uptime"`
	GPUs       []GPUInfo `json:"gpus"`
}

type GPUInfo struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	Utilization float64 `json:"utilization"`
	Temperature int     `json:"temperature"`
}

const metricsScript = `echo "===METRICS==="
# CPU usage: read /proc/stat + /proc/uptime for a ratio without sleep
# Use load average as proxy for current pressure (no 1s delay)
CPU_TOTAL=$(head -1 /proc/stat | awk '{s=0;for(i=2;i<=NF;i++)s+=$i;print s}')
CPU_IDLE=$(head -1 /proc/stat | awk '{print $5}')
CPU_USAGE=$(awk "BEGIN{u=$CPU_TOTAL-$CPU_IDLE; if($CPU_TOTAL>0) printf \"%.2f\", u/$CPU_TOTAL*100; else print \"0.00\"}")
echo "CPU_USAGE=$CPU_USAGE"

# CPU hardware info
CPU_MODEL=$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2 | sed 's/^ *//')
echo "CPU_MODEL=$CPU_MODEL"
CPU_CORES=$(lscpu 2>/dev/null | grep '^Core(s) per socket' | awk '{print $NF}')
[ -z "$CPU_CORES" ] && CPU_CORES=$(grep -c '^processor' /proc/cpuinfo 2>/dev/null)
echo "CPU_CORES=$CPU_CORES"
CPU_SOCKETS=$(lscpu 2>/dev/null | grep '^Socket(s)' | awk '{print $NF}')
[ -z "$CPU_SOCKETS" ] && CPU_SOCKETS=1
echo "CPU_SOCKETS=$CPU_SOCKETS"
CPU_THREADS=$(grep -c '^processor' /proc/cpuinfo 2>/dev/null)
echo "CPU_THREADS=$CPU_THREADS"

# Load average
read -r LOAD1 LOAD5 LOAD15 _ < /proc/loadavg
echo "LOAD_1=$LOAD1"
echo "LOAD_5=$LOAD5"
echo "LOAD_15=$LOAD15"

# Memory (in KB)
MEM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print $2}')
MEM_AVAIL=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
SWAP_TOTAL=$(grep SwapTotal /proc/meminfo | awk '{print $2}')
SWAP_FREE=$(grep SwapFree /proc/meminfo | awk '{print $2}')
echo "MEM_TOTAL=$MEM_TOTAL"
echo "MEM_USED=$((MEM_TOTAL - MEM_AVAIL))"
echo "SWAP_TOTAL=$SWAP_TOTAL"
echo "SWAP_USED=$((SWAP_TOTAL - SWAP_FREE))"

# Disk usage for root partition (bytes)
DISK_INFO=$(df -B1 / 2>/dev/null | tail -1)
echo "DISK_TOTAL=$(echo $DISK_INFO | awk '{print $2}')"
echo "DISK_USED=$(echo $DISK_INFO | awk '{print $3}')"

# Network traffic (cumulative bytes on primary interface)
NET_IFACE=$(ip route 2>/dev/null | grep default | awk '{print $5}' | head -1)
if [ -z "$NET_IFACE" ]; then NET_IFACE=$(ls /sys/class/net | grep -v lo | head -1); fi
if [ -n "$NET_IFACE" ]; then
  NET_RX=$(cat /sys/class/net/$NET_IFACE/statistics/rx_bytes 2>/dev/null || echo 0)
  NET_TX=$(cat /sys/class/net/$NET_IFACE/statistics/tx_bytes 2>/dev/null || echo 0)
  echo "NET_IFACE=$NET_IFACE"
  echo "NET_RX=$NET_RX"
  echo "NET_TX=$NET_TX"
fi

# Virtualization detection
VIRT_TYPE=$(systemd-detect-virt 2>/dev/null || echo "")
if [ -z "$VIRT_TYPE" ]; then
  if grep -qa hypervisor /proc/cpuinfo 2>/dev/null; then
    VIRT_TYPE="vm"
  else
    VIRT_TYPE="bare-metal"
  fi
fi
echo "VIRT_TYPE=$VIRT_TYPE"

# Uptime
echo "UPTIME=$(awk '{print int($1)}' /proc/uptime 2>/dev/null)"

# GPU (nvidia-smi)
if command -v nvidia-smi &>/dev/null; then
  echo "===GPU==="
  nvidia-smi --query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu --format=csv,noheader,nounits 2>/dev/null | while IFS=',' read -r idx name mtotal mused util temp; do
    echo "GPU=$(echo $idx)|$(echo $name)|$(echo $mtotal)|$(echo $mused)|$(echo $util)|$(echo $temp)"
  done
fi
echo "===END==="
`

func parseMetrics(output string) *MetricsResult {
	m := &MetricsResult{}
	inGPU := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "===GPU===" {
			inGPU = true
			continue
		}
		if line == "===END===" {
			break
		}
		if inGPU {
			if strings.HasPrefix(line, "GPU=") {
				parts := strings.SplitN(line[4:], "|", 6)
				if len(parts) >= 6 {
					gpu := GPUInfo{}
					gpu.Index = atoiSafe(parts[0])
					gpu.Name = strings.TrimSpace(parts[1])
					gpu.MemoryTotal = parseUintSafe(parts[2]) * 1024 * 1024
					gpu.MemoryUsed = parseUintSafe(parts[3]) * 1024 * 1024
					gpu.Utilization = parseFloatSafe(parts[4])
					gpu.Temperature = atoiSafe(parts[5])
					m.GPUs = append(m.GPUs, gpu)
				}
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], strings.TrimSpace(parts[1])
		switch key {
		case "CPU_USAGE":
			m.CPUUsage = parseFloatSafe(val)
		case "CPU_MODEL":
			m.CPUModel = val
		case "CPU_CORES":
			m.CPUCores = atoiSafe(val)
		case "CPU_SOCKETS":
			m.CPUSockets = atoiSafe(val)
		case "CPU_THREADS":
			m.CPUThreads = atoiSafe(val)
		case "LOAD_1":
			m.LoadAvg1 = parseFloatSafe(val)
		case "LOAD_5":
			m.LoadAvg5 = parseFloatSafe(val)
		case "LOAD_15":
			m.LoadAvg15 = parseFloatSafe(val)
		case "MEM_TOTAL":
			m.MemTotal = parseUintSafe(val) * 1024
		case "MEM_USED":
			m.MemUsed = parseUintSafe(val) * 1024
		case "SWAP_TOTAL":
			m.SwapTotal = parseUintSafe(val) * 1024
		case "SWAP_USED":
			m.SwapUsed = parseUintSafe(val) * 1024
		case "DISK_TOTAL":
			m.DiskTotal = parseUintSafe(val)
		case "DISK_USED":
			m.DiskUsed = parseUintSafe(val)
		case "VIRT_TYPE":
			m.VirtType = val
		case "NET_IFACE":
			m.NetIface = val
		case "NET_RX":
			m.NetRx = parseUintSafe(val)
		case "NET_TX":
			m.NetTx = parseUintSafe(val)
		case "UPTIME":
			m.Uptime = parseUintSafe(val)
		}
	}
	return m
}

func parseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseUintSafe(s string) uint64 {
	s = strings.TrimSpace(s)
	var i uint64
	fmt.Sscanf(s, "%d", &i)
	return i
}

type lineReader struct {
	reader io.Reader
	buf    []byte
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{reader: r}
}

func (lr *lineReader) Scan() bool {
	return true
}

func (lr *lineReader) Text() string {
	buf := make([]byte, 4096)
	n, err := lr.reader.Read(buf)
	if err != nil && n == 0 {
		time.Sleep(10 * time.Millisecond)
		return ""
	}
	return string(buf[:n])
}
