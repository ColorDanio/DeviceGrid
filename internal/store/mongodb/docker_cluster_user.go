package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/michael/device_grid/internal/model"
)

type ContainerRepository struct {
	coll *mongo.Collection
}

func (r *ContainerRepository) ListByNodeID(ctx context.Context, nodeID string) ([]*model.Container, error) {
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.M{"node_id": nodeID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer cursor.Close(ctx)

	var containers []*model.Container
	if err := cursor.All(ctx, &containers); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}
	return containers, nil
}

func (r *ContainerRepository) Upsert(ctx context.Context, c *model.Container) error {
	filter := bson.M{"node_id": c.NodeID, "name": c.Name}
	update := bson.M{"$set": c}
	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("upsert container: %w", err)
	}
	return nil
}

func (r *ContainerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete container: %w", err)
	}
	return nil
}

type ClusterRepository struct {
	coll *mongo.Collection
}

func (r *ClusterRepository) Create(ctx context.Context, c *model.Cluster) error {
	_, err := r.coll.InsertOne(ctx, c)
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}
	return nil
}

func (r *ClusterRepository) GetByID(ctx context.Context, id string) (*model.Cluster, error) {
	var c model.Cluster
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&c); err != nil {
		return nil, fmt.Errorf("get cluster %s: %w", id, err)
	}
	return &c, nil
}

func (r *ClusterRepository) List(ctx context.Context) ([]*model.Cluster, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer cursor.Close(ctx)

	var clusters []*model.Cluster
	if err := cursor.All(ctx, &clusters); err != nil {
		return nil, fmt.Errorf("decode clusters: %w", err)
	}
	return clusters, nil
}

func (r *ClusterRepository) Update(ctx context.Context, c *model.Cluster) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": c.ID}, c)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
	}
	return nil
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	return nil
}

type UserRepository struct {
	coll *mongo.Collection
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.coll.InsertOne(ctx, u)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := r.coll.FindOne(ctx, bson.M{"username": username}).Decode(&u); err != nil {
		return nil, fmt.Errorf("get user by username %s: %w", username, err)
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer cursor.Close(ctx)

	var users []*model.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *model.User) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": u.ID}, bson.M{
		"$set": bson.M{
			"username":      u.Username,
			"password_hash": u.PasswordHash,
			"role":          u.Role,
		},
	})
	if err != nil {
		return fmt.Errorf("update user %s: %w", u.ID, err)
	}
	return nil
}

func timeNow() time.Time {
	return time.Now()
}
