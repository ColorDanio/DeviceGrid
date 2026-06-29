package repo

import (
	"context"
	"fmt"

	"github.com/michael/device_grid/internal/config"
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

// Factory creates a Repositories instance from a database config.
// The driver implementation reads its own sub-config (SQLiteConfig, MongoDBConfig)
// and is responsible for opening the underlying connection.
type Factory func(ctx context.Context, dbCfg config.DatabaseConfig) (Repositories, error)

var factories = map[string]Factory{}

func Register(driver string, f Factory) {
	factories[driver] = f
}

func New(ctx context.Context, dbCfg config.DatabaseConfig) (Repositories, error) {
	f, ok := factories[dbCfg.Driver]
	if !ok {
		return nil, fmt.Errorf("unsupported database driver: %s", dbCfg.Driver)
	}
	return f(ctx, dbCfg)
}
