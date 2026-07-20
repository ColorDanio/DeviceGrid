package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

// These tests exercise the Agent gRPC tunnel end to end. They start a real
// gRPC server with TunnelServer, register a fake agent that connects via a
// gRPC client stream and answers CommandRequest by running the command
// locally, then call TunnelTransport.Exec / Upload / FileList and verify the
// round trip.
//
// Hermetic (no external process). Runs under `make test` without preflight.

const (
	testNodeID   = "node-tunnel-test"
	testNodeName = "tunnel-test"
	requestWait  = 5 * time.Second
)

// fakeAgent is a minimal AgentService client that connects to the tunnel
// server, registers itself, and answers a subset of server messages by
// executing them locally.
type fakeAgent struct {
	nodeID   string
	nodeName string
	conn     *grpc.ClientConn
	stream   agentpb.TunnelService_ConnectClient
	// rootDir scopes local exec/ls to a temp dir for the test agent's "shell".
	rootDir string
	running atomic.Bool
}

func newFakeAgent(t *testing.T, addr, nodeID, nodeName, rootDir string) *fakeAgent {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	stream, err := agentpb.NewTunnelServiceClient(conn).Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	fa := &fakeAgent{
		nodeID:   nodeID,
		nodeName: nodeName,
		conn:     conn,
		stream:   stream,
		rootDir:  rootDir,
	}
	// First message must carry NodeId or TunnelServer rejects us.
	if err := stream.Send(&agentpb.ClientMessage{
		NodeId:   nodeID,
		NodeName: nodeName,
	}); err != nil {
		t.Fatalf("send hello: %v", err)
	}
	return fa
}

func (f *fakeAgent) run(ctx context.Context) {
	f.running.Store(true)
	go func() {
		defer f.running.Store(false)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msg, err := f.stream.Recv()
			if err != nil {
				return
			}
			f.handle(msg)
		}
	}()
}

func (f *fakeAgent) close() {
	_ = f.stream.CloseSend()
	_ = f.conn.Close()
}

func (f *fakeAgent) handle(msg *agentpb.ServerMessage) {
	switch p := msg.Payload.(type) {
	case *agentpb.ServerMessage_CommandRequest:
		f.handleCommand(p.CommandRequest)
	case *agentpb.ServerMessage_FileUploadRequest:
		f.handleUpload(p.FileUploadRequest)
	case *agentpb.ServerMessage_FileListRequest:
		f.handleFileList(p.FileListRequest)
	case *agentpb.ServerMessage_FileInfoRequest:
		f.handleFileInfo(p.FileInfoRequest)
	case *agentpb.ServerMessage_FileDownloadRequest:
		f.handleFileDownload(p.FileDownloadRequest)
	}
}

// handleCommand runs the requested command locally and returns the result.
func (f *fakeAgent) handleCommand(req *agentpb.CommandRequest) {
	cmd := exec.Command("sh", "-c", req.Command)
	if req.TimeoutSeconds > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
	}
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			stderr.WriteString(err.Error())
			exitCode = -1
		}
	}
	_ = f.stream.Send(&agentpb.ClientMessage{
		Payload: &agentpb.ClientMessage_CommandResponse{
			CommandResponse: &agentpb.CommandResponse{
				RequestId: req.RequestId,
				Stdout:    stdout.String(),
				Stderr:    stderr.String(),
				ExitCode:  int32(exitCode),
				Done:      true,
			},
		},
	})
}

func (f *fakeAgent) handleUpload(req *agentpb.FileUploadRequest) {
	// Simulate success — the test does not check file landing, only the ack.
	_ = f.stream.Send(&agentpb.ClientMessage{
		Payload: &agentpb.ClientMessage_CommandResponse{
			CommandResponse: &agentpb.CommandResponse{
				RequestId: req.RequestId,
				ExitCode:  0,
				Done:      true,
			},
		},
	})
}

func (f *fakeAgent) handleFileList(req *agentpb.FileListRequest) {
	// Reply with an empty listing so the test can assert path echo.
	_ = f.stream.Send(&agentpb.ClientMessage{
		Payload: &agentpb.ClientMessage_FileList{
			FileList: &agentpb.FileListResponse{
				RequestId: req.RequestId,
				Entries:   []*agentpb.FileEntry{},
			},
		},
	})
}

func (f *fakeAgent) handleFileInfo(req *agentpb.FileInfoRequest) {
	_ = f.stream.Send(&agentpb.ClientMessage{
		Payload: &agentpb.ClientMessage_FileInfo{
			FileInfo: &agentpb.FileInfoResponse{
				RequestId: req.RequestId,
				Exists:    false,
			},
		},
	})
}

func (f *fakeAgent) handleFileDownload(req *agentpb.FileDownloadRequest) {
	_ = f.stream.Send(&agentpb.ClientMessage{
		Payload: &agentpb.ClientMessage_FileData{
			FileData: &agentpb.FileData{
				RequestId: req.RequestId,
				Eof:       true,
			},
		},
	})
}

// startTunnelServer starts a real TunnelServer on a random port backed by a
// fresh Registry. Returns the registry, the server address, and a teardown.
func startTunnelServer(t *testing.T) (*Registry, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	registry := NewRegistry()
	srv := grpc.NewServer()
	agentpb.RegisterTunnelServiceServer(srv, NewTunnelServer(registry))
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = ln.Close()
	})
	return registry, addr
}

// waitForAgent polls the registry until the agent is registered or times out.
func waitForAgent(t *testing.T, reg *Registry, nodeID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if reg.IsConnected(nodeID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("agent %s never registered", nodeID)
}

// ===== Tests =====

// TestTunnelIntegration_Exec_RoundTrip exercises the full request/response
// path: TunnelTransport sends CommandRequest via gRPC, fake agent runs the
// command, response lands on the pending channel, Exec returns it.
func TestTunnelIntegration_Exec_RoundTrip(t *testing.T) {
	reg, addr := startTunnelServer(t)

	fa := newFakeAgent(t, addr, testNodeID, testNodeName, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fa.run(ctx)
	t.Cleanup(fa.close)

	waitForAgent(t, reg, testNodeID)

	transport := NewTunnelTransport(reg)
	result, err := transport.Exec(ctx, testNodeID, "echo round-trip")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit: got %d want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "round-trip" {
		t.Errorf("stdout: got %q want %q", got, "round-trip")
	}
}

// TestTunnelIntegration_Exec_NonZeroExit verifies the exit code is propagated.
func TestTunnelIntegration_Exec_NonZeroExit(t *testing.T) {
	reg, addr := startTunnelServer(t)

	fa := newFakeAgent(t, addr, testNodeID, testNodeName, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fa.run(ctx)
	t.Cleanup(fa.close)
	waitForAgent(t, reg, testNodeID)

	transport := NewTunnelTransport(reg)
	result, err := transport.Exec(ctx, testNodeID, "sh -c 'exit 7'")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 7 {
		t.Errorf("exit: got %d want 7", result.ExitCode)
	}
}

// TestTunnelIntegration_Exec_NotConnected ensures we surface a friendly
// error when no agent is registered for the node.
func TestTunnelIntegration_Exec_NotConnected(t *testing.T) {
	reg, _ := startTunnelServer(t)

	transport := NewTunnelTransport(reg)
	_, err := transport.Exec(context.Background(), "missing-node", "echo hi")
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("err: got %v want ErrNotConnected", err)
	}
}

// TestTunnelIntegration_Upload verifies the FileUploadRequest/CommandResponse
// round trip returns nil on agent-reported success.
func TestTunnelIntegration_Upload(t *testing.T) {
	reg, addr := startTunnelServer(t)

	fa := newFakeAgent(t, addr, testNodeID, testNodeName, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fa.run(ctx)
	t.Cleanup(fa.close)
	waitForAgent(t, reg, testNodeID)

	transport := NewTunnelTransport(reg)
	err := transport.Upload(ctx, testNodeID, "/tmp/dg-test.txt", strings.NewReader("hi"), 0o644)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
}

// TestTunnelIntegration_FileList exercises the FileListRequest/response flow
// used by SFTP-over-tunnel and verifies the agent's empty-listing reply makes
// it back to the caller.
func TestTunnelIntegration_FileList(t *testing.T) {
	reg, addr := startTunnelServer(t)

	fa := newFakeAgent(t, addr, testNodeID, testNodeName, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fa.run(ctx)
	t.Cleanup(fa.close)
	waitForAgent(t, reg, testNodeID)

	transport := NewTunnelTransport(reg)
	// TunnelTransport exposes FileList via the SFTP-like interface on
	// TunnelTransport — verify the underlying sendToAgent round trips through
	// the registry by issuing a request and waiting for the channel to fire.
	// We bypass the public API (which is spread across transport/ssh-style
	// shims) and instead verify the registry + pending-file plumbing, since
	// that is the contract we are integration-testing here.
	_ = transport // keep linter happy

	reqID := NewRequestID()
	ch := RegisterPendingFileList(reqID)
	err := transport.sendToAgent(testNodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_FileListRequest{
			FileListRequest: &agentpb.FileListRequest{
				RequestId: reqID,
				Path:      "/",
			},
		},
	})
	if err != nil {
		t.Fatalf("send file list req: %v", err)
	}
	select {
	case resp := <-ch:
		if resp.RequestId != reqID {
			t.Errorf("request id: got %s want %s", resp.RequestId, reqID)
		}
		if len(resp.Entries) != 0 {
			t.Errorf("entries: got %d want 0", len(resp.Entries))
		}
	case <-time.After(requestWait):
		t.Fatal("file list response timed out")
	}
}

// TestTunnelIntegration_DisconnectUnregisters verifies the agent is removed
// from the registry once the gRPC stream is closed by the client.
func TestTunnelIntegration_DisconnectUnregisters(t *testing.T) {
	reg, addr := startTunnelServer(t)

	fa := newFakeAgent(t, addr, testNodeID, testNodeName, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	fa.run(ctx)
	waitForAgent(t, reg, testNodeID)

	// Tear down the agent.
	cancel()
	fa.close()

	// Give the server a moment to observe EOF.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !reg.IsConnected(testNodeID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("agent %s still registered after disconnect", testNodeID)
}

// Compile-time guard: ensure we use io (for future expansion of fake agent
// helpers like file downloads with body streaming).
var _ = io.EOF
