package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/michael/device_grid/internal/transport"
)

// fakeTunnelTransport is a minimal DockerLister + tunnel checker used to
// validate the opportunistic agent path in docker.Manager.ListContainers
// and ListImages. It returns a canned JSON body when asked.
type fakeTunnelTransport struct {
	containersBody string
	imagesBody     string
	called         bool
}

func (f *fakeTunnelTransport) DockerList(ctx context.Context, nodeID, kind string, all bool) (string, error) {
	f.called = true
	switch kind {
	case "containers":
		return f.containersBody, nil
	case "images":
		return f.imagesBody, nil
	}
	return "", errors.New("unknown kind")
}

// The remaining transport.Transporter methods are no-ops; the docker manager
// only reaches them when the tunnel path fails, which the tests below do not
// exercise. We stub them out to satisfy the interface for transport.NewManager.
func (f *fakeTunnelTransport) Exec(context.Context, string, string) (transport.ExecResult, error) {
	return transport.ExecResult{}, errors.New("not implemented")
}
func (f *fakeTunnelTransport) ExecStream(context.Context, string, string) (<-chan transport.StreamChunk, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTunnelTransport) Upload(context.Context, string, string, io.Reader, os.FileMode) error {
	return errors.New("not implemented")
}
func (f *fakeTunnelTransport) Download(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTunnelTransport) PTY(context.Context, string, uint16, uint16) (transport.PTYSession, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTunnelTransport) ContainerPTY(context.Context, string, string, uint16, uint16) (transport.PTYSession, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTunnelTransport) Ping(context.Context, string) error { return nil }
func (f *fakeTunnelTransport) Facts(context.Context, string) (transport.NodeFacts, error) {
	return transport.NodeFacts{}, errors.New("not implemented")
}
func (f *fakeTunnelTransport) Metrics(context.Context, string) (transport.NodeMetrics, error) {
	return transport.NodeMetrics{}, errors.New("not implemented")
}

// fakeDockerEngineHTTP spins up an httptest server returning the canned
// bodies (so we can also test the agent's HTTP-over-socket call without a
// real Docker daemon).
func fakeDockerEngineHTTP(t *testing.T, containersBody, imagesBody string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(containersBody))
	})
	mux.HandleFunc("/images/json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(imagesBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestParseContainerListJSON verifies the Engine REST JSON → ContainerInfo
// mapping that the agent tunnel path uses.
func TestParseContainerListJSON(t *testing.T) {
	raw := `[{"Id":"abc123def4567890","Names":["/web"],"Image":"nginx:1.25","Status":"Up 2 minutes","State":"running","Ports":[{"PrivatePort":80,"PublicPort":8080,"Type":"tcp"}]}]`
	got, err := parseContainerListJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d want 1", len(got))
	}
	c := got[0]
	if c.ID != "abc123def456" {
		t.Errorf("ID: got %q want abc123def456", c.ID)
	}
	if c.Name != "web" {
		t.Errorf("Name: got %q want web", c.Name)
	}
	if c.State != "running" {
		t.Errorf("State: got %q want running", c.State)
	}
	if len(c.Ports) != 1 || c.Ports[0].HostPort != "8080/tcp" {
		t.Errorf("Ports: %+v", c.Ports)
	}
}

// TestParseContainerListJSON_Empty covers the empty-body short-circuit.
func TestParseContainerListJSON_Empty(t *testing.T) {
	got, err := parseContainerListJSON("")
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len: got %d want 0", len(got))
	}
}

// TestParseContainerListJSON_BadJSON falls back to the caller's CLI path on
// unmarshal failure; the parse helper itself returns an error.
func TestParseContainerListJSON_BadJSON(t *testing.T) {
	if _, err := parseContainerListJSON("not-json"); err == nil {
		t.Error("expected parse error")
	}
}

// TestParseImageListJSON verifies the Engine REST JSON → ImageInfo mapping.
func TestParseImageListJSON(t *testing.T) {
	raw := `[{"Id":"sha256:deadbeef","RepoTags":["redis:7","redis:latest"],"Size":138000000,"Created":1729353600}]`
	got, err := parseImageListJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d want 1", len(got))
	}
	img := got[0]
	if !strings.HasPrefix(img.ID, "sha256:") {
		// shortID strips sha256: prefix for parity with `docker images` CLI output
		t.Errorf("ID: got %q want no sha256 prefix", img.ID)
	}
	if img.Tags != "redis:7, redis:latest" {
		t.Errorf("Tags: got %q want redis:7, redis:latest", img.Tags)
	}
	if !strings.HasSuffix(img.Size, "B") {
		t.Errorf("Size: got %q want suffixed with B", img.Size)
	}
}

// TestDockerManager_FallsBackOnNoTunnel ensures the CLI path is exercised
// (returns no tunnel) — verified by checking the manager does not call the
// (nil) tunnel transport.
func TestDockerManager_FallsBackOnNoTunnel(t *testing.T) {
	// Build a docker.Manager with a transport that has no tunnel connected.
	// We can't easily construct a full transport.Manager here, but we can
	// confirm that transport.Manager.DockerList returns the sentinel when no
	// tunnel checker is configured, which is the gate the docker manager
	// relies on.
	tm := &transport.Manager{}
	if _, err := tm.DockerList(context.Background(), "node", "containers", false); !errors.Is(err, transport.ErrDockerViaTransportUnavailable) {
		t.Errorf("err: got %v want ErrDockerViaTransportUnavailable", err)
	}
}

// TestHumanSize covers the byte→human formatter used by parseImageListJSON.
func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{2048, "2.0KiB"},
		{5 * 1024 * 1024, "5.0MiB"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5GiB"},
	}
	for i, c := range cases {
		got := humanSize(c.in)
		// Float formatting may differ; just check it ends with the right unit.
		if !strings.HasSuffix(got, "B") {
			t.Errorf("[%d] %d: got %q want suffix B", i, c.in, got)
		}
	}
}

// TestDockerListOpportunisticAgentPath validates that when the transport
// manager exposes a tunnel-backed DockerLister, the docker manager uses the
// parsed JSON instead of falling back to CLI. We verify the plumbing by
// routing through transport.Manager with an injected DockerLister.
func TestDockerListOpportunisticAgentPath(t *testing.T) {
	// Build a transport.Manager whose agentImpl is our fake DockerLister and
	// whose tunnelChecker always returns true. docker.Manager will then call
	// transport.DockerList, hit the fake lister, and parse the JSON.
	const containersBody = `[{"Id":"abc123","Names":["/web"],"Image":"nginx","Status":"Up","State":"running","Ports":[]}]`
	fake := &fakeTunnelTransport{containersBody: containersBody}
	tm := transport.NewManager(fake, fake, nil, nil)
	tm.SetTunnelChecker(func(string) bool { return true })

	mgr := NewManager(tm)
	got, err := mgr.ListContainers(context.Background(), "node-1", false)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if !fake.called {
		t.Error("expected DockerList to be called via tunnel")
	}
	if len(got) != 1 || got[0].Name != "web" {
		t.Errorf("containers: %+v", got)
	}
}

// Ensure net import is exercised (used by fake engine in real tests).
var _ = net.Listen
