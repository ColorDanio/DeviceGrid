package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/michael/device_grid/internal/model"
)

type DeployTaskRepository struct {
	coll *mongo.Collection
}

func (r *DeployTaskRepository) Create(ctx context.Context, t *model.DeployTask) error {
	_, err := r.coll.InsertOne(ctx, t)
	if err != nil {
		return fmt.Errorf("create deploy task: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) GetByID(ctx context.Context, id string) (*model.DeployTask, error) {
	var t model.DeployTask
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&t); err != nil {
		return nil, fmt.Errorf("get deploy task %s: %w", id, err)
	}
	return &t, nil
}

func (r *DeployTaskRepository) List(ctx context.Context, limit, offset int) ([]*model.DeployTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).SetSkip(int64(offset))
	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list deploy tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*model.DeployTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("decode deploy tasks: %w", err)
	}
	return tasks, nil
}

func (r *DeployTaskRepository) Update(ctx context.Context, t *model.DeployTask) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": t.ID}, t)
	if err != nil {
		return fmt.Errorf("update deploy task: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) UpdateStatus(ctx context.Context, id string, status model.DeployTaskStatus) error {
	update := bson.M{"status": status}
	if status == model.DeployCompleted || status == model.DeployFailed || status == model.DeployCancelled {
		update["finished_at"] = timeNow()
	}
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("update deploy task status: %w", err)
	}
	return nil
}

func (r *DeployTaskRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete deploy task: %w", err)
	}
	return nil
}

type DeployResultRepository struct {
	coll *mongo.Collection
}

func (r *DeployResultRepository) Create(ctx context.Context, res *model.DeployResult) error {
	_, err := r.coll.InsertOne(ctx, res)
	if err != nil {
		return fmt.Errorf("create deploy result: %w", err)
	}
	return nil
}

func (r *DeployResultRepository) GetByID(ctx context.Context, id string) (*model.DeployResult, error) {
	var res model.DeployResult
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&res); err != nil {
		return nil, fmt.Errorf("get deploy result %s: %w", id, err)
	}
	return &res, nil
}

func (r *DeployResultRepository) ListByTaskID(ctx context.Context, taskID string) ([]*model.DeployResult, error) {
	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.M{"task_id": taskID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list deploy results: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*model.DeployResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode deploy results: %w", err)
	}
	return results, nil
}

func (r *DeployResultRepository) Update(ctx context.Context, res *model.DeployResult) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": res.ID}, res)
	if err != nil {
		return fmt.Errorf("update deploy result: %w", err)
	}
	return nil
}
