package node

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
	"github.com/michael/device_grid/internal/ws"
)

type HealthChecker struct {
	repos     repo.Repositories
	transport *transport.Manager
	hub       *ws.Hub
	interval  time.Duration
	mu        sync.Mutex
	running   bool
	cancel    context.CancelFunc
}

func NewHealthChecker(repos repo.Repositories, tm *transport.Manager, hub *ws.Hub) *HealthChecker {
	return &HealthChecker{
		repos:     repos,
		transport: tm,
		hub:       hub,
		interval:  10 * time.Second,
	}
}

func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	ctx, cancel := context.WithCancel(context.Background())
	hc.cancel = cancel
	hc.mu.Unlock()

	go hc.run(ctx)
	slog.Info("node health checker started", "interval", hc.interval)
}

func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.cancel != nil {
		hc.cancel()
	}
	hc.running = false
}

func (hc *HealthChecker) run(ctx context.Context) {
	hc.checkAll(ctx)
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.checkAll(ctx)
		}
	}
}

func (hc *HealthChecker) checkAll(ctx context.Context) {
	nodes, err := hc.repos.Nodes().List(ctx, model.NodeFilter{})
	if err != nil {
		slog.Error("health check: list nodes", "error", err)
		return
	}

	var wg sync.WaitGroup
	statuses := make([]NodeStatus, 0, len(nodes))
	var statusMu sync.Mutex

	for _, node := range nodes {
		if node.Status == model.NodeStatusUntrusted {
			statusMu.Lock()
			statuses = append(statuses, NodeStatus{
				NodeID: node.ID,
				Name:   node.Name,
				Status: string(node.Status),
			})
			statusMu.Unlock()
			continue
		}

		wg.Add(1)
		go func(n *model.Node) {
			defer wg.Done()
			status := hc.checkNode(ctx, n)
			statusMu.Lock()
			statuses = append(statuses, status)
			statusMu.Unlock()
		}(node)
	}

	wg.Wait()

	online := 0
	offline := 0
	untrusted := 0
	for _, s := range statuses {
		switch s.Status {
		case "online":
			online++
		case "offline":
			offline++
		case "untrusted":
			untrusted++
		}
	}

	hc.hub.Broadcast("kanban", map[string]interface{}{
		"type":      "status_update",
		"timestamp": time.Now().Unix(),
		"nodes":     statuses,
		"count":     len(statuses),
		"online":    online,
		"offline":   offline,
		"untrusted": untrusted,
	})
}

func (hc *HealthChecker) checkNode(ctx context.Context, node *model.Node) NodeStatus {
	status := NodeStatus{
		NodeID: node.ID,
		Name:   node.Name,
		Status: string(node.Status),
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := hc.transport.Ping(pingCtx, node.ID)
	if err != nil {
		slog.Debug("node health check failed", "node", node.Name, "error", err)
		status.Status = string(model.NodeStatusOffline)
		_ = hc.repos.Nodes().UpdateStatus(ctx, node.ID, model.NodeStatusOffline)
		return status
	}

	status.Status = string(model.NodeStatusOnline)
	_ = hc.repos.Nodes().UpdateStatus(ctx, node.ID, model.NodeStatusOnline)

	if facts, err := hc.transport.Facts(ctx, node.ID); err == nil {
		if node.OS != facts.OS || node.Arch != facts.Arch || node.DockerVersion != facts.DockerVersion {
			node.OS = facts.OS
			node.Arch = facts.Arch
			node.DockerVersion = facts.DockerVersion
			_ = hc.repos.Nodes().Update(ctx, node)
		}
		status.OS = facts.OS
		status.Arch = facts.Arch
		status.DockerVersion = facts.DockerVersion
	}

	metricsCtx, mCancel := context.WithTimeout(ctx, 5*time.Second)
	defer mCancel()
	if metrics, err := hc.transport.Metrics(metricsCtx, node.ID); err == nil {
		status.CPUUsage = metrics.CPUUsage
		status.CPUCores = metrics.CPUCores
		status.MemTotal = metrics.MemTotal
		status.MemUsed = metrics.MemUsed
		status.DiskTotal = metrics.DiskTotal
		status.DiskUsed = metrics.DiskUsed
		status.Uptime = metrics.Uptime
		status.GPUs = metrics.GPUs
	}

	return status
}

type NodeStatus struct {
	NodeID        string             `json:"node_id"`
	Name          string             `json:"name"`
	Status        string             `json:"status"`
	OS            string             `json:"os,omitempty"`
	Arch          string             `json:"arch,omitempty"`
	DockerVersion string             `json:"docker_version,omitempty"`
	CPUUsage      float64             `json:"cpu_usage,omitempty"`
	CPUCores      int                 `json:"cpu_cores,omitempty"`
	MemTotal      uint64             `json:"mem_total,omitempty"`
	MemUsed       uint64             `json:"mem_used,omitempty"`
	DiskTotal     uint64             `json:"disk_total,omitempty"`
	DiskUsed      uint64             `json:"disk_used,omitempty"`
	Uptime        uint64             `json:"uptime,omitempty"`
	GPUs          []transport.GPUInfo `json:"gpus,omitempty"`
}
