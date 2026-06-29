package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HelmList lists installed Helm releases on the cluster's server node
func (h *RKE2Handler) HelmList(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	output, err := h.rke2.HelmList(c.Request.Context(), cluster.ServerNode)
	if err != nil {
		Error(c, http.StatusBadGateway, "helm list failed: "+err.Error())
		return
	}
	OK(c, gin.H{"output": output})
}

// HelmInstall installs a Helm chart
func (h *RKE2Handler) HelmInstall(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		RepoName  string            `json:"repo_name"`
		RepoURL   string            `json:"repo_url"`
		ChartName string            `json:"chart_name" binding:"required"`
		Namespace string            `json:"namespace"`
		Values    map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	stream, err := h.rke2.HelmInstall(c.Request.Context(), cluster.ServerNode, req.RepoName, req.RepoURL, req.ChartName, req.Namespace, req.Values)
	if err != nil {
		Error(c, http.StatusBadGateway, "helm install failed: "+err.Error())
		return
	}
	go h.streamToHub(stream, "rke2-"+clusterID)
	OK(c, gin.H{"status": "installing", "chart": req.ChartName})
}

// HelmUninstall removes a Helm release
func (h *RKE2Handler) HelmUninstall(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	releaseName := c.Param("release")
	namespace := c.Query("namespace")
	stream, err := h.rke2.HelmUninstall(c.Request.Context(), cluster.ServerNode, releaseName, namespace)
	if err != nil {
		Error(c, http.StatusBadGateway, "helm uninstall failed: "+err.Error())
		return
	}
	go h.streamToHub(stream, "rke2-"+clusterID)
	OK(c, gin.H{"status": "uninstalling", "release": releaseName})
}

// InstallRancher installs Rancher Manager via Helm
func (h *RKE2Handler) InstallRancher(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		Hostname  string `json:"hostname" binding:"required"`
		Password  string `json:"password"`
		UseMirror bool   `json:"use_mirror"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "hostname is required")
		return
	}
	if req.Password == "" {
		req.Password = "admin123"
	}

	// Auto-detect mirror from server node geo
	if !req.UseMirror {
		if serverNode, _ := h.repos.Nodes().GetByID(c.Request.Context(), cluster.ServerNode); serverNode != nil {
			req.UseMirror = serverNode.CountryCode == "CN"
		}
	}

	stream, err := h.rke2.InstallRancher(c.Request.Context(), cluster.ServerNode, req.Hostname, req.Password, req.UseMirror)
	if err != nil {
		Error(c, http.StatusBadGateway, "rancher install failed: "+err.Error())
		return
	}
	go h.streamToHub(stream, "rke2-"+clusterID)
	OK(c, gin.H{
		"status":   "installing",
		"hostname": req.Hostname,
		"mirror":   req.UseMirror,
		"message":  "Rancher 正在安装，预计需要 3-5 分钟",
	})
}

// RancherStatus checks if Rancher is installed
func (h *RKE2Handler) RancherStatus(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	installed, version, err := h.rke2.GetRancherStatus(c.Request.Context(), cluster.ServerNode)
	if err != nil {
		Error(c, http.StatusBadGateway, "check rancher failed: "+err.Error())
		return
	}
	OK(c, gin.H{"installed": installed, "version": version})
}

// GetPods lists pods on the cluster
func (h *RKE2Handler) GetPods(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	namespace := c.DefaultQuery("namespace", "all")
	pods, err := h.rke2.GetClusterPods(c.Request.Context(), cluster.ServerNode, namespace)
	if err != nil {
		Error(c, http.StatusBadGateway, "get pods failed: "+err.Error())
		return
	}
	OK(c, gin.H{"pods": pods})
}

// PreFlightCheck runs hardware and system checks before RKE2 installation
func (h *RKE2Handler) PreFlightCheck(c *gin.Context) {
	nodeID := c.Param("id")
	autoFix := c.Query("autofix") == "true"

	result, err := h.rke2.PreFlightCheck(c.Request.Context(), nodeID, autoFix)
	if err != nil {
		Error(c, http.StatusBadGateway, "pre-flight check failed: "+err.Error())
		return
	}
	OK(c, result)
}
