package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/michael/device_grid/internal/transport"
)

type K8sHandler struct {
	router *Router
}

func NewK8sHandler(r *Router) *K8sHandler {
	return &K8sHandler{router: r}
}

func (h *K8sHandler) streamToHub(stream <-chan transport.StreamChunk, topic string) {
	for chunk := range stream {
		h.router.hub.Broadcast(topic, map[string]interface{}{
			"type": chunk.Type,
			"data": chunk.Data,
		})
		if chunk.Type == "exit" {
			h.router.hub.Broadcast(topic, map[string]interface{}{"type": "done", "exit_code": chunk.ExitCode})
			return
		}
	}
}

// ApplyYAML applies Kubernetes YAML to a cluster via kubectl apply -f
func (h *K8sHandler) ApplyYAML(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.router.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "yaml content required")
		return
	}

	script := "export PATH=$PATH:/var/lib/rancher/rke2/bin\nexport KUBECONFIG=/etc/rancher/rke2/rke2.yaml\ncat << 'YAMLEOF' | kubectl apply -f -\n" + req.YAML + "\nYAMLEOF"
	stream, err := h.router.transport.ExecStream(c.Request.Context(), cluster.ServerNode, script)
	if err != nil {
		Error(c, http.StatusBadGateway, "kubectl apply failed: "+err.Error())
		return
	}
	go h.streamToHub(stream, "rke2-"+clusterID)
	OK(c, gin.H{"status": "applying"})
}

// GetResources lists Kubernetes resources
func (h *K8sHandler) GetResources(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.router.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	resourceType := c.DefaultQuery("type", "all")
	namespace := c.Query("namespace")

	nsFlag := "-A"
	if namespace != "" {
		nsFlag = "-n " + namespace
	}

	var cmd string
	switch resourceType {
	case "deployments":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get deployments " + nsFlag + " 2>/dev/null"
	case "services":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get services " + nsFlag + " 2>/dev/null"
	case "pods":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get pods " + nsFlag + " 2>/dev/null"
	case "ingress":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get ingress " + nsFlag + " 2>/dev/null"
	case "configmaps":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get configmaps " + nsFlag + " 2>/dev/null"
	case "secrets":
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get secrets " + nsFlag + " 2>/dev/null"
	default:
		cmd = "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl get all " + nsFlag + " 2>/dev/null"
	}

	result, err := h.router.transport.Exec(c.Request.Context(), cluster.ServerNode, cmd)
	if err != nil {
		Error(c, http.StatusBadGateway, "kubectl get failed: "+err.Error())
		return
	}
	OK(c, gin.H{"output": result.Stdout})
}

// DeleteResource deletes a Kubernetes resource
func (h *K8sHandler) DeleteResource(c *gin.Context) {
	clusterID := c.Param("cid")
	cluster, err := h.router.repos.Clusters().GetByID(c.Request.Context(), clusterID)
	if err != nil {
		NotFound(c, "cluster not found")
		return
	}

	var req struct {
		Kind      string `json:"kind" binding:"required"`
		Name      string `json:"name" binding:"required"`
		Namespace string `json:"namespace"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "kind and name required")
		return
	}

	nsFlag := ""
	if req.Namespace != "" {
		nsFlag = " -n " + req.Namespace
	}

	cmd := "export PATH=$PATH:/var/lib/rancher/rke2/bin; export KUBECONFIG=/etc/rancher/rke2/rke2.yaml; kubectl delete " + req.Kind + " " + req.Name + nsFlag + " 2>&1"
	result, err := h.router.transport.Exec(c.Request.Context(), cluster.ServerNode, cmd)
	if err != nil {
		Error(c, http.StatusBadGateway, "kubectl delete failed: "+err.Error())
		return
	}
	OK(c, gin.H{"output": result.Stdout})
}
