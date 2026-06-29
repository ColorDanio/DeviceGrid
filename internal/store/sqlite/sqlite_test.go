package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/config"
	"github.com/michael/device_grid/internal/model"
)

// newTestStore returns an in-memory SQLite store for testing.
// Uses a temp file (modernc.org/sqlite doesn't support ":memory:" DSN
// the same way as the C driver).
func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := config.DatabaseConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: dbPath},
	}

	repo, err := New(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	_ = cfg
	return repo
}

func makeNode(name string) *model.Node {
	return &model.Node{
		ID:            uuid.NewString(),
		Name:          name,
		Host:          "10.0.0.1",
		Port:          22,
		Username:      "root",
		AuthMode:      model.AuthPassword,
		TransportMode: model.TransportSSH,
		AgentPort:     9090,
		Status:        model.NodeStatusUntrusted,
		Tags:          []string{"test"},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// ====================
// NodeRepository tests
// ====================

func TestNodeRepository_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	node := makeNode("test-node-1")
	if err := s.Nodes().Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Nodes().GetByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != node.Name {
		t.Errorf("name: got %s want %s", got.Name, node.Name)
	}
	if got.Host != node.Host {
		t.Errorf("host: got %s want %s", got.Host, node.Host)
	}
}

func TestNodeRepository_GetByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.Nodes().GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestNodeRepository_List_FilterByStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	online := makeNode("online-node")
	online.Status = model.NodeStatusOnline
	offline := makeNode("offline-node")
	offline.Status = model.NodeStatusOffline

	_ = s.Nodes().Create(ctx, online)
	_ = s.Nodes().Create(ctx, offline)

	all, err := s.Nodes().List(ctx, model.NodeFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("list all: got %d want 2", len(all))
	}

	onlyOnline, err := s.Nodes().List(ctx, model.NodeFilter{Status: model.NodeStatusOnline})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(onlyOnline) != 1 {
		t.Errorf("list online: got %d want 1", len(onlyOnline))
	}
	if onlyOnline[0].ID != online.ID {
		t.Errorf("filter returned wrong node")
	}
}

func TestNodeRepository_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	node := makeNode("update-test")
	if err := s.Nodes().Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	node.Name = "updated-name"
	node.Status = model.NodeStatusOnline
	node.OS = "Ubuntu 22.04"
	if err := s.Nodes().Update(ctx, node); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.Nodes().GetByID(ctx, node.ID)
	if got.Name != "updated-name" {
		t.Errorf("name after update: got %s", got.Name)
	}
	if got.Status != model.NodeStatusOnline {
		t.Errorf("status after update: got %s", got.Status)
	}
	if got.OS != "Ubuntu 22.04" {
		t.Errorf("os after update: got %s", got.OS)
	}
}

func TestNodeRepository_UpdateStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	node := makeNode("status-test")
	_ = s.Nodes().Create(ctx, node)

	if err := s.Nodes().UpdateStatus(ctx, node.ID, model.NodeStatusOnline); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ := s.Nodes().GetByID(ctx, node.ID)
	if got.Status != model.NodeStatusOnline {
		t.Errorf("status: got %s want online", got.Status)
	}
}

func TestNodeRepository_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	node := makeNode("delete-test")
	_ = s.Nodes().Create(ctx, node)

	if err := s.Nodes().Delete(ctx, node.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := s.Nodes().GetByID(ctx, node.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestNodeRepository_UniqueID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	node := makeNode("dup-test")
	_ = s.Nodes().Create(ctx, node)

	// Creating a node with the same ID should fail (primary key constraint)
	dup := *node
	dup.Name = "dup-name-2"
	if err := s.Nodes().Create(ctx, &dup); err == nil {
		t.Error("expected error on duplicate ID")
	}
}

// ====================
// UserRepository tests
// ====================

func TestUserRepository_CreateAndGetByUsername(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u := &model.User{
		ID:           uuid.NewString(),
		Username:     "alice",
		PasswordHash: "bcrypt-hash",
		Role:         model.RoleAdmin,
		CreatedAt:    time.Now(),
	}
	if err := s.Users().Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Users().GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("username: got %s want alice", got.Username)
	}
	if got.Role != model.RoleAdmin {
		t.Errorf("role: got %s want admin", got.Role)
	}
}

func TestUserRepository_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u := &model.User{
		ID:           uuid.NewString(),
		Username:     "bob",
		PasswordHash: "old-hash",
		Role:         model.RoleViewer,
		CreatedAt:    time.Now(),
	}
	_ = s.Users().Create(ctx, u)

	u.PasswordHash = "new-hash"
	u.Role = model.RoleOperator
	if err := s.Users().Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.Users().GetByUsername(ctx, "bob")
	if got.PasswordHash != "new-hash" {
		t.Errorf("password: got %s want new-hash", got.PasswordHash)
	}
	if got.Role != model.RoleOperator {
		t.Errorf("role: got %s want operator", got.Role)
	}
}

func TestUserRepository_UniqueUsername(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u1 := &model.User{ID: uuid.NewString(), Username: "carol", PasswordHash: "h", Role: "viewer", CreatedAt: time.Now()}
	u2 := &model.User{ID: uuid.NewString(), Username: "carol", PasswordHash: "h", Role: "viewer", CreatedAt: time.Now()}

	if err := s.Users().Create(ctx, u1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	if err := s.Users().Create(ctx, u2); err == nil {
		t.Error("expected error on duplicate username")
	}
}

func TestUserRepository_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u := &model.User{ID: uuid.NewString(), Username: "dave", PasswordHash: "h", Role: "viewer", CreatedAt: time.Now()}
	_ = s.Users().Create(ctx, u)

	if err := s.Users().Delete(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := s.Users().GetByID(ctx, u.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestUserRepository_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Users().Create(ctx, &model.User{ID: uuid.NewString(), Username: "u1", PasswordHash: "h", Role: "viewer", CreatedAt: time.Now()})
	_ = s.Users().Create(ctx, &model.User{ID: uuid.NewString(), Username: "u2", PasswordHash: "h", Role: "admin", CreatedAt: time.Now()})

	users, err := s.Users().List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}
}

// ====================
// ClusterRepository tests
// ====================

func TestClusterRepository_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c := &model.Cluster{
		ID:         uuid.NewString(),
		Name:       "test-cluster",
		Version:    "v1.28.5",
		ServerNode: "node-1",
		Config:     "key: value",
		Nodes: []model.ClusterNode{
			{NodeID: "node-1", Role: model.RoleServer, Ready: true},
			{NodeID: "node-2", Role: model.RoleAgent, Ready: true},
		},
		Status:    model.ClusterProvisioning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.Clusters().Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Clusters().GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-cluster" {
		t.Errorf("name: got %s", got.Name)
	}
	if len(got.Nodes) != 2 {
		t.Errorf("nodes: got %d want 2", len(got.Nodes))
	}
}

func TestClusterRepository_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c := &model.Cluster{
		ID:        uuid.NewString(),
		Name:      "upd-cluster",
		Version:   "v1.28.5",
		Nodes:     []model.ClusterNode{{NodeID: "a", Role: model.RoleServer}},
		Status:    model.ClusterProvisioning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = s.Clusters().Create(ctx, c)

	c.Status = model.ClusterHealthy
	c.Nodes = []model.ClusterNode{
		{NodeID: "a", Role: model.RoleServer, Ready: true},
		{NodeID: "b", Role: model.RoleAgent, Ready: true},
		{NodeID: "c", Role: model.RoleAgent, Ready: true},
	}
	if err := s.Clusters().Update(ctx, c); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.Clusters().GetByID(ctx, c.ID)
	if got.Status != model.ClusterHealthy {
		t.Errorf("status: got %s want healthy", got.Status)
	}
	if len(got.Nodes) != 3 {
		t.Errorf("nodes: got %d want 3", len(got.Nodes))
	}
}

// ====================
// DeployTaskRepository tests
// ====================

func TestDeployTaskRepository_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	task := &model.DeployTask{
		ID:        uuid.NewString(),
		Name:      "deploy-app",
		Type:      "script",
		NodeIDs:   []string{"n1", "n2"},
		Status:    model.DeployPending,
		CreatedBy: "admin",
		CreatedAt: time.Now(),
	}
	if err := s.DeployTasks().Create(ctx, task); err != nil {
		t.Fatalf("create: %v", err)
	}

	tasks, err := s.DeployTasks().List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("got %d want 1", len(tasks))
	}
}

func TestDeployTaskRepository_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	task := &model.DeployTask{
		ID:        uuid.NewString(),
		Name:      "task-update",
		Type:      "script",
		NodeIDs:   []string{"n1"},
		Status:    model.DeployPending,
		CreatedBy: "admin",
		CreatedAt: time.Now(),
	}
	_ = s.DeployTasks().Create(ctx, task)

	now := time.Now()
	task.Status = model.DeployRunning
	task.StartedAt = &now
	if err := s.DeployTasks().Update(ctx, task); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.DeployTasks().GetByID(ctx, task.ID)
	if got.Status != model.DeployRunning {
		t.Errorf("status: got %s want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("started_at should be set")
	}
}

// ====================
// ContainerRepository tests
// ====================

func TestContainerRepository_UpsertAndListByNodeID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c := &model.Container{
		ID:     uuid.NewString(),
		NodeID: "node-1",
		Name:   "web",
		Image:  "nginx:latest",
		Status: model.ContainerRunning,
		State:  "running",
		Ports: []model.PortMapping{
			{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "tcp"},
		},
	}
	if err := s.Containers().Upsert(ctx, c); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	containers, err := s.Containers().ListByNodeID(ctx, "node-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(containers) != 1 {
		t.Errorf("got %d want 1", len(containers))
	}

	// Upsert with same node+name should update
	c.Status = "exited"
	if err := s.Containers().Upsert(ctx, c); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	containers, _ = s.Containers().ListByNodeID(ctx, "node-1")
	if len(containers) != 1 {
		t.Errorf("after upsert: got %d want 1 (no duplicate)", len(containers))
	}
	if containers[0].Status != "exited" {
		t.Errorf("status: got %s want exited", containers[0].Status)
	}
}

func TestContainerRepository_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c := &model.Container{ID: uuid.NewString(), NodeID: "n1", Name: "c1"}
	_ = s.Containers().Upsert(ctx, c)

	if err := s.Containers().Delete(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	containers, _ := s.Containers().ListByNodeID(ctx, "n1")
	if len(containers) != 0 {
		t.Errorf("after delete: got %d want 0", len(containers))
	}
}
