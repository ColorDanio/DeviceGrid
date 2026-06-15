package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/michael/device_grid/internal/model"
)

type DeployTaskRepository struct {
	db *sql.DB
}

func (r *DeployTaskRepository) Create(ctx context.Context, t *model.DeployTask) error {
	nodeIDs, _ := json.Marshal(t.NodeIDs)
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deploy_tasks (id, name, type, node_ids, payload, timeout, concurrency,
			status, created_by, created_at, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Type, string(nodeIDs), t.Payload, t.Timeout, t.Concurrency,
		t.Status, t.CreatedBy, t.CreatedAt, t.StartedAt, t.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("create deploy task: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) GetByID(ctx context.Context, id string) (*model.DeployTask, error) {
	t := &model.DeployTask{}
	var nodeIDsJSON string
	var startedAt, finishedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, type, node_ids, payload, timeout, concurrency, status, created_by,
			created_at, started_at, finished_at
		FROM deploy_tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Name, &t.Type, &nodeIDsJSON, &t.Payload, &t.Timeout, &t.Concurrency,
		&t.Status, &t.CreatedBy, &t.CreatedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get deploy task %s: %w", id, err)
	}
	_ = json.Unmarshal([]byte(nodeIDsJSON), &t.NodeIDs)
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}
	return t, nil
}

func (r *DeployTaskRepository) List(ctx context.Context, limit, offset int) ([]*model.DeployTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, node_ids, payload, timeout, concurrency, status, created_by,
			created_at, started_at, finished_at
		FROM deploy_tasks ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list deploy tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.DeployTask
	for rows.Next() {
		t := &model.DeployTask{}
		var nodeIDsJSON string
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Type, &nodeIDsJSON, &t.Payload, &t.Timeout, &t.Concurrency,
			&t.Status, &t.CreatedBy, &t.CreatedAt, &startedAt, &finishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deploy task: %w", err)
		}
		_ = json.Unmarshal([]byte(nodeIDsJSON), &t.NodeIDs)
		if startedAt.Valid {
			t.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			t.FinishedAt = &finishedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *DeployTaskRepository) Update(ctx context.Context, t *model.DeployTask) error {
	nodeIDs, _ := json.Marshal(t.NodeIDs)
	_, err := r.db.ExecContext(ctx, `
		UPDATE deploy_tasks SET name=?, type=?, node_ids=?, payload=?, timeout=?, concurrency=?,
			status=?, started_at=?, finished_at=? WHERE id=?`,
		t.Name, t.Type, string(nodeIDs), t.Payload, t.Timeout, t.Concurrency,
		t.Status, t.StartedAt, t.FinishedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update deploy task: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) UpdateStatus(ctx context.Context, id string, status model.DeployTaskStatus) error {
	now := time.Now()
	var finishedAt interface{}
	if status == model.DeployCompleted || status == model.DeployFailed || status == model.DeployCancelled {
		finishedAt = now
	} else {
		finishedAt = nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE deploy_tasks SET status=?, finished_at=? WHERE id=?`,
		status, finishedAt, id)
	if err != nil {
		return fmt.Errorf("update deploy task status: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM deploy_tasks WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete deploy task: %w", err)
	}
	return nil
}

type DeployResultRepository struct {
	db *sql.DB
}

func (r *DeployResultRepository) Create(ctx context.Context, res *model.DeployResult) error {
	if res.StartedAt.IsZero() {
		res.StartedAt = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deploy_results (id, task_id, node_id, node_name, status, exit_code,
			output, error, duration_ms, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.ID, res.TaskID, res.NodeID, res.NodeName, res.Status, res.ExitCode,
		res.Output, res.Error, res.Duration, res.StartedAt, res.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("create deploy result: %w", err)
	}
	return nil
}

func (r *DeployResultRepository) GetByID(ctx context.Context, id string) (*model.DeployResult, error) {
	res := &model.DeployResult{}
	var finishedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, task_id, node_id, node_name, status, exit_code, output, error,
			duration_ms, started_at, finished_at
		FROM deploy_results WHERE id = ?`, id).Scan(
		&res.ID, &res.TaskID, &res.NodeID, &res.NodeName, &res.Status, &res.ExitCode,
		&res.Output, &res.Error, &res.Duration, &res.StartedAt, &finishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get deploy result %s: %w", id, err)
	}
	if finishedAt.Valid {
		res.FinishedAt = &finishedAt.Time
	}
	return res, nil
}

func (r *DeployResultRepository) ListByTaskID(ctx context.Context, taskID string) ([]*model.DeployResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, node_id, node_name, status, exit_code, output, error,
			duration_ms, started_at, finished_at
		FROM deploy_results WHERE task_id = ? ORDER BY started_at`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list deploy results: %w", err)
	}
	defer rows.Close()

	var results []*model.DeployResult
	for rows.Next() {
		res := &model.DeployResult{}
		var finishedAt sql.NullTime
		if err := rows.Scan(
			&res.ID, &res.TaskID, &res.NodeID, &res.NodeName, &res.Status, &res.ExitCode,
			&res.Output, &res.Error, &res.Duration, &res.StartedAt, &finishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deploy result: %w", err)
		}
		if finishedAt.Valid {
			res.FinishedAt = &finishedAt.Time
		}
		results = append(results, res)
	}
	return results, nil
}

func (r *DeployResultRepository) Update(ctx context.Context, res *model.DeployResult) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deploy_results SET status=?, exit_code=?, output=?, error=?, duration_ms=?,
			finished_at=? WHERE id=?`,
		res.Status, res.ExitCode, res.Output, res.Error, res.Duration, res.FinishedAt, res.ID,
	)
	if err != nil {
		return fmt.Errorf("update deploy result: %w", err)
	}
	return nil
}
