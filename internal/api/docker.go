package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/michael/device_grid/internal/docker"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type DockerHandler struct {
	repos  repo.Repositories
	docker *docker.Manager
	hub    hubBroadcaster
}

type hubBroadcaster interface {
	Broadcast(topic string, data interface{})
}

func NewDockerHandler(repos repo.Repositories, tm *transport.Manager, hub hubBroadcaster) *DockerHandler {
	return &DockerHandler{
		repos:  repos,
		docker: docker.NewManager(tm),
		hub:    hub,
	}
}

func (h *DockerHandler) Info(c *gin.Context) {
	nodeID := c.Param("id")
	installed, version, err := h.docker.IsInstalled(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "check docker: "+err.Error())
		return
	}
	OK(c, gin.H{
		"installed": installed,
		"version":   version,
	})
}

func (h *DockerHandler) Install(c *gin.Context) {
	nodeID := c.Param("id")
	var opts docker.InstallOptions
	_ = c.ShouldBindJSON(&opts)

	stream, err := h.docker.Install(c.Request.Context(), nodeID, opts)
	if err != nil {
		Error(c, http.StatusBadGateway, "start install: "+err.Error())
		return
	}

	go h.streamToHub(stream, "docker-install-"+nodeID)
	OK(c, gin.H{"status": "installing", "message": "Docker installation started, check logs via WebSocket"})
}

func (h *DockerHandler) Uninstall(c *gin.Context) {
	nodeID := c.Param("id")
	stream, err := h.docker.Uninstall(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "start uninstall: "+err.Error())
		return
	}

	go h.streamToHub(stream, "docker-uninstall-"+nodeID)
	OK(c, gin.H{"status": "uninstalling"})
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	nodeID := c.Param("id")
	all := c.Query("all") == "true"
	containers, err := h.docker.ListContainers(c.Request.Context(), nodeID, all)
	if err != nil {
		Error(c, http.StatusBadGateway, "list containers: "+err.Error())
		return
	}
	OK(c, containers)
}

func (h *DockerHandler) CreateContainer(c *gin.Context) {
	nodeID := c.Param("id")
	var req docker.CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	id, err := h.docker.CreateContainer(c.Request.Context(), nodeID, req)
	if err != nil {
		Error(c, http.StatusBadGateway, "create container: "+err.Error())
		return
	}
	Created(c, gin.H{"id": id})
}

func (h *DockerHandler) ContainerAction(c *gin.Context) {
	nodeID := c.Param("id")
	containerID := c.Param("cid")
	action := model.ContainerAction(c.Query("action"))
	if action == "" {
		var body struct {
			Action string `json:"action"`
		}
		_ = c.ShouldBindJSON(&body)
		action = model.ContainerAction(body.Action)
	}

	if action == "" {
		BadRequest(c, "action is required")
		return
	}

	output, err := h.docker.ContainerAction(c.Request.Context(), nodeID, containerID, action)
	if err != nil {
		Error(c, http.StatusBadGateway, "container action: "+err.Error())
		return
	}
	OK(c, gin.H{"output": output})
}

func (h *DockerHandler) ListImages(c *gin.Context) {
	nodeID := c.Param("id")
	images, err := h.docker.ListImages(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "list images: "+err.Error())
		return
	}
	OK(c, images)
}

func (h *DockerHandler) PullImage(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Image string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "image is required")
		return
	}

	stream, err := h.docker.PullImage(c.Request.Context(), nodeID, req.Image)
	if err != nil {
		Error(c, http.StatusBadGateway, "pull image: "+err.Error())
		return
	}

	go h.streamToHub(stream, "docker-pull-"+nodeID)
	OK(c, gin.H{"status": "pulling"})
}

func (h *DockerHandler) RemoveImage(c *gin.Context) {
	nodeID := c.Param("id")
	imageID := c.Param("iid")
	force := c.Query("force") == "true"

	if err := h.docker.RemoveImage(c.Request.Context(), nodeID, imageID, force); err != nil {
		Error(c, http.StatusBadGateway, "remove image: "+err.Error())
		return
	}
	OK(c, gin.H{"removed": true})
}

func (h *DockerHandler) ComposeUp(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name   string `json:"name" binding:"required"`
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "name and config are required")
		return
	}

	stream, err := h.docker.ComposeUp(c.Request.Context(), nodeID, req.Name, req.Config)
	if err != nil {
		Error(c, http.StatusBadGateway, "compose up: "+err.Error())
		return
	}

	go h.streamToHub(stream, "docker-compose-"+nodeID)
	OK(c, gin.H{"status": "starting"})
}

func (h *DockerHandler) ComposeDown(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "name is required")
		return
	}

	stream, err := h.docker.ComposeDown(c.Request.Context(), nodeID, req.Name)
	if err != nil {
		Error(c, http.StatusBadGateway, "compose down: "+err.Error())
		return
	}

	go h.streamToHub(stream, "docker-compose-"+nodeID)
	OK(c, gin.H{"status": "stopping"})
}

func (h *DockerHandler) ListNetworks(c *gin.Context) {
	nodeID := c.Param("id")
	networks, err := h.docker.ListNetworks(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "list networks: "+err.Error())
		return
	}
	OK(c, networks)
}

func (h *DockerHandler) ListVolumes(c *gin.Context) {
	nodeID := c.Param("id")
	volumes, err := h.docker.ListVolumes(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "list volumes: "+err.Error())
		return
	}
	OK(c, volumes)
}

func (h *DockerHandler) streamToHub(stream <-chan transport.StreamChunk, topic string) {
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
