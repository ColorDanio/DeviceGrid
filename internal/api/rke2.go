package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/rke2"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type RKE2Handler struct {
	repos repo.Repositories
	rke2  *rke2.Manager
	hub   hubBroadcaster
}

func NewRKE2Handler(repos repo.Repositories, tm *transport.Manager, hub hubBroadcaster) *RKE2Handler {
	return &RKE2Handler{
		repos: repos,
		rke2:  rke2.NewManager(repos, tm),
		hub:   hub,
	}
}

func (h *RKE2Handler) List(c *gin.Context) {
	clusters, err := h.repos.Clusters().List(c.Request.Context())
	if err != nil {
		InternalError(c, "list clusters: "+err.Error())
		return
	}
	OK(c, clusters)
}

func (h *RKE2Handler) Get(c *gin.Context) {
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), c.Param("cid"))
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}
	OK(c, cluster)
}

type createClusterRequest struct {
	Name       string             `json:"name" binding:"required"`
	Version    string             `json:"version"`
	ServerNode string             `json:"server_node" binding:"required"`
	AgentNodes []string           `json:"agent_nodes"`
	Config     rke2.ClusterConfig `json:"config"`
}

func (h *RKE2Handler) Create(c *gin.Context) {
	var req createClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	clusterID := uuid.NewString()
	configContent := rke2.DefaultServerConfig(req.Config)

	cluster := &model.Cluster{
		ID:         clusterID,
		Name:       req.Name,
		Version:    req.Version,
		ServerNode: req.ServerNode,
		Config:     configContent,
		Nodes: []model.ClusterNode{
			{NodeID: req.ServerNode, Role: model.RoleServer, Ready: false},
		},
		Status:    model.ClusterProvisioning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	for _, agentID := range req.AgentNodes {
		cluster.Nodes = append(cluster.Nodes, model.ClusterNode{
			NodeID: agentID,
			Role:   model.RoleAgent,
			Ready:  false,
		})
	}

	if err := h.repos.Clusters().Create(c.Request.Context(), cluster); err != nil {
		InternalError(c, "create cluster: "+err.Error())
		return
	}

	go h.provisionCluster(cluster, req.Config)

	Created(c, cluster)
}

func (h *RKE2Handler) provisionCluster(cluster *model.Cluster, cfg rke2.ClusterConfig) {
	ctx := context.Background()

	stream, err := h.rke2.InstallServer(ctx, cluster.ServerNode, cluster.Config)
	if err != nil {
		cluster.Status = model.ClusterError
		_ = h.repos.Clusters().Update(ctx, cluster)
		return
	}
	h.streamToHub(stream, "rke2-"+cluster.ID)

	token, _ := h.rke2.GetServerToken(ctx, cluster.ServerNode)
	serverURL, _ := h.rke2.GetServerURL(ctx, cluster.ServerNode)

	for _, cn := range cluster.Nodes {
		if cn.Role == model.RoleAgent {
			agentConfig := rke2.DefaultAgentConfig(serverURL, token, cfg)
			stream, err := h.rke2.InstallAgent(ctx, cn.NodeID, agentConfig)
			if err != nil {
				continue
			}
			h.streamToHub(stream, "rke2-"+cluster.ID)
		}
	}

	cluster.Status = model.ClusterHealthy
	cluster.UpdatedAt = time.Now()
	_ = h.repos.Clusters().Update(ctx, cluster)
}

func (h *RKE2Handler) UpdateConfig(c *gin.Context) {
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), c.Param("cid"))
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "config is required")
		return
	}

	cluster.Config = req.Config
	cluster.UpdatedAt = time.Now()
	if err := h.repos.Clusters().Update(c.Request.Context(), cluster); err != nil {
		InternalError(c, "update cluster: "+err.Error())
		return
	}
	OK(c, cluster)
}

func (h *RKE2Handler) Status(c *gin.Context) {
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), c.Param("cid"))
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	status, err := h.rke2.GetClusterStatus(c.Request.Context(), cluster.ServerNode)
	if err != nil {
		OK(c, gin.H{"error": err.Error(), "status": "unknown"})
		return
	}
	OK(c, status)
}

func (h *RKE2Handler) Upgrade(c *gin.Context) {
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), c.Param("cid"))
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "version is required")
		return
	}

	stream, err := h.rke2.Upgrade(c.Request.Context(), cluster.ServerNode, req.Version)
	if err != nil {
		Error(c, 502, "upgrade failed: "+err.Error())
		return
	}

	go h.streamToHub(stream, "rke2-"+cluster.ID)
	OK(c, gin.H{"status": "upgrading", "version": req.Version})
}

func (h *RKE2Handler) Delete(c *gin.Context) {
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), c.Param("cid"))
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	for _, cn := range cluster.Nodes {
		stream, _ := h.rke2.Uninstall(c.Request.Context(), cn.NodeID, cn.Role)
		go h.streamToHub(stream, "rke2-"+cluster.ID)
	}

	_ = h.repos.Clusters().Delete(c.Request.Context(), cluster.ID)
	OK(c, gin.H{"deleted": true})
}

func (h *RKE2Handler) streamToHub(stream <-chan transport.StreamChunk, topic string) {
	for chunk := range stream {
		h.hub.Broadcast(topic, map[string]interface{}{
			"type": chunk.Type,
			"data": chunk.Data,
		})
		if chunk.Type == "exit" {
			h.hub.Broadcast(topic, map[string]interface{}{
				"type":      "done",
				"exit_code": chunk.ExitCode,
			})
			return
		}
	}
}
