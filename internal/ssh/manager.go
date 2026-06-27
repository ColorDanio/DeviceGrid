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
	repos repo.Repositories
	enc   *crypto.Encryptor
	mu    sync.Mutex
	pools map[string]*nodePool
	config Config
}

type Config struct {
	ConnectTimeout    time.Duration
	KeepaliveInterval time.Duration
	MaxConnections    int
}

func NewManager(repos repo.Repositories, enc *crypto.Encryptor, cfg Config) *Manager {
	return &Manager{
		repos:  repos,
		enc:    enc,
		pools:  make(map[string]*nodePool),
		config: cfg,
	}
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

	// Try to get an existing idle connection first (fast path)
	if client := pool.get(); client != nil {
		return client, nil
	}

	// No idle connection — acquire dial lock to prevent thundering herd
	// Multiple goroutines requesting the same node will wait for one dial
	pool.dialMu.Lock()
	defer pool.dialMu.Unlock()

	// Double-check after acquiring lock — someone else may have just dialed
	if client := pool.get(); client != nil {
		return client, nil
	}

	// Dial a new connection
	client, err := m.dial(node)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// releaseClient returns a client to the pool for reuse
func (m *Manager) releaseClient(nodeID string, client *ssh.Client) {
	m.mu.Lock()
	pool, ok := m.pools[nodeID]
	if !ok {
		pool = newNodePool(nodeID, m.config.MaxConnections)
		m.pools[nodeID] = pool
	}
	m.mu.Unlock()
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

// getHostKeyCallback implements TOFU (Trust On First Use):
// - If node has no stored host key: accept and store it (first connection)
// - If node has a stored host key: verify it matches
func (m *Manager) getHostKeyCallback(node *model.Node) ssh.HostKeyCallback {
	if node.HostKey == "" {
		// First connection — accept any key and store it
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			storedKey := string(ssh.MarshalAuthorizedKey(key))
			storedKey = strings.TrimSpace(storedKey)
			// Persist to database
			ctx := context.Background()
			node.HostKey = storedKey
			_ = m.repos.Nodes().Update(ctx, node)
			slog.Info("ssh host key stored (TOFU)", "node", node.Name, "key_type", key.Type())
			return nil
		}
	}

	// Parse stored host key
	storedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(node.HostKey))
	if err != nil {
		// Corrupted stored key — fall back to TOFU
		slog.Warn("ssh host key parse failed, re-trusting", "node", node.Name, "error", err)
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			storedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			ctx := context.Background()
			node.HostKey = storedKey
			_ = m.repos.Nodes().Update(ctx, node)
			return nil
		}
	}

	// Verify against stored key
	return ssh.FixedHostKey(storedKey)
}

func (m *Manager) getAuthMethods(node *model.Node) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try private key first (more secure, preferred by servers with PasswordAuthentication no)
	if node.PrivateKeyEnc != "" {
		keyBytes, err := m.enc.DecryptString(node.PrivateKeyEnc)
		if err == nil {
			if signer, err := ssh.ParsePrivateKey([]byte(keyBytes)); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	// Then password + keyboard-interactive
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
		return nil, fmt.Errorf("no authentication method available for node %s (no password or private key)", node.ID)
	}
	return methods, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pool := range m.pools {
		pool.closeAll()
	}
}

// RemovePool cleans up SSH connections for a deleted node
func (m *Manager) RemovePool(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, ok := m.pools[nodeID]; ok {
		pool.closeAll()
		delete(m.pools, nodeID)
	}
}

type nodePool struct {
	nodeID  string
	max     int
	mu      sync.Mutex
	clients []*ssh.Client
	dialMu  sync.Mutex // Prevents thundering herd of concurrent dials
}

func newNodePool(nodeID string, max int) *nodePool {
	pool := &nodePool{nodeID: nodeID, max: max}
	// Start keepalive goroutine for this pool
	go pool.keepalive()
	return pool
}

func (p *nodePool) keepalive() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		var alive []*ssh.Client
		for _, c := range p.clients {
			_, _, err := c.SendRequest("keepalive@golang.org", true, nil)
			if err != nil {
				c.Close()
			} else {
				alive = append(alive, c)
			}
		}
		p.clients = alive
		p.mu.Unlock()
	}
}

func (p *nodePool) get() *ssh.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Try connections from the end, discard dead ones
	for len(p.clients) > 0 {
		client := p.clients[len(p.clients)-1]
		p.clients = p.clients[:len(p.clients)-1]
		// Quick health check — send a global request
		_, _, err := client.SendRequest("keepalive@golang.org", true, nil)
		if err != nil {
			// Dead connection — discard and try next
			client.Close()
			continue
		}
		return client
	}
	return nil
}

func (p *nodePool) put(client *ssh.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.clients) >= p.max {
		client.Close()
		return
	}
	p.clients = append(p.clients, client)
}

func (p *nodePool) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.clients {
		c.Close()
	}
	p.clients = nil
}

func dialWithPassword(host string, port int, username, password string, timeout time.Duration) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            username,
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
			return nil // Trust establishment — key will be stored by the trust flow
		}),
		Timeout:         timeout,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, config)
}
