package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

type Server struct {
	agentpb.UnimplementedAgentServiceServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Ping(ctx context.Context, req *agentpb.PingRequest) (*agentpb.PingResponse, error) {
	return &agentpb.PingResponse{
		AgentVersion: "1.0.0",
		Timestamp:    time.Now().Unix(),
	}, nil
}

func (s *Server) Exec(req *agentpb.ExecRequest, stream agentpb.AgentService_ExecServer) error {
	ctx := stream.Context()

	var cmd *exec.Cmd
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, "bash", "-c", req.Command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", req.Command)
	}

	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				stream.Send(&agentpb.ExecChunk{
					Stream: "stdout",
					Data:   append([]byte{}, buf[:n]...),
				})
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				stream.Send(&agentpb.ExecChunk{
					Stream: "stderr",
					Data:   append([]byte{}, buf[:n]...),
				})
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	exitCode := 0
	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	return stream.Send(&agentpb.ExecChunk{
		ExitCode: int32(exitCode),
		Eof:      true,
	})
}

func (s *Server) Upload(ctx context.Context, req *agentpb.UploadRequest) (*agentpb.UploadResponse, error) {
	mode := os.FileMode(0644)
	if req.Mode > 0 {
		mode = os.FileMode(req.Mode)
	}

	if err := os.WriteFile(req.RemotePath, req.Content, mode); err != nil {
		return &agentpb.UploadResponse{Success: false, Error: err.Error()}, nil
	}
	return &agentpb.UploadResponse{Success: true}, nil
}

func (s *Server) Download(req *agentpb.DownloadRequest, stream agentpb.AgentService_DownloadServer) error {
	f, err := os.Open(req.RemotePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 32768)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&agentpb.Chunk{
				Data: append([]byte{}, buf[:n]...),
			}); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			stream.Send(&agentpb.Chunk{Eof: true})
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *Server) SystemInfo(ctx context.Context, req *agentpb.Empty) (*agentpb.SystemInfoResponse, error) {
	info := &agentpb.SystemInfoResponse{
		Hostname: hostname(),
		Os:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CpuCores: uint64(runtime.NumCPU()),
	}

	osRelease, _ := os.ReadFile("/etc/os-release")
	if val := parseOSRelease(string(osRelease), "ID"); val != "" {
		info.Os = val
	}
	if val := parseOSRelease(string(osRelease), "VERSION_ID"); val != "" {
		info.OsVersion = val
	}

	if kernel, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(kernel))
	}

	if dockerVer, err := exec.Command("docker", "--version").Output(); err == nil {
		info.DockerVersion = strings.TrimSpace(string(dockerVer))
	}

	return info, nil
}

func (s *Server) PTY(req *agentpb.PTYRequest, stream agentpb.AgentService_PTYServer) error {
	return fmt.Errorf("PTY streaming not yet implemented in agent")
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func parseOSRelease(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return strings.Trim(parts[1], `"`)
		}
	}
	return ""
}
