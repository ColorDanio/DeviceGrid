package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
	"github.com/michael/device_grid/internal/ws"
)

var pkgNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+:~-]*$`)

func isValidPackageName(pkg string) bool {
	return pkgNameRe.MatchString(pkg)
}

type Engine struct {
	repos     repo.Repositories
	transport *transport.Manager
	hub       *ws.Hub
	mu        sync.Mutex
	running   map[string]context.CancelFunc
}

func NewEngine(repos repo.Repositories, tm *transport.Manager, hub *ws.Hub) *Engine {
	return &Engine{
		repos:     repos,
		transport: tm,
		hub:       hub,
		running:   make(map[string]context.CancelFunc),
	}
}

type CreateTaskRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required"`
	NodeIDs     []string `json:"node_ids" binding:"required"`
	Payload     string `json:"payload" binding:"required"`
	Timeout     int    `json:"timeout"`
	Concurrency int    `json:"concurrency"`
	CreatedBy   string `json:"-"`
}

func (e *Engine) CreateAndRun(ctx context.Context, req CreateTaskRequest) (*model.DeployTask, error) {
	task := &model.DeployTask{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Type:        model.DeployTaskType(req.Type),
		NodeIDs:     req.NodeIDs,
		Payload:     req.Payload,
		Timeout:     req.Timeout,
		Concurrency: req.Concurrency,
		Status:      model.DeployPending,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now(),
	}

	if err := e.repos.DeployTasks().Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	for _, nodeID := range task.NodeIDs {
		result := &model.DeployResult{
			ID:        uuid.NewString(),
			TaskID:    task.ID,
			NodeID:    nodeID,
			Status:    model.ResultRunning,
			StartedAt: time.Now(),
		}
		if err := e.repos.DeployResults().Create(ctx, result); err != nil {
			slog.Error("create deploy result", "error", err)
		}
	}

	go e.runTask(task)

	return task, nil
}

func (e *Engine) runTask(task *model.DeployTask) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e.mu.Lock()
	e.running[task.ID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.running, task.ID)
		e.mu.Unlock()
	}()

	now := time.Now()
	task.Status = model.DeployRunning
	task.StartedAt = &now
	_ = e.repos.DeployTasks().Update(ctx, task)

	e.hub.Broadcast("deploy-"+task.ID, map[string]interface{}{
		"type":    "started",
		"task_id": task.ID,
		"name":    task.Name,
	})

	concurrency := task.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	allSuccess := true

	for _, nodeID := range task.NodeIDs {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			success := e.runOnNode(ctx, task, nid)
			if !success {
				allSuccess = false
			}
		}(nodeID)
	}

	wg.Wait()

	finishedAt := time.Now()
	task.FinishedAt = &finishedAt
	if allSuccess {
		task.Status = model.DeployCompleted
	} else {
		task.Status = model.DeployFailed
	}
	_ = e.repos.DeployTasks().Update(ctx, task)

	e.hub.Broadcast("deploy-"+task.ID, map[string]interface{}{
		"type":   "finished",
		"status": string(task.Status),
	})
}

func (e *Engine) runOnNode(ctx context.Context, task *model.DeployTask, nodeID string) bool {
	results, err := e.repos.DeployResults().ListByTaskID(ctx, task.ID)
	if err != nil {
		return false
	}

	var result *model.DeployResult
	for _, r := range results {
		if r.NodeID == nodeID {
			result = r
			break
		}
	}
	if result == nil {
		return false
	}

	node, err := e.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		e.finishResult(ctx, result, model.ResultFailed, 1, "", fmt.Sprintf("node not found: %v", err))
		return false
	}
	result.NodeName = node.Name

	startTime := time.Now()

	var stream <-chan transport.StreamChunk
	switch task.Type {
	case model.DeployScript:
		stream, err = e.transport.ExecStream(ctx, nodeID, task.Payload)
	case model.DeployFile:
		err = fmt.Errorf("file deployment not yet implemented")
	case model.DeployPackage:
		// Validate package names to prevent shell injection
		// Split by whitespace, validate each package name individually
		packages := strings.Fields(task.Payload)
		var validatedPkgs []string
		for _, pkg := range packages {
			// Only allow alphanumeric, dots, dashes, plus, colons, tildes
			if !isValidPackageName(pkg) {
				e.finishResult(ctx, result, model.ResultFailed, 1, "", fmt.Sprintf("invalid package name: %s", pkg))
				e.broadcastNode(task.ID, nodeID, node.Name, "error", "", fmt.Sprintf("invalid package name: %s", pkg))
				return false
			}
			validatedPkgs = append(validatedPkgs, pkg)
		}
		if len(validatedPkgs) == 0 {
			e.finishResult(ctx, result, model.ResultFailed, 1, "", "no valid package names")
			return false
		}
		pkgList := strings.Join(validatedPkgs, " ")
		script := fmt.Sprintf("OS_TYPE=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '\"'); if [ \"$OS_TYPE\" = \"ubuntu\" ] || [ \"$OS_TYPE\" = \"debian\" ]; then apt-get install -y %s; elif [ \"$OS_TYPE\" = \"centos\" ] || [ \"$OS_TYPE\" = \"rhel\" ]; then yum install -y %s; fi", pkgList, pkgList)
		stream, err = e.transport.ExecStream(ctx, nodeID, script)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if err != nil {
		e.finishResult(ctx, result, model.ResultFailed, 1, "", err.Error())
		e.broadcastNode(task.ID, nodeID, node.Name, "error", "", err.Error())
		return false
	}

	var outputBuilder []byte
	exitCode := 0
	for chunk := range stream {
		if chunk.Type == "exit" {
			exitCode = chunk.ExitCode
			break
		}
		if chunk.Type == "stdout" || chunk.Type == "stderr" {
			outputBuilder = append(outputBuilder, []byte(chunk.Data+"\n")...)
			e.broadcastNode(task.ID, nodeID, node.Name, chunk.Type, chunk.Data, "")
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()
	result.Output = string(outputBuilder)

	status := model.ResultSuccess
	if exitCode != 0 {
		status = model.ResultFailed
	}

	e.finishResult(ctx, result, status, exitCode, "", "")
	e.broadcastNode(task.ID, nodeID, node.Name, "exit", "", "", exitCode)

	return status == model.ResultSuccess
}

func (e *Engine) finishResult(ctx context.Context, result *model.DeployResult, status model.DeployResultStatus, exitCode int, errMsg, errStr string) {
	now := time.Now()
	result.Status = status
	result.ExitCode = exitCode
	result.FinishedAt = &now
	if errStr != "" {
		result.Error = errStr
	}
	if errMsg != "" {
		result.Output = errMsg
	}
	_ = e.repos.DeployResults().Update(ctx, result)
}

func (e *Engine) broadcastNode(taskID, nodeID, nodeName, chunkType, data, errMsg string, exitCode ...int) {
	msg := map[string]interface{}{
		"type":      chunkType,
		"node_id":   nodeID,
		"node_name": nodeName,
		"data":      data,
	}
	if len(exitCode) > 0 {
		msg["exit_code"] = exitCode[0]
	}
	if errMsg != "" {
		msg["error"] = errMsg
	}
	e.hub.Broadcast("deploy-"+taskID, msg)
}

func (e *Engine) Cancel(taskID string) error {
	e.mu.Lock()
	cancel, ok := e.running[taskID]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s is not running", taskID)
	}
	cancel()
	_ = e.repos.DeployTasks().UpdateStatus(context.Background(), taskID, model.DeployCancelled)
	return nil
}

func (e *Engine) GetTaskWithResults(ctx context.Context, taskID string) (*model.DeployTask, []*model.DeployResult, error) {
	task, err := e.repos.DeployTasks().GetByID(ctx, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("get task: %w", err)
	}
	results, err := e.repos.DeployResults().ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("get results: %w", err)
	}
	return task, results, nil
}
