package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server      ServerConfig
	Profile     APIConfig
	Transaction APIConfig
	Cache       CacheConfig
	Resilience  ResilienceConfig
	Log         LogConfig
	Tracing     TracingConfig
}

type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type APIConfig struct {
	URL     string
	Timeout time.Duration
}

type CacheConfig struct {
	TTL             time.Duration
	CleanupInterval time.Duration
}

type ResilienceConfig struct {
	MaxRetries         int
	RetryBaseDelay     time.Duration
	RetryMaxDelay      time.Duration
	CBFailureThreshold uint32
	CBTimeout          time.Duration
	BulkheadMaxConc    int
}

type LogConfig struct {
	Level string
}

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	ExporterURL string
}

func Load() *Config {
	godotenv.Load()
	return &Config{
		Server: ServerConfig{
			Port:         getEnvInt("PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		},
		Profile: APIConfig{
			URL:     getEnv("PROFILE_API_URL", ""),
			Timeout: getEnvDuration("PROFILE_API_TIMEOUT", 5*time.Second),
		},
		Transaction: APIConfig{
			URL:     getEnv("TRANSACTIONS_API_URL", ""),
			Timeout: getEnvDuration("TRANSACTIONS_API_TIMEOUT", 5*time.Second),
		},
		Cache: CacheConfig{
			TTL:             getEnvDuration("CACHE_TTL", 5*time.Minute),
			CleanupInterval: getEnvDuration("CACHE_CLEANUP_INTERVAL", 10*time.Minute),
		},
		Resilience: ResilienceConfig{
			MaxRetries:         getEnvInt("RETRY_MAX_ATTEMPTS", 3),
			RetryBaseDelay:     getEnvDuration("RETRY_BASE_DELAY", 200*time.Millisecond),
			RetryMaxDelay:      getEnvDuration("RETRY_MAX_DELAY", 10*time.Second),
			CBFailureThreshold: uint32(getEnvInt("CB_FAILURE_THRESHOLD", 5)),
			CBTimeout:          getEnvDuration("CB_TIMEOUT", 10*time.Second),
			BulkheadMaxConc:    getEnvInt("BULKHEAD_MAX_CONCURRENT", 100),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Tracing: TracingConfig{
			Enabled:     getEnvBool("TRACING_ENABLED", true),
			ServiceName: getEnv("OTEL_SERVICE_NAME", "bfa-go"),
			ExporterURL: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
