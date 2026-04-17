package providers

import (
	"context"
	"testing"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/session"
)

// MockASRProvider is a mock implementation of ASRProvider for testing
type MockASRProvider struct {
	name string
}

func (m *MockASRProvider) StreamRecognize(ctx context.Context, audioStream chan []byte, opts ASROptions) (chan ASRResult, error) {
	return make(chan ASRResult), nil
}

func (m *MockASRProvider) Cancel(ctx context.Context, sessionID string) error {
	return nil
}

func (m *MockASRProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{}
}

func (m *MockASRProvider) Name() string {
	return m.name
}

// MockLLMProvider is a mock implementation of LLMProvider for testing
type MockLLMProvider struct {
	name string
}

func (m *MockLLMProvider) StreamGenerate(ctx context.Context, messages []contracts.ChatMessage, opts LLMOptions) (chan LLMToken, error) {
	return make(chan LLMToken), nil
}

func (m *MockLLMProvider) Cancel(ctx context.Context, sessionID string) error {
	return nil
}

func (m *MockLLMProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{}
}

func (m *MockLLMProvider) Name() string {
	return m.name
}

// MockTTSProvider is a mock implementation of TTSProvider for testing
type MockTTSProvider struct {
	name string
}

func (m *MockTTSProvider) StreamSynthesize(ctx context.Context, text string, opts TTSOptions) (chan []byte, error) {
	return make(chan []byte), nil
}

func (m *MockTTSProvider) Cancel(ctx context.Context, sessionID string) error {
	return nil
}

func (m *MockTTSProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{}
}

func (m *MockTTSProvider) Name() string {
	return m.name
}

// MockVADProvider is a mock implementation of VADProvider for testing
type MockVADProvider struct {
	name string
}

func (m *MockVADProvider) ProcessAudio(ctx context.Context, audio []byte) (VADResult, error) {
	return VADResult{}, nil
}

func (m *MockVADProvider) Reset(sessionID string) {}

func (m *MockVADProvider) Name() string {
	return m.name
}

func TestRegisterAndGetProvider(t *testing.T) {
	registry := NewProviderRegistry()

	// Register mock providers
	asrProvider := &MockASRProvider{name: "mock_asr"}
	llmProvider := &MockLLMProvider{name: "mock_llm"}
	ttsProvider := &MockTTSProvider{name: "mock_tts"}
	vadProvider := &MockVADProvider{name: "mock_vad"}

	registry.RegisterASR("mock_asr", asrProvider)
	registry.RegisterLLM("mock_llm", llmProvider)
	registry.RegisterTTS("mock_tts", ttsProvider)
	registry.RegisterVAD("mock_vad", vadProvider)

	// Retrieve and verify ASR provider
	retrievedASR, err := registry.GetASR("mock_asr")
	if err != nil {
		t.Errorf("unexpected error getting ASR provider: %v", err)
	}
	if retrievedASR.Name() != "mock_asr" {
		t.Errorf("expected ASR provider name 'mock_asr', got %s", retrievedASR.Name())
	}

	// Retrieve and verify LLM provider
	retrievedLLM, err := registry.GetLLM("mock_llm")
	if err != nil {
		t.Errorf("unexpected error getting LLM provider: %v", err)
	}
	if retrievedLLM.Name() != "mock_llm" {
		t.Errorf("expected LLM provider name 'mock_llm', got %s", retrievedLLM.Name())
	}

	// Retrieve and verify TTS provider
	retrievedTTS, err := registry.GetTTS("mock_tts")
	if err != nil {
		t.Errorf("unexpected error getting TTS provider: %v", err)
	}
	if retrievedTTS.Name() != "mock_tts" {
		t.Errorf("expected TTS provider name 'mock_tts', got %s", retrievedTTS.Name())
	}

	// Retrieve and verify VAD provider
	retrievedVAD, err := registry.GetVAD("mock_vad")
	if err != nil {
		t.Errorf("unexpected error getting VAD provider: %v", err)
	}
	if retrievedVAD.Name() != "mock_vad" {
		t.Errorf("expected VAD provider name 'mock_vad', got %s", retrievedVAD.Name())
	}
}

func TestProviderNotFound(t *testing.T) {
	registry := NewProviderRegistry()

	// Try to get non-existent providers
	_, err := registry.GetASR("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent ASR provider")
	}

	_, err = registry.GetLLM("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent LLM provider")
	}

	_, err = registry.GetTTS("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent TTS provider")
	}

	_, err = registry.GetVAD("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent VAD provider")
	}
}

func TestListProviders(t *testing.T) {
	registry := NewProviderRegistry()

	// Register multiple providers
	registry.RegisterASR("asr1", &MockASRProvider{name: "asr1"})
	registry.RegisterASR("asr2", &MockASRProvider{name: "asr2"})
	registry.RegisterLLM("llm1", &MockLLMProvider{name: "llm1"})
	registry.RegisterLLM("llm2", &MockLLMProvider{name: "llm2"})
	registry.RegisterTTS("tts1", &MockTTSProvider{name: "tts1"})
	registry.RegisterVAD("vad1", &MockVADProvider{name: "vad1"})

	// List ASR providers
	asrList := registry.ListASR()
	if len(asrList) != 2 {
		t.Errorf("expected 2 ASR providers, got %d", len(asrList))
	}

	// List LLM providers
	llmList := registry.ListLLM()
	if len(llmList) != 2 {
		t.Errorf("expected 2 LLM providers, got %d", len(llmList))
	}

	// List TTS providers
	ttsList := registry.ListTTS()
	if len(ttsList) != 1 {
		t.Errorf("expected 1 TTS provider, got %d", len(ttsList))
	}

	// List VAD providers
	vadList := registry.ListVAD()
	if len(vadList) != 1 {
		t.Errorf("expected 1 VAD provider, got %d", len(vadList))
	}
}

func TestProviderForSession(t *testing.T) {
	registry := NewProviderRegistry()

	// Register providers
	registry.RegisterASR("global_asr", &MockASRProvider{name: "global_asr"})
	registry.RegisterASR("tenant_asr", &MockASRProvider{name: "tenant_asr"})
	registry.RegisterASR("session_asr", &MockASRProvider{name: "session_asr"})
	registry.RegisterLLM("global_llm", &MockLLMProvider{name: "global_llm"})
	registry.RegisterTTS("global_tts", &MockTTSProvider{name: "global_tts"})

	// Set global defaults
	registry.SetConfig(
		session.SelectedProviders{
			ASR: "global_asr",
			LLM: "global_llm",
			TTS: "global_tts",
		},
		map[string]session.SelectedProviders{
			"tenant-123": {
				ASR: "tenant_asr",
			},
		},
	)

	tests := []struct {
		name             string
		session          *session.Session
		requestProviders *session.SelectedProviders
		expectedASR      string
	}{
		{
			name: "global default",
			session: func() *session.Session {
				s := session.NewSession("sess-1", "trace-1", session.TransportTypeWebSocket)
				return s
			}(),
			requestProviders: nil,
			expectedASR:      "global_asr",
		},
		{
			name: "tenant override",
			session: func() *session.Session {
				s := session.NewSession("sess-2", "trace-2", session.TransportTypeWebSocket)
				s.SetTenantID("tenant-123")
				return s
			}(),
			requestProviders: nil,
			expectedASR:      "tenant_asr",
		},
		{
			name: "session override",
			session: func() *session.Session {
				s := session.NewSession("sess-3", "trace-3", session.TransportTypeWebSocket)
				s.SetTenantID("tenant-123")
				s.SetProviders(session.SelectedProviders{ASR: "session_asr"})
				return s
			}(),
			requestProviders: nil,
			expectedASR:      "session_asr",
		},
		{
			name: "request override (highest priority)",
			session: func() *session.Session {
				s := session.NewSession("sess-4", "trace-4", session.TransportTypeWebSocket)
				s.SetTenantID("tenant-123")
				s.SetProviders(session.SelectedProviders{ASR: "session_asr"})
				return s
			}(),
			requestProviders: &session.SelectedProviders{ASR: "request_asr"},
			expectedASR:      "request_asr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register request_asr for the test
			if tt.requestProviders != nil && tt.requestProviders.ASR == "request_asr" {
				registry.RegisterASR("request_asr", &MockASRProvider{name: "request_asr"})
			}

			result, err := registry.ResolveForSession(tt.session, tt.requestProviders)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result.ASR != tt.expectedASR {
				t.Errorf("expected ASR %s, got %s", tt.expectedASR, result.ASR)
			}
		})
	}
}

func TestResolveForSessionProviderNotFound(t *testing.T) {
	registry := NewProviderRegistry()

	// Set config with non-existent provider
	registry.SetConfig(
		session.SelectedProviders{
			ASR: "nonexistent_asr",
		},
		nil,
	)

	sess := session.NewSession("sess-1", "trace-1", session.TransportTypeWebSocket)
	_, err := registry.ResolveForSession(sess, nil)

	if err == nil {
		t.Error("expected error when resolving non-existent provider")
	}
}

func TestProviderRegistrationOverwrite(t *testing.T) {
	registry := NewProviderRegistry()

	// Register initial provider
	provider1 := &MockASRProvider{name: "provider1"}
	registry.RegisterASR("test", provider1)

	// Register another provider with same name (should overwrite)
	provider2 := &MockASRProvider{name: "provider2"}
	registry.RegisterASR("test", provider2)

	// Retrieve and verify it's the second provider
	retrieved, err := registry.GetASR("test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrieved.Name() != "provider2" {
		t.Errorf("expected provider2, got %s", retrieved.Name())
	}
}

func TestEmptyRegistry(t *testing.T) {
	registry := NewProviderRegistry()

	// List should return empty slices, not nil
	asrList := registry.ListASR()
	if asrList == nil || len(asrList) != 0 {
		t.Errorf("expected empty ASR list, got %v", asrList)
	}

	llmList := registry.ListLLM()
	if llmList == nil || len(llmList) != 0 {
		t.Errorf("expected empty LLM list, got %v", llmList)
	}

	ttsList := registry.ListTTS()
	if ttsList == nil || len(ttsList) != 0 {
		t.Errorf("expected empty TTS list, got %v", ttsList)
	}

	vadList := registry.ListVAD()
	if vadList == nil || len(vadList) != 0 {
		t.Errorf("expected empty VAD list, got %v", vadList)
	}
}

func TestResolveForSessionWithPartialOverrides(t *testing.T) {
	registry := NewProviderRegistry()

	// Register all providers
	registry.RegisterASR("global_asr", &MockASRProvider{name: "global_asr"})
	registry.RegisterLLM("global_llm", &MockLLMProvider{name: "global_llm"})
	registry.RegisterTTS("global_tts", &MockTTSProvider{name: "global_tts"})
	registry.RegisterVAD("global_vad", &MockVADProvider{name: "global_vad"})

	// Only override ASR at tenant level
	registry.SetConfig(
		session.SelectedProviders{
			ASR: "global_asr",
			LLM: "global_llm",
			TTS: "global_tts",
			VAD: "global_vad",
		},
		map[string]session.SelectedProviders{
			"tenant-123": {
				ASR: "tenant_asr",
			},
		},
	)

	// Register tenant ASR
	registry.RegisterASR("tenant_asr", &MockASRProvider{name: "tenant_asr"})

	sess := session.NewSession("sess-1", "trace-1", session.TransportTypeWebSocket)
	sess.SetTenantID("tenant-123")

	result, err := registry.ResolveForSession(sess, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// ASR should be overridden by tenant
	if result.ASR != "tenant_asr" {
		t.Errorf("expected ASR tenant_asr, got %s", result.ASR)
	}

	// LLM, TTS, VAD should still be global
	if result.LLM != "global_llm" {
		t.Errorf("expected LLM global_llm, got %s", result.LLM)
	}
	if result.TTS != "global_tts" {
		t.Errorf("expected TTS global_tts, got %s", result.TTS)
	}
	if result.VAD != "global_vad" {
		t.Errorf("expected VAD global_vad, got %s", result.VAD)
	}
}
