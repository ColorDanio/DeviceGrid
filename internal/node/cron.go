package node

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

// CronTask defines a scheduled script execution task
type CronTask struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	NodeIDs   []string      `json:"node_ids"`
	Script    string        `json:"script"`
	Interval  time.Duration `json:"interval"`
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"created_at"`
	LastRun   *time.Time    `json:"last_run,omitempty"`
	NextRun   time.Time     `json:"next_run"`
}

type CronScheduler struct {
	repos     repo.Repositories
	transport *transport.Manager
	mu        sync.Mutex
	tasks     map[string]*CronTask
	cancel    context.CancelFunc
}

func NewCronScheduler(repos repo.Repositories, tm *transport.Manager) *CronScheduler {
	return &CronScheduler{
		repos:     repos,
		transport: tm,
		tasks:     make(map[string]*CronTask),
	}
}

func (cs *CronScheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	cs.cancel = cancel
	go cs.run(ctx)
	slog.Info("cron scheduler started")
}

func (cs *CronScheduler) Stop() {
	if cs.cancel != nil {
		cs.cancel()
	}
}

func (cs *CronScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cs.tick(ctx)
		}
	}
}

func (cs *CronScheduler) tick(ctx context.Context) {
	cs.mu.Lock()
	tasks := make([]*CronTask, 0, len(cs.tasks))
	for _, t := range cs.tasks {
		if t.Enabled && time.Now().After(t.NextRun) {
			tasks = append(tasks, t)
		}
	}
	cs.mu.Unlock()

	for _, task := range tasks {
		cs.executeTask(ctx, task)
	}
}

func (cs *CronScheduler) executeTask(ctx context.Context, task *CronTask) {
	slog.Info("cron task executing", "name", task.Name, "nodes", len(task.NodeIDs))

	for _, nodeID := range task.NodeIDs {
		go func(nid string) {
			stream, err := cs.transport.ExecStream(ctx, nid, task.Script)
			if err != nil {
				slog.Error("cron task failed", "task", task.Name, "node", nid, "error", err)
				return
			}
			// Drain the stream
			for range stream {
			}
		}(nodeID)
	}

	cs.mu.Lock()
	now := time.Now()
	task.LastRun = &now
	task.NextRun = now.Add(task.Interval)
	cs.mu.Unlock()
}

func (cs *CronScheduler) AddTask(task *CronTask) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	task.NextRun = time.Now().Add(task.Interval)
	cs.tasks[task.ID] = task
	slog.Info("cron task added", "name", task.Name, "interval", task.Interval)
}

func (cs *CronScheduler) RemoveTask(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.tasks, id)
}

func (cs *CronScheduler) ListTasks() []*CronTask {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	result := make([]*CronTask, 0, len(cs.tasks))
	for _, t := range cs.tasks {
		result = append(result, t)
	}
	return result
}

func (cs *CronScheduler) GetTask(id string) (*CronTask, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	t, ok := cs.tasks[id]
	return t, ok
}
