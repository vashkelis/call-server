# CloudApp — Real-Time AI Voice Engine

A production-grade, real-time voice conversation platform built with Go and Python. CloudApp enables natural, low-latency voice interactions with AI through a WebSocket API, featuring barge-in support, pluggable AI providers, and enterprise-grade observability.

## Architecture Overview

```
┌─────────────┐      WebSocket       ┌──────────────┐      gRPC       ┌─────────────────┐
│   Client    │◄────────────────────►│  Media-Edge  │◄───────────────►│    Orchestrator │
│  (Web/App)  │    (Audio/JSON)      │  (Go/WS)     │                 │     (Go)        │
└─────────────┘                      └──────────────┘                 └────────┬────────┘
                                                                               │
                                                                               │ gRPC
                                                                               ▼
                                                                      ┌─────────────────┐
                                                                      │ Provider Gateway │
                                                                      │    (Python)      │
                                                                      └────────┬────────┘
                                                                               │
                    ┌──────────────────────────────────────────────────────────┼──────────┐
                    │                                                          │          │
                    ▼                                                          ▼          ▼
            ┌───────────────┐                                        ┌─────────────┐ ┌──────────┐
            │  Redis Cache  │                                        │    ASR      │ │   LLM    │
            │  (Sessions)   │                                        │ (Whisper,   │ │ (Groq,   │
            └───────────────┘                                        │  Google)    │ │  vLLM)   │
                    │                                                └─────────────┘ └──────────┘
                    │                                                         │          │
                    ▼                                                         ▼          ▼
            ┌───────────────┐                                        ┌─────────────┐ ┌──────────┐
            │   PostgreSQL  │                                        │    TTS      │ │   VAD    │
            │ (Persistence) │                                        │ (Google,    │ │ (Energy) │
            └───────────────┘                                        │  XTTS)      │ │          │
                                                                     └─────────────┘ └──────────┘
```

## Key Features

- **Real-time WebSocket API**: Low-latency bidirectional audio streaming with JSON control messages
- **Barge-in / Interruption Support**: Users can interrupt the AI mid-response; unspoken text is discarded
- **Pluggable Provider Architecture**: Swap ASR, LLM, and TTS providers without code changes
- **Session State Machine**: Robust lifecycle management with states: Idle → Listening → Processing → Speaking → Interrupted
- **Multi-tenant**: Per-tenant provider configuration and session isolation
- **Observability**: Structured logging, Prometheus metrics, OpenTelemetry tracing
- **Horizontal Scaling**: Stateless media-edge with Redis-backed session store

## Repository Structure

```
CloudApp/
├── go/                          # Go services
│   ├── media-edge/              # WebSocket gateway service
│   │   ├── cmd/main.go          # Entry point
│   │   └── internal/
│   │       ├── handler/         # WebSocket handlers, session management
│   │       ├── transport/       # Audio transport abstractions
│   │       └── vad/             # Voice Activity Detection
│   ├── orchestrator/            # Pipeline orchestration service
│   │   ├── cmd/main.go          # Entry point
│   │   └── internal/
│   │       ├── pipeline/        # ASR→LLM→TTS pipeline engine
│   │       ├── statemachine/    # Session FSM and turn management
│   │       └── persistence/     # Redis/Postgres persistence
│   └── pkg/                     # Shared packages
│       ├── audio/               # Audio processing (buffer, resample, playout)
│       ├── config/              # Configuration loading
│       ├── contracts/           # Internal type definitions
│       ├── events/              # WebSocket event types
│       ├── observability/       # Logging, metrics, tracing
│       ├── providers/           # Provider registry and gRPC clients
│       └── session/             # Session management, state, history
├── py/provider_gateway/         # Python provider gateway
│   ├── app/
│   │   ├── core/                # Base provider ABCs, registry, capabilities
│   │   ├── grpc_api/            # gRPC service implementations
│   │   ├── providers/           # ASR, LLM, TTS provider implementations
│   │   │   ├── asr/             # Whisper, Google Speech, Mock
│   │   │   ├── llm/             # OpenAI-compatible, Groq, Mock
│   │   │   ├── tts/             # Google TTS, XTTS, Mock
│   │   │   └── vad/             # Energy-based VAD
│   │   └── config/              # Pydantic settings
│   └── main.py                  # Entry point
├── proto/                       # Protocol Buffer definitions
│   ├── asr.proto                # ASR service
│   ├── llm.proto                # LLM service
│   ├── tts.proto                # TTS service
│   ├── provider.proto           # Provider management
│   └── common.proto             # Shared types
├── infra/                       # Infrastructure
│   ├── compose/                 # Docker Compose files
│   ├── docker/                  # Dockerfiles
│   ├── k8s/                     # Kubernetes manifests
│   ├── migrations/              # Database migrations
│   └── prometheus/              # Monitoring config
├── examples/                    # Configuration examples
│   ├── config-mock.yaml         # Mock provider config
│   ├── config-cloud.yaml        # Cloud provider config
│   └── config-vllm.yaml         # Local vLLM config
└── scripts/                     # Utility scripts
    ├── ws-client.py             # WebSocket test client
    └── simulate-session.py      # Session simulator
```

## Quickstart

### Prerequisites

- Docker and Docker Compose
- Go 1.22+ (for local development)
- Python 3.11+ (for provider gateway development)

### Run with Docker Compose (Mock Mode)

```bash
# Clone the repository
cd /Users/vash/Dev/Parlona/CloudApp

# Start all services with mock providers
cd infra/compose
docker-compose --env-file .env.mock up --build

# Services will be available at:
# - Media-Edge: ws://localhost:8080/ws
# - Orchestrator: http://localhost:8081
# - Provider Gateway: localhost:50051
# - Redis: localhost:6379
# - Prometheus: http://localhost:9090
```

### Run WebSocket Client Example

```bash
# Install dependencies
pip install websockets

# Run the example client with synthetic audio
python scripts/ws-client.py --server ws://localhost:8080/ws --synthetic-duration 5

# Or with a WAV file
python scripts/ws-client.py --server ws://localhost:8080/ws --audio-file input.wav
```

## Configuration

Configuration is loaded from YAML files with environment variable overrides. See:

- [docs/config-reference.md](docs/config-reference.md) — Full configuration reference
- [examples/config-mock.yaml](examples/config-mock.yaml) — Mock provider configuration
- [examples/config-cloud.yaml](examples/config-cloud.yaml) — Cloud provider configuration

## Provider System

The provider system allows pluggable ASR, LLM, and TTS backends. Providers are implemented in Python and exposed via gRPC to the Go orchestrator.

See:

- [docs/provider-architecture.md](docs/provider-architecture.md) — Provider system design
- [docs/adding-asr-provider.md](docs/adding-asr-provider.md) — Adding a new ASR provider
- [docs/adding-llm-provider.md](docs/adding-llm-provider.md) — Adding a new LLM provider
- [docs/adding-tts-provider.md](docs/adding-tts-provider.md) — Adding a new TTS provider

## WebSocket API

The WebSocket API is the primary interface for clients. See:

- [docs/websocket-api.md](docs/websocket-api.md) — Full API reference with message examples

Quick example:

```json
// Client -> Server: Start session
{
  "type": "session.start",
  "session_id": "sess_123",
  "audio_profile": {"sample_rate": 16000, "channels": 1, "encoding": "pcm16"},
  "providers": {"asr": "mock", "llm": "mock", "tts": "mock"},
  "system_prompt": "You are a helpful assistant."
}

// Server -> Client: TTS audio chunk
{
  "type": "tts.audio_chunk",
  "session_id": "sess_123",
  "audio_data": "<base64-encoded-pcm16>",
  "segment_index": 0,
  "is_final": false
}
```

## Session Interruption

CloudApp supports barge-in (interruption) when the user speaks while the AI is responding. The system tracks playout position and only commits actually spoken text to conversation history.

See [docs/session-interruption.md](docs/session-interruption.md) for details.

## Testing

```bash
# Go tests
cd go
go test ./...

# Python tests
cd py/provider_gateway
python -m pytest

# Integration test with mock providers
cd infra/compose
docker-compose --env-file .env.mock up
cd ../..
python scripts/ws-client.py --server ws://localhost:8080/ws --synthetic
```

See [docs/testing.md](docs/testing.md) for detailed testing guide.

## Deployment

See [docs/deployment.md](docs/deployment.md) for:

- Docker Compose deployment options (mock, vllm, cloud)
- Kubernetes deployment
- GPU considerations for provider-gateway
- Environment variables reference
- Scaling considerations

## Documentation Index

- [docs/provider-architecture.md](docs/provider-architecture.md) — Provider system design
- [docs/adding-asr-provider.md](docs/adding-asr-provider.md) — Adding ASR providers
- [docs/adding-llm-provider.md](docs/adding-llm-provider.md) — Adding LLM providers
- [docs/adding-tts-provider.md](docs/adding-tts-provider.md) — Adding TTS providers
- [docs/websocket-api.md](docs/websocket-api.md) — WebSocket API reference
- [docs/config-reference.md](docs/config-reference.md) — Configuration reference
- [docs/session-interruption.md](docs/session-interruption.md) — Session lifecycle and interruption
- [docs/deployment.md](docs/deployment.md) — Deployment guide
- [docs/testing.md](docs/testing.md) — Testing guide
- [docs/implementation-notes.md](docs/implementation-notes.md) — Production readiness details

## Status

**MVP** — See [docs/implementation-notes.md](docs/implementation-notes.md) for production readiness details.

## License

[License TBD]
