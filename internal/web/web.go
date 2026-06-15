package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

func RegisterStaticFiles(engine *gin.Engine, debug bool) {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("embed dist: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	engine.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")

		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(subFS, path); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		indexData, err := fs.ReadFile(subFS, "index.html")
		if err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}
