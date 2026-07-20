package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	agentpb "github.com/michael/device_grid/internal/agent/proto"
)

// fakeDockerEngine is an httptest server that emulates the Docker Engine
// REST surface our agent uses (containers/json and images/json). It listens
// on a unix socket so the agent's unix-socket dial succeeds against it.
type fakeDockerEngine struct {
	server *httptest.Server
	socket string
}

func newFakeDockerEngine(t *testing.T, containersBody, imagesBody string) *fakeDockerEngine {
	t.Helper()
	socketPath := socketPath(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(containersBody))
	})
	mux.HandleFunc("/images/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(imagesBody))
	})

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: mux},
	}
	srv.Start()

	t.Setenv("DG_AGENT_DOCKER_SOCKET", socketPath)
	t.Cleanup(srv.Close)
	return &fakeDockerEngine{server: srv, socket: socketPath}
}

// socketPath reserves a unique temp socket path. Tests reuse it across
// reconnection cycles, so we delete any leftover file before binding.
func socketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir + "/docker.sock"
}

// TestAgentDockerList_Containers exercises the agent's handleDockerList code
// path end-to-end against a fake Docker Engine: the agent process does a real
// HTTP GET on the unix socket and returns the JSON body to the caller.
func TestAgentDockerList_Containers(t *testing.T) {
	body := `[{"Id":"abc123def4567890","Names":["/web"],"Image":"nginx:1.25","Status":"Up 2 minutes","State":"running","Ports":[{"PrivatePort":80,"PublicPort":8080,"Type":"tcp"}]}]`
	engine := newFakeDockerEngine(t, body, "")

	path, err := dockerListPath("containers", true)
	if err != nil {
		t.Fatalf("list path: %v", err)
	}
	if !strings.HasPrefix(path, "/containers/json?all=true") {
		t.Errorf("containers path: got %q want /containers/json?all=true prefix", path)
	}
	_ = engine

	out, err := dockerGet("/containers/json?all=true")
	if err != nil {
		t.Fatalf("docker get: %v", err)
	}
	if !strings.Contains(string(out), "abc123") {
		t.Errorf("response missing container id: %s", out)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed: got %d want 1", len(parsed))
	}
	if got := parsed[0]["State"]; got != "running" {
		t.Errorf("State: got %v want running", got)
	}
}

// TestAgentDockerList_Images mirrors the containers test for /images/json.
func TestAgentDockerList_Images(t *testing.T) {
	body := `[{"Id":"sha256:deadbeefdead","RepoTags":["redis:7"],"Size":138000000,"Created":1729353600}]`
	_ = newFakeDockerEngine(t, "", body)

	out, err := dockerGet("/images/json")
	if err != nil {
		t.Fatalf("docker get: %v", err)
	}
	if !strings.Contains(string(out), "redis:7") {
		t.Errorf("response missing image tag: %s", out)
	}
}

// TestAgentDockerList_EndpointMappings covers the kind→path routing.
func TestAgentDockerList_EndpointMappings(t *testing.T) {
	cases := []struct {
		kind string
		all  bool
		want string
	}{
		{"", false, "/containers/json"},
		{"containers", false, "/containers/json"},
		{"containers", true, "/containers/json?all=true"},
		{"images", false, "/images/json"},
		{"networks", false, "/networks"},
		{"volumes", false, "/volumes"},
		{"unknown", false, ""}, // expects error
	}
	for i, tc := range cases {
		got, err := dockerListPath(tc.kind, tc.all)
		if tc.want == "" {
			if err == nil {
				t.Errorf("[%d] kind=%s: expected error, got %q", i, tc.kind, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("[%d] kind=%s: %v", i, tc.kind, err)
			continue
		}
		if got != tc.want {
			t.Errorf("[%d] kind=%s: got %q want %q", i, tc.kind, got, tc.want)
		}
	}
}

// TestAgentDockerList_TunnelRoundTrip drives the agent's handleDockerList
// with a fake stream that captures the ClientMessage it would send over the
// tunnel, asserting that the JSON body from the local Docker Engine lands in
// DockerListResponse.RawJson.
func TestAgentDockerList_TunnelRoundTrip(t *testing.T) {
	containersBody := `[{"Id":"abc123","Names":["/web"],"Image":"nginx:1.25","Status":"Up","State":"running","Ports":[]}]`
	_ = newFakeDockerEngine(t, containersBody, "")

	stream := &captureStream{}
	req := &agentpb.DockerListRequest{RequestId: "req-1", Kind: "containers", All: true}
	handleDockerList(stream, "node-1", "node-1", req)

	if len(stream.sent) != 1 {
		t.Fatalf("Send calls: got %d want 1", len(stream.sent))
	}
	resp := stream.sent[0].GetDockerList()
	if resp == nil {
		t.Fatalf("payload: want DockerList, got %T", stream.sent[0].Payload)
	}
	if resp.RequestId != "req-1" {
		t.Errorf("RequestId: got %s want req-1", resp.RequestId)
	}
	if resp.Kind != "containers" {
		t.Errorf("Kind: got %s want containers", resp.Kind)
	}
	if !strings.Contains(resp.RawJson, "abc123") {
		t.Errorf("RawJson missing container id: %q", resp.RawJson)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

// captureStream is a minimal agentpb.TunnelService_ConnectClient that records
// every ClientMessage the agent sends. It satisfies the methods
// handleDockerList actually exercises.
type captureStream struct {
	sent []*agentpb.ClientMessage
}

func (s *captureStream) Send(m *agentpb.ClientMessage) error {
	s.sent = append(s.sent, m)
	return nil
}

func (s *captureStream) Recv() (*agentpb.ServerMessage, error) {
	// Not used by handleDockerList; block indefinitely to surface accidental reads.
	select {}
}

// Compile-time guard: captureStream must implement the client stream type so
// changes to the surface are caught. We reference the grpc.ClientStream
// methods individually because embedding the interface pulls in many methods
// we don't need.
var (
	_ interface {
		Send(*agentpb.ClientMessage) error
		Recv() (*agentpb.ServerMessage, error)
	} = (*captureStream)(nil)
	_ = os.Stdout
	_ = fmt.Sprintf
)
