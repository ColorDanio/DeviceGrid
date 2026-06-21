package api

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/michael/device_grid/internal/auth"
	"github.com/michael/device_grid/internal/config"
	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/deploy"
	"github.com/michael/device_grid/internal/node"
	"github.com/michael/device_grid/internal/ssh"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
	"github.com/michael/device_grid/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
}

type Router struct {
	repos        repo.Repositories
	jm           *auth.JWTManager
	enc          *crypto.Encryptor
	transport    *transport.Manager
	hub          *ws.Hub
	sshMgr       *ssh.Manager
	metricsCache *node.MetricsCache
	network      config.NetworkConfig
	corsOrigins  []string
}

func NewRouter(
	repos repo.Repositories,
	jm *auth.JWTManager,
	enc *crypto.Encryptor,
	transportMgr *transport.Manager,
	hub *ws.Hub,
	sshMgr *ssh.Manager,
	metricsCache *node.MetricsCache,
	network config.NetworkConfig,
) *Router {
	return &Router{
		repos:        repos,
		jm:           jm,
		enc:          enc,
		transport:    transportMgr,
		hub:          hub,
		sshMgr:       sshMgr,
		metricsCache: metricsCache,
		network:      network,
	}
}

func (r *Router) SetCORSOrigins(origins []string) {
	r.corsOrigins = origins
}

func (r *Router) Setup(mode string) *gin.Engine {
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz", "/api/health"},
	}))
	engine.Use(AuditLog())

	if mode == "debug" {
		origins := []string{"http://localhost:5173", "http://localhost:4173"}
		if len(r.corsOrigins) > 0 {
			origins = r.corsOrigins
		}
		engine.Use(cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := engine.Group("/api")
	{
		authHandler := NewAuthHandler(r.repos, r.jm)
		api.GET("/health", func(c *gin.Context) {
			OK(c, gin.H{"status": "ok"})
		})

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", RateLimit(10, time.Minute), authHandler.Login)
		}

		protected := api.Group("")
		protected.Use(auth.AuthRequired(r.jm))
		protected.Use(RateLimit(200, time.Minute))
		{
			authGroup := protected.Group("/auth")
			{
				authGroup.POST("/refresh", authHandler.Refresh)
				authGroup.GET("/me", authHandler.Me)
			}

			r.registerNodeRoutes(protected)
			r.registerSFTPRoutes(protected)
			r.registerDockerRoutes(protected)
			r.registerDeployRoutes(protected)
			r.registerRKE2Routes(protected)
			r.registerUserRoutes(protected)
		}
	}

	r.registerWSRoutes(engine)

	return engine
}

func (r *Router) registerWSRoutes(engine *gin.Engine) {
	engine.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}
		if _, err := r.jm.Parse(token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		r.hub.HandleClient(conn)
	})

	engine.GET("/ws/terminal/:nodeID", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}
		if _, err := r.jm.Parse(token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		nodeID := c.Param("nodeID")
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		r.handleTerminalWS(conn, nodeID)
	})

	engine.GET("/ws/container/:nodeID/:containerID", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}
		if _, err := r.jm.Parse(token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		nodeID := c.Param("nodeID")
		containerID := c.Param("containerID")
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		r.handleContainerTerminalWS(conn, nodeID, containerID)
	})
}

func (r *Router) registerNodeRoutes(rg *gin.RouterGroup) {
	h := NewNodeHandler(r.repos, r.enc, r.transport, r.sshMgr, r.metricsCache, r.network.EnableGeoLookup)
	nodes := rg.Group("/nodes")
	{
		nodes.GET("", h.List)
		nodes.POST("", h.Create)
		nodes.GET("/:id", h.Get)
		nodes.PUT("/:id", h.Update)
		nodes.DELETE("/:id", h.Delete)
		nodes.POST("/:id/health", h.Health)
		nodes.POST("/:id/trust", h.Trust)
		nodes.POST("/:id/deploy-agent", h.DeployAgent)
		nodes.POST("/:id/facts", h.Facts)
		nodes.GET("/:id/metrics", h.Metrics)
		nodes.GET("/:id/processes", h.TopProcesses)
		nodes.GET("/:id/logins", h.LoginHistory)
		nodes.POST("/:id/refresh-geo", h.RefreshGeo)
	}

	// Network checks — conditional based on network config
	nc := NewNetCheckHandler(r.repos, r.transport)
	{
		if r.network.EnableStreamingCheck {
			nodes.GET("/:id/streaming", nc.StreamingCheck)
		}
		if r.network.EnableAICheck {
			nodes.GET("/:id/ai-check", nc.AICheck)
		}
		if r.network.EnableConnectivityTest {
			nodes.GET("/:id/connectivity", nc.ConnectivityTest)
		}
		if r.network.EnableReturnRoute {
			nodes.GET("/:id/return-route", nc.ReturnRouteTest)
		}
	}

	// Network feature flags endpoint
	protected := rg
	{
		protected.GET("/network/config", func(c *gin.Context) {
			OK(c, gin.H{
				"environment":         r.network.Environment,
				"enable_geo":          r.network.EnableGeoLookup,
				"enable_streaming":    r.network.EnableStreamingCheck,
				"enable_ai":           r.network.EnableAICheck,
				"enable_connectivity": r.network.EnableConnectivityTest,
				"enable_route":        r.network.EnableReturnRoute,
			})
		})
	}

	imp := NewImportHandler(r.repos, r.enc)
	{
		protected.GET("/nodes/import/template", imp.DownloadTemplate)
		protected.POST("/nodes/import", imp.Import)
		protected.GET("/nodes/export", imp.Export)
	}
}

func (r *Router) registerSFTPRoutes(rg *gin.RouterGroup) {
	h := NewSFTPHandler(r.repos, r.sshMgr)
	sftp := rg.Group("/nodes/:id/sftp")
	{
		sftp.GET("/list", h.List)
		sftp.POST("/upload", h.Upload)
		sftp.GET("/download", h.Download)
		sftp.DELETE("/delete", h.Delete)
		sftp.POST("/mkdir", h.Mkdir)
		sftp.POST("/rename", h.Rename)
	}
}

func (r *Router) registerDockerRoutes(rg *gin.RouterGroup) {
	h := NewDockerHandler(r.repos, r.transport, r.hub)
	docker := rg.Group("/nodes/:id/docker")
	{
		docker.GET("/info", h.Info)
		docker.POST("/install", h.Install)
		docker.DELETE("", h.Uninstall)
		docker.GET("/containers", h.ListContainers)
		docker.POST("/containers", h.CreateContainer)
		docker.POST("/containers/:cid/action", h.ContainerAction)
		docker.GET("/images", h.ListImages)
		docker.POST("/images/pull", h.PullImage)
		docker.DELETE("/images/:iid", h.RemoveImage)
		docker.POST("/compose", h.ComposeUp)
		docker.DELETE("/compose", h.ComposeDown)
		docker.GET("/networks", h.ListNetworks)
		docker.GET("/volumes", h.ListVolumes)
	}
}

func (r *Router) registerDeployRoutes(rg *gin.RouterGroup) {
	h := NewDeployHandler(r.repos, deploy.NewEngine(r.repos, r.transport, r.hub))
	deploy := rg.Group("/deploys")
	{
		deploy.GET("", h.List)
		deploy.POST("", h.Create)
		deploy.GET("/:tid", h.Get)
		deploy.DELETE("/:tid", h.Cancel)
	}
}

func (r *Router) registerRKE2Routes(rg *gin.RouterGroup) {
	h := NewRKE2Handler(r.repos, r.transport, r.hub)
	clusters := rg.Group("/clusters")
	{
		clusters.GET("", h.List)
		clusters.POST("", h.Create)
		clusters.GET("/:cid", h.Get)
		clusters.PUT("/:cid/config", h.UpdateConfig)
		clusters.GET("/:cid/status", h.Status)
		clusters.POST("/:cid/upgrade", h.Upgrade)
		clusters.DELETE("/:cid", h.Delete)
		// Helm management
		clusters.GET("/:cid/helm", h.HelmList)
		clusters.POST("/:cid/helm/install", h.HelmInstall)
		clusters.DELETE("/:cid/helm/:release", h.HelmUninstall)
		// Rancher
		clusters.POST("/:cid/rancher", h.InstallRancher)
		clusters.GET("/:cid/rancher", h.RancherStatus)
		// Pods
		clusters.GET("/:cid/pods", h.GetPods)
	}
}

func stubHandler(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		OK(c, gin.H{
			"message":  name + " — will be implemented in later phases",
			"endpoint": c.Request.URL.Path,
		})
	}
}

func (r *Router) registerUserRoutes(rg *gin.RouterGroup) {
	h := NewUserHandler(r.repos, r.jm)
	users := rg.Group("/users")
	{
		users.GET("", h.List)
		users.POST("", h.Create)
		users.DELETE("/:uid", h.Delete)
	}
}
