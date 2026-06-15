package ssh

import (
	"context"
	"io"
	"os"

	"github.com/michael/device_grid/internal/ssh"
	"github.com/michael/device_grid/internal/transport"
)

type Transport struct {
	mgr *ssh.Manager
}

func New(mgr *ssh.Manager) *Transport {
	return &Transport{mgr: mgr}
}

func (t *Transport) Exec(ctx context.Context, nodeID string, cmd string) (transport.ExecResult, error) {
	result, err := t.mgr.Exec(ctx, nodeID, cmd)
	if err != nil {
		return transport.ExecResult{}, err
	}
	return transport.ExecResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, nil
}

func (t *Transport) ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan transport.StreamChunk, error) {
	src, err := t.mgr.ExecStream(ctx, nodeID, cmd)
	if err != nil {
		return nil, err
	}
	dst := make(chan transport.StreamChunk, 100)
	go func() {
		defer close(dst)
		for chunk := range src {
			dst <- transport.StreamChunk{
				Type:     chunk.Type,
				Data:     chunk.Data,
				ExitCode: chunk.ExitCode,
			}
		}
	}()
	return dst, nil
}

func (t *Transport) Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error {
	return t.mgr.Upload(ctx, nodeID, remotePath, content, mode)
}

func (t *Transport) Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error) {
	return t.mgr.Download(ctx, nodeID, remotePath)
}

func (t *Transport) PTY(ctx context.Context, nodeID string, cols, rows uint16) (transport.PTYSession, error) {
	return t.mgr.NewPTYSession(ctx, nodeID, cols, rows)
}

func (t *Transport) ContainerPTY(ctx context.Context, nodeID, containerID string, cols, rows uint16) (transport.PTYSession, error) {
	return t.mgr.NewContainerPTYSession(ctx, nodeID, containerID, cols, rows)
}

func (t *Transport) Ping(ctx context.Context, nodeID string) error {
	return t.mgr.Ping(ctx, nodeID)
}

func (t *Transport) Facts(ctx context.Context, nodeID string) (transport.NodeFacts, error) {
	facts, err := t.mgr.Facts(ctx, nodeID)
	if err != nil {
		return transport.NodeFacts{}, err
	}
	return transport.NodeFacts{
		OS:            facts["OS"],
		OSVersion:     facts["OS_VERSION"],
		Arch:          facts["ARCH"],
		Kernel:        facts["KERNEL"],
		DockerVersion: facts["DOCKER"],
		RKE2Version:   facts["RKE2"],
	}, nil
}

func (t *Transport) Metrics(ctx context.Context, nodeID string) (transport.NodeMetrics, error) {
	m, err := t.mgr.Metrics(ctx, nodeID)
	if err != nil {
		return transport.NodeMetrics{}, err
	}
	gpus := make([]transport.GPUInfo, 0, len(m.GPUs))
	for _, g := range m.GPUs {
		gpus = append(gpus, transport.GPUInfo{
			Index:       g.Index,
			Name:        g.Name,
			MemoryTotal: g.MemoryTotal,
			MemoryUsed:  g.MemoryUsed,
			Utilization: g.Utilization,
			Temperature: g.Temperature,
		})
	}
	return transport.NodeMetrics{
		CPUUsage:   m.CPUUsage,
		CPUModel:   m.CPUModel,
		CPUCores:   m.CPUCores,
		CPUSockets: m.CPUSockets,
		CPUThreads: m.CPUThreads,
		LoadAvg1:   m.LoadAvg1,
		LoadAvg5:   m.LoadAvg5,
		LoadAvg15:  m.LoadAvg15,
		MemTotal:   m.MemTotal,
		MemUsed:    m.MemUsed,
		SwapTotal:  m.SwapTotal,
		SwapUsed:   m.SwapUsed,
		DiskTotal:  m.DiskTotal,
		DiskUsed:   m.DiskUsed,
		VirtType:   m.VirtType,
		NetIface:   m.NetIface,
		NetRx:      m.NetRx,
		NetTx:      m.NetTx,
		Uptime:     m.Uptime,
		GPUs:       gpus,
	}, nil
}
