package api

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditLog records all authenticated API operations
func AuditLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip health checks and static files
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/assets") || path == "/" || path == "/favicon.svg" {
			c.Next()
			return
		}

		start := time.Now()

		c.Next()

		// Only log mutations (POST/PUT/DELETE)
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "DELETE" {
			return
		}

		username, _ := c.Get("username")
		if username == nil {
			username = "anonymous"
		}

		slog.Info("audit",
			"method", method,
			"path", path,
			"user", username,
			"ip", c.ClientIP(),
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
		)
	}
}
