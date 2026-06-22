package ssh

import (
	"context"
	"fmt"
	"net"
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

	config := &ssh.ClientConfig{
		User:            node.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         m.config.ConnectTimeout,
	}

	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return client, nil
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
	return &nodePool{nodeID: nodeID, max: max}
}

func (p *nodePool) get() *ssh.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.clients) == 0 {
		return nil
	}
	client := p.clients[len(p.clients)-1]
	p.clients = p.clients[:len(p.clients)-1]
	return client
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, config)
}
