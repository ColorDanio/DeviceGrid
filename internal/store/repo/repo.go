package repo

import (
	"context"
	"fmt"
)

type Repositories interface {
	Nodes() NodeRepository
	DeployTasks() DeployTaskRepository
	DeployResults() DeployResultRepository
	Containers() ContainerRepository
	Clusters() ClusterRepository
	Users() UserRepository
	Close() error
	Ping(ctx context.Context) error
}

type Factory func(ctx context.Context) (Repositories, error)

var factories = map[string]Factory{}

func Register(driver string, f Factory) {
	factories[driver] = f
}

func New(ctx context.Context, driver string) (Repositories, error) {
	f, ok := factories[driver]
	if !ok {
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
	return f(ctx)
}
