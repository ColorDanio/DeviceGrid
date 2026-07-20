package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

// dockerSocketPath is the default Docker Engine listen socket. Overridable via
// DG_AGENT_DOCKER_SOCKET env var for tests and non-standard installs.
const dockerSocketPath = "/var/run/docker.sock"

// dockerSocketOverride lets tests redirect the agent's Docker API client to a
// fake listener (set via DG_AGENT_DOCKER_SOCKET in main_test.go).
func dockerSocketOverride() string {
	if v := envOrEmpty("DG_AGENT_DOCKER_SOCKET"); v != "" {
		return v
	}
	return dockerSocketPath
}

func envOrEmpty(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// dockerClient builds an http.Client over the unix socket. Cheap to construct.
func dockerClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", dockerSocketOverride(), 5*time.Second)
			},
		},
	}
}

// clientMessageStream is the narrow subset of grpc.BidiStreamingClient that
// handleDockerList needs. Declaring it locally keeps the handler testable
// without pulling the full grpc.ClientStream surface (12+ methods) into mocks.
type clientMessageStream interface {
	Send(*agentpb.ClientMessage) error
}

// handleDockerList serves DockerListRequest by calling the local Docker Engine
// REST API over /var/run/docker.sock. The agent stays a thin proxy: it returns
// the raw JSON body so the server can parse against the Engine schema.
func handleDockerList(stream clientMessageStream, nodeID, nodeName string, req *agentpb.DockerListRequest) {
	resp := &agentpb.DockerListResponse{RequestId: req.RequestId, Kind: req.Kind}

	path, err := dockerListPath(req.Kind, req.All)
	if err != nil {
		resp.Error = err.Error()
		_ = stream.Send(&agentpb.ClientMessage{
			NodeId: nodeID, NodeName: nodeName,
			Payload: &agentpb.ClientMessage_DockerList{DockerList: resp},
		})
		return
	}

	body, err := dockerGet(path)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.RawJson = string(body)
	}
	_ = stream.Send(&agentpb.ClientMessage{
		NodeId: nodeID, NodeName: nodeName,
		Payload: &agentpb.ClientMessage_DockerList{DockerList: resp},
	})
}

// dockerListPath maps a (kind, all) request to an Engine REST endpoint.
func dockerListPath(kind string, all bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "containers":
		path := "/containers/json"
		if all {
			path += "?all=true"
		}
		return path, nil
	case "images":
		return "/images/json", nil
	case "networks":
		return "/networks", nil
	case "volumes":
		return "/volumes", nil
	default:
		return "", fmt.Errorf("unsupported docker list kind: %q", kind)
	}
}

func dockerGet(path string) ([]byte, error) {
	client := dockerClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("docker engine %s: %s", resp.Status, string(body))
	}
	return io.ReadAll(resp.Body)
}
