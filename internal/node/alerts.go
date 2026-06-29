package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

// AlertRule defines a threshold-based alert
type AlertRule struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Enabled    bool    `json:"enabled"`
	Metric     string  `json:"metric"`   // "cpu" | "mem" | "disk" | "node_offline"
	Operator   string  `json:"operator"` // ">" | "<" | "=="
	Threshold  float64 `json:"threshold"`
	WebhookURL string  `json:"webhook_url"`
	CooldownM  int     `json:"cooldown_min"`
}

// AlertEvent records when an alert fired
type AlertEvent struct {
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	FiredAt   time.Time `json:"fired_at"`
}

type AlertManager struct {
	repos     repo.Repositories
	transport *transport.Manager
	mu        sync.Mutex
	rules     []AlertRule
	lastFired map[string]time.Time // rule_id:node_id → last fired time
	client    *http.Client
}

func NewAlertManager(repos repo.Repositories, tm *transport.Manager) *AlertManager {
	return &AlertManager{
		repos:     repos,
		transport: tm,
		lastFired: make(map[string]time.Time),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (am *AlertManager) SetRules(rules []AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = rules
	slog.Info("alert rules updated", "count", len(rules))
}

func (am *AlertManager) GetRules() []AlertRule {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.rules
}

// CheckAll evaluates all rules against current node metrics
func (am *AlertManager) CheckAll(ctx context.Context) []AlertEvent {
	am.mu.Lock()
	rules := am.rules
	am.mu.Unlock()

	if len(rules) == 0 {
		return nil
	}

	// Get all nodes
	nodes, err := am.repos.Nodes().List(ctx, model.NodeFilter{})
	if err != nil {
		return nil
	}

	var events []AlertEvent
	for _, node := range nodes {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}

			var value float64
			fired := false
			msg := ""

			switch rule.Metric {
			case "node_offline":
				if node.Status == "offline" || node.Status == "error" {
					fired = true
					value = 1
					msg = fmt.Sprintf("节点 %s 离线", node.Name)
				}
			case "cpu":
				if node.Status == "online" {
					m, err := am.transport.Metrics(ctx, node.ID)
					if err == nil {
						value = m.CPUUsage
						fired = am.evaluate(rule.Operator, value, rule.Threshold)
						msg = fmt.Sprintf("节点 %s CPU %.2f%% (阈值 %s %.0f%%)", node.Name, value, rule.Operator, rule.Threshold)
					}
				}
			case "mem":
				if node.Status == "online" {
					m, err := am.transport.Metrics(ctx, node.ID)
					if err == nil && m.MemTotal > 0 {
						value = float64(m.MemUsed) / float64(m.MemTotal) * 100
						fired = am.evaluate(rule.Operator, value, rule.Threshold)
						msg = fmt.Sprintf("节点 %s 内存 %.2f%% (阈值 %s %.0f%%)", node.Name, value, rule.Operator, rule.Threshold)
					}
				}
			case "disk":
				if node.Status == "online" {
					m, err := am.transport.Metrics(ctx, node.ID)
					if err == nil && m.DiskTotal > 0 {
						value = float64(m.DiskUsed) / float64(m.DiskTotal) * 100
						fired = am.evaluate(rule.Operator, value, rule.Threshold)
						msg = fmt.Sprintf("节点 %s 磁盘 %.2f%% (阈值 %s %.0f%%)", node.Name, value, rule.Operator, rule.Threshold)
					}
				}
			}

			if !fired {
				continue
			}

			// Check cooldown
			key := rule.ID + ":" + node.ID
			am.mu.Lock()
			lastFired, exists := am.lastFired[key]
			am.mu.Unlock()

			cooldown := time.Duration(rule.CooldownM) * time.Minute
			if cooldown == 0 {
				cooldown = 30 * time.Minute
			}
			if exists && time.Since(lastFired) < cooldown {
				continue
			}

			// Fire alert
			event := AlertEvent{
				NodeID:    node.ID,
				NodeName:  node.Name,
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Metric:    rule.Metric,
				Value:     value,
				Threshold: rule.Threshold,
				Message:   msg,
				FiredAt:   time.Now(),
			}
			events = append(events, event)

			am.mu.Lock()
			am.lastFired[key] = time.Now()
			am.mu.Unlock()

			// Send webhook
			if rule.WebhookURL != "" {
				go am.sendWebhook(rule.WebhookURL, event)
			}

			slog.Warn("alert fired", "rule", rule.Name, "node", node.Name, "metric", rule.Metric, "value", value)
		}
	}

	return events
}

func (am *AlertManager) evaluate(operator string, value, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return value > threshold
	}
}

func (am *AlertManager) sendWebhook(url string, event AlertEvent) {
	payload, _ := json.Marshal(map[string]interface{}{
		"text":  event.Message,
		"alert": event,
	})
	resp, err := am.client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		slog.Error("alert webhook failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
	slog.Info("alert webhook sent", "url", url, "status", resp.StatusCode)
}

func (am *AlertManager) SendTestWebhook(url string) {
	event := AlertEvent{
		RuleName: "测试告警",
		Message:  "DeviceGrid 告警测试 — Webhook 连接正常",
		FiredAt:  time.Now(),
	}
	am.sendWebhook(url, event)
}
