package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Performance PerformanceConfig `yaml:"performance"`
	Limits      LimitsConfig      `yaml:"limits"`
	Cache       CacheConfig       `yaml:"cache"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
	Healthcheck HealthcheckConfig `yaml:"healthcheck"`
}

type ServerConfig struct {
	Host                 string        `yaml:"host"`
	Port                 int           `yaml:"port"`
	MaxFileSize          string        `yaml:"max_file_size"`
	MaxConcurrentReqs    int           `yaml:"max_concurrent_requests"`
	RequestTimeout       time.Duration `yaml:"request_timeout"`
	ShutdownGracePeriod  time.Duration `yaml:"shutdown_grace_period"`
}

type PerformanceConfig struct {
	WorkerPoolSize   int    `yaml:"worker_pool_size"`
	BufferSize       string `yaml:"buffer_size"`
	StreamThreshold  string `yaml:"stream_threshold"`
}

type LimitsConfig struct {
	AnalyzeFile     ToolLimits `yaml:"analyze_file"`
	BuildNavigation ToolLimits `yaml:"build_navigation"`
	QueryData       ToolLimits `yaml:"query_data"`
}

type ToolLimits struct {
	Rate      string        `yaml:"rate"`
	Timeout   time.Duration `yaml:"timeout"`
	MaxMemory string        `yaml:"max_memory"`
}

type CacheConfig struct {
	MaxMemory       string        `yaml:"max_memory"`
	DefaultTTL      time.Duration `yaml:"default_ttl"`
	HotDataTTL      time.Duration `yaml:"hot_data_ttl"`
	EvictionPolicy  string        `yaml:"eviction_policy"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type MonitoringConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Metrics    []string         `yaml:"metrics"`
	Tracing    TracingConfig    `yaml:"tracing"`
	Logging    LoggingConfig    `yaml:"logging"`
}

type PrometheusConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Port      int    `yaml:"port"`
	Namespace string `yaml:"namespace"`
}

type TracingConfig struct {
	Enabled      bool    `yaml:"enabled"`
	SamplingRate float64 `yaml:"sampling_rate"`
	Exporter     string  `yaml:"exporter"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	Output    string `yaml:"output"`
	ErrorFile string `yaml:"error_file"`
}

type HealthcheckConfig struct {
	Endpoint  string        `yaml:"endpoint"`
	Interval  time.Duration `yaml:"interval"`
	Threshold int           `yaml:"threshold"`
}

func Load() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	return LoadFromPath(configPath)
}

func LoadFromPath(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Return default config if file doesn't exist
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:                "0.0.0.0",
			Port:                3000,
			MaxFileSize:         "500MB",
			MaxConcurrentReqs:   10,
			RequestTimeout:      30 * time.Second,
			ShutdownGracePeriod: 10 * time.Second,
		},
		Performance: PerformanceConfig{
			WorkerPoolSize:  8,
			BufferSize:      "64KB",
			StreamThreshold: "10MB",
		},
		Limits: LimitsConfig{
			AnalyzeFile: ToolLimits{
				Rate:      "10/min",
				Timeout:   30 * time.Second,
				MaxMemory: "2GB",
			},
			BuildNavigation: ToolLimits{
				Rate:      "30/min",
				Timeout:   20 * time.Second,
				MaxMemory: "1GB",
			},
			QueryData: ToolLimits{
				Rate:      "100/min",
				Timeout:   10 * time.Second,
				MaxMemory: "500MB",
			},
		},
		Cache: CacheConfig{
			MaxMemory:       "100MB",
			DefaultTTL:      5 * time.Minute,
			HotDataTTL:      10 * time.Minute,
			EvictionPolicy:  "lru",
			CleanupInterval: 1 * time.Minute,
		},
		Monitoring: MonitoringConfig{
			Prometheus: PrometheusConfig{
				Enabled:   true,
				Port:      9090,
				Namespace: "mcp_xlsm",
			},
			Metrics: []string{
				"request_duration_seconds",
				"token_usage_total",
				"cache_hit_ratio",
				"memory_usage_bytes",
				"index_rebuild_total",
			},
			Tracing: TracingConfig{
				Enabled:      true,
				SamplingRate: 0.1,
				Exporter:     "jaeger",
			},
			Logging: LoggingConfig{
				Level:     "info",
				Format:    "json",
				Output:    "stdout",
				ErrorFile: "/var/log/mcp-xlsm/error.log",
			},
		},
		Healthcheck: HealthcheckConfig{
			Endpoint:  "/health",
			Interval:  10 * time.Second,
			Threshold: 3,
		},
	}
}