package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/michael/device_grid/internal/store/repo"
)

type Store struct {
	db          *sql.DB
	nodeRepo    *NodeRepository
	taskRepo    *DeployTaskRepository
	resultRepo  *DeployResultRepository
	containerRepo *ContainerRepository
	clusterRepo *ClusterRepository
	userRepo    *UserRepository
}

func init() {
	repo.Register("sqlite", func(ctx context.Context) (repo.Repositories, error) {
		return New(ctx, "./data/device_grid.db")
	})
}

func New(ctx context.Context, dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	s := &Store{db: db}
	s.nodeRepo = &NodeRepository{db: db}
	s.taskRepo = &DeployTaskRepository{db: db}
	s.resultRepo = &DeployResultRepository{db: db}
	s.containerRepo = &ContainerRepository{db: db}
	s.clusterRepo = &ClusterRepository{db: db}
	s.userRepo = &UserRepository{db: db}
	return s, nil
}

func (s *Store) Nodes() repo.NodeRepository         { return s.nodeRepo }
func (s *Store) DeployTasks() repo.DeployTaskRepository { return s.taskRepo }
func (s *Store) DeployResults() repo.DeployResultRepository { return s.resultRepo }
func (s *Store) Containers() repo.ContainerRepository { return s.containerRepo }
func (s *Store) Clusters() repo.ClusterRepository    { return s.clusterRepo }
func (s *Store) Users() repo.UserRepository          { return s.userRepo }

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func migrate(ctx context.Context, db *sql.DB) error {
	schema := schemaSQL
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	// Run column migrations for existing databases
	migrations := []string{
		`ALTER TABLE nodes ADD COLUMN country TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN country_code TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN region TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN isp TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN host_key TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN force_password_change INTEGER DEFAULT 0`,
	}
	for _, m := range migrations {
		db.ExecContext(ctx, m) // Ignore "duplicate column" errors
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS nodes (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    host            TEXT NOT NULL,
    port            INTEGER NOT NULL DEFAULT 22,
    username        TEXT NOT NULL DEFAULT 'root',
    auth_mode       TEXT NOT NULL DEFAULT 'password',
    password_enc    TEXT DEFAULT '',
    private_key_enc TEXT DEFAULT '',
    transport_mode  TEXT NOT NULL DEFAULT 'ssh',
    agent_port      INTEGER NOT NULL DEFAULT 9090,
    status          TEXT NOT NULL DEFAULT 'untrusted',
    tags            TEXT DEFAULT '[]',
    os              TEXT DEFAULT '',
    arch            TEXT DEFAULT '',
    docker_version  TEXT DEFAULT '',
    rke2_role       TEXT DEFAULT '',
    cluster_id      TEXT DEFAULT '',
    country         TEXT DEFAULT '',
    country_code    TEXT DEFAULT '',
    region          TEXT DEFAULT '',
    isp             TEXT DEFAULT '',
    last_seen_at    DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_cluster ON nodes(cluster_id);

CREATE TABLE IF NOT EXISTS deploy_tasks (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,
    node_ids     TEXT NOT NULL DEFAULT '[]',
    payload      TEXT DEFAULT '',
    timeout      INTEGER DEFAULT 0,
    concurrency  INTEGER DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    created_by   TEXT DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at   DATETIME,
    finished_at  DATETIME
);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_status ON deploy_tasks(status);

CREATE TABLE IF NOT EXISTS deploy_results (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL,
    node_id     TEXT NOT NULL,
    node_name   TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'running',
    exit_code   INTEGER DEFAULT 0,
    output      TEXT DEFAULT '',
    error       TEXT DEFAULT '',
    duration_ms INTEGER DEFAULT 0,
    started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    FOREIGN KEY (task_id) REFERENCES deploy_tasks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_deploy_results_task ON deploy_results(task_id);

CREATE TABLE IF NOT EXISTS containers (
    id        TEXT PRIMARY KEY,
    node_id   TEXT NOT NULL,
    name      TEXT DEFAULT '',
    image     TEXT DEFAULT '',
    status    TEXT DEFAULT '',
    state     TEXT DEFAULT '',
    ports     TEXT DEFAULT '[]',
    env       TEXT DEFAULT '{}',
    labels    TEXT DEFAULT '{}',
    created   DATETIME,
    UNIQUE(node_id, name)
);
CREATE INDEX IF NOT EXISTS idx_containers_node ON containers(node_id);

CREATE TABLE IF NOT EXISTS clusters (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT DEFAULT '',
    server_node TEXT DEFAULT '',
    config      TEXT DEFAULT '',
    nodes       TEXT DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'provisioning',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
