package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

func main() {
	serverAddr := flag.String("server", "localhost:9090", "DeviceGrid server gRPC address")
	nodeID := flag.String("node-id", "", "This node's unique ID")
	nodeName := flag.String("node-name", "", "This node's display name")
	interval := flag.Int("interval", 5, "Metrics report interval in seconds")
	flag.Parse()

	if *nodeID == "" {
		hostname, _ := os.Hostname()
		*nodeID = hostname
	}
	if *nodeName == "" {
		*nodeName = *nodeID
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	for {
		err := connectAndRun(*serverAddr, *nodeID, *nodeName, *interval)
		if err != nil {
			slog.Warn("tunnel disconnected, retrying", "error", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func connectAndRun(serverAddr, nodeID, nodeName string, interval int) error {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return fmt.Errorf("dial server: %w", err)
	}
	defer conn.Close()

	client := agentpb.NewTunnelServiceClient(conn)
	stream, err := client.Connect(context.Background())
	if err != nil {
		return fmt.Errorf("connect tunnel: %w", err)
	}

	// Send initial registration with node info
	firstMsg := &agentpb.ClientMessage{
		NodeId:   nodeID,
		NodeName: nodeName,
		Payload:  &agentpb.ClientMessage_Heartbeat{Heartbeat: &agentpb.Heartbeat{Timestamp: time.Now().Unix()}},
	}
	if err := stream.Send(firstMsg); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	slog.Info("connected to server", "addr", serverAddr, "node", nodeID)

	ctx := stream.Context()

	// Metrics reporter goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		reportMetrics(stream, nodeID, nodeName)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportMetrics(stream, nodeID, nodeName)
			}
		}
	}()

	// Heartbeat goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stream.Send(&agentpb.ClientMessage{
					NodeId:   nodeID,
					NodeName: nodeName,
					Payload:  &agentpb.ClientMessage_Heartbeat{Heartbeat: &agentpb.Heartbeat{Timestamp: time.Now().Unix()}},
				})
			}
		}
	}()

	// Receive loop: handle server commands
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		switch p := msg.Payload.(type) {
		case *agentpb.ServerMessage_CommandRequest:
			go handleCommand(stream, nodeID, nodeName, p.CommandRequest)
		case *agentpb.ServerMessage_FileUploadRequest:
			go handleFileUpload(stream, nodeID, nodeName, p.FileUploadRequest)
		case *agentpb.ServerMessage_FileDownloadRequest:
			go handleFileDownload(stream, nodeID, nodeName, p.FileDownloadRequest)
		case *agentpb.ServerMessage_FileListRequest:
			go handleFileList(stream, nodeID, nodeName, p.FileListRequest)
		}
	}
}

func reportMetrics(stream agentpb.TunnelService_ConnectClient, nodeID, nodeName string) {
	metrics := collectMetrics()
	stream.Send(&agentpb.ClientMessage{
		NodeId:   nodeID,
		NodeName: nodeName,
		Payload: &agentpb.ClientMessage_Metrics{
			Metrics: metrics,
		},
	})
}

func collectMetrics() *agentpb.MetricsReport {
	m := &agentpb.MetricsReport{}

	// CPU usage (quick sample)
	m.CpuCores = uint64(runtime.NumCPU())
	if cpuUsage, err := quickCPUUsage(); err == nil {
		m.CpuUsage = cpuUsage
	}

	// Load average
	if load, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(load))
		if len(parts) >= 3 {
			fmt.Sscanf(parts[0], "%f", &m.LoadAvg_1)
			fmt.Sscanf(parts[1], "%f", &m.LoadAvg_5)
			fmt.Sscanf(parts[2], "%f", &m.LoadAvg_15)
		}
	}

	// Memory
	if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
		content := string(meminfo)
		m.MemTotal = parseMemKB(content, "MemTotal:") * 1024
		memAvail := parseMemKB(content, "MemAvailable:") * 1024
		m.MemUsed = m.MemTotal - memAvail
		m.SwapTotal = parseMemKB(content, "SwapTotal:") * 1024
		swapFree := parseMemKB(content, "SwapFree:") * 1024
		m.SwapUsed = m.SwapTotal - swapFree
	}

	// Disk
	if out, err := exec.Command("df", "-B1", "/").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 3 {
				fmt.Sscanf(fields[1], "%d", &m.DiskTotal)
				fmt.Sscanf(fields[2], "%d", &m.DiskUsed)
			}
		}
	}

	// Uptime
	if up, err := os.ReadFile("/proc/uptime"); err == nil {
		var sec uint64
		fmt.Sscanf(string(up), "%d", &sec)
		m.Uptime = sec
	}

	// OS and Docker
	if release, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(release), "\n") {
			if strings.HasPrefix(line, "ID=") {
				m.Os = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			}
		}
	}
	m.Arch = runtime.GOARCH
	if out, err := exec.Command("docker", "--version").Output(); err == nil {
		m.DockerVersion = strings.TrimSpace(strings.ReplaceAll(string(out), "\n", ""))
	}

	return m
}

func quickCPUUsage() (float64, error) {
	stat1, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	var idle1, total1 uint64
	parseCPUStat(string(stat1), &idle1, &total1)

	time.Sleep(500 * time.Millisecond)

	stat2, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	var idle2, total2 uint64
	parseCPUStat(string(stat2), &idle2, &total2)

	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta == 0 {
		return 0, nil
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, nil
}

func parseCPUStat(stat string, idle, total *uint64) {
	lines := strings.Split(stat, "\n")
	if len(lines) == 0 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return
	}
	var sum uint64
	for i := 1; i < len(fields); i++ {
		var v uint64
		fmt.Sscanf(fields[i], "%d", &v)
		sum += v
		if i == 4 {
			*idle = v
		}
	}
	*total = sum
}

func parseMemKB(content, prefix string) uint64 {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var v uint64
				fmt.Sscanf(fields[1], "%d", &v)
				return v
			}
		}
	}
	return 0
}

func handleCommand(stream agentpb.TunnelService_ConnectClient, nodeID, nodeName string, req *agentpb.CommandRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxInt(int(req.TimeoutSeconds), 300))*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", req.Command)
	stdout, err := cmd.Output()
	stderr := ""
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		} else {
			stderr = err.Error()
			exitCode = 1
		}
	}

	stream.Send(&agentpb.ClientMessage{
		NodeId:   nodeID,
		NodeName: nodeName,
		Payload: &agentpb.ClientMessage_CommandResponse{
			CommandResponse: &agentpb.CommandResponse{
				RequestId: req.RequestId,
				Stdout:    string(stdout),
				Stderr:    stderr,
				ExitCode:  int32(exitCode),
				Done:      true,
			},
		},
	})
}

func handleFileUpload(stream agentpb.TunnelService_ConnectClient, nodeID, nodeName string, req *agentpb.FileUploadRequest) {
	mode := os.FileMode(0644)
	if req.Mode > 0 {
		mode = os.FileMode(req.Mode)
	}
	err := os.WriteFile(req.RemotePath, req.Content, mode)
	resp := &agentpb.CommandResponse{
		RequestId: req.RequestId,
		Done:      true,
	}
	if err != nil {
		resp.Stderr = err.Error()
		resp.ExitCode = 1
	} else {
		resp.Stdout = "ok"
	}
	stream.Send(&agentpb.ClientMessage{
		NodeId:   nodeID,
		NodeName: nodeName,
		Payload:  &agentpb.ClientMessage_CommandResponse{CommandResponse: resp},
	})
}

func handleFileDownload(stream agentpb.TunnelService_ConnectClient, nodeID, nodeName string, req *agentpb.FileDownloadRequest) {
	data, err := os.ReadFile(req.RemotePath)
	if err != nil {
		stream.Send(&agentpb.ClientMessage{
			NodeId: nodeID, NodeName: nodeName,
			Payload: &agentpb.ClientMessage_FileData{FileData: &agentpb.FileData{
				RequestId: req.RequestId, Error: err.Error(),
			}},
		})
		return
	}
	stream.Send(&agentpb.ClientMessage{
		NodeId: nodeID, NodeName: nodeName,
		Payload: &agentpb.ClientMessage_FileData{FileData: &agentpb.FileData{
			RequestId: req.RequestId, Data: data, Eof: true,
		}},
	})
}

func handleFileList(stream agentpb.TunnelService_ConnectClient, nodeID, nodeName string, req *agentpb.FileListRequest) {
	dirPath := req.Path
	if dirPath == "" {
		dirPath = "/"
	}
	entries, err := os.ReadDir(dirPath)
	resp := &agentpb.FileListResponse{RequestId: req.RequestId}
	if err != nil {
		resp.Error = err.Error()
	} else {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			resp.Entries = append(resp.Entries, &agentpb.FileEntry{
				Name:    e.Name(),
				Path:    dirPath + "/" + e.Name(),
				IsDir:   e.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime().Unix(),
				Mode:    info.Mode().String(),
			})
		}
	}
	stream.Send(&agentpb.ClientMessage{
		NodeId: nodeID, NodeName: nodeName,
		Payload: &agentpb.ClientMessage_FileList{FileList: resp},
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure we don't exit on signal — keep trying to reconnect
func init() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		slog.Info("agent shutting down")
		os.Exit(0)
	}()
}
