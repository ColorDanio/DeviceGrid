package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/michael/device_grid/internal/model"
)

type NodeRepository struct {
	db *sql.DB
}

func (r *NodeRepository) Create(ctx context.Context, n *model.Node) error {
	tags, _ := json.Marshal(n.Tags)
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	n.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO nodes (id, name, host, port, username, auth_mode, password_enc, private_key_enc,
			transport_mode, agent_port, status, tags, os, arch, docker_version, rke2_role, cluster_id,
			country, country_code, region, isp, last_seen_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Host, n.Port, n.Username, n.AuthMode, n.PasswordEnc, n.PrivateKeyEnc,
		n.TransportMode, n.AgentPort, n.Status, string(tags), n.OS, n.Arch, n.DockerVersion,
		n.RKE2Role, n.ClusterID, n.Country, n.CountryCode, n.Region, n.ISP, n.LastSeenAt, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

func (r *NodeRepository) GetByID(ctx context.Context, id string) (*model.Node, error) {
	n := &model.Node{}
	var tagsJSON string
	var lastSeen sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, host, port, username, auth_mode, password_enc, private_key_enc,
			transport_mode, agent_port, status, tags, os, arch, docker_version, rke2_role,
			cluster_id, country, country_code, region, isp, last_seen_at, created_at, updated_at
		FROM nodes WHERE id = ?`, id).Scan(
		&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.AuthMode, &n.PasswordEnc, &n.PrivateKeyEnc,
		&n.TransportMode, &n.AgentPort, &n.Status, &tagsJSON, &n.OS, &n.Arch, &n.DockerVersion,
		&n.RKE2Role, &n.ClusterID, &n.Country, &n.CountryCode, &n.Region, &n.ISP, &lastSeen, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	if lastSeen.Valid {
		n.LastSeenAt = lastSeen.Time
	}
	_ = json.Unmarshal([]byte(tagsJSON), &n.Tags)
	return n, nil
}

func (r *NodeRepository) List(ctx context.Context, filter model.NodeFilter) ([]*model.Node, error) {
	query := `SELECT id, name, host, port, username, auth_mode, password_enc, private_key_enc,
		transport_mode, agent_port, status, tags, os, arch, docker_version, rke2_role,
		cluster_id, country, country_code, region, isp, last_seen_at, created_at, updated_at FROM nodes WHERE 1=1`
	args := []interface{}{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Tag != "" {
		query += " AND tags LIKE ?"
		args = append(args, "%\""+filter.Tag+"\"%")
	}
	if filter.Search != "" {
		query += " AND (name LIKE ? OR host LIKE ?)"
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	return scanNodes(rows)
}

func (r *NodeRepository) Update(ctx context.Context, n *model.Node) error {
	tags, _ := json.Marshal(n.Tags)
	n.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE nodes SET name=?, host=?, port=?, username=?, auth_mode=?, password_enc=?,
			private_key_enc=?, transport_mode=?, agent_port=?, status=?, tags=?, os=?, arch=?,
			docker_version=?, rke2_role=?, cluster_id=?, country=?, country_code=?, region=?, isp=?,
			last_seen_at=?, updated_at=?
		WHERE id=?`,
		n.Name, n.Host, n.Port, n.Username, n.AuthMode, n.PasswordEnc, n.PrivateKeyEnc,
		n.TransportMode, n.AgentPort, n.Status, string(tags), n.OS, n.Arch,
		n.DockerVersion, n.RKE2Role, n.ClusterID, n.Country, n.CountryCode, n.Region, n.ISP,
		n.LastSeenAt, n.UpdatedAt, n.ID,
	)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	return nil
}

func (r *NodeRepository) UpdateStatus(ctx context.Context, id string, status model.NodeStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE nodes SET status=?, last_seen_at=?, updated_at=? WHERE id=?`,
		status, time.Now(), time.Now(), id)
	if err != nil {
		return fmt.Errorf("update node status: %w", err)
	}
	return nil
}

func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM nodes WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	return nil
}

func scanNodes(rows *sql.Rows) ([]*model.Node, error) {
	var nodes []*model.Node
	for rows.Next() {
		n := &model.Node{}
		var tagsJSON string
		var lastSeen sql.NullTime
		if err := rows.Scan(
			&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.AuthMode, &n.PasswordEnc, &n.PrivateKeyEnc,
			&n.TransportMode, &n.AgentPort, &n.Status, &tagsJSON, &n.OS, &n.Arch, &n.DockerVersion,
			&n.RKE2Role, &n.ClusterID, &n.Country, &n.CountryCode, &n.Region, &n.ISP, &lastSeen, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		if lastSeen.Valid {
			n.LastSeenAt = lastSeen.Time
		}
		_ = json.Unmarshal([]byte(tagsJSON), &n.Tags)
		nodes = append(nodes, n)
	}
	return nodes, nil
}
