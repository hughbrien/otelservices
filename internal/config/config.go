package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	ClickHouse  ClickHouseConfig  `yaml:"clickhouse"`
	OTLP        OTLPConfig        `yaml:"otlp"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
	Performance PerformanceConfig `yaml:"performance"`
}

// ServerConfig contains server-specific settings
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// ClickHouseConfig contains ClickHouse connection settings
type ClickHouseConfig struct {
	Addresses       []string      `yaml:"addresses"`
	Database        string        `yaml:"database"`
	Username        string        `yaml:"username"`
	Password        string        `yaml:"password"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	DialTimeout     time.Duration `yaml:"dial_timeout"`
	Compression     string        `yaml:"compression"`
	TLSEnabled      bool          `yaml:"tls_enabled"`
	TLSSkipVerify   bool          `yaml:"tls_skip_verify"`
}

// OTLPConfig contains OTLP receiver settings
type OTLPConfig struct {
	GRPCPort      int  `yaml:"grpc_port"`
	HTTPPort      int  `yaml:"http_port"`
	EnableGRPC    bool `yaml:"enable_grpc"`
	EnableHTTP    bool `yaml:"enable_http"`
	MaxRecvMsgSizeMB int `yaml:"max_recv_msg_size_mb"`
}

// MonitoringConfig contains monitoring and observability settings
type MonitoringConfig struct {
	MetricsPort     int           `yaml:"metrics_port"`
	MetricsPath     string        `yaml:"metrics_path"`
	LogLevel        string        `yaml:"log_level"`
	LogFormat       string        `yaml:"log_format"`
	HealthCheckPath string        `yaml:"health_check_path"`
	ReadyCheckPath  string        `yaml:"ready_check_path"`
	TraceSampleRate float64       `yaml:"trace_sample_rate"`
}

// PerformanceConfig contains performance tuning settings
type PerformanceConfig struct {
	BatchSize            int           `yaml:"batch_size"`
	BatchTimeout         time.Duration `yaml:"batch_timeout"`
	MaxBatchBytes        int           `yaml:"max_batch_bytes"`
	WorkerCount          int           `yaml:"worker_count"`
	QueueSize            int           `yaml:"queue_size"`
	MemoryLimitMiB       int           `yaml:"memory_limit_mib"`
	MemorySpikeLimit     int           `yaml:"memory_spike_limit"`
	RetryMaxAttempts     int           `yaml:"retry_max_attempts"`
	RetryInitialInterval time.Duration `yaml:"retry_initial_interval"`
	RetryMaxInterval     time.Duration `yaml:"retry_max_interval"`
	CacheTTL             time.Duration `yaml:"cache_ttl"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.ClickHouse.Addresses) == 0 {
		return fmt.Errorf("clickhouse addresses cannot be empty")
	}
	if c.ClickHouse.Database == "" {
		return fmt.Errorf("clickhouse database cannot be empty")
	}
	if c.Performance.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if c.Performance.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(config *Config) {
	if val := os.Getenv("CLICKHOUSE_HOST"); val != "" {
		config.ClickHouse.Addresses = []string{val}
	}
	if val := os.Getenv("CLICKHOUSE_DATABASE"); val != "" {
		config.ClickHouse.Database = val
	}
	if val := os.Getenv("CLICKHOUSE_USERNAME"); val != "" {
		config.ClickHouse.Username = val
	}
	if val := os.Getenv("CLICKHOUSE_PASSWORD"); val != "" {
		config.ClickHouse.Password = val
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		config.Monitoring.LogLevel = val
	}
	if val := os.Getenv("OTLP_GRPC_PORT"); val != "" {
		fmt.Sscanf(val, "%d", &config.OTLP.GRPCPort)
	}
	if val := os.Getenv("OTLP_HTTP_PORT"); val != "" {
		fmt.Sscanf(val, "%d", &config.OTLP.HTTPPort)
	}
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		ClickHouse: ClickHouseConfig{
			Addresses:       []string{"localhost:9000"},
			Database:        "otel",
			Username:        "default",
			Password:        "",
			MaxOpenConns:    50,
			MaxIdleConns:    5,
			ConnMaxLifetime: 1 * time.Hour,
			DialTimeout:     10 * time.Second,
			Compression:     "zstd",
		},
		OTLP: OTLPConfig{
			GRPCPort:         4317,
			HTTPPort:         4318,
			EnableGRPC:       true,
			EnableHTTP:       true,
			MaxRecvMsgSizeMB: 4,
		},
		Monitoring: MonitoringConfig{
			MetricsPort:     9090,
			MetricsPath:     "/metrics",
			LogLevel:        "info",
			LogFormat:       "json",
			HealthCheckPath: "/health",
			ReadyCheckPath:  "/ready",
			TraceSampleRate: 0.1,
		},
		Performance: PerformanceConfig{
			BatchSize:            10000,
			BatchTimeout:         10 * time.Second,
			MaxBatchBytes:        100 * 1024 * 1024, // 100MB
			WorkerCount:          4,
			QueueSize:            100000,
			MemoryLimitMiB:       3200,
			MemorySpikeLimit:     800,
			RetryMaxAttempts:     5,
			RetryInitialInterval: 1 * time.Second,
			RetryMaxInterval:     30 * time.Second,
			CacheTTL:             15 * time.Minute,
		},
	}
}