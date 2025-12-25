package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}

	// Test ClickHouse defaults
	if len(cfg.ClickHouse.Addresses) != 1 {
		t.Errorf("Expected 1 ClickHouse address, got %d", len(cfg.ClickHouse.Addresses))
	}
	if cfg.ClickHouse.Database != "otel" {
		t.Errorf("Expected database 'otel', got %s", cfg.ClickHouse.Database)
	}

	// Test OTLP defaults
	if cfg.OTLP.GRPCPort != 4317 {
		t.Errorf("Expected OTLP gRPC port 4317, got %d", cfg.OTLP.GRPCPort)
	}
	if cfg.OTLP.HTTPPort != 4318 {
		t.Errorf("Expected OTLP HTTP port 4318, got %d", cfg.OTLP.HTTPPort)
	}

	// Test performance defaults
	if cfg.Performance.BatchSize != 10000 {
		t.Errorf("Expected batch size 10000, got %d", cfg.Performance.BatchSize)
	}
	if cfg.Performance.WorkerCount != 4 {
		t.Errorf("Expected worker count 4, got %d", cfg.Performance.WorkerCount)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "missing clickhouse addresses",
			config: &Config{
				ClickHouse: ClickHouseConfig{
					Addresses: []string{},
					Database:  "otel",
				},
				Performance: PerformanceConfig{
					BatchSize:   10000,
					WorkerCount: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "missing database",
			config: &Config{
				ClickHouse: ClickHouseConfig{
					Addresses: []string{"localhost:9000"},
					Database:  "",
				},
				Performance: PerformanceConfig{
					BatchSize:   10000,
					WorkerCount: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid batch size",
			config: &Config{
				ClickHouse: ClickHouseConfig{
					Addresses: []string{"localhost:9000"},
					Database:  "otel",
				},
				Performance: PerformanceConfig{
					BatchSize:   0,
					WorkerCount: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid worker count",
			config: &Config{
				ClickHouse: ClickHouseConfig{
					Addresses: []string{"localhost:9000"},
					Database:  "otel",
				},
				Performance: PerformanceConfig{
					BatchSize:   10000,
					WorkerCount: 0,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	configContent := `
server:
  host: "127.0.0.1"
  port: 9999
  read_timeout: 60s
  write_timeout: 60s
  shutdown_timeout: 60s

clickhouse:
  addresses:
    - "clickhouse:9000"
  database: "test_otel"
  username: "testuser"
  password: "testpass"
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 2h
  dial_timeout: 20s
  compression: "zstd"

otlp:
  grpc_port: 5317
  http_port: 5318
  enable_grpc: true
  enable_http: true
  max_recv_msg_size_mb: 8

monitoring:
  metrics_port: 9999
  metrics_path: "/prometheus"
  log_level: "debug"
  log_format: "text"
  health_check_path: "/healthz"
  ready_check_path: "/readyz"
  trace_sample_rate: 0.5

performance:
  batch_size: 5000
  batch_timeout: 5s
  max_batch_bytes: 50000000
  worker_count: 8
  queue_size: 50000
  memory_limit_mib: 2048
  memory_spike_limit: 512
  retry_max_attempts: 3
  retry_initial_interval: 500ms
  retry_max_interval: 10s
  cache_ttl: 10m
`

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify loaded values
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.ClickHouse.Database != "test_otel" {
		t.Errorf("Expected database test_otel, got %s", cfg.ClickHouse.Database)
	}
	if cfg.Performance.BatchSize != 5000 {
		t.Errorf("Expected batch size 5000, got %d", cfg.Performance.BatchSize)
	}
	if cfg.Performance.WorkerCount != 8 {
		t.Errorf("Expected worker count 8, got %d", cfg.Performance.WorkerCount)
	}
}

func TestLoadConfigWithInvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error loading nonexistent config file")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("CLICKHOUSE_HOST", "env-host:9000")
	os.Setenv("CLICKHOUSE_DATABASE", "env_db")
	os.Setenv("CLICKHOUSE_USERNAME", "env_user")
	os.Setenv("CLICKHOUSE_PASSWORD", "env_pass")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("OTLP_GRPC_PORT", "5317")
	os.Setenv("OTLP_HTTP_PORT", "5318")

	defer func() {
		os.Unsetenv("CLICKHOUSE_HOST")
		os.Unsetenv("CLICKHOUSE_DATABASE")
		os.Unsetenv("CLICKHOUSE_USERNAME")
		os.Unsetenv("CLICKHOUSE_PASSWORD")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("OTLP_GRPC_PORT")
		os.Unsetenv("OTLP_HTTP_PORT")
	}()

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.ClickHouse.Addresses[0] != "env-host:9000" {
		t.Errorf("Expected env-host:9000, got %s", cfg.ClickHouse.Addresses[0])
	}
	if cfg.ClickHouse.Database != "env_db" {
		t.Errorf("Expected env_db, got %s", cfg.ClickHouse.Database)
	}
	if cfg.ClickHouse.Username != "env_user" {
		t.Errorf("Expected env_user, got %s", cfg.ClickHouse.Username)
	}
	if cfg.ClickHouse.Password != "env_pass" {
		t.Errorf("Expected env_pass, got %s", cfg.ClickHouse.Password)
	}
	if cfg.Monitoring.LogLevel != "debug" {
		t.Errorf("Expected debug, got %s", cfg.Monitoring.LogLevel)
	}
	if cfg.OTLP.GRPCPort != 5317 {
		t.Errorf("Expected 5317, got %d", cfg.OTLP.GRPCPort)
	}
	if cfg.OTLP.HTTPPort != 5318 {
		t.Errorf("Expected 5318, got %d", cfg.OTLP.HTTPPort)
	}
}

func TestConfigTimeouts(t *testing.T) {
	cfg := DefaultConfig()

	expectedReadTimeout := 30 * time.Second
	expectedWriteTimeout := 30 * time.Second
	expectedShutdownTimeout := 30 * time.Second

	if cfg.Server.ReadTimeout != expectedReadTimeout {
		t.Errorf("Expected read timeout %v, got %v", expectedReadTimeout, cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != expectedWriteTimeout {
		t.Errorf("Expected write timeout %v, got %v", expectedWriteTimeout, cfg.Server.WriteTimeout)
	}
	if cfg.Server.ShutdownTimeout != expectedShutdownTimeout {
		t.Errorf("Expected shutdown timeout %v, got %v", expectedShutdownTimeout, cfg.Server.ShutdownTimeout)
	}
}

func TestConfigPerformanceSettings(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Performance.BatchTimeout != 10*time.Second {
		t.Errorf("Expected batch timeout 10s, got %v", cfg.Performance.BatchTimeout)
	}
	if cfg.Performance.RetryMaxAttempts != 5 {
		t.Errorf("Expected retry max attempts 5, got %d", cfg.Performance.RetryMaxAttempts)
	}
	if cfg.Performance.RetryInitialInterval != 1*time.Second {
		t.Errorf("Expected retry initial interval 1s, got %v", cfg.Performance.RetryInitialInterval)
	}
	if cfg.Performance.CacheTTL != 15*time.Minute {
		t.Errorf("Expected cache TTL 15m, got %v", cfg.Performance.CacheTTL)
	}
}
