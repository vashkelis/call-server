package contracts

// ProviderType represents the type of provider.
type ProviderType int32

const (
	ProviderTypeUnspecified ProviderType = 0
	ProviderTypeASR         ProviderType = 1
	ProviderTypeLLM         ProviderType = 2
	ProviderTypeTTS         ProviderType = 3
	ProviderTypeVAD         ProviderType = 4
)

// String returns the string representation of the provider type.
func (t ProviderType) String() string {
	switch t {
	case ProviderTypeASR:
		return "ASR"
	case ProviderTypeLLM:
		return "LLM"
	case ProviderTypeTTS:
		return "TTS"
	case ProviderTypeVAD:
		return "VAD"
	default:
		return "UNSPECIFIED"
	}
}

// ProviderStatus represents the operational status of a provider.
type ProviderStatus int32

const (
	ProviderStatusUnspecified ProviderStatus = 0
	ProviderStatusAvailable   ProviderStatus = 1
	ProviderStatusUnavailable ProviderStatus = 2
	ProviderStatusDegraded    ProviderStatus = 3
)

// String returns the string representation of the provider status.
func (s ProviderStatus) String() string {
	switch s {
	case ProviderStatusAvailable:
		return "AVAILABLE"
	case ProviderStatusUnavailable:
		return "UNAVAILABLE"
	case ProviderStatusDegraded:
		return "DEGRADED"
	default:
		return "UNSPECIFIED"
	}
}

// ProviderInfo contains detailed information about a provider.
type ProviderInfo struct {
	Name         string             `json:"name"`
	ProviderType ProviderType       `json:"provider_type"`
	Version      string             `json:"version"`
	Capabilities ProviderCapability `json:"capabilities"`
	Status       ProviderStatus     `json:"status"`
	Metadata     map[string]string  `json:"metadata,omitempty"`
}

// ListProvidersRequest is used to list available providers.
type ListProvidersRequest struct {
	ProviderType ProviderType `json:"provider_type"`
}

// ListProvidersResponse contains a list of providers.
type ListProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

// GetProviderInfoRequest is used to get info about a specific provider.
type GetProviderInfoRequest struct {
	ProviderName string       `json:"provider_name"`
	ProviderType ProviderType `json:"provider_type"`
}
