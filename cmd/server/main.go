package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/michael/device_grid/internal/agent"
	agentpb "github.com/michael/device_grid/internal/agent/proto"
	"github.com/michael/device_grid/internal/api"
	"github.com/michael/device_grid/internal/auth"
	"github.com/michael/device_grid/internal/config"
	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/node"
	"github.com/michael/device_grid/internal/ssh"
	_ "github.com/michael/device_grid/internal/store/mongodb"
	"github.com/michael/device_grid/internal/store/repo"
	_ "github.com/michael/device_grid/internal/store/sqlite"
	"github.com/michael/device_grid/internal/transport"
	agenttransport "github.com/michael/device_grid/internal/transport/agent"
	sshtransport "github.com/michael/device_grid/internal/transport/ssh"
	"github.com/michael/device_grid/internal/web"
	"github.com/michael/device_grid/internal/ws"
	"google.golang.org/grpc"
)

func main() {
	configPath := os.Getenv("DG_CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	level := slog.LevelInfo
	if cfg.IsDebug() {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	masterKey := cfg.Crypto.MasterKey
	if masterKey == "" {
		masterKey, err = crypto.GenerateMasterKey()
		if err != nil {
			log.Fatalf("generate master key: %v", err)
		}
		// Persist to a separate key file (NOT config.yaml) with restricted permissions
		keyFile := filepath.Join(filepath.Dir(configPath), ".master_key")
		os.WriteFile(keyFile, []byte(masterKey), 0600)
		slog.Warn("crypto.master_key not set, generated and saved to " + keyFile + " (chmod 600)")
		slog.Warn("For production: set DG_CRYPTO_MASTER_KEY env var and remove this file")
	}

	enc, err := crypto.New(masterKey)
	if err != nil {
		log.Fatalf("init crypto: %v", err)
	}

	// Auto-generate JWT secret in debug mode (use crypto/rand, not timestamp)
	if cfg.Auth.JWTSecret == "" && cfg.IsDebug() {
		jwtBytes := make([]byte, 32)
		rand.Read(jwtBytes)
		cfg.Auth.JWTSecret = fmt.Sprintf("%x", jwtBytes)
		slog.Warn("auth.jwt_secret not set, auto-generated for debug mode. Set DG_AUTH_JWT_SECRET for production")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("connecting database", "driver", cfg.Database.Driver)
	repos, err := repo.New(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer repos.Close()

	if err := repos.Ping(ctx); err != nil {
		log.Fatalf("ping store: %v", err)
	}
	slog.Info("database connected")

	if err := api.EnsureDefaultUser(repos); err != nil {
		slog.Error("ensure default user", "error", err)
	}

	// Backfill geo data for existing nodes (async, non-blocking)
	go api.BackfillGeo(repos, cfg.Network.EnableGeoLookup)

	sshMgr := ssh.NewManager(repos, enc, ssh.Config{
		ConnectTimeout:    cfg.SSH.ConnectTimeout,
		KeepaliveInterval: cfg.SSH.KeepaliveInterval,
		MaxConnections:    cfg.SSH.MaxConnections,
	})
	defer sshMgr.Close()

	sshTrans := sshtransport.New(sshMgr)
	agentTrans := agenttransport.New(repos, enc)

	agentReg := agent.NewRegistry()
	tunnelTrans := agent.NewTunnelTransport(agentReg)

	// Tunnel transport takes priority when agent is connected, falls back to agent gRPC then SSH
	transportMgr := transport.NewManager(sshTrans, tunnelTrans, repos, enc)
	transportMgr.SetTunnelChecker(agentReg.IsConnected)
	_ = agentTrans // kept for direct agent gRPC fallback

	// Start gRPC tunnel server (agents connect here)
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Agent.GRPCPort))
	if err != nil {
		log.Fatalf("listen grpc: %v", err)
	}
	grpcServer := grpc.NewServer()
	agentpb.RegisterTunnelServiceServer(grpcServer, agent.NewTunnelServer(agentReg))
	go func() {
		slog.Info("grpc tunnel server starting", "port", cfg.Agent.GRPCPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			slog.Error("grpc serve", "error", err)
		}
	}()

	hub := ws.NewHub()
	go hub.Run()

	healthChecker := node.NewHealthChecker(repos, transportMgr, hub)
	healthChecker.SetInterval(cfg.Node.HealthCheckInterval)
	healthChecker.Start()
	defer healthChecker.Stop()

	metricsCache := node.NewMetricsCache(transportMgr, repos, hub)
	metricsCache.SetInterval(cfg.Node.MetricsInterval)
	metricsCache.SetConcurrency(cfg.Node.MetricsConcurrency)
	metricsCache.Start()
	defer metricsCache.Stop()

	alertMgr := node.NewAlertManager(repos, transportMgr)
	// Default alert rules
	alertMgr.SetRules([]node.AlertRule{
		{ID: "default-offline", Name: "节点离线告警", Enabled: true, Metric: "node_offline", Operator: ">", Threshold: 0, CooldownM: 30},
		{ID: "default-cpu", Name: "CPU > 90%", Enabled: true, Metric: "cpu", Operator: ">", Threshold: 90, CooldownM: 15},
		{ID: "default-disk", Name: "磁盘 > 90%", Enabled: true, Metric: "disk", Operator: ">", Threshold: 90, CooldownM: 60},
	})

	// Alert checker goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		alertMgr.CheckAll(context.Background())
		for range ticker.C {
			alertMgr.CheckAll(context.Background())
		}
	}()

	// Cron scheduler
	cronSched := node.NewCronScheduler(repos, transportMgr)
	cronSched.Start()
	defer cronSched.Stop()

	jm := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpire)
	router := api.NewRouter(repos, jm, enc, transportMgr, hub, sshMgr, metricsCache, cfg.Network, alertMgr, cronSched)
	router.SetCORSOrigins(cfg.Server.CORSOrigins)
	router.SetDeployMaxConcurrency(cfg.Deploy.MaxConcurrent)
	engine := router.Setup(cfg.Server.Mode)

	web.RegisterStaticFiles(engine, cfg.IsDebug())

	srv := &http.Server{
		Addr:         cfg.ListenAddr(),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: 0, // Disable write timeout for WebSocket/long-poll endpoints
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr(), "mode", cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "error", err)
	}
	grpcServer.GracefulStop()
	slog.Info("server stopped")
}

func persistMasterKey(configPath, key string) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("read config for master key persist", "error", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "master_key:") {
			lines[i] = "  master_key: \"" + key + "\""
			found = true
			break
		}
	}

	var newContent string
	if found {
		newContent = strings.Join(lines, "\n")
	} else {
		cryptoHeader := "crypto:\n  master_key: \"" + key + "\"\n"
		newContent = cryptoHeader + string(content)
	}

	if err := os.WriteFile(configPath, []byte(newContent), 0600); err != nil {
		slog.Error("persist master key", "error", err)
	}
}
