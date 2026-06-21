package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

type TunnelServer struct {
	agentpb.UnimplementedTunnelServiceServer
	registry *Registry
}

func NewTunnelServer(registry *Registry) *TunnelServer {
	return &TunnelServer{registry: registry}
}

func (s *TunnelServer) Connect(stream agentpb.TunnelService_ConnectServer) error {
	var nodeID string
	var nodeName string
	ctx := stream.Context()

	registered := false
	var unregisterOnce sync.Once

	defer func() {
		if registered {
			unregisterOnce.Do(func() {
				s.registry.Unregister(nodeID)
			})
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			slog.Debug("tunnel recv error", "node", nodeID, "error", err)
			return err
		}

		if !registered {
			nodeID = msg.NodeId
			if nodeID == "" {
				return fmt.Errorf("first message must include node_id")
			}
			nodeName = msg.NodeName
			regCtx := s.registry.Register(nodeID, nodeName, &grpcStreamWrapper{stream: stream})
			registered = true
			go func() {
				<-regCtx.Done()
				unregisterOnce.Do(func() {
					s.registry.Unregister(nodeID)
				})
			}()
			slog.Info("agent tunnel connected", "node_id", nodeID, "name", nodeName)
		}

		s.registry.Touch(nodeID)

		switch p := msg.Payload.(type) {
		case *agentpb.ClientMessage_Heartbeat:
			slog.Debug("agent heartbeat", "node", nodeID)
		case *agentpb.ClientMessage_Metrics:
			lastMetricsMu.Lock()
			lastMetrics[nodeID] = p.Metrics
			lastMetricsTs[nodeID] = time.Now()
			lastMetricsMu.Unlock()
		case *agentpb.ClientMessage_CommandResponse:
			handleCommandResponse(p.CommandResponse)
		case *agentpb.ClientMessage_PtyOutput:
			handlePtyOutput(p.PtyOutput)
		case *agentpb.ClientMessage_FileList:
			handleFileListResponse(p.FileList)
		case *agentpb.ClientMessage_FileData:
			handleFileData(p.FileData)
		case *agentpb.ClientMessage_FileInfo:
			handleFileInfoResponse(p.FileInfo)
		}
	}
}

// grpcStreamWrapper adapts the gRPC stream to AgentStream interface
type grpcStreamWrapper struct {
	stream agentpb.TunnelService_ConnectServer
}

func (w *grpcStreamWrapper) Send(data []byte) error {
	return w.stream.Send(&agentpb.ServerMessage{})
}

func (w *grpcStreamWrapper) Recv() ([]byte, error) {
	return nil, nil
}

func (w *grpcStreamWrapper) Context() context.Context {
	return w.stream.Context()
}

// ===== Pending request tracking =====

var (
	pendingCmds   = make(map[string]chan *agentpb.CommandResponse)
	pendingCmdsMu sync.Mutex

	pendingFiles   = make(map[string]chan *agentpb.FileListResponse)
	pendingFilesMu sync.Mutex

	pendingFileData   = make(map[string]chan *agentpb.FileData)
	pendingFileDataMu sync.Mutex

	lastMetrics   = make(map[string]*agentpb.MetricsReport)
	lastMetricsTs = make(map[string]time.Time)
	lastMetricsMu sync.Mutex

	ptyOutputs   = make(map[string]chan *agentpb.PtyOutput)
	ptyOutputsMu sync.Mutex
)

func RegisterPendingCmd(reqID string) chan *agentpb.CommandResponse {
	ch := make(chan *agentpb.CommandResponse, 16)
	pendingCmdsMu.Lock()
	pendingCmds[reqID] = ch
	pendingCmdsMu.Unlock()
	// Auto-cleanup after timeout to prevent memory leak
	go func() {
		time.Sleep(5 * time.Minute)
		pendingCmdsMu.Lock()
		delete(pendingCmds, reqID)
		pendingCmdsMu.Unlock()
	}()
	return ch
}

func handleCommandResponse(resp *agentpb.CommandResponse) {
	pendingCmdsMu.Lock()
	ch, ok := pendingCmds[resp.RequestId]
	if ok && resp.Done {
		delete(pendingCmds, resp.RequestId)
	}
	pendingCmdsMu.Unlock()
	if ok {
		ch <- resp
	}
}

func RegisterPtyOutput(sessionID string) chan *agentpb.PtyOutput {
	ch := make(chan *agentpb.PtyOutput, 256)
	ptyOutputsMu.Lock()
	ptyOutputs[sessionID] = ch
	ptyOutputsMu.Unlock()
	// Auto-cleanup after 1 hour to prevent memory leak
	go func() {
		time.Sleep(1 * time.Hour)
		ptyOutputsMu.Lock()
		delete(ptyOutputs, sessionID)
		ptyOutputsMu.Unlock()
	}()
	return ch
}

func handlePtyOutput(out *agentpb.PtyOutput) {
	ptyOutputsMu.Lock()
	ch, ok := ptyOutputs[out.SessionId]
	if out.Closed {
		delete(ptyOutputs, out.SessionId)
	}
	ptyOutputsMu.Unlock()
	if ok {
		ch <- out
	}
}

func RegisterPendingFileList(reqID string) chan *agentpb.FileListResponse {
	ch := make(chan *agentpb.FileListResponse, 4)
	pendingFilesMu.Lock()
	pendingFiles[reqID] = ch
	pendingFilesMu.Unlock()
	go func() { time.Sleep(60 * time.Second); pendingFilesMu.Lock(); delete(pendingFiles, reqID); pendingFilesMu.Unlock() }()
	return ch
}

func handleFileListResponse(resp *agentpb.FileListResponse) {
	pendingFilesMu.Lock()
	ch, ok := pendingFiles[resp.RequestId]
	if ok {
		delete(pendingFiles, resp.RequestId)
	}
	pendingFilesMu.Unlock()
	if ok {
		ch <- resp
	}
}

func RegisterPendingFileData(reqID string) chan *agentpb.FileData {
	ch := make(chan *agentpb.FileData, 16)
	pendingFileDataMu.Lock()
	pendingFileData[reqID] = ch
	pendingFileDataMu.Unlock()
	go func() { time.Sleep(5 * time.Minute); pendingFileDataMu.Lock(); delete(pendingFileData, reqID); pendingFileDataMu.Unlock() }()
	return ch
}

func handleFileData(data *agentpb.FileData) {
	pendingFileDataMu.Lock()
	ch, ok := pendingFileData[data.RequestId]
	if ok && (data.Eof || data.Error != "") {
		delete(pendingFileData, data.RequestId)
	}
	pendingFileDataMu.Unlock()
	if ok {
		ch <- data
	}
}

func handleFileInfoResponse(resp *agentpb.FileInfoResponse) {
	// handled through file data channels if needed
}

func GetLastMetrics(nodeID string) (*agentpb.MetricsReport, time.Time, bool) {
	lastMetricsMu.Lock()
	defer lastMetricsMu.Unlock()
	m, ok := lastMetrics[nodeID]
	if !ok {
		return nil, time.Time{}, false
	}
	return m, lastMetricsTs[nodeID], true
}

func NewRequestID() string {
	return uuid.NewString()
}
