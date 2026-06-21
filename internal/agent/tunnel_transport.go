package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
	"github.com/michael/device_grid/internal/transport"
)

type TunnelTransport struct {
	registry *Registry
}

func NewTunnelTransport(reg *Registry) *TunnelTransport {
	return &TunnelTransport{registry: reg}
}

func (t *TunnelTransport) isAgentOnline(nodeID string) bool {
	return t.registry.IsConnected(nodeID)
}

func (t *TunnelTransport) sendToAgent(nodeID string, msg *agentpb.ServerMessage) error {
	conn, ok := t.registry.Get(nodeID)
	if !ok {
		return ErrNotConnected
	}
	wrapper, ok := conn.Stream.(*grpcStreamWrapper)
	if !ok {
		return fmt.Errorf("invalid stream type")
	}
	return wrapper.stream.Send(msg)
}

func (t *TunnelTransport) Exec(ctx context.Context, nodeID string, cmd string) (transport.ExecResult, error) {
	if !t.isAgentOnline(nodeID) {
		return transport.ExecResult{}, ErrNotConnected
	}

	reqID := NewRequestID()
	ch := RegisterPendingCmd(reqID)

	err := t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_CommandRequest{
			CommandRequest: &agentpb.CommandRequest{
				RequestId: reqID,
				Command:   cmd,
			},
		},
	})
	if err != nil {
		return transport.ExecResult{}, fmt.Errorf("send command: %w", err)
	}

	var stdout, stderr string
	exitCode := 0
	for {
		select {
		case <-ctx.Done():
			return transport.ExecResult{}, ctx.Err()
		case resp := <-ch:
			if resp.Stdout != "" {
				stdout += resp.Stdout
			}
			if resp.Stderr != "" {
				stderr += resp.Stderr
			}
			if resp.Done {
				exitCode = int(resp.ExitCode)
				return transport.ExecResult{
					Stdout:   stdout,
					Stderr:   stderr,
					ExitCode: exitCode,
				}, nil
			}
		case <-time.After(60 * time.Second):
			return transport.ExecResult{}, fmt.Errorf("command timeout")
		}
	}
}

func (t *TunnelTransport) ExecStream(ctx context.Context, nodeID string, cmd string) (<-chan transport.StreamChunk, error) {
	if !t.isAgentOnline(nodeID) {
		return nil, ErrNotConnected
	}

	reqID := NewRequestID()
	ch := RegisterPendingCmd(reqID)

	err := t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_CommandRequest{
			CommandRequest: &agentpb.CommandRequest{
				RequestId: reqID,
				Command:   cmd,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	out := make(chan transport.StreamChunk, 100)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case resp := <-ch:
				if resp.Stdout != "" {
					out <- transport.StreamChunk{Type: "stdout", Data: resp.Stdout}
				}
				if resp.Stderr != "" {
					out <- transport.StreamChunk{Type: "stderr", Data: resp.Stderr}
				}
				if resp.Done {
					out <- transport.StreamChunk{Type: "exit", ExitCode: int(resp.ExitCode)}
					return
				}
			case <-time.After(5 * time.Minute):
				return
			}
		}
	}()
	return out, nil
}

func (t *TunnelTransport) Upload(ctx context.Context, nodeID string, remotePath string, content io.Reader, mode os.FileMode) error {
	if !t.isAgentOnline(nodeID) {
		return ErrNotConnected
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	reqID := NewRequestID()
	ch := RegisterPendingCmd(reqID)
	err = t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_FileUploadRequest{
			FileUploadRequest: &agentpb.FileUploadRequest{
				RequestId:  reqID,
				RemotePath: remotePath,
				Content:    data,
				Mode:       uint32(mode.Perm()),
			},
		},
	})
	if err != nil {
		return err
	}
	select {
	case resp := <-ch:
		if resp.ExitCode != 0 {
			return fmt.Errorf("upload failed: %s", resp.Stderr)
		}
		return nil
	case <-time.After(60 * time.Second):
		return fmt.Errorf("upload timeout")
	}
}

func (t *TunnelTransport) Download(ctx context.Context, nodeID string, remotePath string) (io.ReadCloser, error) {
	if !t.isAgentOnline(nodeID) {
		return nil, ErrNotConnected
	}
	reqID := NewRequestID()
	ch := RegisterPendingFileData(reqID)
	err := t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_FileDownloadRequest{
			FileDownloadRequest: &agentpb.FileDownloadRequest{
				RequestId:  reqID,
				RemotePath: remotePath,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-ch:
				if data.Error != "" {
					writer.CloseWithError(fmt.Errorf("%s", data.Error))
					return
				}
				if len(data.Data) > 0 {
					writer.Write(data.Data)
				}
				if data.Eof {
					return
				}
			case <-time.After(60 * time.Second):
				return
			}
		}
	}()
	return reader, nil
}

func (t *TunnelTransport) PTY(ctx context.Context, nodeID string, cols, rows uint16) (transport.PTYSession, error) {
	if !t.isAgentOnline(nodeID) {
		return nil, ErrNotConnected
	}

	sessionID := NewRequestID()

	// Send PtyStart to agent
	if err := t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_PtyStart{
			PtyStart: &agentpb.PtyStart{
				SessionId: sessionID,
				Cols:      uint32(cols),
				Rows:      uint32(rows),
				Term:      "xterm-256color",
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("send pty start: %w", err)
	}

	// Register output channel
	outputCh := RegisterPtyOutput(sessionID)

	return &tunnelPTYSession{
		nodeID:    nodeID,
		sessionID: sessionID,
		transport: t,
		outputCh:  outputCh,
		done:      make(chan struct{}),
	}, nil
}

func (t *TunnelTransport) ContainerPTY(ctx context.Context, nodeID, containerID string, cols, rows uint16) (transport.PTYSession, error) {
	// For containers, exec into container via the agent's shell
	if !t.isAgentOnline(nodeID) {
		return nil, ErrNotConnected
	}

	sessionID := NewRequestID()
	if err := t.sendToAgent(nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_PtyStart{
			PtyStart: &agentpb.PtyStart{
				SessionId: sessionID,
				Cols:      uint32(cols),
				Rows:      uint32(rows),
				Term:      "xterm-256color",
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("send container pty start: %w", err)
	}

	// After PTY starts, send the docker exec command
	go func() {
		time.Sleep(100 * time.Millisecond)
		t.sendToAgent(nodeID, &agentpb.ServerMessage{
			Payload: &agentpb.ServerMessage_PtyInput{
				PtyInput: &agentpb.PtyInput{
					SessionId: sessionID,
					Data:      []byte(fmt.Sprintf("docker exec -it %s bash || docker exec -it %s sh\n", containerID, containerID)),
				},
			},
		})
	}()

	outputCh := RegisterPtyOutput(sessionID)

	return &tunnelPTYSession{
		nodeID:    nodeID,
		sessionID: sessionID,
		transport: t,
		outputCh:  outputCh,
		done:      make(chan struct{}),
	}, nil
}

type tunnelPTYSession struct {
	nodeID    string
	sessionID string
	transport *TunnelTransport
	outputCh  chan *agentpb.PtyOutput
	done      chan struct{}
	closed    bool
}

func (s *tunnelPTYSession) Write(data []byte) error {
	return s.transport.sendToAgent(s.nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_PtyInput{
			PtyInput: &agentpb.PtyInput{
				SessionId: s.sessionID,
				Data:      data,
			},
		},
	})
}

func (s *tunnelPTYSession) Read() ([]byte, error) {
	select {
	case out := <-s.outputCh:
		if out.Closed {
			close(s.done)
			return nil, fmt.Errorf("session closed")
		}
		return out.Data, nil
	case <-s.done:
		return nil, fmt.Errorf("session closed")
	}
}

func (s *tunnelPTYSession) Resize(cols, rows uint16) error {
	return s.transport.sendToAgent(s.nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_PtyResize{
			PtyResize: &agentpb.PtyResize{
				SessionId: s.sessionID,
				Cols:      uint32(cols),
				Rows:      uint32(rows),
			},
		},
	})
}

func (s *tunnelPTYSession) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return s.transport.sendToAgent(s.nodeID, &agentpb.ServerMessage{
		Payload: &agentpb.ServerMessage_PtyClose{
			PtyClose: &agentpb.PtyClose{
				SessionId: s.sessionID,
			},
		},
	})
}

func (s *tunnelPTYSession) Done() <-chan struct{} {
	return s.done
}

func (t *TunnelTransport) Ping(ctx context.Context, nodeID string) error {
	if !t.isAgentOnline(nodeID) {
		return ErrNotConnected
	}
	return nil
}

func (t *TunnelTransport) Facts(ctx context.Context, nodeID string) (transport.NodeFacts, error) {
	m, ts, ok := GetLastMetrics(nodeID)
	if !ok || time.Since(ts) > 30*time.Second {
		return transport.NodeFacts{}, fmt.Errorf("no recent metrics")
	}
	return transport.NodeFacts{
		OS:            m.Os,
		Arch:          m.Arch,
		DockerVersion: m.DockerVersion,
	}, nil
}

func (t *TunnelTransport) Metrics(ctx context.Context, nodeID string) (transport.NodeMetrics, error) {
	m, ts, ok := GetLastMetrics(nodeID)
	if !ok {
		return transport.NodeMetrics{}, ErrNotConnected
	}
	if time.Since(ts) > 30*time.Second {
		return transport.NodeMetrics{}, fmt.Errorf("metrics stale")
	}
	return transport.NodeMetrics{
		CPUUsage:   m.CpuUsage,
		CPUThreads: int(m.CpuCores),
		CPUCores:   int(m.CpuCores),
		LoadAvg1:   m.LoadAvg_1,
		LoadAvg5:   m.LoadAvg_5,
		LoadAvg15:  m.LoadAvg_15,
		MemTotal:   m.MemTotal,
		MemUsed:    m.MemUsed,
		SwapTotal:  m.SwapTotal,
		SwapUsed:   m.SwapUsed,
		DiskTotal:  m.DiskTotal,
		DiskUsed:   m.DiskUsed,
		Uptime:     m.Uptime,
	}, nil
}
