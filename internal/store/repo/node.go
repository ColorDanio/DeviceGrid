package repo

import (
	"context"

	"github.com/michael/device_grid/internal/model"
)

type NodeRepository interface {
	Create(ctx context.Context, node *model.Node) error
	GetByID(ctx context.Context, id string) (*model.Node, error)
	List(ctx context.Context, filter model.NodeFilter) ([]*model.Node, error)
	Update(ctx context.Context, node *model.Node) error
	UpdateStatus(ctx context.Context, id string, status model.NodeStatus) error
	Delete(ctx context.Context, id string) error
}
