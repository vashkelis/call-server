"""Generated proto stub equivalents for parlona.voice.v1."""

from .common_pb2 import (
    AudioEncoding,
    AudioFormat,
    CancelRequest,
    CancelResponse,
    CapabilityRequest,
    HealthCheckRequest,
    HealthCheckResponse,
    ProviderCapability,
    ProviderError,
    ProviderErrorCode,
    ProviderStatus,
    ProviderType,
    SessionContext,
    TimingMetadata,
)
from .asr_pb2 import (
    ASRRequest,
    ASRResponse,
    WordTimestamp,
)
from .llm_pb2 import (
    ChatMessage,
    LLMRequest,
    LLMResponse,
    UsageMetadata,
)
from .tts_pb2 import (
    TTSRequest,
    TTSResponse,
)
from .provider_pb2 import (
    GetProviderInfoRequest,
    ListProvidersRequest,
    ListProvidersResponse,
    ProviderInfo,
)

__all__ = [
    # Common
    "AudioEncoding",
    "AudioFormat",
    "CancelRequest",
    "CancelResponse",
    "CapabilityRequest",
    "HealthCheckRequest",
    "HealthCheckResponse",
    "ProviderCapability",
    "ProviderError",
    "ProviderErrorCode",
    "ProviderStatus",
    "ProviderType",
    "SessionContext",
    "TimingMetadata",
    # ASR
    "ASRRequest",
    "ASRResponse",
    "WordTimestamp",
    # LLM
    "ChatMessage",
    "LLMRequest",
    "LLMResponse",
    "UsageMetadata",
    # TTS
    "TTSRequest",
    "TTSResponse",
    # Provider
    "GetProviderInfoRequest",
    "ListProvidersRequest",
    "ListProvidersResponse",
    "ProviderInfo",
]
