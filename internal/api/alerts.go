package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/node"
)

type AlertHandler struct {
	alertMgr *node.AlertManager
}

func NewAlertHandler(am *node.AlertManager) *AlertHandler {
	return &AlertHandler{alertMgr: am}
}

func (h *AlertHandler) ListRules(c *gin.Context) {
	OK(c, h.alertMgr.GetRules())
}

func (h *AlertHandler) CreateRule(c *gin.Context) {
	var rule node.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		BadRequest(c, "invalid: "+err.Error())
		return
	}
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	rules := h.alertMgr.GetRules()
	rules = append(rules, rule)
	h.alertMgr.SetRules(rules)
	Created(c, rule)
}

func (h *AlertHandler) DeleteRule(c *gin.Context) {
	ruleID := c.Param("rid")
	rules := h.alertMgr.GetRules()
	var filtered []node.AlertRule
	for _, r := range rules {
		if r.ID != ruleID {
			filtered = append(filtered, r)
		}
	}
	h.alertMgr.SetRules(filtered)
	OK(c, gin.H{"deleted": true})
}

func (h *AlertHandler) TestWebhook(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "url is required")
		return
	}

	rules := h.alertMgr.GetRules()
	if len(rules) == 0 {
		// Create a test rule temporarily
		testRule := node.AlertRule{
			ID:         "test",
			Name:       "测试告警",
			WebhookURL: req.URL,
			CooldownM:  0,
		}
		h.alertMgr.SetRules([]node.AlertRule{testRule})
	} else {
		// Send directly
		h.alertMgr.SendTestWebhook(req.URL)
	}

	OK(c, gin.H{"sent": true})
}
