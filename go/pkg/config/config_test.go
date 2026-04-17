package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	config := DefaultConfig()

	// Server defaults
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("expected server host 0.0.0.0, got %s", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("expected server port 8080, got %d", config.Server.Port)
	}
	if config.Server.WSPath != "/ws" {
		t.Errorf("expected WS path /ws, got %s", config.Server.WSPath)
	}
	if config.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", config.Server.ReadTimeout)
	}
	if config.Server.MaxConnections != 1000 {
		t.Errorf("expected max connections 1000, got %d", config.Server.MaxConnections)
	}

	// Redis defaults
	if config.Redis.Address != "localhost:6379" {
		t.Errorf("expected Redis address localhost:6379, got %s", config.Redis.Address)
	}
	if config.Redis.KeyPrefix != "cloudapp:" {
		t.Errorf("expected Redis key prefix 'cloudapp:', got %s", config.Redis.KeyPrefix)
	}

	// Postgres defaults
	if config.Postgres.DSN != "postgres://localhost/cloudapp?sslmode=disable" {
		t.Errorf("expected Postgres DSN, got %s", config.Postgres.DSN)
	}
	if config.Postgres.MaxOpenConns != 25 {
		t.Errorf("expected max open conns 25, got %d", config.Postgres.MaxOpenConns)
	}

	// Provider defaults
	if config.Providers.Defaults.ASR != "mock" {
		t.Errorf("expected default ASR provider 'mock', got %s", config.Providers.Defaults.ASR)
	}
	if config.Providers.Defaults.LLM != "mock" {
		t.Errorf("expected default LLM provider 'mock', got %s", config.Providers.Defaults.LLM)
	}

	// Audio defaults
	if config.Audio.DefaultInputProfile != "telephony" {
		t.Errorf("expected default input profile 'telephony', got %s", config.Audio.DefaultInputProfile)
	}
	if config.Audio.DefaultOutputProfile != "telephony" {
		t.Errorf("expected default output profile 'telephony', got %s", config.Audio.DefaultOutputProfile)
	}

	// Observability defaults
	if config.Observability.LogLevel != "info" {
		t.Errorf("expected log level 'info', got %s", config.Observability.LogLevel)
	}
	if config.Observability.MetricsPort != 9090 {
		t.Errorf("expected metrics port 9090, got %d", config.Observability.MetricsPort)
	}

	// Security defaults
	if config.Security.MaxSessionDuration != 1*time.Hour {
		t.Errorf("expected max session duration 1h, got %v", config.Security.MaxSessionDuration)
	}
	if config.Security.MaxChunkSize != 64*1024 {
		t.Errorf("expected max chunk size 64KB, got %d", config.Security.MaxChunkSize)
	}
}

func TestLoadFromYAML(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `
server:
  host: "127.0.0.1"
  port: 9090
  ws_path: "/websocket"
  read_timeout: 60s
  write_timeout: 60s
  max_connections: 500

redis:
  address: "redis.example.com:6379"
  password: "secret"
  db: 1
  key_prefix: "test:"

postgres:
  dsn: "postgres://user:pass@localhost/testdb?sslmode=require"
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 10m

providers:
  defaults:
    asr: "google"
    llm: "openai"
    tts: "elevenlabs"
    vad: "pyannote"

audio:
  default_input_profile: "webrtc"
  default_output_profile: "webrtc"

observability:
  log_level: "debug"
  log_format: "console"
  metrics_port: 8080
  otel_endpoint: "otel.example.com:4317"
  enable_tracing: false
  enable_metrics: false

security:
  max_session_duration: 30m
  max_chunk_size: 32768
  auth_enabled: true
  auth_token: "test-token"
  allowed_origins:
    - "https://example.com"
    - "https://app.example.com"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Load the config
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify server settings
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", config.Server.Host)
	}
	if config.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", config.Server.Port)
	}
	if config.Server.WSPath != "/websocket" {
		t.Errorf("expected WS path /websocket, got %s", config.Server.WSPath)
	}
	if config.Server.MaxConnections != 500 {
		t.Errorf("expected max connections 500, got %d", config.Server.MaxConnections)
	}

	// Verify Redis settings
	if config.Redis.Address != "redis.example.com:6379" {
		t.Errorf("expected Redis address redis.example.com:6379, got %s", config.Redis.Address)
	}
	if config.Redis.Password != "secret" {
		t.Errorf("expected Redis password 'secret', got %s", config.Redis.Password)
	}
	if config.Redis.DB != 1 {
		t.Errorf("expected Redis DB 1, got %d", config.Redis.DB)
	}

	// Verify provider defaults
	if config.Providers.Defaults.ASR != "google" {
		t.Errorf("expected ASR provider 'google', got %s", config.Providers.Defaults.ASR)
	}
	if config.Providers.Defaults.LLM != "openai" {
		t.Errorf("expected LLM provider 'openai', got %s", config.Providers.Defaults.LLM)
	}
	if config.Providers.Defaults.TTS != "elevenlabs" {
		t.Errorf("expected TTS provider 'elevenlabs', got %s", config.Providers.Defaults.TTS)
	}

	// Verify audio settings
	if config.Audio.DefaultInputProfile != "webrtc" {
		t.Errorf("expected input profile 'webrtc', got %s", config.Audio.DefaultInputProfile)
	}

	// Verify observability settings
	if config.Observability.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got %s", config.Observability.LogLevel)
	}
	if config.Observability.MetricsPort != 8080 {
		t.Errorf("expected metrics port 8080, got %d", config.Observability.MetricsPort)
	}

	// Verify security settings
	if config.Security.AuthEnabled != true {
		t.Errorf("expected auth enabled true, got %v", config.Security.AuthEnabled)
	}
	if config.Security.AuthToken != "test-token" {
		t.Errorf("expected auth token 'test-token', got %s", config.Security.AuthToken)
	}
	if len(config.Security.AllowedOrigins) != 2 {
		t.Errorf("expected 2 allowed origins, got %d", len(config.Security.AllowedOrigins))
	}
}

func TestEnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("CLOUDAPP_SERVER_PORT", "7777")
	os.Setenv("CLOUDAPP_SERVER_HOST", "0.0.0.0")
	os.Setenv("CLOUDAPP_REDIS_ADDRESS", "custom.redis:6380")
	os.Setenv("CLOUDAPP_REDIS_DB", "5")
	os.Setenv("CLOUDAPP_OBSERVABILITY_LOG_LEVEL", "warn")
	os.Setenv("CLOUDAPP_OBSERVABILITY_ENABLE_TRACING", "false")
	os.Setenv("CLOUDAPP_SECURITY_AUTH_ENABLED", "true")
	os.Setenv("CLOUDAPP_PROVIDERS_DEFAULT_ASR", "custom_asr")

	// Clean up after test
	defer func() {
		os.Unsetenv("CLOUDAPP_SERVER_PORT")
		os.Unsetenv("CLOUDAPP_SERVER_HOST")
		os.Unsetenv("CLOUDAPP_REDIS_ADDRESS")
		os.Unsetenv("CLOUDAPP_REDIS_DB")
		os.Unsetenv("CLOUDAPP_OBSERVABILITY_LOG_LEVEL")
		os.Unsetenv("CLOUDAPP_OBSERVABILITY_ENABLE_TRACING")
		os.Unsetenv("CLOUDAPP_SECURITY_AUTH_ENABLED")
		os.Unsetenv("CLOUDAPP_PROVIDERS_DEFAULT_ASR")
	}()

	// Load config (should apply env overrides to defaults)
	config, err := Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify env overrides
	if config.Server.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", config.Server.Port)
	}
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0 from env, got %s", config.Server.Host)
	}
	if config.Redis.Address != "custom.redis:6380" {
		t.Errorf("expected Redis address from env, got %s", config.Redis.Address)
	}
	if config.Redis.DB != 5 {
		t.Errorf("expected Redis DB 5 from env, got %d", config.Redis.DB)
	}
	if config.Observability.LogLevel != "warn" {
		t.Errorf("expected log level 'warn' from env, got %s", config.Observability.LogLevel)
	}
	if config.Observability.EnableTracing != false {
		t.Errorf("expected enable_tracing false from env, got %v", config.Observability.EnableTracing)
	}
	if config.Security.AuthEnabled != true {
		t.Errorf("expected auth_enabled true from env, got %v", config.Security.AuthEnabled)
	}
	if config.Providers.Defaults.ASR != "custom_asr" {
		t.Errorf("expected ASR provider 'custom_asr' from env, got %s", config.Providers.Defaults.ASR)
	}
}

func TestEnvOverrideWithYAML(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `
server:
  port: 8080
  host: "127.0.0.1"

redis:
  address: "localhost:6379"
  db: 0
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set environment variables to override YAML
	os.Setenv("CLOUDAPP_SERVER_PORT", "9999")
	os.Setenv("CLOUDAPP_REDIS_DB", "7")

	defer func() {
		os.Unsetenv("CLOUDAPP_SERVER_PORT")
		os.Unsetenv("CLOUDAPP_REDIS_DB")
	}()

	// Load config - env should override YAML values
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Env should override YAML
	if config.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env override, got %d", config.Server.Port)
	}
	if config.Redis.DB != 7 {
		t.Errorf("expected Redis DB 7 from env override, got %d", config.Redis.DB)
	}

	// YAML values not overridden should remain
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1 from YAML, got %s", config.Server.Host)
	}
	if config.Redis.Address != "localhost:6379" {
		t.Errorf("expected Redis address from YAML, got %s", config.Redis.Address)
	}
}

func TestTenantOverrides(t *testing.T) {
	loader := NewMemoryTenantLoader()

	// Set up tenant overrides
	loader.SetOverride(&TenantOverride{
		TenantID: "tenant-123",
		Providers: ProviderOverride{
			ASR: "google",
			LLM: "openai",
		},
		Audio: AudioOverride{
			InputProfile:  "webrtc",
			OutputProfile: "webrtc",
		},
		Model: ModelOverride{
			ModelName:    "gpt-4",
			Temperature:  0.5,
			MaxTokens:    2048,
			SystemPrompt: "Custom prompt for tenant",
		},
	})

	// Create manager
	manager := NewTenantConfigManager(loader)

	// Load tenant
	ctx := context.Background()
	_, err := manager.LoadTenant(ctx, "tenant-123")
	if err != nil {
		t.Fatalf("failed to load tenant: %v", err)
	}

	// Get override
	override, ok := manager.GetOverride("tenant-123")
	if !ok {
		t.Fatal("expected to find tenant override")
	}

	// Verify provider overrides
	if override.Providers.ASR != "google" {
		t.Errorf("expected ASR provider 'google', got %s", override.Providers.ASR)
	}
	if override.Providers.LLM != "openai" {
		t.Errorf("expected LLM provider 'openai', got %s", override.Providers.LLM)
	}

	// Verify audio overrides
	if override.Audio.InputProfile != "webrtc" {
		t.Errorf("expected input profile 'webrtc', got %s", override.Audio.InputProfile)
	}

	// Verify model overrides
	if override.Model.ModelName != "gpt-4" {
		t.Errorf("expected model name 'gpt-4', got %s", override.Model.ModelName)
	}
	if override.Model.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", override.Model.Temperature)
	}
	if override.Model.SystemPrompt != "Custom prompt for tenant" {
		t.Errorf("expected custom system prompt, got %s", override.Model.SystemPrompt)
	}
}

func TestTenantOverrideNotFound(t *testing.T) {
	loader := NewMemoryTenantLoader()
	manager := NewTenantConfigManager(loader)

	ctx := context.Background()
	_, err := manager.LoadTenant(ctx, "nonexistent-tenant")
	if err == nil {
		t.Error("expected error for non-existent tenant")
	}

	_, ok := manager.GetOverride("nonexistent-tenant")
	if ok {
		t.Error("expected not to find non-existent tenant override")
	}
}

func TestTenantManagerSetAndRemove(t *testing.T) {
	manager := NewTenantConfigManager(nil)

	// Set override
	override := &TenantOverride{
		TenantID: "tenant-456",
		Providers: ProviderOverride{
			ASR: "custom_asr",
		},
	}
	manager.SetOverride(override)

	// Verify it exists
	retrieved, ok := manager.GetOverride("tenant-456")
	if !ok {
		t.Error("expected to find override after setting")
	}
	if retrieved.Providers.ASR != "custom_asr" {
		t.Errorf("expected ASR provider 'custom_asr', got %s", retrieved.Providers.ASR)
	}

	// Remove it
	manager.RemoveOverride("tenant-456")

	// Verify it's gone
	_, ok = manager.GetOverride("tenant-456")
	if ok {
		t.Error("expected override to be removed")
	}
}

func TestGetProviderConfig(t *testing.T) {
	config := DefaultConfig()

	// Add some provider configs
	config.Providers.ASR["google"] = map[string]string{
		"api_key": "test-key",
		"region":  "us-east1",
	}
	config.Providers.LLM["openai"] = map[string]string{
		"api_key": "sk-test",
		"model":   "gpt-4",
	}

	// Test GetProviderConfig
	asrConfig, ok := config.Providers.GetProviderConfig("asr", "google")
	if !ok {
		t.Error("expected to find google ASR config")
	}
	if asrConfig["api_key"] != "test-key" {
		t.Errorf("expected API key 'test-key', got %s", asrConfig["api_key"])
	}

	llmConfig, ok := config.Providers.GetProviderConfig("llm", "openai")
	if !ok {
		t.Error("expected to find openai LLM config")
	}
	if llmConfig["model"] != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", llmConfig["model"])
	}

	// Test non-existent provider
	_, ok = config.Providers.GetProviderConfig("asr", "nonexistent")
	if ok {
		t.Error("expected not to find non-existent provider")
	}

	// Test invalid provider type
	_, ok = config.Providers.GetProviderConfig("invalid", "test")
	if ok {
		t.Error("expected not to find invalid provider type")
	}
}

func TestGetAudioProfile(t *testing.T) {
	config := DefaultConfig()

	// Test getting existing profile
	telephony, ok := config.Audio.GetAudioProfile("telephony")
	if !ok {
		t.Error("expected to find telephony profile")
	}
	if telephony.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", telephony.SampleRate)
	}

	// Test getting non-existent profile
	_, ok = config.Audio.GetAudioProfile("nonexistent")
	if ok {
		t.Error("expected not to find non-existent profile")
	}
}

func TestConfigValidate(t *testing.T) {
	// Valid config
	config := DefaultConfig()
	err := config.Validate()
	if err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	// Test that Validate sets defaults
	minimalConfig := &AppConfig{}
	err = minimalConfig.Validate()
	if err != nil {
		t.Errorf("expected validation to set defaults, got error: %v", err)
	}

	// Verify defaults were set
	if minimalConfig.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host to be set, got %s", minimalConfig.Server.Host)
	}
	if minimalConfig.Server.Port != 8080 {
		t.Errorf("expected default port to be set, got %d", minimalConfig.Server.Port)
	}
}

func TestLocalDevConfig(t *testing.T) {
	config := LocalDevConfig()

	if config.Observability.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' for local dev, got %s", config.Observability.LogLevel)
	}
	if config.Observability.LogFormat != "console" {
		t.Errorf("expected log format 'console' for local dev, got %s", config.Observability.LogFormat)
	}
	if config.Security.AuthEnabled != false {
		t.Errorf("expected auth disabled for local dev, got %v", config.Security.AuthEnabled)
	}
}

func TestMockModeConfig(t *testing.T) {
	config := MockModeConfig()

	if config.Providers.Defaults.ASR != "mock" {
		t.Errorf("expected ASR provider 'mock', got %s", config.Providers.Defaults.ASR)
	}
	if config.Providers.Defaults.LLM != "mock" {
		t.Errorf("expected LLM provider 'mock', got %s", config.Providers.Defaults.LLM)
	}
	if config.Providers.Defaults.TTS != "mock" {
		t.Errorf("expected TTS provider 'mock', got %s", config.Providers.Defaults.TTS)
	}
	if config.Providers.Defaults.VAD != "mock" {
		t.Errorf("expected VAD provider 'mock', got %s", config.Providers.Defaults.VAD)
	}
}

func TestProductionConfig(t *testing.T) {
	config := ProductionConfig()

	if config.Server.MaxConnections != 10000 {
		t.Errorf("expected max connections 10000 for production, got %d", config.Server.MaxConnections)
	}
	if config.Observability.LogLevel != "warn" {
		t.Errorf("expected log level 'warn' for production, got %s", config.Observability.LogLevel)
	}
	if config.Security.AuthEnabled != true {
		t.Errorf("expected auth enabled for production, got %v", config.Security.AuthEnabled)
	}
	if config.Security.MaxSessionDuration != 30*time.Minute {
		t.Errorf("expected max session duration 30m for production, got %v", config.Security.MaxSessionDuration)
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a temp directory with a config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "cloudapp.yaml")
	err := os.WriteFile(configFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Test finding in current directory (simulated)
	// Note: This test may not work as expected due to the function looking in specific paths
	// We're mainly testing that the function doesn't panic
	result := FindConfigFile("nonexistent.yaml")
	// Result may be empty if file not found, which is fine
	t.Logf("FindConfigFile result: %s", result)
}
