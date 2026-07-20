package ssh

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/sqlite"
)

// These tests spin up an in-process SSH + SFTP server on a random localhost
// port and exercise the real ssh.Manager against it. They cover Exec,
// Upload (SCP), Download (cat), Ping, Facts, and SFTPListDir.
//
// The server is hermetic (no external process or container) so it runs under
// `make test` without preflight. Build-tag free on purpose.

const (
	testNodeID   = "node-int-test"
	testUsername = "testuser"
)

// testSSHServer is a hermetic SSH server backed by a temp dir filesystem.
type testSSHServer struct {
	ln        net.Listener
	hostKey   ssh.Signer
	clientPub ssh.PublicKey
	fsRoot    string
	mu        sync.Mutex
	uploaded  map[string][]byte
	closeOnce sync.Once
}

func newTestSSHServer(t *testing.T) *testSSHServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen client key: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatalf("client signer: %v", err)
	}
	clientPubKey := clientSigner.PublicKey()

	// Persist client key as PEM so we can store it on the Node (encrypted).
	privKeyDER, err := x509.MarshalPKCS8PrivateKey(clientPriv)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	privKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privKeyDER})
	t.Setenv("DG_TEST_CLIENT_KEY", string(privKeyPEM))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &testSSHServer{
		ln:        ln,
		hostKey:   hostSigner,
		clientPub: clientPubKey,
		fsRoot:    t.TempDir(),
		uploaded:  make(map[string][]byte),
	}

	// Pre-seed a file so SFTP list / download tests have known content.
	seed := filepath.Join(srv.fsRoot, "seed.txt")
	if err := os.WriteFile(seed, []byte("seed-content\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	go srv.serve()
	t.Cleanup(srv.close)
	return srv
}

func (s *testSSHServer) addr() string { return s.ln.Addr().String() }

func (s *testSSHServer) close() {
	s.closeOnce.Do(func() { _ = s.ln.Close() })
}

func (s *testSSHServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *testSSHServer) handleConn(nconn net.Conn) {
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), s.clientPub.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown public key")
		},
	}
	cfg.AddHostKey(s.hostKey)

	sconn, chans, reqs, err := ssh.NewServerConn(nconn, cfg)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ch, chReqs)
	}
}

func (s *testSSHServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			s.handleExec(ch, payload.Command)
			return
		case "subsystem":
			var payload struct{ Name string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				req.Reply(false, nil)
				return
			}
			if payload.Name == "sftp" {
				req.Reply(true, nil)
				s.handleSFTP(ch)
				return
			}
			req.Reply(false, nil)
		default:
			req.Reply(false, nil)
		}
	}
}

// handleExec implements the small subset of shell behavior our manager needs:
//   - "echo <text>"           → stdout text\n
//   - "cat <path>"            → contents of fsRoot/<basename>
//   - "scp -t <path>"         → receive SCP upload into fsRoot/<basename>
//   - "false"                 → exit code 1
//   - facts/metrics scripts   → minimal canned output for parsing tests
func (s *testSSHServer) handleExec(ch ssh.Channel, cmd string) {
	exit := 0
	defer func() {
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{uint32(exit)}))
	}()

	cmd = strings.TrimSpace(cmd)
	switch {
	case strings.Contains(cmd, "/etc/os-release"):
		// Facts script: emit OS= lines the manager parses. Must be matched
		// before the generic "echo " prefix case below.
		_, _ = ch.Write([]byte("OS=ubuntu\nOS_VERSION=22.04\nARCH=x86_64\nKERNEL=test-kernel\nDOCKER=24.0.0\nRKE2=\n"))
	case strings.HasPrefix(cmd, "scp -t"):
		s.handleSCPUpload(ch, cmd)
	case cmd == "echo ok":
		_, _ = ch.Write([]byte("ok\n"))
	case strings.HasPrefix(cmd, "echo "):
		_, _ = ch.Write([]byte(strings.TrimPrefix(cmd, "echo ") + "\n"))
	case strings.HasPrefix(cmd, "cat "):
		path := strings.Trim(strings.TrimPrefix(cmd, "cat "), "\"'")
		name := filepath.Base(path)
		data, err := os.ReadFile(filepath.Join(s.fsRoot, name))
		if err != nil {
			fmt.Fprintf(ch, "cat: %s: no such file\n", name)
			exit = 1
			return
		}
		_, _ = ch.Write(data)
	case cmd == "false":
		exit = 1
	default:
		// No-op success: keep stdout empty.
	}
}

// handleSCPUpload parses the minimal SCP "to" (-t) protocol our client emits.
// The DeviceGrid client (internal/ssh/exec.go Upload) sends:
//
//	C<mode> 0 <basename>\n   (size always 0 — the client streams content)
//	<content bytes>
//	\x00
//
// Real OpenSSH servers reject this; our test server is intentionally lenient
// and reads content until the trailing \x00 marker so we can exercise the
// round trip regardless of the header's size field.
func (s *testSSHServer) handleSCPUpload(ch ssh.Channel, cmd string) {
	// Send initial ack so the client may proceed (the client ignores it but
	// real scp servers always send it).
	_, _ = ch.Write([]byte{0})

	r := bufio.NewReader(ch)

	header, err := r.ReadString('\n')
	if err != nil {
		return
	}
	header = strings.TrimRight(header, "\n")
	var mode uint32
	var declaredSize int64
	var fname string
	if _, err := fmt.Sscanf(header, "C%04o %d %s", &mode, &declaredSize, &fname); err != nil {
		return
	}
	_, _ = ch.Write([]byte{0})

	// Read until \x00 terminator — resilient to size=0 headers.
	var content []byte
	if declaredSize > 0 {
		content = make([]byte, declaredSize)
		if _, err := io.ReadFull(r, content); err != nil {
			return
		}
		// Discard trailing \x00.
		_, _ = r.ReadByte()
	} else {
		for {
			b, err := r.ReadByte()
			if err != nil {
				return
			}
			if b == 0 {
				break
			}
			content = append(content, b)
		}
	}
	_, _ = ch.Write([]byte{0})

	if fname == "" {
		return
	}
	dst := filepath.Join(s.fsRoot, fname)
	_ = os.WriteFile(dst, content, os.FileMode(mode))
	s.mu.Lock()
	s.uploaded[fname] = content
	s.mu.Unlock()
}

// handleSFTP runs an SFTP server backed by the temp fsRoot so list/read
// operations see the seed file and any file uploaded via SCP.
func (s *testSSHServer) handleSFTP(ch ssh.Channel) {
	defer ch.Close()
	fs := &sftpFS{root: s.fsRoot}
	srv := sftp.NewRequestServer(ch, sftp.Handlers{
		FileGet:  fs,
		FilePut:  fs,
		FileCmd:  fs,
		FileList: fs,
	})
	_ = srv.Serve()
}

// sftpFS is a minimal os-backed filesystem rooted at root, scoped to tests.
type sftpFS struct {
	root string
}

func (f *sftpFS) abs(p string) string {
	clean := filepath.Clean("/" + p)
	return filepath.Join(f.root, clean)
}

func (f *sftpFS) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	file, err := os.Open(f.abs(r.Filepath))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (f *sftpFS) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	file, err := os.Create(f.abs(r.Filepath))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (f *sftpFS) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Mkdir":
		return os.MkdirAll(f.abs(r.Filepath), 0o755)
	case "Rename":
		return os.Rename(f.abs(r.Filepath), f.abs(r.Target))
	case "Rmdir", "Remove":
		return os.RemoveAll(f.abs(r.Filepath))
	case "Setstat":
		return nil
	}
	return nil
}

func (f *sftpFS) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	switch r.Method {
	case "List":
		entries, err := os.ReadDir(f.abs(r.Filepath))
		if err != nil {
			return nil, err
		}
		var infos []os.FileInfo
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			infos = append(infos, info)
		}
		return listerAt(infos), nil
	case "Stat":
		info, err := os.Stat(f.abs(r.Filepath))
		if err != nil {
			return nil, err
		}
		return listerAt([]os.FileInfo{info}), nil
	case "Readlink":
		target, err := os.Readlink(f.abs(r.Filepath))
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		return listerAt([]os.FileInfo{info}), nil
	}
	return nil, fmt.Errorf("unsupported method %s", r.Method)
}

// listerAt adapts []os.FileInfo to sftp.ListerAt.
type listerAt []os.FileInfo

func (l listerAt) ListAt(buf []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(buf, l[offset:])
	if n < len(buf) {
		return n, io.EOF
	}
	return n, nil
}

// setupManagerWithTestServer starts the in-process SSH server, builds a
// real ssh.Manager backed by SQLite (so node lookup + crypto work), and
// returns the manager plus the server's fsRoot for assertions.
func setupManagerWithTestServer(t *testing.T) (*Manager, *testSSHServer) {
	t.Helper()

	srv := newTestSSHServer(t)
	host, port, _ := net.SplitHostPort(srv.addr())
	portI := 22
	fmt.Sscanf(port, "%d", &portI)

	// Encrypt the client key and store a Node the manager can resolve.
	keyHex := hex.EncodeToString(generateTestKey(t))
	enc, err := crypto.New(keyHex)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	privKeyPEM := os.Getenv("DG_TEST_CLIENT_KEY")
	encKey, err := enc.EncryptString(privKeyPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	store := newTestSQLiteStore(t)
	node := &model.Node{
		ID:            testNodeID,
		Name:          "integration-node",
		Host:          host,
		Port:          portI,
		Username:      testUsername,
		AuthMode:      model.AuthKey,
		PrivateKeyEnc: encKey,
		TransportMode: model.TransportSSH,
		Status:        model.NodeStatusOnline,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := store.Nodes().Create(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	mgr := NewManager(store, enc, Config{
		ConnectTimeout:    2 * time.Second,
		KeepaliveInterval: 30 * time.Second,
		MaxConnections:    3,
	})
	t.Cleanup(mgr.Close)
	return mgr, srv
}

// newTestSQLiteStore opens an isolated SQLite store. The store/sqlite package
// owns the helper but does not export it, so we re-create one here at the
// integration test level (same pattern, same lifecycle).
func newTestSQLiteStore(t *testing.T) *sqlite.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.New(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func generateTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return key
}

// ===== Tests =====

func TestSSHIntegration_Exec_Success(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	ctx := context.Background()

	result, err := mgr.Exec(ctx, testNodeID, "echo hello")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit: got %d want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello" {
		t.Errorf("stdout: got %q want %q", got, "hello")
	}
}

func TestSSHIntegration_Exec_NonZeroExit(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	ctx := context.Background()

	result, err := mgr.Exec(ctx, testNodeID, "false")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("exit: got %d want 1", result.ExitCode)
	}
}

func TestSSHIntegration_Exec_PoolReuse(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	ctx := context.Background()

	// Run several commands back-to-back; pool should reuse the connection.
	for i := 0; i < 5; i++ {
		result, err := mgr.Exec(ctx, testNodeID, "echo iter")
		if err != nil {
			t.Fatalf("exec[%d]: %v", i, err)
		}
		if got := strings.TrimSpace(result.Stdout); got != "iter" {
			t.Errorf("stdout[%d]: got %q want iter", i, got)
		}
	}

	// The pool entry must exist and have one idle client.
	mgr.mu.Lock()
	pool, ok := mgr.pools[testNodeID]
	mgr.mu.Unlock()
	if !ok {
		t.Fatal("pool missing after repeated exec")
	}
	pool.mu.Lock()
	idle := len(pool.clients)
	pool.mu.Unlock()
	if idle == 0 {
		t.Errorf("expected idle conn in pool, got %d", idle)
	}
}

func TestSSHIntegration_Ping(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	if err := mgr.Ping(context.Background(), testNodeID); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestSSHIntegration_Upload_ThenDownload(t *testing.T) {
	mgr, srv := setupManagerWithTestServer(t)
	ctx := context.Background()

	body := []byte("integration-body\n")
	remotePath := "/uploaded.txt"

	if err := mgr.Upload(ctx, testNodeID, remotePath, bytes.NewReader(body), 0o644); err != nil {
		t.Fatalf("upload: %v", err)
	}

	srv.mu.Lock()
	got, ok := srv.uploaded["uploaded.txt"]
	srv.mu.Unlock()
	if !ok {
		t.Fatal("server never saw the uploaded file")
	}
	if !bytes.Equal(got, body) {
		t.Errorf("uploaded body mismatch: got %q want %q", got, body)
	}

	// Download via "cat <path>".
	reader, err := mgr.Download(ctx, testNodeID, remotePath)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer reader.Close()
	downloaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(downloaded, body) {
		t.Errorf("downloaded body mismatch: got %q want %q", downloaded, body)
	}
}

func TestSSHIntegration_SFTPListDir(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	ctx := context.Background()

	entries, err := mgr.SFTPListDir(ctx, testNodeID, "/")
	if err != nil {
		t.Fatalf("sftp list: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least the seed file in listing")
	}

	var found bool
	for _, e := range entries {
		if e.Name == "seed.txt" && !e.IsDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("seed.txt missing from listing: %+v", entries)
	}
}

func TestSSHIntegration_Facts(t *testing.T) {
	mgr, _ := setupManagerWithTestServer(t)
	ctx := context.Background()

	facts, err := mgr.Facts(ctx, testNodeID)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}
	if facts["OS"] != "ubuntu" {
		t.Errorf("OS: got %q want ubuntu", facts["OS"])
	}
	if facts["ARCH"] != "x86_64" {
		t.Errorf("ARCH: got %q want x86_64", facts["ARCH"])
	}
	if facts["DOCKER"] != "24.0.0" {
		t.Errorf("DOCKER: got %q want 24.0.0", facts["DOCKER"])
	}
}
