# gRPC Service Implementation

<cite>
**Referenced Files in This Document**
- [main.go](file://go/orchestrator/cmd/main.go)
- [grpc_client.go](file://go/pkg/providers/grpc_client.go)
- [interfaces.go](file://go/pkg/providers/interfaces.go)
- [registry.go](file://go/pkg/providers/registry.go)
- [server.py](file://py/provider_gateway/app/grpc_api/server.py)
- [asr_servicer.py](file://py/provider_gateway/app/grpc_api/asr_servicer.py)
- [llm_servicer.py](file://py/provider_gateway/app/grpc_api/llm_servicer.py)
- [tts_servicer.py](file://py/provider_gateway/app/grpc_api/tts_servicer.py)
- [provider_servicer.py](file://py/provider_gateway/app/grpc_api/provider_servicer.py)
- [registry.py](file://py/provider_gateway/app/core/registry.py)
- [asr.proto](file://proto/asr.proto)
- [llm.proto](file://proto/llm.proto)
- [tts.proto](file://proto/tts.proto)
- [provider.proto](file://proto/provider.proto)
- [common.proto](file://proto/common.proto)
- [config.go](file://go/pkg/config/config.go)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Project Structure](#project-structure)
3. [Core Components](#core-components)
4. [Architecture Overview](#architecture-overview)
5. [Detailed Component Analysis](#detailed-component-analysis)
6. [Dependency Analysis](#dependency-analysis)
7. [Performance Considerations](#performance-considerations)
8. [Troubleshooting Guide](#troubleshooting-guide)
9. [Conclusion](#conclusion)

## Introduction
This document explains the gRPC service implementation for the Parlona CloudApp, focusing on the asynchronous gRPC server setup and service handlers. It covers the server architecture, connection management, and service registration patterns for ASR, LLM, TTS, and Provider services. It also documents request/response handling, streaming support, error propagation, server configuration, threading model, graceful shutdown, and cross-language communication patterns between the Go orchestrator and the Python provider-gateway.

## Project Structure
The gRPC implementation spans two languages:
- Go orchestrator registers gRPC provider clients and coordinates sessions.
- Python provider-gateway exposes gRPC services for ASR, LLM, TTS, and Provider management.

```mermaid
graph TB
subgraph "Go Orchestrator"
GO_MAIN["go/orchestrator/cmd/main.go"]
GO_CLIENT["go/pkg/providers/grpc_client.go"]
GO_REGISTRY["go/pkg/providers/registry.go"]
GO_IFACES["go/pkg/providers/interfaces.go"]
end
subgraph "Python Provider-Gateway"
PY_SERVER["py/provider_gateway/app/grpc_api/server.py"]
PY_ASRS["py/provider_gateway/app/grpc_api/asr_servicer.py"]
PY_LLMS["py/provider_gateway/app/grpc_api/llm_servicer.py"]
PY_TTSS["py/provider_gateway/app/grpc_api/tts_servicer.py"]
PY_PROV["py/provider_gateway/app/grpc_api/provider_servicer.py"]
PY_REG["py/provider_gateway/app/core/registry.py"]
end
subgraph "Protobuf Definitions"
PROTO_ASR["proto/asr.proto"]
PROTO_LLM["proto/llm.proto"]
PROTO_TTS["proto/tts.proto"]
PROTO_PROV["proto/provider.proto"]
PROTO_COMMON["proto/common.proto"]
end
GO_MAIN --> GO_REGISTRY
GO_REGISTRY --> GO_CLIENT
GO_CLIENT --> PY_SERVER
PY_SERVER --> PY_ASRS
PY_SERVER --> PY_LLMS
PY_SERVER --> PY_TTSS
PY_SERVER --> PY_PROV
PY_ASRS --> PY_REG
PY_LLMS --> PY_REG
PY_TTSS --> PY_REG
PY_ASRS -.uses.-> PROTO_ASR
PY_LLMS -.uses.-> PROTO_LLM
PY_TTSS -.uses.-> PROTO_TTS
PY_PROV -.uses.-> PROTO_PROV
PY_ASRS -.uses.-> PROTO_COMMON
PY_LLMS -.uses.-> PROTO_COMMON
PY_TTSS -.uses.-> PROTO_COMMON
PY_PROV -.uses.-> PROTO_COMMON
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)
- [interfaces.go:10-107](file://go/pkg/providers/interfaces.go#L10-L107)
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)
- [asr_servicer.py:28-239](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L28-L239)
- [llm_servicer.py:24-218](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L24-L218)
- [tts_servicer.py:27-228](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L27-L228)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [asr.proto:1-53](file://proto/asr.proto#L1-L53)
- [llm.proto:1-59](file://proto/llm.proto#L1-L59)
- [tts.proto:1-45](file://proto/tts.proto#L1-L45)
- [provider.proto:1-63](file://proto/provider.proto#L1-L63)
- [common.proto:1-110](file://proto/common.proto#L1-L110)

**Section sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)

## Core Components
- Go gRPC provider clients: Implement ASR, LLM, and TTS provider interfaces and manage gRPC connections to the Python provider-gateway.
- Python gRPC server: Asynchronous gRPC server exposing ASRService, LLMService, TTSService, and ProviderService.
- Provider registry: Manages provider factories and instances in Python; orchestrator registers gRPC provider clients in Go.
- Protobuf contracts: Define messages and services for cross-language compatibility.

Key responsibilities:
- Streaming recognition, generation, and synthesis with bidirectional/server streaming semantics.
- Capability queries and cancellation support.
- Session context propagation and timing metadata.
- Graceful shutdown and signal handling.

**Section sources**
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [interfaces.go:10-107](file://go/pkg/providers/interfaces.go#L10-L107)
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)
- [asr_servicer.py:28-239](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L28-L239)
- [llm_servicer.py:24-218](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L24-L218)
- [tts_servicer.py:27-228](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L27-L228)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)

## Architecture Overview
The system uses a hybrid architecture:
- Go orchestrator registers gRPC provider clients and delegates work to the Python provider-gateway.
- Python provider-gateway exposes asynchronous gRPC services with streaming support and capability discovery.

```mermaid
sequenceDiagram
participant Client as "External Client"
participant GoOrchestrator as "Go Orchestrator"
participant GRPCClient as "Go gRPC Provider Client"
participant PyGateway as "Python gRPC Server"
participant Servicer as "ASR/LLM/TTS Servicer"
Client->>GoOrchestrator : Configure session and providers
GoOrchestrator->>GRPCClient : Create gRPC client with address
Client->>PyGateway : Establish gRPC connection
Client->>Servicer : StreamingRecognize/StreamGenerate/StreamSynthesize
Servicer->>Servicer : Lookup provider in registry
Servicer-->>Client : Streamed responses (transcripts/tokens/audio)
Client->>Servicer : Cancel(session_context)
Servicer-->>Client : CancelResponse
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [grpc_client.go:45-60](file://go/pkg/providers/grpc_client.go#L45-L60)
- [server.py:54-90](file://py/provider_gateway/app/grpc_api/server.py#L54-L90)
- [asr_servicer.py:42-122](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L42-L122)
- [llm_servicer.py:38-105](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L38-L105)
- [tts_servicer.py:41-110](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L41-L110)

## Detailed Component Analysis

### Go gRPC Provider Clients
The Go implementation defines provider interfaces and stubbed gRPC clients that connect to the Python provider-gateway. These clients:
- Dial the provider-gateway address using insecure credentials.
- Implement streaming methods with channels and context cancellation.
- Expose capability queries and provider names.
- Support graceful closing of connections.

```mermaid
classDiagram
class ASRProvider {
+StreamRecognize(ctx, audioStream, opts) ASRResult
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class LLMProvider {
+StreamGenerate(ctx, messages, opts) LLMToken
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class TTSProvider {
+StreamSynthesize(ctx, text, opts) []byte
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class GRPCASRProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamRecognize(...)
+Cancel(...)
+Capabilities()
+Close()
}
class GRPCLLMProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamGenerate(...)
+Cancel(...)
+Capabilities()
+Close()
}
class GRPCTTSProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamSynthesize(...)
+Cancel(...)
+Capabilities()
+Close()
}
ASRProvider <|.. GRPCASRProvider
LLMProvider <|.. GRPCLLMProvider
TTSProvider <|.. GRPCTTSProvider
```

**Diagram sources**
- [interfaces.go:21-76](file://go/pkg/providers/interfaces.go#L21-L76)
- [grpc_client.go:35-277](file://go/pkg/providers/grpc_client.go#L35-L277)

**Section sources**
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [interfaces.go:10-107](file://go/pkg/providers/interfaces.go#L10-L107)

### Python gRPC Server and Servicers
The Python provider-gateway runs an asynchronous gRPC server with:
- ThreadPoolExecutor-based worker pool.
- Registration of ASRService, LLMService, TTSService, and ProviderService.
- Signal handling for graceful shutdown.
- Per-service streaming handlers delegating to provider registries.

```mermaid
sequenceDiagram
participant Server as "GRPCServer"
participant ASR as "ASRServicer"
participant LLM as "LLMServicer"
participant TTS as "TTSServicer"
participant Prov as "ProviderServicer"
participant Registry as "ProviderRegistry"
Server->>Server : start()
Server->>ASR : add_ASRServiceServicer_to_server
Server->>LLM : add_LLMServiceServicer_to_server
Server->>TTS : add_TTSServiceServicer_to_server
Server->>Prov : add_ProviderServiceServicer_to_server
Server->>Server : add_insecure_port(host : port)
Server->>Server : start()
Note over ASR,Registry : StreamingRecognize(request_iterator)
ASR->>Registry : get_asr(provider_name)
ASR-->>ASR : stream_recognize(audio_stream, options)
ASR-->>Client : ASRResponse stream
Note over LLM,Registry : StreamGenerate(request)
LLM->>Registry : get_llm(provider_name)
LLM-->>LLM : stream_generate(messages, options)
LLM-->>Client : LLMResponse stream
Note over TTS,Registry : StreamSynthesize(request)
TTS->>Registry : get_tts(provider_name)
TTS-->>TTS : stream_synthesize(text, options)
TTS-->>Client : TTSResponse stream
```

**Diagram sources**
- [server.py:54-90](file://py/provider_gateway/app/grpc_api/server.py#L54-L90)
- [asr_servicer.py:42-122](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L42-L122)
- [llm_servicer.py:38-105](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L38-L105)
- [tts_servicer.py:41-110](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L41-L110)
- [registry.py:85-169](file://py/provider_gateway/app/core/registry.py#L85-L169)

**Section sources**
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)
- [asr_servicer.py:28-239](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L28-L239)
- [llm_servicer.py:24-218](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L24-L218)
- [tts_servicer.py:27-228](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L27-L228)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)

### ASR Service Implementation
- StreamingRecognize: Bidirectional streaming for audio input and transcript output. Extracts session context and provider name from the first request, looks up the provider, streams audio chunks, and yields ASRResponse messages.
- Cancel: Cancels an ongoing recognition by delegating to the provider’s cancel method.
- GetCapabilities: Returns provider capability flags.

```mermaid
flowchart TD
Start(["StreamingRecognize Entry"]) --> FirstReq["Read first ASRRequest"]
FirstReq --> Extract["Extract session_id and provider_name"]
Extract --> Lookup{"Provider found?"}
Lookup --> |No| RaiseErr["Raise ProviderError"]
Lookup --> |Yes| BuildStream["Build audio stream from iterator"]
BuildStream --> CallProvider["Call provider.stream_recognize(...)"]
CallProvider --> YieldResp["Yield ASRResponse"]
YieldResp --> FinalCheck{"is_final?"}
FinalCheck --> |Yes| Done(["Exit"])
FinalCheck --> |No| NextChunk["Next request in iterator"]
NextChunk --> CallProvider
```

**Diagram sources**
- [asr_servicer.py:42-122](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L42-L122)

**Section sources**
- [asr_servicer.py:28-239](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L28-L239)
- [asr.proto:9-53](file://proto/asr.proto#L9-L53)

### LLM Service Implementation
- StreamGenerate: Server streaming for prompt input and token output. Converts ChatMessage list and options, streams tokens, and yields LLMResponse messages.
- Cancel: Cancels an ongoing generation.
- GetCapabilities: Returns provider capability flags.

```mermaid
sequenceDiagram
participant Client as "Client"
participant LLM as "LLMServicer.StreamGenerate"
participant Reg as "ProviderRegistry"
participant Prov as "LLM Provider"
Client->>LLM : LLMRequest(messages, options)
LLM->>Reg : get_llm(provider_name)
Reg-->>LLM : LLMProvider
LLM->>Prov : stream_generate(messages, options)
loop For each token
Prov-->>LLM : LLMToken
LLM-->>Client : LLMResponse(token, usage, timing)
end
Prov-->>LLM : finish_reason
LLM-->>Client : LLMResponse(is_final=true)
```

**Diagram sources**
- [llm_servicer.py:38-105](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L38-L105)

**Section sources**
- [llm_servicer.py:24-218](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L24-L218)
- [llm.proto:9-59](file://proto/llm.proto#L9-L59)

### TTS Service Implementation
- StreamSynthesize: Server streaming for text input and audio output. Converts audio format and options, streams audio chunks, and yields TTSResponse messages.
- Cancel: Cancels an ongoing synthesis.
- GetCapabilities: Returns provider capability flags.

```mermaid
flowchart TD
Start(["StreamSynthesize Entry"]) --> ReadReq["Read TTSRequest"]
ReadReq --> Extract["Extract session_id and provider_name"]
Extract --> Lookup{"Provider found?"}
Lookup --> |No| RaiseErr["Raise ProviderError"]
Lookup --> |Yes| BuildOpts["Build TTSOptions (voice_id, audio_format)"]
BuildOpts --> CallProv["Call provider.stream_synthesize(text, options)"]
CallProv --> Yield["Yield TTSResponse(audio_chunk, timing)"]
Yield --> More{"More audio?"}
More --> |Yes| CallProv
More --> |No| Done(["Exit"])
```

**Diagram sources**
- [tts_servicer.py:41-110](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L41-L110)

**Section sources**
- [tts_servicer.py:27-228](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L27-L228)
- [tts.proto:9-45](file://proto/tts.proto#L9-L45)

### Provider Service Implementation
- ListProviders: Lists providers by type or all providers, mapping internal capabilities to proto ProviderInfo.
- GetProviderInfo: Retrieves detailed info for a specific provider.
- HealthCheck: Returns service health status and version.

```mermaid
sequenceDiagram
participant Client as "Client"
participant ProvSvc as "ProviderServicer"
participant Reg as "ProviderRegistry"
Client->>ProvSvc : ListProviders(provider_type)
ProvSvc->>Reg : list_asr/llm/tts_providers()
Reg-->>ProvSvc : Names[]
ProvSvc-->>Client : ListProvidersResponse
Client->>ProvSvc : GetProviderInfo(name, type)
ProvSvc->>Reg : get_provider_capabilities(name, type)
Reg-->>ProvSvc : ProviderCapability
ProvSvc-->>Client : ProviderInfo
Client->>ProvSvc : HealthCheck(service_name)
ProvSvc-->>Client : HealthCheckResponse(status, version)
```

**Diagram sources**
- [provider_servicer.py:43-187](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L43-L187)

**Section sources**
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [provider.proto:26-63](file://proto/provider.proto#L26-L63)

### Cross-Language Communication Patterns
- Protobuf contracts define messages and enums shared across languages.
- Go orchestrator uses provider interfaces and stubbed gRPC clients; Python provider-gateway implements the actual streaming logic.
- SessionContext, AudioFormat, TimingMetadata, and ProviderCapability are mirrored in both proto and Go contracts.

```mermaid
graph LR
Proto["proto/*.proto"] --> GoTypes["Go contracts/*.go"]
Proto --> PyModels["Python models/*.py"]
GoTypes --> GoImpl["Go provider clients"]
PyModels --> PyImpl["Python servicers"]
GoImpl --> PyImpl
```

**Diagram sources**
- [common.proto:33-110](file://proto/common.proto#L33-L110)
- [asr.proto:26-52](file://proto/asr.proto#L26-L52)
- [llm.proto:39-58](file://proto/llm.proto#L39-L58)
- [tts.proto:26-44](file://proto/tts.proto#L26-L44)
- [provider.proto:43-62](file://proto/provider.proto#L43-L62)
- [config.go:9-94](file://go/pkg/config/config.go#L9-L94)

**Section sources**
- [common.proto:1-110](file://proto/common.proto#L1-L110)
- [config.go:1-276](file://go/pkg/config/config.go#L1-L276)

## Dependency Analysis
- Go orchestrator depends on provider registry and gRPC client implementations to connect to the Python provider-gateway.
- Python provider-gateway depends on provider registries and servicers to expose streaming APIs.
- Protobuf definitions define the contract for cross-language compatibility.

```mermaid
graph TB
GO_ORCH["go/orchestrator/cmd/main.go"] --> GO_REG["go/pkg/providers/registry.go"]
GO_ORCH --> GO_CLI["go/pkg/providers/grpc_client.go"]
GO_CLI --> PY_SRV["py/provider_gateway/app/grpc_api/server.py"]
PY_SRV --> PY_SVC["py/provider_gateway/app/grpc_api/*_servicer.py"]
PY_SVC --> PY_REG["py/provider_gateway/app/core/registry.py"]
PY_SVC -.uses.-> PROTO["proto/*.proto"]
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)
- [asr_servicer.py:28-239](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L28-L239)
- [llm_servicer.py:24-218](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L24-L218)
- [tts_servicer.py:27-228](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L27-L228)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [asr.proto:1-53](file://proto/asr.proto#L1-L53)
- [llm.proto:1-59](file://proto/llm.proto#L1-L59)
- [tts.proto:1-45](file://proto/tts.proto#L1-L45)
- [provider.proto:1-63](file://proto/provider.proto#L1-L63)

**Section sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)
- [server.py:25-171](file://py/provider_gateway/app/grpc_api/server.py#L25-L171)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)

## Performance Considerations
- Threading model: Python server uses a ThreadPoolExecutor to handle concurrent RPCs; tune max_workers according to CPU and provider latency characteristics.
- Message sizes: gRPC server options configure maximum send/receive message length; adjust based on audio payload sizes.
- Streaming: Prefer server-streaming for tokenized outputs and bidirectional streaming for real-time audio to minimize latency.
- Backpressure: Channel buffer sizes in Go clients should match downstream processing rates to avoid blocking.
- Connection lifecycle: Reuse gRPC connections per provider; close gracefully during shutdown.
- Observability: Enable metrics and tracing to monitor latency, throughput, and error rates.

[No sources needed since this section provides general guidance]

## Troubleshooting Guide
Common issues and resolutions:
- Provider not found: Ensure provider names match between orchestrator and gateway; verify registration steps.
- Streaming errors: Check session context propagation and provider capability support for streaming.
- Cancellation failures: Verify provider cancel implementation and session tracking in servicers.
- Connection errors: Validate provider-gateway address and network connectivity; confirm insecure credentials usage.
- Graceful shutdown: Confirm signal handlers and server stop routines are invoked.

Operational checks:
- Health endpoints: Use readiness probes to validate Redis connectivity and service status.
- Logs and traces: Enable logging and OpenTelemetry tracing for end-to-end visibility.

**Section sources**
- [asr_servicer.py:112-122](file://py/provider_gateway/app/grpc_api/asr_servicer.py#L112-L122)
- [llm_servicer.py:97-105](file://py/provider_gateway/app/grpc_api/llm_servicer.py#L97-L105)
- [tts_servicer.py:101-109](file://py/provider_gateway/app/grpc_api/tts_servicer.py#L101-L109)
- [main.go:125-145](file://go/orchestrator/cmd/main.go#L125-L145)

## Conclusion
The gRPC implementation combines a Go orchestrator with a Python provider-gateway to deliver asynchronous streaming services for ASR, LLM, and TTS. The design leverages protobuf contracts for cross-language compatibility, supports capability discovery and cancellation, and provides robust streaming semantics. Proper configuration of threading, timeouts, and graceful shutdown ensures reliable operation at scale.