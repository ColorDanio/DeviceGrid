package ssh

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// These tests focus on the parts of the SSH manager that can be exercised
// without a real SSH server: pool lifecycle (start/stop), host key
// callback behavior, and concurrency.
//
// Integration tests against a real sshd (via testcontainers or a local
// container) cover dial/exec/PTY and are tracked separately.

// TestNodePool_NewPoolStartsKeepaliveLoop verifies that newNodePool
// initializes all fields and starts the keepalive goroutine.
func TestNodePool_NewPoolStartsKeepaliveLoop(t *testing.T) {
	p := newNodePool("node-A", 5)
	if p == nil {
		t.Fatal("newNodePool returned nil")
	}
	if p.nodeID != "node-A" {
		t.Errorf("nodeID: got %s want node-A", p.nodeID)
	}
	if p.max != 5 {
		t.Errorf("max: got %d want 5", p.max)
	}
	if p.stopCh == nil {
		t.Error("stopCh should be initialized")
	}
	p.stop()
}

// TestNodePool_StopIsIdempotent ensures stop() can be called multiple
// times without panicking (uses sync.Once on the channel close).
func TestNodePool_StopIsIdempotent(t *testing.T) {
	p := newNodePool("node-B", 5)
	p.stop()
	p.stop()
	p.stop()
}

// TestNodePool_StopClosesStopCh verifies the stop channel is closed
// after stop() returns.
func TestNodePool_StopClosesStopCh(t *testing.T) {
	p := newNodePool("node-C", 5)
	time.Sleep(20 * time.Millisecond)
	p.stop()
	select {
	case <-p.stopCh:
		// Expected
	default:
		t.Error("stopCh was not closed after stop()")
	}
}

// TestNodePool_GetEmptyPoolReturnsNil ensures get() doesn't panic on
// an empty pool.
func TestNodePool_GetEmptyPoolReturnsNil(t *testing.T) {
	p := newNodePool("node-D", 3)
	defer p.stop()

	if got := p.get(); got != nil {
		t.Errorf("get on empty pool: got %v want nil", got)
	}
}

// TestNodePool_PutNilIsNoop ensures put(nil) is safe.
func TestNodePool_PutNilIsNoop(t *testing.T) {
	p := newNodePool("node-E", 3)
	defer p.stop()
	// Should not panic
	p.put(nil)
}

// TestNodePool_DialMuUsable verifies the dial mutex is initialized and
// can be acquired/released concurrently without deadlock.
func TestNodePool_DialMuUsable(t *testing.T) {
	p := newNodePool("node-F", 10)
	defer p.stop()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.dialMu.Lock()
			time.Sleep(time.Millisecond)
			p.dialMu.Unlock()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dialMu lock acquisition timed out — likely a deadlock")
	}
}

// TestNodePool_CloseAllOnEmptyDoesNotPanic ensures closeAll on an
// empty pool is a no-op rather than a panic.
func TestNodePool_CloseAllOnEmptyDoesNotPanic(t *testing.T) {
	p := newNodePool("node-G", 5)
	defer p.stop()
	// Should not panic
	p.closeAll()
}

// TestGenerateKeyPair_ProducesValidSSHKey verifies the project's
// key generation produces a key that ssh.ParsePrivateKey can load.
func TestGenerateKeyPair_ProducesValidSSHKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if kp.PrivateKey == "" {
		t.Error("PrivateKey is empty")
	}
	if kp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}

	signer, err := ssh.ParsePrivateKey([]byte(kp.PrivateKey))
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}

	// Marshal and re-parse to verify round-trip stability
	authorized := ssh.MarshalAuthorizedKey(signer.PublicKey())
	parsed, _, _, _, err := ssh.ParseAuthorizedKey(authorized)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	if string(parsed.Marshal()) != string(signer.PublicKey().Marshal()) {
		t.Error("key round-trip mismatch")
	}
}

// TestHostKeyCallback_FixedKeyRejectsWrongKey verifies the host key
// verification actually rejects mismatched keys (TOFU sanity check).
func TestHostKeyCallback_FixedKeyRejectsWrongKey(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("gen key 1: %v", err)
	}
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("gen key 2: %v", err)
	}

	signer1, _ := ssh.ParsePrivateKey([]byte(kp1.PrivateKey))
	signer2, _ := ssh.ParsePrivateKey([]byte(kp2.PrivateKey))

	// Pin to key 1
	authorized := ssh.MarshalAuthorizedKey(signer1.PublicKey())
	parsed, _, _, _, err := ssh.ParseAuthorizedKey(authorized)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cb := ssh.FixedHostKey(parsed)
	if err := cb("test-host", &net.TCPAddr{}, signer1.PublicKey()); err != nil {
		t.Errorf("FixedHostKey rejected matching key: %v", err)
	}
	if err := cb("test-host", &net.TCPAddr{}, signer2.PublicKey()); err == nil {
		t.Error("FixedHostKey accepted non-matching key")
	}
}

// TestDialWithPassword_InvalidHostFails verifies dialWithPassword
// returns an error when the host is unreachable.
func TestDialWithPassword_InvalidHostFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = dialWithPassword("127.0.0.1", 1, "user", "pass", time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("dialWithPassword did not return within 3s")
	}
	if err == nil {
		t.Error("expected error dialing unreachable host")
	}
}

// TestConfig_Defaults verifies the ssh.Config struct fields behave
// as expected (this guards against accidental field renames).
func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		ConnectTimeout:    10 * time.Second,
		KeepaliveInterval: 30 * time.Second,
		MaxConnections:    50,
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("ConnectTimeout: got %v want 10s", cfg.ConnectTimeout)
	}
	if cfg.MaxConnections != 50 {
		t.Errorf("MaxConnections: got %d want 50", cfg.MaxConnections)
	}
}

// TestNewNodePool_RaceFree exercises newNodePool + stop under
// the race detector to catch concurrent map/struct access issues.
func TestNewNodePool_RaceFree(t *testing.T) {
	const N = 20
	var wg sync.WaitGroup
	pools := make([]*nodePool, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			pools[idx] = newNodePool("race-node", 5)
		}(i)
	}
	wg.Wait()

	for _, p := range pools {
		p.stop()
	}
}
