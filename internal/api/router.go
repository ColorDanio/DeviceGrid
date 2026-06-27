package api

import (
	"net/http"
	"strings"
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
	"github.com/michael/device_grid/internal/version"
	"github.com/michael/device_grid/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow same-origin requests
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Non-browser clients
		}
		host := r.Host
		// Allow if origin matches the server host
		if strings.Contains(origin, host) {
			return true
		}
		// Allow localhost in debug mode
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			return true
		}
		return false
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
	alertMgr     *node.AlertManager
	cronSched    *node.CronScheduler
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
	alertMgr *node.AlertManager,
	cronSched *node.CronScheduler,
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
		alertMgr:     alertMgr,
		cronSched:    cronSched,
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

	engine.GET("/api/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":    version.Version,
			"build_date": version.BuildDate,
			"git_commit": version.GitCommit,
		})
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
				authGroup.POST("/change-password", authHandler.ChangePassword)
			}

			// Read-only routes: all authenticated users (admin, operator, viewer)
			// Mutation routes: admin + operator only
			// Admin-only routes: user management, cluster delete

			r.registerNodeRoutes(protected)
			r.registerSFTPRoutes(protected)
			r.registerDockerRoutes(protected)
			r.registerDeployRoutes(protected)
			r.registerRKE2Routes(protected)
			r.registerAlertRoutes(protected)
			r.registerCronRoutes(protected)

			// User management: admin only
			adminOnly := protected.Group("")
			adminOnly.Use(auth.RoleRequired("admin"))
			{
				adminOnly.GET("/users", func(c *gin.Context) {
					h := NewUserHandler(r.repos, r.jm)
					h.List(c)
				})
				adminOnly.POST("/users", func(c *gin.Context) {
					h := NewUserHandler(r.repos, r.jm)
					h.Create(c)
				})
				adminOnly.DELETE("/users/:uid", func(c *gin.Context) {
					h := NewUserHandler(r.repos, r.jm)
					h.Delete(c)
				})
			}
		}
	}

	r.registerWSRoutes(engine, RateLimit(60, time.Minute))

	return engine
}

func (r *Router) registerWSRoutes(engine *gin.Engine, wsRateLimit gin.HandlerFunc) {
	engine.Use(wsRateLimit)
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

	// Container exec (terminal) WebSocket
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

	// Container log WebSocket
	engine.GET("/ws/logs/:nodeID/:containerID", func(c *gin.Context) {
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
		r.handleContainerLogWS(conn, nodeID, containerID)
	})
}

func (r *Router) registerNodeRoutes(rg *gin.RouterGroup) {
	h := NewNodeHandler(r.repos, r.enc, r.transport, r.sshMgr, r.metricsCache, r.network.EnableGeoLookup)
	nodes := rg.Group("/nodes")
	{
		// Read: all authenticated users
		nodes.GET("", h.List)
		nodes.GET("/:id", h.Get)
		nodes.GET("/:id/metrics", h.Metrics)
		nodes.GET("/:id/processes", h.TopProcesses)
		nodes.GET("/:id/logins", h.LoginHistory)

		// Mutations: admin + operator
		opNodes := nodes.Group("", auth.RoleRequired("admin", "operator"))
		{
			opNodes.POST("", h.Create)
			opNodes.PUT("/:id", h.Update)
			opNodes.DELETE("/:id", h.Delete)
			opNodes.POST("/:id/health", h.Health)
			opNodes.POST("/:id/trust", h.Trust)
			opNodes.POST("/:id/deploy-agent", h.DeployAgent)
			opNodes.POST("/:id/facts", h.Facts)
			opNodes.POST("/:id/refresh-geo", h.RefreshGeo)
		}
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
		// Read: all users
		docker.GET("/info", h.Info)
		docker.GET("/containers", h.ListContainers)
		docker.GET("/images", h.ListImages)
		docker.GET("/networks", h.ListNetworks)
		docker.GET("/volumes", h.ListVolumes)

		// Mutations: admin + operator
		opDocker := docker.Group("", auth.RoleRequired("admin", "operator"))
		{
			opDocker.POST("/install", h.Install)
			opDocker.DELETE("", h.Uninstall)
			opDocker.POST("/containers", h.CreateContainer)
			opDocker.POST("/containers/:cid/action", h.ContainerAction)
			opDocker.POST("/images/pull", h.PullImage)
			opDocker.DELETE("/images/:iid", h.RemoveImage)
			opDocker.POST("/compose", h.ComposeUp)
			opDocker.DELETE("/compose", h.ComposeDown)
			opDocker.GET("/containers/:cid/stats", h.ContainerStats)
			opDocker.POST("/batch-action", h.BatchContainerAction)
		}
	}
}

func (r *Router) registerDeployRoutes(rg *gin.RouterGroup) {
	h := NewDeployHandler(r.repos, deploy.NewEngine(r.repos, r.transport, r.hub))
	deploy := rg.Group("/deploys")
	{
		deploy.GET("", h.List)
		deploy.GET("/:tid", h.Get)
		// Mutations: admin + operator
		opDeploy := deploy.Group("", auth.RoleRequired("admin", "operator"))
		{
			opDeploy.POST("", h.Create)
			opDeploy.DELETE("/:tid", h.Cancel)
		}
	}
}

func (r *Router) registerRKE2Routes(rg *gin.RouterGroup) {
	h := NewRKE2Handler(r.repos, r.transport, r.hub)
	clusters := rg.Group("/clusters")
	{
		// Read: all users
		clusters.GET("", h.List)
		clusters.GET("/:cid", h.Get)
		clusters.GET("/:cid/status", h.Status)
		clusters.GET("/:cid/helm", h.HelmList)
		clusters.GET("/:cid/rancher", h.RancherStatus)
		clusters.GET("/:cid/pods", h.GetPods)

		// Mutations: admin + operator
		opClusters := clusters.Group("", auth.RoleRequired("admin", "operator"))
		{
			opClusters.POST("", h.Create)
			opClusters.PUT("/:cid/config", h.UpdateConfig)
			opClusters.POST("/:cid/upgrade", h.Upgrade)
			opClusters.POST("/:cid/helm/install", h.HelmInstall)
			opClusters.DELETE("/:cid/helm/:release", h.HelmUninstall)
			opClusters.POST("/:cid/rancher", h.InstallRancher)
		}

		// Cluster delete: admin only
		adminClusters := clusters.Group("", auth.RoleRequired("admin"))
		{
			adminClusters.DELETE("/:cid", h.Delete)
		}
	}

	// Pre-flight check (per node)
	{
		rg.GET("/nodes/:id/preflight", h.PreFlightCheck)
		rg.POST("/nodes/:id/preflight", h.PreFlightCheck)
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

func (r *Router) registerAlertRoutes(rg *gin.RouterGroup) {
	ah := NewAlertHandler(r.alertMgr)
	alerts := rg.Group("/alerts")
	{
		alerts.GET("/rules", ah.ListRules)
		alerts.POST("/rules", ah.CreateRule)
		alerts.DELETE("/rules/:rid", ah.DeleteRule)
		alerts.POST("/test", ah.TestWebhook)
	}
}

func (r *Router) registerCronRoutes(rg *gin.RouterGroup) {
	ch := NewCronHandler(r.cronSched)
	cron := rg.Group("/cron")
	{
		cron.GET("", ch.List)
		cron.POST("", ch.Create)
		cron.DELETE("/:tid", ch.Delete)
		cron.POST("/:tid/toggle", ch.Toggle)
	}
}


