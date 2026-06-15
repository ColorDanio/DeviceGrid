package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/michael/device_grid/internal/store/repo"
)

type Store struct {
	client   *mongo.Client
	db       *mongo.Database
	nodeRepo *NodeRepository
	taskRepo *DeployTaskRepository
	resultRepo *DeployResultRepository
	containerRepo *ContainerRepository
	clusterRepo *ClusterRepository
	userRepo *UserRepository
}

func init() {
	repo.Register("mongodb", func(ctx context.Context) (repo.Repositories, error) {
		return New(ctx, "mongodb://localhost:27017", "device_grid")
	})
}

func New(ctx context.Context, uri, database string) (*Store, error) {
	clientOpts := options.Client().ApplyURI(uri).SetConnectTimeout(10 * time.Second)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := client.Database(database)

	if err := ensureIndexes(ctx, db); err != nil {
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}

	s := &Store{
		client: client,
		db:     db,
	}
	s.nodeRepo = &NodeRepository{coll: db.Collection("nodes")}
	s.taskRepo = &DeployTaskRepository{coll: db.Collection("deploy_tasks")}
	s.resultRepo = &DeployResultRepository{coll: db.Collection("deploy_results")}
	s.containerRepo = &ContainerRepository{coll: db.Collection("containers")}
	s.clusterRepo = &ClusterRepository{coll: db.Collection("clusters")}
	s.userRepo = &UserRepository{coll: db.Collection("users")}
	return s, nil
}

func ensureIndexes(ctx context.Context, db *mongo.Database) error {
	indexes := map[string][]mongo.IndexModel{
		"nodes": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "cluster_id", Value: 1}}},
			{Keys: bson.D{{Key: "name", Value: "text"}, {Key: "host", Value: "text"}}},
		},
		"deploy_tasks": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		"deploy_results": {
			{Keys: bson.D{{Key: "task_id", Value: 1}}},
		},
		"containers": {
			{Keys: bson.D{{Key: "node_id", Value: 1}, {Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"users": {
			{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
	}

	for coll, models := range indexes {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes for %s: %w", coll, err)
		}
	}
	return nil
}

func (s *Store) Nodes() repo.NodeRepository              { return s.nodeRepo }
func (s *Store) DeployTasks() repo.DeployTaskRepository   { return s.taskRepo }
func (s *Store) DeployResults() repo.DeployResultRepository { return s.resultRepo }
func (s *Store) Containers() repo.ContainerRepository    { return s.containerRepo }
func (s *Store) Clusters() repo.ClusterRepository         { return s.clusterRepo }
func (s *Store) Users() repo.UserRepository               { return s.userRepo }

func (s *Store) Close() error {
	return s.client.Disconnect(context.Background())
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}
