package repo

import (
	"context"

	"github.com/michael/device_grid/internal/model"
)

type ContainerRepository interface {
	ListByNodeID(ctx context.Context, nodeID string) ([]*model.Container, error)
	Upsert(ctx context.Context, container *model.Container) error
	Delete(ctx context.Context, id string) error
}

type ClusterRepository interface {
	Create(ctx context.Context, cluster *model.Cluster) error
	GetByID(ctx context.Context, id string) (*model.Cluster, error)
	List(ctx context.Context) ([]*model.Cluster, error)
	Update(ctx context.Context, cluster *model.Cluster) error
	Delete(ctx context.Context, id string) error
}

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context) ([]*model.User, error)
	Delete(ctx context.Context, id string) error
}
