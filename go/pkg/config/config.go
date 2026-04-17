// Package config provides configuration loading and management.
package config

import (
	"fmt"
	"time"
)

// AppConfig is the root configuration structure.
type AppConfig struct {
	Server        ServerConfig        `yaml:"server"`
	Redis         RedisConfig         `yaml:"redis"`
	Postgres      PostgresConfig      `yaml:"postgres"`
	Providers     ProviderConfig      `yaml:"providers"`
	Audio         AudioConfig         `yaml:"audio"`
	Observability ObservabilityConfig `yaml:"observability"`
	Security      SecurityConfig      `yaml:"security"`
}

// ServerConfig contains server-related configuration.
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	WSPath         string        `yaml:"ws_path"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	MaxConnections int           `yaml:"max_connections"`
}

// RedisConfig contains Redis connection configuration.
type RedisConfig struct {
	Address   string `yaml:"address"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}

// PostgresConfig contains PostgreSQL connection configuration.
type PostgresConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// ProviderConfig contains provider-related configuration.
type ProviderConfig struct {
	Defaults DefaultProviders             `yaml:"defaults"`
	ASR      map[string]map[string]string `yaml:"asr"`
	LLM      map[string]map[string]string `yaml:"llm"`
	TTS      map[string]map[string]string `yaml:"tts"`
	VAD      map[string]map[string]string `yaml:"vad"`
}

// DefaultProviders contains default provider names.
type DefaultProviders struct {
	ASR string `yaml:"asr"`
	LLM string `yaml:"llm"`
	TTS string `yaml:"tts"`
	VAD string `yaml:"vad"`
}

// AudioConfig contains audio-related configuration.
type AudioConfig struct {
	DefaultInputProfile  string                   `yaml:"default_input_profile"`
	DefaultOutputProfile string                   `yaml:"default_output_profile"`
	Profiles             map[string]ProfileConfig `yaml:"profiles"`
}

// ProfileConfig contains audio profile configuration.
type ProfileConfig struct {
	SampleRate int    `yaml:"sample_rate"`
	Channels   int    `yaml:"channels"`
	Encoding   string `yaml:"encoding"`
}

// ObservabilityConfig contains observability-related configuration.
type ObservabilityConfig struct {
	LogLevel      string `yaml:"log_level"`
	LogFormat     string `yaml:"log_format"`
	MetricsPort   int    `yaml:"metrics_port"`
	OTelEndpoint  string `yaml:"otel_endpoint"`
	EnableTracing bool   `yaml:"enable_tracing"`
	EnableMetrics bool   `yaml:"enable_metrics"`
}

// SecurityConfig contains security-related configuration.
type SecurityConfig struct {
	MaxSessionDuration time.Duration `yaml:"max_session_duration"`
	MaxChunkSize       int           `yaml:"max_chunk_size"`
	AuthEnabled        bool          `yaml:"auth_enabled"`
	AuthToken          string        `yaml:"auth_token"`
	AllowedOrigins     []string      `yaml:"allowed_origins"`
}

// Validate validates the configuration.
func (c *AppConfig) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	if err := c.Redis.Validate(); err != nil {
		return fmt.Errorf("redis config: %w", err)
	}
	if err := c.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres config: %w", err)
	}
	if err := c.Providers.Validate(); err != nil {
		return fmt.Errorf("providers config: %w", err)
	}
	if err := c.Audio.Validate(); err != nil {
		return fmt.Errorf("audio config: %w", err)
	}
	if err := c.Observability.Validate(); err != nil {
		return fmt.Errorf("observability config: %w", err)
	}
	if err := c.Security.Validate(); err != nil {
		return fmt.Errorf("security config: %w", err)
	}
	return nil
}

// Validate validates server configuration.
func (c *ServerConfig) Validate() error {
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port <= 0 {
		c.Port = 8080
	}
	if c.WSPath == "" {
		c.WSPath = "/ws"
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.MaxConnections <= 0 {
		c.MaxConnections = 1000
	}
	return nil
}

// Validate validates Redis configuration.
func (c *RedisConfig) Validate() error {
	if c.Address == "" {
		c.Address = "localhost:6379"
	}
	if c.KeyPrefix == "" {
		c.KeyPrefix = "cloudapp:"
	}
	return nil
}

// Validate validates PostgreSQL configuration.
func (c *PostgresConfig) Validate() error {
	if c.DSN == "" {
		c.DSN = "postgres://localhost/cloudapp?sslmode=disable"
	}
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = 25
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = 5 * time.Minute
	}
	return nil
}

// Validate validates provider configuration.
func (c *ProviderConfig) Validate() error {
	// Providers are optional for testing
	if c.ASR == nil {
		c.ASR = make(map[string]map[string]string)
	}
	if c.LLM == nil {
		c.LLM = make(map[string]map[string]string)
	}
	if c.TTS == nil {
		c.TTS = make(map[string]map[string]string)
	}
	if c.VAD == nil {
		c.VAD = make(map[string]map[string]string)
	}
	return nil
}

// Validate validates audio configuration.
func (c *AudioConfig) Validate() error {
	if c.DefaultInputProfile == "" {
		c.DefaultInputProfile = "telephony"
	}
	if c.DefaultOutputProfile == "" {
		c.DefaultOutputProfile = "telephony"
	}
	if c.Profiles == nil {
		c.Profiles = make(map[string]ProfileConfig)
	}
	// Ensure default profiles exist
	if _, ok := c.Profiles["telephony"]; !ok {
		c.Profiles["telephony"] = ProfileConfig{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   "pcm16",
		}
	}
	if _, ok := c.Profiles["webrtc"]; !ok {
		c.Profiles["webrtc"] = ProfileConfig{
			SampleRate: 48000,
			Channels:   1,
			Encoding:   "pcm16",
		}
	}
	return nil
}

// Validate validates observability configuration.
func (c *ObservabilityConfig) Validate() error {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "json"
	}
	if c.MetricsPort <= 0 {
		c.MetricsPort = 9090
	}
	if c.OTelEndpoint == "" {
		c.OTelEndpoint = "localhost:4317"
	}
	return nil
}

// Validate validates security configuration.
func (c *SecurityConfig) Validate() error {
	if c.MaxSessionDuration <= 0 {
		c.MaxSessionDuration = 1 * time.Hour
	}
	if c.MaxChunkSize <= 0 {
		c.MaxChunkSize = 64 * 1024 // 64KB
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	return nil
}

// GetProviderConfig returns the configuration for a specific provider.
func (c *ProviderConfig) GetProviderConfig(providerType, name string) (map[string]string, bool) {
	var providers map[string]map[string]string
	switch providerType {
	case "asr":
		providers = c.ASR
	case "llm":
		providers = c.LLM
	case "tts":
		providers = c.TTS
	case "vad":
		providers = c.VAD
	default:
		return nil, false
	}

	config, ok := providers[name]
	return config, ok
}

// GetAudioProfile returns the audio profile configuration by name.
func (c *AudioConfig) GetAudioProfile(name string) (ProfileConfig, bool) {
	profile, ok := c.Profiles[name]
	return profile, ok
}
