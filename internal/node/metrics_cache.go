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

type MetricsCache struct {
	transport *transport.Manager
	repos     repo.Repositories
	hub       *ws.Hub
	mu        sync.RWMutex
	cache     map[string]*CachedMetrics
	cancel    context.CancelFunc
}

type CachedMetrics struct {
	Data      transport.NodeMetrics
	FetchedAt time.Time
}

func NewMetricsCache(tm *transport.Manager, repos repo.Repositories, hub *ws.Hub) *MetricsCache {
	return &MetricsCache{
		transport: tm,
		repos:     repos,
		hub:       hub,
		cache:     make(map[string]*CachedMetrics),
	}
}

func (mc *MetricsCache) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	mc.cancel = cancel
	go mc.run(ctx)
	slog.Info("metrics cache started", "interval", "15s")
}

func (mc *MetricsCache) Stop() {
	if mc.cancel != nil {
		mc.cancel()
	}
}

func (mc *MetricsCache) run(ctx context.Context) {
	// Initial fetch after 2s (let server fully start)
	time.Sleep(2 * time.Second)
	mc.fetchAll(ctx)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.fetchAll(ctx)
		}
	}
}

func (mc *MetricsCache) fetchAll(ctx context.Context) {
	nodes, err := mc.repos.Nodes().List(ctx, model.NodeFilter{})
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, node := range nodes {
		if node.Status != model.NodeStatusOnline {
			continue
		}

		wg.Add(1)
		go func(n *model.Node) {
			defer wg.Done()
			mc.fetchOne(ctx, n.ID)
		}(node)
	}
	wg.Wait()
}

func (mc *MetricsCache) fetchOne(ctx context.Context, nodeID string) {
	mCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	m, err := mc.transport.Metrics(mCtx, nodeID)
	if err != nil {
		slog.Debug("metrics fetch failed", "node", nodeID, "error", err)
		return
	}

	mc.mu.Lock()
	mc.cache[nodeID] = &CachedMetrics{Data: m, FetchedAt: time.Now()}
	mc.mu.Unlock()

	// Broadcast to WebSocket subscribers
	mc.hub.Broadcast("metrics-"+nodeID, m)
}

func (mc *MetricsCache) Get(nodeID string) (*CachedMetrics, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	m, ok := mc.cache[nodeID]
	return m, ok
}
