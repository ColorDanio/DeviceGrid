package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Crypto   CryptoConfig   `mapstructure:"crypto"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Agent    AgentConfig    `mapstructure:"agent"`
	SSH      SSHConfig      `mapstructure:"ssh"`
	Deploy   DeployConfig   `mapstructure:"deploy"`
	Network  NetworkConfig  `mapstructure:"network"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type AuthConfig struct {
	JWTSecret string        `mapstructure:"jwt_secret"`
	JWTExpire time.Duration `mapstructure:"jwt_expire"`
}

type CryptoConfig struct {
	MasterKey string `mapstructure:"master_key"`
}

type DatabaseConfig struct {
	Driver  string         `mapstructure:"driver"`
	SQLite  SQLiteConfig   `mapstructure:"sqlite"`
	MongoDB MongoDBConfig  `mapstructure:"mongodb"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type MongoDBConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AgentConfig struct {
	GRPCPort   int    `mapstructure:"grpc_port"`
	CACert     string `mapstructure:"ca_cert"`
	ServerCert string `mapstructure:"server_cert"`
	ServerKey  string `mapstructure:"server_key"`
	AgentCert  string `mapstructure:"agent_cert"`
	AgentKey   string `mapstructure:"agent_key"`
}

type SSHConfig struct {
	KeyAlgorithm     string        `mapstructure:"key_algorithm"`
	ConnectTimeout   time.Duration `mapstructure:"connect_timeout"`
	KeepaliveInterval time.Duration `mapstructure:"keepalive_interval"`
	MaxConnections   int           `mapstructure:"max_connections"`
}

type DeployConfig struct {
	MaxConcurrent int           `mapstructure:"max_concurrent"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

type NetworkConfig struct {
	Environment     string `mapstructure:"environment"`      // "public" (default) | "internal"
	EnableGeoLookup bool   `mapstructure:"enable_geo"`       // IP geo lookup (auto: true for public, false for internal)
	EnableStreamingCheck bool `mapstructure:"enable_streaming"` // Streaming unlock detection
	EnableAICheck   bool   `mapstructure:"enable_ai"`        // AI service availability
	EnableConnectivityTest bool `mapstructure:"enable_connectivity"` // Global connectivity test
	EnableReturnRoute bool  `mapstructure:"enable_route"`    // China ISP return route test
}

func (n *NetworkConfig) IsInternal() bool { return n.Environment == "internal" }

// ApplyDefaults sets smart defaults based on environment
func (n *NetworkConfig) ApplyDefaults() {
	if n.Environment == "" {
		n.Environment = "public"
	}
	internal := n.IsInternal()
	// Only set if not explicitly configured (zero value)
	if !n.EnableGeoLookup && !n.EnableStreamingCheck && !n.EnableAICheck && !n.EnableConnectivityTest && !n.EnableReturnRoute {
		// All zero → apply environment-based defaults
		n.EnableGeoLookup = !internal
		n.EnableStreamingCheck = !internal
		n.EnableAICheck = !internal
		n.EnableConnectivityTest = !internal
		n.EnableReturnRoute = !internal
	}
}

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetEnvPrefix("DG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.Network.ApplyDefaults()

	if cfg.Crypto.MasterKey == "" {
		cfg.Crypto.MasterKey = os.Getenv("DG_CRYPTO_MASTER_KEY")
	}

	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = os.Getenv("DG_AUTH_JWT_SECRET")
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")

	v.SetDefault("auth.jwt_expire", "24h")

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.sqlite.path", "./data/device_grid.db")
	v.SetDefault("database.mongodb.uri", "mongodb://localhost:27017")
	v.SetDefault("database.mongodb.database", "device_grid")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)

	v.SetDefault("agent.grpc_port", 9090)

	v.SetDefault("ssh.key_algorithm", "ed25519")
	v.SetDefault("ssh.connect_timeout", "10s")
	v.SetDefault("ssh.keepalive_interval", "30s")
	v.SetDefault("ssh.max_connections", 50)

	v.SetDefault("deploy.max_concurrent", 20)
	v.SetDefault("deploy.timeout", "30m")

	v.SetDefault("network.environment", "public")
}

func (c *Config) validate() error {
	if c.Database.Driver != "sqlite" && c.Database.Driver != "mongodb" {
		return fmt.Errorf("invalid database driver: %s (must be 'sqlite' or 'mongodb')", c.Database.Driver)
	}
	if c.Deploy.MaxConcurrent < 1 {
		return fmt.Errorf("deploy.max_concurrent must be >= 1")
	}
	// In release mode, require explicit secrets
	if c.Server.Mode == "release" {
		if c.Auth.JWTSecret == "" {
			return fmt.Errorf("auth.jwt_secret must be set in release mode (use DG_AUTH_JWT_SECRET env var)")
		}
		if c.Crypto.MasterKey == "" {
			return fmt.Errorf("crypto.master_key must be set in release mode (use DG_CRYPTO_MASTER_KEY env var)")
		}
	}
	return nil
}

func (c *Config) IsDebug() bool {
	return c.Server.Mode == "debug"
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
