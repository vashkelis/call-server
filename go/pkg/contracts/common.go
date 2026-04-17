package contracts
// Package contracts provides internal Go type definitions that mirror the protobuf messages.
// These types are used until proper proto generation is set up.
package contracts

import (
	"fmt"
	"time"
)

// AudioEncoding represents the audio encoding format.
type AudioEncoding int32

const (
	AudioEncodingUnspecified AudioEncoding = 0
	PCM16                    AudioEncoding = 1
	OPUS                     AudioEncoding = 2
	G711ULAW                 AudioEncoding = 3
	G711ALAW                 AudioEncoding = 4
)

// String returns the string representation of the encoding.
func (e AudioEncoding) String() string {
	switch e {
	case PCM16:
		return "PCM16"
	case OPUS:
		return "OPUS"
	case G711ULAW:
		return "G711_ULAW"
	case G711ALAW:
		return "G711_ALAW"
	default:
		return "UNSPECIFIED"
	}
}

// ProviderErrorCode represents error codes for provider errors.
type ProviderErrorCode int32

const (
	ProviderErrorCodeUnspecified       ProviderErrorCode = 0
	ProviderErrorCodeInternal          ProviderErrorCode = 1
	ProviderErrorCodeInvalidRequest    ProviderErrorCode = 2
	ProviderErrorCodeRateLimited       ProviderErrorCode = 3
	ProviderErrorCodeQuotaExceeded     ProviderErrorCode = 4
	ProviderErrorCodeTimeout           ProviderErrorCode = 5
	ProviderErrorCodeServiceUnavailable ProviderErrorCode = 6
	ProviderErrorCodeAuthentication    ProviderErrorCode = 7
	ProviderErrorCodeAuthorization     ProviderErrorCode = 8
	ProviderErrorCodeUnsupportedFormat ProviderErrorCode = 9
	ProviderErrorCodeCanceled          ProviderErrorCode = 10
)

// String returns the string representation of the error code.
func (c ProviderErrorCode) String() string {
	switch c {
	case ProviderErrorCodeInternal:
		return "INTERNAL"
	case ProviderErrorCodeInvalidRequest:
		return "INVALID_REQUEST"
	case ProviderErrorCodeRateLimited:
		return "RATE_LIMITED"
	case ProviderErrorCodeQuotaExceeded:
		return "QUOTA_EXCEEDED"
	case ProviderErrorCodeTimeout:
		return "TIMEOUT"
	case ProviderErrorCodeServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	case ProviderErrorCodeAuthentication:
		return "AUTHENTICATION"
	case ProviderErrorCodeAuthorization:
		return "AUTHORIZATION"
	case ProviderErrorCodeUnsupportedFormat:
		return "UNSUPPORTED_FORMAT"
	case ProviderErrorCodeCanceled:
		return "CANCELED"
	default:
		return "UNSPECIFIED"
	}
}

// SessionContext is shared across all services for a session.
type SessionContext struct {
	SessionID    string            `json:"session_id"`
	TurnID       string            `json:"turn_id"`
	GenerationID string            `json:"generation_id"`
	TenantID     *string           `json:"tenant_id,omitempty"`
	TraceID      string            `json:"trace_id"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Options      map[string]string `json:"options,omitempty"`
	ProviderName string            `json:"provider_name"`
	ModelName    string            `json:"model_name"`
}

// AudioFormat specifies the audio format.
type AudioFormat struct {
	SampleRate int32         `json:"sample_rate"`
	Channels   int32         `json:"channels"`
	Encoding   AudioEncoding `json:"encoding"`
}

// ProviderError represents an error from a provider.
type ProviderError struct {
	Code         string            `json:"code"`
	Message      string            `json:"message"`
	ProviderName string            `json:"provider_name"`
	Retriable    bool              `json:"retriable"`
	Details      map[string]string `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s error [%s]: %s", e.ProviderName, e.Code, e.Message)
}

// CancelRequest is sent to cancel an ongoing operation.
type CancelRequest struct {
	SessionContext *SessionContext `json:"session_context"`
	Reason         string          `json:"reason"`
}

// CancelResponse is returned after a cancel request.
type CancelResponse struct {
	Acknowledged  bool   `json:"acknowledged"`
	GenerationID  string `json:"generation_id"`
}

// ProviderCapability describes the capabilities of a provider.
type ProviderCapability struct {
	SupportsStreamingInput       bool     `json:"supports_streaming_input"`
	SupportsStreamingOutput      bool     `json:"supports_streaming_output"`
	SupportsWordTimestamps       bool     `json:"supports_word_timestamps"`
	SupportsVoices               bool     `json:"supports_voices"`
	SupportsInterruptibleGeneration bool  `json:"supports_interruptible_generation"`
	PreferredSampleRates         []int32  `json:"preferred_sample_rates,omitempty"`
	SupportedCodecs              []string `json:"supported_codecs,omitempty"`
}

// ServingStatus represents the health status of a service.
type ServingStatus int32

const (
	ServingStatusUnknown       ServingStatus = 0
	ServingStatusServing       ServingStatus = 1
	ServingStatusNotServing    ServingStatus = 2
	ServingStatusServiceUnknown ServingStatus = 3
)

// HealthCheckRequest is sent to check service health.
type HealthCheckRequest struct {
	ServiceName string `json:"service_name"`
}

// HealthCheckResponse is returned from a health check.
type HealthCheckResponse struct {
	Status      ServingStatus `json:"status"`
	ServiceName string        `json:"service_name"`
	Version     string        `json:"version"`
}

// TimingMetadata tracks operation timing.
type TimingMetadata struct {
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	DurationMs  int64     `json:"duration_ms"`
}
