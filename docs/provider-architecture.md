# Provider Architecture

## Overview

CloudApp uses a pluggable provider architecture that separates AI service implementations from the core orchestration logic. Providers are implemented in Python (for rapid development and ecosystem access) and communicate with the Go orchestrator via gRPC.

## Provider Types

| Type | Purpose | Example Providers |
|------|---------|-------------------|
| **ASR** | Automatic Speech Recognition | Whisper, Google Speech-to-Text, Azure Speech |
| **LLM** | Language Model Generation | Groq, OpenAI, vLLM, Anthropic |
| **TTS** | Text-to-Speech Synthesis | Google TTS, ElevenLabs, XTTS, Azure TTS |
| **VAD** | Voice Activity Detection | Energy-based, WebRTC VAD, Silero |

## Architecture Flow

```
┌─────────────────┐         gRPC          ┌──────────────────┐
│   Orchestrator  │◄─────────────────────►│ Provider Gateway │
│      (Go)       │   (Streaming calls)   │    (Python)      │
└─────────────────┘                       └────────┬─────────┘
                                                   │
                          ┌────────────────────────┼────────────────────────┐
                          │                        │                        │
                          ▼                        ▼                        ▼
                   ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
                   │  ASR Provider│        │  LLM Provider│        │  TTS Provider│
                   │  (Whisper)   │        │   (Groq)     │        │   (Google)   │
                   └─────────────┘        └─────────────┘        └─────────────┘
```

## Provider Registration

Providers are registered in the Python provider gateway via the registry system at `/py/provider_gateway/app/core/registry.py`.

### Registration Flow

1. **Discovery**: The registry auto-discovers providers by importing provider modules
2. **Factory Registration**: Each provider module exports a `register_providers()` function
3. **Lazy Instantiation**: Providers are created on first use with configuration

### Example Registration (ASR)

```python
# app/providers/asr/__init__.py
from app.providers.asr.mock_asr import create_mock_asr_provider
from app.providers.asr.faster_whisper import create_faster_whisper_provider

def register_providers(registry: "ProviderRegistry") -> None:
    registry.register_asr("mock", create_mock_asr_provider)
    registry.register_asr("faster_whisper", create_faster_whisper_provider)
```

## Provider Interface Contracts

All providers implement abstract base classes defined in `/py/provider_gateway/app/core/base_provider.py`.

### Base Provider Interface

```python
class BaseProvider(ABC):
    @abstractmethod
    def name(self) -> str:
        """Return the provider name."""
        pass

    @abstractmethod
    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        pass

    @abstractmethod
    async def cancel(self, session_id: str) -> bool:
        """Cancel an ongoing operation."""
        pass
```

### ASR Provider Interface

```python
class BaseASRProvider(BaseProvider):
    @abstractmethod
    async def stream_recognize(
        self,
        audio_stream: AsyncIterator[bytes],
        options: Optional[ASROptions] = None,
    ) -> AsyncIterator[ASRResponse]:
        """Stream audio and receive transcript responses."""
        pass
```

### LLM Provider Interface

```python
class BaseLLMProvider(BaseProvider):
    @abstractmethod
    async def stream_generate(
        self,
        messages: list,
        options: Optional[LLMOptions] = None,
    ) -> AsyncIterator[LLMResponse]:
        """Stream generate tokens from the LLM."""
        pass
```

### TTS Provider Interface

```python
class BaseTTSProvider(BaseProvider):
    @abstractmethod
    async def stream_synthesize(
        self,
        text: str,
        options: Optional[TTSOptions] = None,
    ) -> AsyncIterator[TTSResponse]:
        """Stream synthesize text to audio."""
        pass
```

## Capability Model

Providers advertise their capabilities via the `ProviderCapability` dataclass:

```python
@dataclass
class ProviderCapability:
    supports_streaming_input: bool       # Accepts streaming input
    supports_streaming_output: bool      # Produces streaming output
    supports_word_timestamps: bool       # ASR: provides word-level timing
    supports_voices: bool                # TTS: supports multiple voices
    supports_interruptible_generation: bool  # Can be cancelled mid-operation
    preferred_sample_rates: List[int]    # Audio sample rates (e.g., [16000, 8000])
    supported_codecs: List[str]          # Audio codecs (e.g., ["pcm16", "opus"])
```

### Capability Query

The orchestrator queries capabilities to:
- Validate audio format compatibility
- Determine if interruption is supported
- Select appropriate providers for session requirements

## Provider Selection Hierarchy

Providers can be selected at multiple levels (highest priority first):

1. **Request-level**: Specified in `session.start` message
2. **Session-level**: Stored in session configuration
3. **Tenant-level**: Configured per tenant
4. **Global default**: From configuration file

### Configuration Example

```yaml
# Global defaults
providers:
  defaults:
    asr: "mock"
    llm: "mock"
    tts: "mock"

  # Per-provider configuration
  asr:
    faster_whisper:
      model_size: "base"
      device: "cpu"
    google_speech:
      credentials_path: "/path/to/creds.json"

  llm:
    groq:
      api_key: "${GROQ_API_KEY}"
      model: "llama3-70b-8192"
```

## gRPC Communication

The Go orchestrator communicates with the Python provider gateway via gRPC:

### Service Definitions

```protobuf
// ASR Service
service ASRService {
  rpc StreamingRecognize(stream ASRRequest) returns (stream ASRResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
  rpc GetCapabilities(CapabilityRequest) returns (ProviderCapability);
}

// LLM Service
service LLMService {
  rpc StreamGenerate(LLMRequest) returns (stream LLMResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
}

// TTS Service
service TTSService {
  rpc StreamSynthesize(TTSRequest) returns (stream TTSResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
}
```

### Go Client Implementation

Go clients for gRPC providers are in `/go/pkg/providers/grpc_client.go`:

```go
// GRPCLLMProvider implements the LLM provider interface using gRPC
type GRPCLLMProvider struct {
    client LLMServiceClient
    config GRPCClientConfig
}

func (p *GRPCLLMProvider) Generate(ctx context.Context, req *contracts.LLMRequest) (<-chan *contracts.LLMResponse, error) {
    stream, err := p.client.StreamGenerate(ctx, protoReq)
    // Handle streaming response...
}
```

## Error Normalization

All provider errors are normalized to a common error structure:

```python
class ProviderError(Exception):
    def __init__(
        self,
        code: ProviderErrorCode,
        message: str,
        provider_name: str,
        retriable: bool = False,
    ):
        self.code = code
        self.message = message
        self.provider_name = provider_name
        self.retriable = retriable
```

### Error Codes

- `INTERNAL` — Provider internal error
- `INVALID_REQUEST` — Invalid input parameters
- `RATE_LIMITED` — Rate limit exceeded
- `QUOTA_EXCEEDED` — Usage quota exceeded
- `TIMEOUT` — Operation timed out
- `SERVICE_UNAVAILABLE` — Provider service unavailable
- `AUTHENTICATION` — Authentication failed
- `AUTHORIZATION` — Permission denied
- `UNSUPPORTED_FORMAT` — Audio/text format not supported
- `CANCELED` — Operation was cancelled

## Mock Providers

Mock providers are included for development and testing:

- **MockASR**: Returns deterministic transcripts with configurable delays
- **MockLLM**: Echoes input or returns canned responses
- **MockTTS**: Generates sine wave audio at 440Hz

Mock providers support:
- Configurable latency simulation
- Cancellation testing
- Deterministic responses for reproducible tests

### Enabling Mock Mode

```yaml
providers:
  defaults:
    asr: "mock"
    llm: "mock"
    tts: "mock"
```

## Provider Flow Diagram

```
┌──────────────┐
│ Session Start │
└──────┬───────┘
       │
       ▼
┌─────────────────┐
│ Provider Registry│
│  (Get Provider)  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│  Capability Check│────►│ Format Validation│
└────────┬────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐
│  Stream Request  │
│   (gRPC/HTTP)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│  Process Stream  │◄───►│  Handle Cancel   │
│  (Yield chunks)  │     │  (if interrupted)│
└────────┬────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐
│  Stream Complete │
│  or Cancelled    │
└─────────────────┘
```

## Adding New Providers

See the provider-specific guides:

- [Adding ASR Providers](adding-asr-provider.md)
- [Adding LLM Providers](adding-llm-provider.md)
- [Adding TTS Providers](adding-tts-provider.md)
