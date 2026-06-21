package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type Transport struct {
	repos repo.Repositories
	enc   *crypto.Encryptor
	mu    sync.Mutex
	conns map[string]*grpcClientConn
}

type grpcClientConn struct {
	conn   *grpc.ClientConn
	client agentpb.AgentServiceClient
}

func New(repos repo.Repositories, enc *crypto.Encryptor) *Transport {
	return &Transport{
		repos: repos,
		enc:   enc,
		conns: make(map[string]*grpcClientConn),
	}
}

func (t *Transport) getClient(ctx context.Context, nodeID string) (agentpb.AgentServiceClient, error) {
	t.mu.Lock()
	if gConn, ok := t.conns[nodeID]; ok {
		t.mu.Unlock()
		return gConn.client, nil
	}
	t.mu.Unlock()

	node, err := t.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	if node.TransportMode != model.TransportAgent {
		return nil, fmt.Errorf("node %s is not in agent mode", node.Name)
	}

	addr := fmt.Sprintf("%s:%d", node.Host, node.AgentPort)
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial agent %s: %w", addr, err)
	}

	client := agentpb.NewAgentServiceClient(conn)

	// Clean up old connection if exists (replace)
	t.mu.Lock()
	if old, ok := t.conns[nodeID]; ok {
		old.conn.Close()
	}
	t.conns[nodeID] = &grpcClientConn{conn: conn, client: client}
	t.mu.Unlock()

	return client, nil
}

func (t *Transport) Exec(ctx context.Context, nodeID string, cmd string) (transport.ExecResult, error) {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return transport.ExecResult{}, err
	}

	stream, err := client.Exec(ctx, &agentpb.ExecRequest{Command: cmd})
	if err != nil {
		return transport.ExecResult{}, fmt.Errorf("exec stream: %w", err)
	}

	var stdout, stderr string
	exitCode := 0
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return transport.ExecResult{}, fmt.Errorf("recv exec: %w", err)
		}
		if chunk.Eof {
			exitCode = int(chunk.ExitCode)
			break
		}
		switch chunk.Stream {
		case "stdout":
			stdout += string(chunk.Data)
		case "stderr":
			stderr += string(chunk.Data)
		}
	}

	return transport.ExecResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}, nil
}

func (t *Transport) ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan transport.StreamChunk, error) {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	stream, err := client.Exec(ctx, &agentpb.ExecRequest{Command: cmd})
	if err != nil {
		return nil, fmt.Errorf("exec stream: %w", err)
	}

	ch := make(chan transport.StreamChunk, 100)
	go func() {
		defer close(ch)
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			if chunk.Eof {
				ch <- transport.StreamChunk{Type: "exit", ExitCode: int(chunk.ExitCode)}
				return
			}
			ch <- transport.StreamChunk{
				Type: chunk.Stream,
				Data: string(chunk.Data),
			}
		}
	}()

	return ch, nil
}

func (t *Transport) Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("read content: %w", err)
	}

	resp, err := client.Upload(ctx, &agentpb.UploadRequest{
		RemotePath: remotePath,
		Content:    data,
		Mode:       uint32(mode),
	})
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("upload failed: %s", resp.Error)
	}
	return nil
}

func (t *Transport) Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error) {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	stream, err := client.Download(ctx, &agentpb.DownloadRequest{RemotePath: remotePath})
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				writer.CloseWithError(err)
				return
			}
			if chunk.Eof {
				return
			}
			writer.Write(chunk.Data)
		}
	}()

	return reader, nil
}

func (t *Transport) PTY(ctx context.Context, nodeID string, cols, rows uint16) (transport.PTYSession, error) {
	return nil, fmt.Errorf("agent PTY not yet implemented")
}

func (t *Transport) ContainerPTY(ctx context.Context, nodeID, containerID string, cols, rows uint16) (transport.PTYSession, error) {
	return nil, fmt.Errorf("agent container PTY not yet implemented")
}

func (t *Transport) Ping(ctx context.Context, nodeID string) error {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = client.Ping(pingCtx, &agentpb.PingRequest{})
	return err
}

func (t *Transport) Facts(ctx context.Context, nodeID string) (transport.NodeFacts, error) {
	client, err := t.getClient(ctx, nodeID)
	if err != nil {
		return transport.NodeFacts{}, err
	}

	info, err := client.SystemInfo(ctx, &agentpb.Empty{})
	if err != nil {
		return transport.NodeFacts{}, fmt.Errorf("system info: %w", err)
	}

	return transport.NodeFacts{
		OS:            info.Os,
		OSVersion:     info.OsVersion,
		Arch:          info.Arch,
		Kernel:        info.Kernel,
		DockerVersion: info.DockerVersion,
	}, nil
}

func (t *Transport) Metrics(ctx context.Context, nodeID string) (transport.NodeMetrics, error) {
	return transport.NodeMetrics{}, fmt.Errorf("agent metrics not yet implemented, use SSH mode")
}

func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, conn := range t.conns {
		conn.conn.Close()
	}
	t.conns = make(map[string]*grpcClientConn)
	return nil
}

// RemoveConn cleans up a single node's connection (call on node delete)
func (t *Transport) RemoveConn(nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if conn, ok := t.conns[nodeID]; ok {
		conn.conn.Close()
		delete(t.conns, nodeID)
	}
}
