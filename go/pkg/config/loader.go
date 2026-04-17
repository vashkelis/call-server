package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a YAML file and overlays environment variables.
func Load(path string) (*AppConfig, error) {
	// Start with defaults
	config := DefaultConfig()

	// If path is provided, load from file
	if path != "" {
		if err := loadFromFile(path, config); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Overlay environment variables
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from env: %w", err)
	}

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// loadFromFile loads configuration from a YAML file.
func loadFromFile(path string, config *AppConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// loadFromEnv overlays environment variables onto the configuration.
// Environment variables should be prefixed with CLOUDAPP_ and use underscores
// to separate nested keys. For example:
//
//	CLOUDAPP_SERVER_PORT=9090
//	CLOUDAPP_REDIS_ADDRESS=redis:6379
func loadFromEnv(config *AppConfig) error {
	prefix := "CLOUDAPP_"

	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key, value := pair[0], pair[1]
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		// Remove prefix and convert to lowercase
		key = strings.ToLower(strings.TrimPrefix(key, prefix))

		if err := setConfigValue(config, key, value); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}
	}

	return nil
}

// setConfigValue sets a configuration value based on the key path.
func setConfigValue(config *AppConfig, key, value string) error {
	parts := strings.Split(key, "_")
	if len(parts) < 2 {
		return fmt.Errorf("invalid key format: %s", key)
	}

	section := parts[0]
	field := strings.Join(parts[1:], "_")

	switch section {
	case "server":
		return setServerValue(&config.Server, field, value)
	case "redis":
		return setRedisValue(&config.Redis, field, value)
	case "postgres":
		return setPostgresValue(&config.Postgres, field, value)
	case "observability":
		return setObservabilityValue(&config.Observability, field, value)
	case "security":
		return setSecurityValue(&config.Security, field, value)
	case "providers":
		return setProvidersValue(&config.Providers, field, value)
	default:
		return fmt.Errorf("unknown config section: %s", section)
	}
}

func setServerValue(cfg *ServerConfig, field, value string) error {
	switch field {
	case "host":
		cfg.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid port: %w", err)
		}
		cfg.Port = port
	case "ws_path":
		cfg.WSPath = value
	case "max_connections":
		max, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_connections: %w", err)
		}
		cfg.MaxConnections = max
	default:
		return fmt.Errorf("unknown server field: %s", field)
	}
	return nil
}

func setRedisValue(cfg *RedisConfig, field, value string) error {
	switch field {
	case "address":
		cfg.Address = value
	case "password":
		cfg.Password = value
	case "db":
		db, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid db: %w", err)
		}
		cfg.DB = db
	case "key_prefix":
		cfg.KeyPrefix = value
	default:
		return fmt.Errorf("unknown redis field: %s", field)
	}
	return nil
}

func setPostgresValue(cfg *PostgresConfig, field, value string) error {
	switch field {
	case "dsn":
		cfg.DSN = value
	case "max_open_conns":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_open_conns: %w", err)
		}
		cfg.MaxOpenConns = n
	case "max_idle_conns":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_idle_conns: %w", err)
		}
		cfg.MaxIdleConns = n
	default:
		return fmt.Errorf("unknown postgres field: %s", field)
	}
	return nil
}

func setObservabilityValue(cfg *ObservabilityConfig, field, value string) error {
	switch field {
	case "log_level":
		cfg.LogLevel = value
	case "log_format":
		cfg.LogFormat = value
	case "metrics_port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid metrics_port: %w", err)
		}
		cfg.MetricsPort = port
	case "otel_endpoint":
		cfg.OTelEndpoint = value
	case "enable_tracing":
		cfg.EnableTracing = parseBool(value)
	case "enable_metrics":
		cfg.EnableMetrics = parseBool(value)
	default:
		return fmt.Errorf("unknown observability field: %s", field)
	}
	return nil
}

func setSecurityValue(cfg *SecurityConfig, field, value string) error {
	switch field {
	case "max_session_duration":
		d, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid max_session_duration: %w", err)
		}
		cfg.MaxSessionDuration = time.Duration(d) * time.Second
	case "max_chunk_size":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_chunk_size: %w", err)
		}
		cfg.MaxChunkSize = n
	case "auth_enabled":
		cfg.AuthEnabled = parseBool(value)
	case "auth_token":
		cfg.AuthToken = value
	default:
		return fmt.Errorf("unknown security field: %s", field)
	}
	return nil
}

func setProvidersValue(cfg *ProviderConfig, field, value string) error {
	// Handle default provider settings
	switch field {
	case "default_asr":
		cfg.Defaults.ASR = value
	case "default_llm":
		cfg.Defaults.LLM = value
	case "default_tts":
		cfg.Defaults.TTS = value
	case "default_vad":
		cfg.Defaults.VAD = value
	default:
		return fmt.Errorf("unknown providers field: %s", field)
	}
	return nil
}

func parseBool(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

func parseDuration(s string) (int64, error) {
	// Simple duration parsing - assumes seconds if no unit
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	// TODO: Add support for duration strings like "1h30m"
	return 0, fmt.Errorf("unsupported duration format: %s", s)
}

// FindConfigFile searches for a configuration file in common locations.
func FindConfigFile(name string) string {
	locations := []string{
		name,
		filepath.Join("config", name),
		filepath.Join("/etc", "cloudapp", name),
		filepath.Join(os.Getenv("HOME"), ".cloudapp", name),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}
