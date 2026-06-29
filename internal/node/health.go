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
	// Track consecutive failures before marking offline
	failures map[string]int
}

func NewHealthChecker(repos repo.Repositories, tm *transport.Manager, hub *ws.Hub) *HealthChecker {
	return &HealthChecker{
		repos:     repos,
		transport: tm,
		hub:       hub,
		interval:  30 * time.Second,
		failures:  make(map[string]int),
	}
}

func (hc *HealthChecker) SetInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	hc.mu.Lock()
	hc.interval = d
	hc.mu.Unlock()
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

	// Single fast Ping check with generous timeout
	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err := hc.transport.Ping(pingCtx, node.ID)
	if err != nil {
		slog.Debug("node health check ping failed", "node", node.Name, "error", err)

		// Increment failure counter — require 2 consecutive failures before marking offline
		hc.mu.Lock()
		hc.failures[node.ID]++
		failureCount := hc.failures[node.ID]
		hc.mu.Unlock()

		if failureCount >= 2 && node.Status == model.NodeStatusOnline {
			slog.Warn("node marked offline after consecutive failures", "node", node.Name, "failures", failureCount)
			status.Status = string(model.NodeStatusOffline)
			_ = hc.repos.Nodes().UpdateStatus(ctx, node.ID, model.NodeStatusOffline)
		} else {
			// Still online — transient failure
			status.Status = string(node.Status)
		}
		return status
	}

	// Ping succeeded — reset failure counter
	hc.mu.Lock()
	delete(hc.failures, node.ID)
	hc.mu.Unlock()

	status.Status = string(model.NodeStatusOnline)
	if node.Status != model.NodeStatusOnline {
		_ = hc.repos.Nodes().UpdateStatus(ctx, node.ID, model.NodeStatusOnline)
	}

	// Do NOT fetch Facts/Metrics here — MetricsCache handles that separately
	// This keeps the health check lightweight (one SSH session per node)

	// Light facts refresh only on status change (offline → online)
	if node.Status != model.NodeStatusOnline {
		factsCtx, fCancel := context.WithTimeout(ctx, 10*time.Second)
		defer fCancel()
		if facts, err := hc.transport.Facts(factsCtx, node.ID); err == nil {
			node.OS = facts.OS
			node.Arch = facts.Arch
			node.DockerVersion = facts.DockerVersion
			_ = hc.repos.Nodes().Update(ctx, node)
			status.OS = facts.OS
			status.Arch = facts.Arch
			status.DockerVersion = facts.DockerVersion
		}
	}

	return status
}

type NodeStatus struct {
	NodeID        string              `json:"node_id"`
	Name          string              `json:"name"`
	Status        string              `json:"status"`
	OS            string              `json:"os,omitempty"`
	Arch          string              `json:"arch,omitempty"`
	DockerVersion string              `json:"docker_version,omitempty"`
	CPUUsage      float64             `json:"cpu_usage,omitempty"`
	CPUCores      int                 `json:"cpu_cores,omitempty"`
	MemTotal      uint64              `json:"mem_total,omitempty"`
	MemUsed       uint64              `json:"mem_used,omitempty"`
	DiskTotal     uint64              `json:"disk_total,omitempty"`
	DiskUsed      uint64              `json:"disk_used,omitempty"`
	Uptime        uint64              `json:"uptime,omitempty"`
	GPUs          []transport.GPUInfo `json:"gpus,omitempty"`
}
