package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
)

type Manager struct {
	repos  repo.Repositories
	enc    *crypto.Encryptor
	mu     sync.Mutex
	pools  map[string]*nodePool
	config Config
}

type Config struct {
	ConnectTimeout    time.Duration
	KeepaliveInterval time.Duration
	MaxConnections    int
}

func NewManager(repos repo.Repositories, enc *crypto.Encryptor, cfg Config) *Manager {
	m := &Manager{
		repos:  repos,
		enc:    enc,
		pools:  make(map[string]*nodePool),
		config: cfg,
	}
	// Background reaper for stale connections
	go m.reaper()
	return m
}

func (m *Manager) getClient(ctx context.Context, nodeID string) (*ssh.Client, error) {
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	return m.getClientForNode(node)
}

func (m *Manager) getClientForNode(node *model.Node) (*ssh.Client, error) {
	m.mu.Lock()
	pool, ok := m.pools[node.ID]
	if !ok {
		pool = newNodePool(node.ID, m.config.MaxConnections)
		m.pools[node.ID] = pool
	}
	m.mu.Unlock()

	// Fast path: reuse idle connection
	if client := pool.get(); client != nil {
		return client, nil
	}

	// Slow path: dial new connection (with herd prevention)
	pool.dialMu.Lock()
	defer pool.dialMu.Unlock()

	// Double-check after acquiring dial lock
	if client := pool.get(); client != nil {
		return client, nil
	}

	client, err := m.dial(node)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// releaseClient returns a client to the pool. If the pool is full or the
// client is dead, it closes the client.
func (m *Manager) releaseClient(nodeID string, client *ssh.Client) {
	if client == nil {
		return
	}
	m.mu.Lock()
	pool, ok := m.pools[nodeID]
	m.mu.Unlock()
	if !ok {
		client.Close()
		return
	}
	pool.put(client)
}

func (m *Manager) dial(node *model.Node) (*ssh.Client, error) {
	authMethods, err := m.getAuthMethods(node)
	if err != nil {
		return nil, err
	}

	hostKeyCallback := m.getHostKeyCallback(node)

	config := &ssh.ClientConfig{
		User:            node.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         m.config.ConnectTimeout,
	}

	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return client, nil
}

// getHostKeyCallback implements TOFU
func (m *Manager) getHostKeyCallback(node *model.Node) ssh.HostKeyCallback {
	if node.HostKey == "" {
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			storedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			ctx := context.Background()
			node.HostKey = storedKey
			_ = m.repos.Nodes().Update(ctx, node)
			slog.Info("ssh host key stored (TOFU)", "node", node.Name, "key_type", key.Type())
			return nil
		}
	}

	storedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(node.HostKey))
	if err != nil {
		slog.Warn("ssh host key parse failed, re-trusting", "node", node.Name, "error", err)
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			storedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			ctx := context.Background()
			node.HostKey = storedKey
			_ = m.repos.Nodes().Update(ctx, node)
			return nil
		}
	}

	return ssh.FixedHostKey(storedKey)
}

func (m *Manager) getAuthMethods(node *model.Node) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if node.PrivateKeyEnc != "" {
		keyBytes, err := m.enc.DecryptString(node.PrivateKeyEnc)
		if err == nil {
			if signer, err := ssh.ParsePrivateKey([]byte(keyBytes)); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	if node.PasswordEnc != "" {
		password, err := m.enc.DecryptString(node.PasswordEnc)
		if err == nil {
			methods = append(methods,
				ssh.Password(password),
				ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
					answers := make([]string, len(questions))
					for i := range questions {
						answers[i] = password
					}
					return answers, nil
				}),
			)
		}
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication method available for node %s", node.ID)
	}
	return methods, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pool := range m.pools {
		pool.stop()
	}
	m.pools = nil
}

func (m *Manager) RemovePool(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, ok := m.pools[nodeID]; ok {
		pool.stop()
		delete(m.pools, nodeID)
	}
}

// reaper periodically scans all pools and removes dead connections
func (m *Manager) reaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		for _, pool := range m.pools {
			pool.reapDead()
		}
		m.mu.Unlock()
	}
}

// ===== nodePool =====

type pooledClient struct {
	client   *ssh.Client
	lastUsed time.Time
}

type nodePool struct {
	nodeID   string
	max      int
	mu       sync.Mutex
	clients  []*pooledClient
	dialMu   sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newNodePool(nodeID string, max int) *nodePool {
	p := &nodePool{
		nodeID: nodeID,
		max:    max,
		stopCh: make(chan struct{}),
	}
	go p.keepaliveLoop()
	return p
}

func (p *nodePool) keepaliveLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.reapDead()
		}
	}
}

// reapDead checks all idle connections and removes dead ones.
// Also closes connections that have been idle too long (>5 min).
func (p *nodePool) reapDead() {
	p.mu.Lock()
	defer p.mu.Unlock()
	var alive []*pooledClient
	maxIdle := 5 * time.Minute
	for _, pc := range p.clients {
		// Check if connection is still alive
		_, _, err := pc.client.SendRequest("keepalive@devicegrid", true, nil)
		if err != nil {
			pc.client.Close()
			continue
		}
		// Check idle timeout
		if time.Since(pc.lastUsed) > maxIdle && len(p.clients) > 1 {
			pc.client.Close()
			continue
		}
		pc.lastUsed = time.Now()
		alive = append(alive, pc)
	}
	p.clients = alive
}

// get retrieves an idle connection from the pool. Returns nil if pool is empty.
// The caller is responsible for calling put when done.
func (p *nodePool) get() *ssh.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.clients) > 0 {
		pc := p.clients[len(p.clients)-1]
		p.clients = p.clients[:len(p.clients)-1]
		// Quick health check
		_, _, err := pc.client.SendRequest("keepalive@devicegrid", true, nil)
		if err != nil {
			pc.client.Close()
			continue // Try next
		}
		return pc.client
	}
	return nil
}

// put returns a client to the pool for reuse.
func (p *nodePool) put(client *ssh.Client) {
	if client == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// Check if pool is being stopped
	select {
	case <-p.stopCh:
		client.Close()
		return
	default:
	}
	if len(p.clients) >= p.max {
		client.Close()
		return
	}
	p.clients = append(p.clients, &pooledClient{
		client:   client,
		lastUsed: time.Now(),
	})
}

func (p *nodePool) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pc := range p.clients {
		pc.client.Close()
	}
	p.clients = nil
}

func (p *nodePool) stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
	p.closeAll()
}

func dialWithPassword(host string, port int, username, password string, timeout time.Duration) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}),
		Timeout: timeout,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, config)
}
