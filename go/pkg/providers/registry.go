package providers

import (
	"fmt"
	"sync"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/session"
)

// ProviderFactory is a function that creates a provider instance.
type ProviderFactory func(config map[string]string) (interface{}, error)

// ProviderRegistry manages provider registration and lookup.
type ProviderRegistry struct {
	mu        sync.RWMutex
	asr       map[string]ASRProvider
	llm       map[string]LLMProvider
	tts       map[string]TTSProvider
	vad       map[string]VADProvider
	factories map[contracts.ProviderType]map[string]ProviderFactory
	config    *registryConfig
}

// registryConfig holds provider resolution configuration.
type registryConfig struct {
	GlobalDefaults  session.SelectedProviders
	TenantOverrides map[string]session.SelectedProviders
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		asr:       make(map[string]ASRProvider),
		llm:       make(map[string]LLMProvider),
		tts:       make(map[string]TTSProvider),
		vad:       make(map[string]VADProvider),
		factories: make(map[contracts.ProviderType]map[string]ProviderFactory),
	}
}

// RegisterASR registers an ASR provider.
func (r *ProviderRegistry) RegisterASR(name string, provider ASRProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asr[name] = provider
}

// RegisterLLM registers an LLM provider.
func (r *ProviderRegistry) RegisterLLM(name string, provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llm[name] = provider
}

// RegisterTTS registers a TTS provider.
func (r *ProviderRegistry) RegisterTTS(name string, provider TTSProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tts[name] = provider
}

// RegisterVAD registers a VAD provider.
func (r *ProviderRegistry) RegisterVAD(name string, provider VADProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vad[name] = provider
}

// GetASR returns an ASR provider by name.
func (r *ProviderRegistry) GetASR(name string) (ASRProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.asr[name]
	if !ok {
		return nil, fmt.Errorf("ASR provider not found: %s", name)
	}
	return provider, nil
}

// GetLLM returns an LLM provider by name.
func (r *ProviderRegistry) GetLLM(name string) (LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.llm[name]
	if !ok {
		return nil, fmt.Errorf("LLM provider not found: %s", name)
	}
	return provider, nil
}

// GetTTS returns a TTS provider by name.
func (r *ProviderRegistry) GetTTS(name string) (TTSProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.tts[name]
	if !ok {
		return nil, fmt.Errorf("TTS provider not found: %s", name)
	}
	return provider, nil
}

// GetVAD returns a VAD provider by name.
func (r *ProviderRegistry) GetVAD(name string) (VADProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.vad[name]
	if !ok {
		return nil, fmt.Errorf("VAD provider not found: %s", name)
	}
	return provider, nil
}

// ListASR returns all registered ASR provider names.
func (r *ProviderRegistry) ListASR() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.asr))
	for name := range r.asr {
		names = append(names, name)
	}
	return names
}

// ListLLM returns all registered LLM provider names.
func (r *ProviderRegistry) ListLLM() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.llm))
	for name := range r.llm {
		names = append(names, name)
	}
	return names
}

// ListTTS returns all registered TTS provider names.
func (r *ProviderRegistry) ListTTS() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tts))
	for name := range r.tts {
		names = append(names, name)
	}
	return names
}

// ListVAD returns all registered VAD provider names.
func (r *ProviderRegistry) ListVAD() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.vad))
	for name := range r.vad {
		names = append(names, name)
	}
	return names
}

// ProviderResolutionConfig defines the priority order for provider resolution.
type ProviderResolutionConfig struct {
	GlobalDefaults  session.SelectedProviders
	TenantOverrides map[string]session.SelectedProviders
}

// ResolveForSession resolves the providers to use for a session.
// Priority: request -> session -> tenant -> global
func (r *ProviderRegistry) ResolveForSession(
	sess *session.Session,
	requestProviders *session.SelectedProviders,
) (session.SelectedProviders, error) {
	result := session.SelectedProviders{}

	// Start with global defaults
	if r.config != nil {
		result = r.config.GlobalDefaults
	}

	// Apply tenant overrides if available
	if sess.TenantID != nil && r.config != nil {
		if tenantProviders, ok := r.config.TenantOverrides[*sess.TenantID]; ok {
			if tenantProviders.ASR != "" {
				result.ASR = tenantProviders.ASR
			}
			if tenantProviders.LLM != "" {
				result.LLM = tenantProviders.LLM
			}
			if tenantProviders.TTS != "" {
				result.TTS = tenantProviders.TTS
			}
			if tenantProviders.VAD != "" {
				result.VAD = tenantProviders.VAD
			}
		}
	}

	// Apply session overrides
	if sess.SelectedProviders.ASR != "" {
		result.ASR = sess.SelectedProviders.ASR
	}
	if sess.SelectedProviders.LLM != "" {
		result.LLM = sess.SelectedProviders.LLM
	}
	if sess.SelectedProviders.TTS != "" {
		result.TTS = sess.SelectedProviders.TTS
	}
	if sess.SelectedProviders.VAD != "" {
		result.VAD = sess.SelectedProviders.VAD
	}

	// Apply request overrides (highest priority)
	if requestProviders != nil {
		if requestProviders.ASR != "" {
			result.ASR = requestProviders.ASR
		}
		if requestProviders.LLM != "" {
			result.LLM = requestProviders.LLM
		}
		if requestProviders.TTS != "" {
			result.TTS = requestProviders.TTS
		}
		if requestProviders.VAD != "" {
			result.VAD = requestProviders.VAD
		}
	}

	// Validate that providers exist
	if result.ASR != "" {
		if _, err := r.GetASR(result.ASR); err != nil {
			return result, fmt.Errorf("ASR provider not available: %w", err)
		}
	}
	if result.LLM != "" {
		if _, err := r.GetLLM(result.LLM); err != nil {
			return result, fmt.Errorf("LLM provider not available: %w", err)
		}
	}
	if result.TTS != "" {
		if _, err := r.GetTTS(result.TTS); err != nil {
			return result, fmt.Errorf("TTS provider not available: %w", err)
		}
	}

	return result, nil
}

// SetConfig sets the provider resolution configuration.
func (r *ProviderRegistry) SetConfig(global session.SelectedProviders, tenantOverrides map[string]session.SelectedProviders) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = &registryConfig{
		GlobalDefaults:  global,
		TenantOverrides: tenantOverrides,
	}
}
