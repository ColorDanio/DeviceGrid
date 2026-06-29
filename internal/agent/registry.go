package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type AgentConnection struct {
	NodeID   string
	NodeName string
	Stream   AgentStream
	LastSeen time.Time
	Cancel   context.CancelFunc
}

type AgentStream interface {
	Send(data []byte) error
	Recv() ([]byte, error)
	Context() context.Context
}

type Registry struct {
	mu    sync.RWMutex
	conns map[string]*AgentConnection
}

func NewRegistry() *Registry {
	return &Registry{conns: make(map[string]*AgentConnection)}
}

func (r *Registry) Register(nodeID, nodeName string, stream AgentStream) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	if old, ok := r.conns[nodeID]; ok {
		old.Cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &AgentConnection{
		NodeID:   nodeID,
		NodeName: nodeName,
		Stream:   stream,
		LastSeen: time.Now(),
		Cancel:   cancel,
	}
	r.conns[nodeID] = conn
	slog.Info("agent registered", "node_id", nodeID, "name", nodeName)
	return ctx
}

func (r *Registry) Unregister(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if conn, ok := r.conns[nodeID]; ok {
		conn.Cancel()
		delete(r.conns, nodeID)
		slog.Info("agent unregistered", "node_id", nodeID)
	}
}

func (r *Registry) Get(nodeID string) (*AgentConnection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.conns[nodeID]
	if ok {
		conn.LastSeen = time.Now()
	}
	return conn, ok
}

func (r *Registry) IsConnected(nodeID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.conns[nodeID]
	return ok
}

func (r *Registry) List() []*AgentConnection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*AgentConnection, 0, len(r.conns))
	for _, c := range r.conns {
		list = append(list, c)
	}
	return list
}

func (r *Registry) SendCommand(nodeID string, data []byte) error {
	r.mu.RLock()
	conn, ok := r.conns[nodeID]
	r.mu.RUnlock()
	if !ok {
		return ErrNotConnected
	}
	return conn.Stream.Send(data)
}

func (r *Registry) Touch(nodeID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if conn, ok := r.conns[nodeID]; ok {
		conn.LastSeen = time.Now()
	}
}

func (r *Registry) StaleAgents(timeout time.Duration) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var stale []string
	cutoff := time.Now().Add(-timeout)
	for id, conn := range r.conns {
		if conn.LastSeen.Before(cutoff) {
			stale = append(stale, id)
		}
	}
	return stale
}
