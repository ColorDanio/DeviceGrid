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
