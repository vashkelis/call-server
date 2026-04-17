package config

import (
	"time"
)

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			WSPath:         "/ws",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxConnections: 1000,
		},
		Redis: RedisConfig{
			Address:   "localhost:6379",
			Password:  "",
			DB:        0,
			KeyPrefix: "cloudapp:",
		},
		Postgres: PostgresConfig{
			DSN:             "postgres://localhost/cloudapp?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Providers: ProviderConfig{
			Defaults: DefaultProviders{
				ASR: "mock",
				LLM: "mock",
				TTS: "mock",
				VAD: "mock",
			},
			ASR: make(map[string]map[string]string),
			LLM: make(map[string]map[string]string),
			TTS: make(map[string]map[string]string),
			VAD: make(map[string]map[string]string),
		},
		Audio: AudioConfig{
			DefaultInputProfile:  "telephony",
			DefaultOutputProfile: "telephony",
			Profiles: map[string]ProfileConfig{
				"telephony": {
					SampleRate: 16000,
					Channels:   1,
					Encoding:   "pcm16",
				},
				"telephony8k": {
					SampleRate: 8000,
					Channels:   1,
					Encoding:   "pcm16",
				},
				"webrtc": {
					SampleRate: 48000,
					Channels:   1,
					Encoding:   "pcm16",
				},
				"internal": {
					SampleRate: 16000,
					Channels:   1,
					Encoding:   "pcm16",
				},
			},
		},
		Observability: ObservabilityConfig{
			LogLevel:      "info",
			LogFormat:     "json",
			MetricsPort:   9090,
			OTelEndpoint:  "localhost:4317",
			EnableTracing: true,
			EnableMetrics: true,
		},
		Security: SecurityConfig{
			MaxSessionDuration: 1 * time.Hour,
			MaxChunkSize:       64 * 1024,
			AuthEnabled:        false,
			AllowedOrigins:     []string{"*"},
		},
	}
}

// LocalDevConfig returns a configuration optimized for local development.
func LocalDevConfig() *AppConfig {
	cfg := DefaultConfig()
	cfg.Observability.LogLevel = "debug"
	cfg.Observability.LogFormat = "console"
	cfg.Security.AuthEnabled = false
	return cfg
}

// MockModeConfig returns a configuration for running with mock providers.
func MockModeConfig() *AppConfig {
	cfg := DefaultConfig()
	cfg.Providers.Defaults = DefaultProviders{
		ASR: "mock",
		LLM: "mock",
		TTS: "mock",
		VAD: "mock",
	}
	return cfg
}

// ProductionConfig returns a configuration optimized for production.
func ProductionConfig() *AppConfig {
	cfg := DefaultConfig()
	cfg.Server.MaxConnections = 10000
	cfg.Redis.DB = 0
	cfg.Observability.LogLevel = "warn"
	cfg.Observability.EnableTracing = true
	cfg.Observability.EnableMetrics = true
	cfg.Security.AuthEnabled = true
	cfg.Security.MaxSessionDuration = 30 * time.Minute
	return cfg
}
