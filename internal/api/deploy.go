package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/michael/device_grid/internal/deploy"
	"github.com/michael/device_grid/internal/store/repo"
)

type DeployHandler struct {
	repos  repo.Repositories
	engine *deploy.Engine
}

func NewDeployHandler(repos repo.Repositories, engine *deploy.Engine) *DeployHandler {
	return &DeployHandler{repos: repos, engine: engine}
}

func (h *DeployHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	tasks, err := h.repos.DeployTasks().List(c.Request.Context(), limit, offset)
	if err != nil {
		InternalError(c, "list deploy tasks: "+err.Error())
		return
	}
	OK(c, tasks)
}

func (h *DeployHandler) Create(c *gin.Context) {
	var req deploy.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	req.CreatedBy = userID.(string)

	task, err := h.engine.CreateAndRun(c.Request.Context(), req)
	if err != nil {
		InternalError(c, "create deploy task: "+err.Error())
		return
	}
	Created(c, task)
}

func (h *DeployHandler) Get(c *gin.Context) {
	taskID := c.Param("tid")
	task, results, err := h.engine.GetTaskWithResults(c.Request.Context(), taskID)
	if err != nil {
		NotFound(c, "deploy task not found")
		return
	}
	OK(c, gin.H{
		"task":    task,
		"results": results,
	})
}

func (h *DeployHandler) Cancel(c *gin.Context) {
	taskID := c.Param("tid")
	if err := h.engine.Cancel(taskID); err != nil {
		BadRequest(c, "cancel task: "+err.Error())
		return
	}
	OK(c, gin.H{"cancelled": true})
}
