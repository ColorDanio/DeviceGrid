package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/michael/device_grid/internal/model"
)

type NodeRepository struct {
	coll *mongo.Collection
}

func (r *NodeRepository) Create(ctx context.Context, n *model.Node) error {
	_, err := r.coll.InsertOne(ctx, n)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

func (r *NodeRepository) GetByID(ctx context.Context, id string) (*model.Node, error) {
	var n model.Node
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&n); err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	return &n, nil
}

func (r *NodeRepository) List(ctx context.Context, filter model.NodeFilter) ([]*model.Node, error) {
	query := bson.M{}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Tag != "" {
		query["tags"] = filter.Tag
	}
	if filter.Search != "" {
		query["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": filter.Search, "$options": "i"}},
			bson.M{"host": bson.M{"$regex": filter.Search, "$options": "i"}},
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer cursor.Close(ctx)

	var nodes []*model.Node
	if err := cursor.All(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}
	return nodes, nil
}

func (r *NodeRepository) Update(ctx context.Context, n *model.Node) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": n.ID}, n)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	return nil
}

func (r *NodeRepository) UpdateStatus(ctx context.Context, id string, status model.NodeStatus) error {
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"status":      status,
		"last_seen_at": timeNow(),
		"updated_at":   timeNow(),
	}})
	if err != nil {
		return fmt.Errorf("update node status: %w", err)
	}
	return nil
}

func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	return nil
}
