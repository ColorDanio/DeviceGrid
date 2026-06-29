package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/transport"
)

// ===== SSH Key Management =====

type SSHKeyHandler struct {
	repos  interface{}
	router *Router
}

func NewSSHKeyHandler(r *Router) *SSHKeyHandler {
	return &SSHKeyHandler{router: r}
}

// GetKeyInfo returns info about the node's SSH key (fingerprint, type)
func (h *SSHKeyHandler) GetKeyInfo(c *gin.Context) {
	nodeID := c.Param("id")
	result, err := h.router.transport.Exec(c.Request.Context(), nodeID,
		"for f in ~/.ssh/id_ed25519 ~/.ssh/id_rsa ~/.ssh/id_ecdsa ~/.ssh/id_ed25519_sk; do [ -f \"$f\" ] && ssh-keygen -lf \"$f\" 2>/dev/null && echo \"KEYFILE=$f\" && break; done 2>/dev/null || echo 'no key found'")
	if err != nil {
		Error(c, http.StatusBadGateway, "get key info: "+err.Error())
		return
	}
	OK(c, gin.H{"output": result.Stdout})
}

// RotateKey generates a new SSH keypair for the node and re-establishes trust
func (h *SSHKeyHandler) RotateKey(c *gin.Context) {
	nodeID := c.Param("id")
	// Use the existing trust establishment which generates a new keypair
	if err := h.router.sshMgr.EstablishTrust(c.Request.Context(), nodeID); err != nil {
		Error(c, http.StatusBadGateway, "key rotation failed: "+err.Error())
		return
	}
	OK(c, gin.H{"rotated": true, "message": "SSH key rotated successfully"})
}

// ===== Node Comparison =====

type CompareHandler struct {
	router *Router
}

func NewCompareHandler(r *Router) *CompareHandler {
	return &CompareHandler{router: r}
}

func (h *CompareHandler) Compare(c *gin.Context) {
	nodeA := c.Query("a")
	nodeB := c.Query("b")
	if nodeA == "" || nodeB == "" {
		BadRequest(c, "both 'a' and 'b' node IDs required")
		return
	}

	node1, err := h.router.repos.Nodes().GetByID(c.Request.Context(), nodeA)
	if err != nil {
		NotFound(c, "node A not found")
		return
	}
	node2, err := h.router.repos.Nodes().GetByID(c.Request.Context(), nodeB)
	if err != nil {
		NotFound(c, "node B not found")
		return
	}

	// Get metrics for both if online
	var metrics1, metrics2 *transport.NodeMetrics
	if node1.Status == "online" {
		if m, err := h.router.transport.Metrics(c.Request.Context(), nodeA); err == nil {
			metrics1 = &m
		}
	}
	if node2.Status == "online" {
		if m, err := h.router.transport.Metrics(c.Request.Context(), nodeB); err == nil {
			metrics2 = &m
		}
	}

	OK(c, gin.H{
		"node_a":    node1,
		"node_b":    node2,
		"metrics_a": metrics1,
		"metrics_b": metrics2,
	})
}

// ===== Audit Log =====

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Status    int    `json:"status"`
	Duration  string `json:"duration"`
}

var auditLog []AuditEntry
var auditMu sync.Mutex

func RecordAudit(entry AuditEntry) {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditLog = append(auditLog, entry)
	if len(auditLog) > 1000 {
		auditLog = auditLog[len(auditLog)-500:]
	}
}

type AuditHandler struct{}

func NewAuditHandler() *AuditHandler { return &AuditHandler{} }

func (h *AuditHandler) List(c *gin.Context) {
	limit := 100
	auditMu.Lock()
	defer auditMu.Unlock()
	start := len(auditLog) - limit
	if start < 0 {
		start = 0
	}
	OK(c, auditLog[start:])
}

// ===== Metrics Export =====

type MetricsExportHandler struct {
	router *Router
}

func NewMetricsExportHandler(r *Router) *MetricsExportHandler {
	return &MetricsExportHandler{router: r}
}

func (h *MetricsExportHandler) ExportCSV(c *gin.Context) {
	nodes, err := h.router.repos.Nodes().List(c.Request.Context(), model.NodeFilter{})
	if err != nil {
		InternalError(c, "list nodes: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", `attachment; filename="devicegrid_metrics.csv"`)

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"name", "host", "status", "os", "arch", "cpu_cores", "cpu_threads", "mem_total", "docker_version", "country", "transport_mode"})

	for _, n := range nodes {
		cpuCores := ""
		cpuThreads := ""
		memTotal := ""
		if n.Status == "online" {
			if m, err := h.router.transport.Metrics(c.Request.Context(), n.ID); err == nil {
				cpuCores = fmt.Sprintf("%d", m.CPUCores)
				cpuThreads = fmt.Sprintf("%d", m.CPUThreads)
				memTotal = fmt.Sprintf("%d", m.MemTotal)
			}
		}
		writer.Write([]string{n.Name, n.Host, string(n.Status), n.OS, n.Arch, cpuCores, cpuThreads, memTotal, n.DockerVersion, n.Country, string(n.TransportMode)})
	}
	writer.Flush()
}

func (h *MetricsExportHandler) PrometheusMetrics(c *gin.Context) {
	nodes, err := h.router.repos.Nodes().List(c.Request.Context(), model.NodeFilter{})
	if err != nil {
		c.String(500, "error")
		return
	}

	var sb strings.Builder
	sb.WriteString("# HELP devicegrid_node_status Node status (1=online, 0=offline)\n")
	sb.WriteString("# TYPE devicegrid_node_status gauge\n")
	for _, n := range nodes {
		val := 0
		if n.Status == model.NodeStatusOnline {
			val = 1
		}
		sb.WriteString(fmt.Sprintf("devicegrid_node_status{name=%q,host=%q} %d\n", n.Name, n.Host, val))
	}

	sb.WriteString("\n# HELP devicegrid_cpu_usage CPU usage percentage\n")
	sb.WriteString("# TYPE devicegrid_cpu_usage gauge\n")
	for _, n := range nodes {
		if n.Status != model.NodeStatusOnline {
			continue
		}
		if m, err := h.router.transport.Metrics(c.Request.Context(), n.ID); err == nil {
			sb.WriteString(fmt.Sprintf("devicegrid_cpu_usage{name=%q,host=%q} %.2f\n", n.Name, n.Host, m.CPUUsage))
			sb.WriteString(fmt.Sprintf("devicegrid_mem_usage{name=%q,host=%q} %.2f\n", n.Name, n.Host, float64(m.MemUsed)/float64(m.MemTotal)*100))
			sb.WriteString(fmt.Sprintf("devicegrid_disk_usage{name=%q,host=%q} %.2f\n", n.Name, n.Host, float64(m.DiskUsed)/float64(m.DiskTotal)*100))
		}
	}

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(200, sb.String())
}

// ===== Batch File Distribution =====

type FileDistributeHandler struct {
	router *Router
}

func NewFileDistributeHandler(r *Router) *FileDistributeHandler {
	return &FileDistributeHandler{router: r}
}

func (h *FileDistributeHandler) Distribute(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "no file uploaded")
		return
	}

	nodeIDs := strings.Split(c.PostForm("node_ids"), ",")
	remotePath := c.PostForm("remote_path")
	if remotePath == "" {
		remotePath = "/tmp/" + file.Filename
	}

	if len(nodeIDs) == 0 || nodeIDs[0] == "" {
		BadRequest(c, "node_ids required")
		return
	}

	src, err := file.Open()
	if err != nil {
		InternalError(c, "open file: "+err.Error())
		return
	}
	defer src.Close()

	results := gin.H{"success": 0, "failed": 0, "details": []gin.H{}}
	details := []gin.H{}

	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}
		// Re-open file for each node
		src2, _ := file.Open()
		err := h.router.sshMgr.SFTPUpload(context.Background(), nodeID, remotePath, src2)
		src2.Close()
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			details = append(details, gin.H{"node_id": nodeID, "status": "failed", "error": err.Error()})
		} else {
			results["success"] = results["success"].(int) + 1
			details = append(details, gin.H{"node_id": nodeID, "status": "success", "path": remotePath})
		}
	}
	results["details"] = details
	OK(c, results)
}

// Dummy to avoid unused import
var _ = time.Second
