package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/michael/device_grid/internal/model"
)

type ContainerRepository struct {
	db *sql.DB
}

func (r *ContainerRepository) ListByNodeID(ctx context.Context, nodeID string) ([]*model.Container, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, node_id, name, image, status, state, ports, env, labels, created
		FROM containers WHERE node_id = ? ORDER BY name`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer rows.Close()
	return scanContainers(rows)
}

func (r *ContainerRepository) Upsert(ctx context.Context, c *model.Container) error {
	ports, _ := json.Marshal(c.Ports)
	env, _ := json.Marshal(c.Env)
	labels, _ := json.Marshal(c.Labels)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO containers (id, node_id, name, image, status, state, ports, env, labels, created)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, name) DO UPDATE SET
			image=excluded.image, status=excluded.status, state=excluded.state,
			ports=excluded.ports, env=excluded.env, labels=excluded.labels, created=excluded.created`,
		c.ID, c.NodeID, c.Name, c.Image, c.Status, c.State, string(ports), string(env), string(labels), c.Created,
	)
	if err != nil {
		return fmt.Errorf("upsert container: %w", err)
	}
	return nil
}

func (r *ContainerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM containers WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete container: %w", err)
	}
	return nil
}

func scanContainers(rows *sql.Rows) ([]*model.Container, error) {
	var containers []*model.Container
	for rows.Next() {
		c := &model.Container{}
		var portsJSON, envJSON, labelsJSON string
		if err := rows.Scan(
			&c.ID, &c.NodeID, &c.Name, &c.Image, &c.Status, &c.State,
			&portsJSON, &envJSON, &labelsJSON, &c.Created,
		); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		_ = json.Unmarshal([]byte(portsJSON), &c.Ports)
		_ = json.Unmarshal([]byte(envJSON), &c.Env)
		_ = json.Unmarshal([]byte(labelsJSON), &c.Labels)
		containers = append(containers, c)
	}
	return containers, nil
}

type ClusterRepository struct {
	db *sql.DB
}

func (r *ClusterRepository) Create(ctx context.Context, c *model.Cluster) error {
	nodes, _ := json.Marshal(c.Nodes)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO clusters (id, name, version, server_node, config, nodes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Version, c.ServerNode, c.Config, string(nodes), c.Status, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}
	return nil
}

func (r *ClusterRepository) GetByID(ctx context.Context, id string) (*model.Cluster, error) {
	c := &model.Cluster{}
	var nodesJSON string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, version, server_node, config, nodes, status, created_at, updated_at
		FROM clusters WHERE id = ?`, id).Scan(
		&c.ID, &c.Name, &c.Version, &c.ServerNode, &c.Config, &nodesJSON, &c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get cluster %s: %w", id, err)
	}
	_ = json.Unmarshal([]byte(nodesJSON), &c.Nodes)
	return c, nil
}

func (r *ClusterRepository) List(ctx context.Context) ([]*model.Cluster, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, version, server_node, config, nodes, status, created_at, updated_at FROM clusters ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*model.Cluster
	for rows.Next() {
		c := &model.Cluster{}
		var nodesJSON string
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Version, &c.ServerNode, &c.Config, &nodesJSON, &c.Status, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		_ = json.Unmarshal([]byte(nodesJSON), &c.Nodes)
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func (r *ClusterRepository) Update(ctx context.Context, c *model.Cluster) error {
	nodes, _ := json.Marshal(c.Nodes)
	_, err := r.db.ExecContext(ctx, `
		UPDATE clusters SET name=?, version=?, server_node=?, config=?, nodes=?, status=?, updated_at=? WHERE id=?`,
		c.Name, c.Version, c.ServerNode, c.Config, string(nodes), c.Status, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
	}
	return nil
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM clusters WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	return nil
}

type UserRepository struct {
	db *sql.DB
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Role, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?`, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by username %s: %w", username, err)
	}
	return u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET username=?, password_hash=?, role=? WHERE id=?`,
		u.Username, u.PasswordHash, u.Role, u.ID)
	if err != nil {
		return fmt.Errorf("update user %s: %w", u.ID, err)
	}
	return nil
}

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, username, password_hash, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
