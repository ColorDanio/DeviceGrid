package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/node"
)

type CronHandler struct {
	cron *node.CronScheduler
}

func NewCronHandler(cs *node.CronScheduler) *CronHandler {
	return &CronHandler{cron: cs}
}

func (h *CronHandler) List(c *gin.Context) {
	OK(c, h.cron.ListTasks())
}

func (h *CronHandler) Create(c *gin.Context) {
	var req struct {
		Name     string   `json:"name" binding:"required"`
		NodeIDs  []string `json:"node_ids" binding:"required"`
		Script   string   `json:"script" binding:"required"`
		Interval string   `json:"interval" binding:"required"` // e.g. "5m", "1h", "30s"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid: "+err.Error())
		return
	}

	dur, err := time.ParseDuration(req.Interval)
	if err != nil {
		BadRequest(c, "invalid interval format (use e.g. 5m, 1h, 30s)")
		return
	}

	task := &node.CronTask{
		ID:        uuid.NewString(),
		Name:      req.Name,
		NodeIDs:   req.NodeIDs,
		Script:    req.Script,
		Interval:  dur,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	h.cron.AddTask(task)
	Created(c, task)
}

func (h *CronHandler) Delete(c *gin.Context) {
	h.cron.RemoveTask(c.Param("tid"))
	OK(c, gin.H{"deleted": true})
}

func (h *CronHandler) Toggle(c *gin.Context) {
	tid := c.Param("tid")
	task, ok := h.cron.GetTask(tid)
	if !ok {
		NotFound(c, "task not found")
		return
	}
	task.Enabled = !task.Enabled
	OK(c, task)
}
