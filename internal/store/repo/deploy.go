package repo

import (
	"context"

	"github.com/michael/device_grid/internal/model"
)

type DeployTaskRepository interface {
	Create(ctx context.Context, task *model.DeployTask) error
	GetByID(ctx context.Context, id string) (*model.DeployTask, error)
	List(ctx context.Context, limit, offset int) ([]*model.DeployTask, error)
	Update(ctx context.Context, task *model.DeployTask) error
	UpdateStatus(ctx context.Context, id string, status model.DeployTaskStatus) error
	Delete(ctx context.Context, id string) error
}

type DeployResultRepository interface {
	Create(ctx context.Context, result *model.DeployResult) error
	GetByID(ctx context.Context, id string) (*model.DeployResult, error)
	ListByTaskID(ctx context.Context, taskID string) ([]*model.DeployResult, error)
	Update(ctx context.Context, result *model.DeployResult) error
}
