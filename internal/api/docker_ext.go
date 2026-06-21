package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/michael/device_grid/internal/model"
)

// ContainerLogWS streams container logs via WebSocket
func (r *Router) handleContainerLogWS(conn *websocket.Conn, nodeID, containerID string) {
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dockerPath := `DPATH=""; for p in /usr/bin/docker /usr/local/bin/docker /snap/bin/docker; do [ -x "$p" ] && DPATH="$p" && break; done; [ -z "$DPATH" ] && DPATH=docker; `

	cmd := dockerPath + fmt.Sprintf(`$DPATH logs --tail 200 --follow %s 2>&1`, containerID)

	stream, err := r.transport.ExecStream(ctx, nodeID, cmd)
	if err != nil {
		return
	}

	// Keepalive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}()

	// Read client close
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Stream logs to client
	go func() {
		for chunk := range stream {
			data, _ := json.Marshal(wsServerMsg{
				Type: "output",
				Data: base64.StdEncoding.EncodeToString([]byte(chunk.Data)),
			})
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				cancel()
				return
			}
		}
		done, _ := json.Marshal(wsServerMsg{Type: "done"})
		conn.WriteMessage(websocket.TextMessage, done)
	}()

	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Minute):
	}
}

// ContainerStats gets real-time stats for a container
func (h *DockerHandler) ContainerStats(c *gin.Context) {
	nodeID := c.Param("id")
	containerID := c.Param("cid")
	stats, err := h.docker.ContainerStats(c.Request.Context(), nodeID, containerID)
	if err != nil {
		Error(c, http.StatusBadGateway, "get stats: "+err.Error())
		return
	}
	OK(c, stats)
}

// BatchContainerAction performs an action on containers across multiple nodes
func (h *DockerHandler) BatchContainerAction(c *gin.Context) {
	var req struct {
		Actions []struct {
			NodeID      string `json:"node_id"`
			ContainerID string `json:"container_id"`
			Action      string `json:"action"`
		} `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid: "+err.Error())
		return
	}

	results := gin.H{"success": 0, "failed": 0, "details": []gin.H{}}
	details := []gin.H{}
	for _, a := range req.Actions {
		_, err := h.docker.ContainerAction(c.Request.Context(), a.NodeID, a.ContainerID, model.ContainerAction(a.Action))
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			details = append(details, gin.H{"container_id": a.ContainerID, "status": "failed", "error": err.Error()})
		} else {
			results["success"] = results["success"].(int) + 1
			details = append(details, gin.H{"container_id": a.ContainerID, "status": "success"})
		}
	}
	results["details"] = details
	OK(c, results)
}
