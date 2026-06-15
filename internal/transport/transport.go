package transport

import (
	"context"
	"io"
	"os"

	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type StreamChunk struct {
	Type     string // "stdout" | "stderr" | "exit"
	Data     string
	ExitCode int
}

type NodeFacts struct {
	OS            string `json:"os"`
	OSVersion     string `json:"os_version"`
	Arch          string `json:"arch"`
	Kernel        string `json:"kernel"`
	DockerVersion string `json:"docker_version"`
	RKE2Version   string `json:"rke2_version"`
}

type NodeMetrics struct {
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
	Index     int     `json:"index"`
	Name      string  `json:"name"`
	MemoryTotal uint64 `json:"memory_total"`
	MemoryUsed  uint64 `json:"memory_used"`
	Utilization float64 `json:"utilization"`
	Temperature int     `json:"temperature"`
}

type PTYSession interface {
	Write(data []byte) error
	Read() ([]byte, error)
	Resize(cols, rows uint16) error
	Close() error
	Done() <-chan struct{}
}

type Transporter interface {
	Exec(ctx context.Context, nodeID string, cmd string) (ExecResult, error)
	ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan StreamChunk, error)
	Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error
	Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error)
	PTY(ctx context.Context, nodeID string, cols, rows uint16) (PTYSession, error)
	ContainerPTY(ctx context.Context, nodeID, containerID string, cols, rows uint16) (PTYSession, error)
	Ping(ctx context.Context, nodeID string) error
	Facts(ctx context.Context, nodeID string) (NodeFacts, error)
	Metrics(ctx context.Context, nodeID string) (NodeMetrics, error)
}

type Manager struct {
	sshImpl       Transporter
	agentImpl     Transporter
	tunnelChecker func(nodeID string) bool
	repos         repo.Repositories
	enc           *crypto.Encryptor
}

func NewManager(sshImpl, agentImpl Transporter, repos repo.Repositories, enc *crypto.Encryptor) *Manager {
	return &Manager{
		sshImpl:   sshImpl,
		agentImpl: agentImpl,
		repos:     repos,
		enc:       enc,
	}
}

func (m *Manager) SetTunnelChecker(fn func(nodeID string) bool) {
	m.tunnelChecker = fn
}

func (m *Manager) getTransport(nodeID string) (Transporter, *model.Node, error) {
	node, err := m.repos.Nodes().GetByID(context.Background(), nodeID)
	if err != nil {
		return nil, nil, err
	}
	// Prefer tunnel if agent is connected (reverse connection model)
	if m.tunnelChecker != nil && m.tunnelChecker(nodeID) {
		return m.agentImpl, node, nil
	}
	// Fall back to configured transport mode
	if node.TransportMode == model.TransportAgent {
		return m.agentImpl, node, nil
	}
	return m.sshImpl, node, nil
}

func (m *Manager) GetNode(nodeID string) (*model.Node, error) {
	return m.repos.Nodes().GetByID(context.Background(), nodeID)
}

func (m *Manager) Exec(ctx context.Context, nodeID string, cmd string) (ExecResult, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return ExecResult{}, err
	}
	return t.Exec(ctx, nodeID, cmd)
}

func (m *Manager) ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan StreamChunk, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return nil, err
	}
	return t.ExecStream(ctx, nodeID, cmd)
}

func (m *Manager) Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return err
	}
	return t.Upload(ctx, nodeID, remotePath, content, mode)
}

func (m *Manager) Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return nil, err
	}
	return t.Download(ctx, nodeID, remotePath)
}

func (m *Manager) PTY(ctx context.Context, nodeID string, cols, rows uint16) (PTYSession, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return nil, err
	}
	return t.PTY(ctx, nodeID, cols, rows)
}

func (m *Manager) Ping(ctx context.Context, nodeID string) error {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return err
	}
	return t.Ping(ctx, nodeID)
}

func (m *Manager) Facts(ctx context.Context, nodeID string) (NodeFacts, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return NodeFacts{}, err
	}
	return t.Facts(ctx, nodeID)
}

func (m *Manager) Metrics(ctx context.Context, nodeID string) (NodeMetrics, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return NodeMetrics{}, err
	}
	return t.Metrics(ctx, nodeID)
}

func (m *Manager) ContainerPTY(ctx context.Context, nodeID, containerID string, cols, rows uint16) (PTYSession, error) {
	t, _, err := m.getTransport(nodeID)
	if err != nil {
		return nil, err
	}
	return t.ContainerPTY(ctx, nodeID, containerID, cols, rows)
}
