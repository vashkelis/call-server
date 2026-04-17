package config

import (
	"context"
	"fmt"
	"sync"
)

// TenantOverride contains per-tenant provider and configuration overrides.
type TenantOverride struct {
	TenantID  string           `yaml:"tenant_id"`
	Providers ProviderOverride `yaml:"providers"`
	Audio     AudioOverride    `yaml:"audio,omitempty"`
	Model     ModelOverride    `yaml:"model,omitempty"`
	Security  SecurityOverride `yaml:"security,omitempty"`
}

// ProviderOverride contains provider overrides for a tenant.
type ProviderOverride struct {
	ASR string `yaml:"asr,omitempty"`
	LLM string `yaml:"llm,omitempty"`
	TTS string `yaml:"tts,omitempty"`
	VAD string `yaml:"vad,omitempty"`
}

// AudioOverride contains audio configuration overrides for a tenant.
type AudioOverride struct {
	InputProfile  string `yaml:"input_profile,omitempty"`
	OutputProfile string `yaml:"output_profile,omitempty"`
}

// ModelOverride contains model configuration overrides for a tenant.
type ModelOverride struct {
	ModelName    string  `yaml:"model_name,omitempty"`
	SystemPrompt string  `yaml:"system_prompt,omitempty"`
	Temperature  float32 `yaml:"temperature,omitempty"`
	MaxTokens    int32   `yaml:"max_tokens,omitempty"`
}

// SecurityOverride contains security configuration overrides for a tenant.
type SecurityOverride struct {
	MaxSessionDuration int  `yaml:"max_session_duration_seconds,omitempty"`
	AuthEnabled        bool `yaml:"auth_enabled,omitempty"`
}

// TenantConfigManager manages tenant-specific configuration overrides.
type TenantConfigManager struct {
	mu         sync.RWMutex
	toverrides map[string]*TenantOverride
	loader     TenantConfigLoader
}

// TenantConfigLoader loads tenant configuration from a data source.
type TenantConfigLoader interface {
	LoadTenantOverride(ctx context.Context, tenantID string) (*TenantOverride, error)
	LoadAllOverrides(ctx context.Context) (map[string]*TenantOverride, error)
}

// NewTenantConfigManager creates a new tenant configuration manager.
func NewTenantConfigManager(loader TenantConfigLoader) *TenantConfigManager {
	return &TenantConfigManager{
		toverrides: make(map[string]*TenantOverride),
		loader:     loader,
	}
}

// GetOverride returns the override for a specific tenant.
func (m *TenantConfigManager) GetOverride(tenantID string) (*TenantOverride, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	override, ok := m.toverrides[tenantID]
	return override, ok
}

// SetOverride sets an override for a tenant.
func (m *TenantConfigManager) SetOverride(override *TenantOverride) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toverrides[override.TenantID] = override
}

// RemoveOverride removes an override for a tenant.
func (m *TenantConfigManager) RemoveOverride(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.toverrides, tenantID)
}

// LoadTenant loads the configuration for a specific tenant.
func (m *TenantConfigManager) LoadTenant(ctx context.Context, tenantID string) (*TenantOverride, error) {
	if m.loader == nil {
		return nil, fmt.Errorf("no tenant config loader configured")
	}

	override, err := m.loader.LoadTenantOverride(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load tenant config: %w", err)
	}

	m.SetOverride(override)
	return override, nil
}

// Refresh reloads all tenant configurations.
func (m *TenantConfigManager) Refresh(ctx context.Context) error {
	if m.loader == nil {
		return fmt.Errorf("no tenant config loader configured")
	}

	overrides, err := m.loader.LoadAllOverrides(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tenant configs: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.toverrides = overrides
	return nil
}

// GetAllOverrides returns all tenant overrides.
func (m *TenantConfigManager) GetAllOverrides() map[string]*TenantOverride {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*TenantOverride, len(m.toverrides))
	for k, v := range m.toverrides {
		result[k] = v
	}
	return result
}

// MemoryTenantLoader loads tenant configuration from an in-memory map.
// Useful for testing and simple deployments.
type MemoryTenantLoader struct {
	overrides map[string]*TenantOverride
}

// NewMemoryTenantLoader creates a new in-memory tenant loader.
func NewMemoryTenantLoader() *MemoryTenantLoader {
	return &MemoryTenantLoader{
		overrides: make(map[string]*TenantOverride),
	}
}

// SetOverride sets an override in memory.
func (l *MemoryTenantLoader) SetOverride(override *TenantOverride) {
	l.overrides[override.TenantID] = override
}

// LoadTenantOverride loads a tenant override from memory.
func (l *MemoryTenantLoader) LoadTenantOverride(ctx context.Context, tenantID string) (*TenantOverride, error) {
	override, ok := l.overrides[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}
	return override, nil
}

// LoadAllOverrides loads all tenant overrides from memory.
func (l *MemoryTenantLoader) LoadAllOverrides(ctx context.Context) (map[string]*TenantOverride, error) {
	result := make(map[string]*TenantOverride, len(l.overrides))
	for k, v := range l.overrides {
		result[k] = v
	}
	return result, nil
}

// PostgresTenantLoader loads tenant configuration from PostgreSQL.
// This is a stub implementation - full implementation is TODO.
type PostgresTenantLoader struct {
	dsn string
}

// NewPostgresTenantLoader creates a new PostgreSQL tenant loader.
func NewPostgresTenantLoader(dsn string) *PostgresTenantLoader {
	return &PostgresTenantLoader{dsn: dsn}
}

// LoadTenantOverride loads a tenant override from PostgreSQL.
// TODO: Implement actual database query
func (l *PostgresTenantLoader) LoadTenantOverride(ctx context.Context, tenantID string) (*TenantOverride, error) {
	return nil, fmt.Errorf("PostgresTenantLoader.LoadTenantOverride not implemented")
}

// LoadAllOverrides loads all tenant overrides from PostgreSQL.
// TODO: Implement actual database query
func (l *PostgresTenantLoader) LoadAllOverrides(ctx context.Context) (map[string]*TenantOverride, error) {
	return nil, fmt.Errorf("PostgresTenantLoader.LoadAllOverrides not implemented")
}

// LoadTenantOverrides is a convenience function to load tenant overrides from various sources.
func LoadTenantOverrides(ctx context.Context, source string) (map[string]*TenantOverride, error) {
	switch source {
	case "memory":
		loader := NewMemoryTenantLoader()
		return loader.LoadAllOverrides(ctx)
	default:
		return nil, fmt.Errorf("unsupported tenant override source: %s", source)
	}
}
