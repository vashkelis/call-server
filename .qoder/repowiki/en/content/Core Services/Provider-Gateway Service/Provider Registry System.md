# Provider Registry System

<cite>
**Referenced Files in This Document**
- [registry.go](file://go/pkg/providers/registry.go)
- [interfaces.go](file://go/pkg/providers/interfaces.go)
- [options.go](file://go/pkg/providers/options.go)
- [grpc_client.go](file://go/pkg/providers/grpc_client.go)
- [provider.go](file://go/pkg/contracts/provider.go)
- [session.go](file://go/pkg/session/session.go)
- [main.go](file://go/orchestrator/cmd/main.go)
- [main.go](file://go/media-edge/cmd/main.go)
- [registry.py](file://py/provider_gateway/app/core/registry.py)
- [capability.py](file://py/provider_gateway/app/core/capability.py)
- [base_provider.py](file://py/provider_gateway/app/core/base_provider.py)
- [provider_servicer.py](file://py/provider_gateway/app/grpc_api/provider_servicer.py)
- [faster_whisper.py](file://py/provider_gateway/app/providers/asr/faster_whisper.py)
- [groq.py](file://py/provider_gateway/app/providers/llm/groq.py)
- [xtts.py](file://py/provider_gateway/app/providers/tts/xtts.py)
- [config-cloud.yaml](file://examples/config-cloud.yaml)
- [provider-architecture.md](file://docs/provider-architecture.md)
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
10. [Appendices](#appendices)

## Introduction
This document describes the Provider Registry System that powers dynamic provider registration, capability discovery, and service orchestration across the CloudApp platform. It explains how providers are registered, discovered, and resolved for sessions, how capabilities guide selection, and how lifecycle, health, and failover concerns are addressed. It also provides practical guidance for registering custom providers, querying capabilities, managing instances, ensuring isolation and performance, and implementing discovery, load balancing, and scaling strategies.

## Project Structure
The system spans two primary languages:
- Go: Orchestrator, provider registry, gRPC clients, and session management
- Python: Provider gateway, provider implementations, and gRPC service

```mermaid
graph TB
subgraph "Go Services"
ORCH["Orchestrator (Go)<br/>cmd/main.go"]
REG_G["Provider Registry (Go)<br/>pkg/providers/registry.go"]
IFACES_G["Provider Interfaces (Go)<br/>pkg/providers/interfaces.go"]
OPT_G["Provider Options (Go)<br/>pkg/providers/options.go"]
GRPC_C["gRPC Clients (Go)<br/>pkg/providers/grpc_client.go"]
SESS_G["Session Model (Go)<br/>pkg/session/session.go"]
end
subgraph "Python Services"
REG_P["Provider Registry (Python)<br/>py/provider_gateway/app/core/registry.py"]
CAP_P["Capability Model (Python)<br/>py/provider_gateway/app/core/capability.py"]
BASE_P["Base Provider Classes (Python)<br/>py/provider_gateway/app/core/base_provider.py"]
SRV_P["Provider gRPC Servicer (Python)<br/>py/provider_gateway/app/grpc_api/provider_servicer.py"]
ASR_P["ASR Provider (Python)<br/>py/provider_gateway/app/providers/asr/faster_whisper.py"]
LLM_P["LLM Provider (Python)<br/>py/provider_gateway/app/providers/llm/groq.py"]
TTS_P["TTS Provider (Python)<br/>py/provider_gateway/app/providers/tts/xtts.py"]
end
ORCH --> REG_G
ORCH --> GRPC_C
ORCH --> SESS_G
REG_G --> GRPC_C
GRPC_C --> REG_P
REG_P --> SRV_P
SRV_P --> ASR_P
SRV_P --> LLM_P
SRV_P --> TTS_P
REG_P --> CAP_P
ASR_P --> CAP_P
LLM_P --> CAP_P
TTS_P --> CAP_P
BASE_P --> ASR_P
BASE_P --> LLM_P
BASE_P --> TTS_P
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [registry.go:14-40](file://go/pkg/providers/registry.go#L14-L40)
- [interfaces.go:21-97](file://go/pkg/providers/interfaces.go#L21-L97)
- [options.go:7-188](file://go/pkg/providers/options.go#L7-L188)
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [session.go:34-84](file://go/pkg/session/session.go#L34-L84)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [capability.py:7-61](file://py/provider_gateway/app/core/capability.py#L7-L61)
- [base_provider.py:12-177](file://py/provider_gateway/app/core/base_provider.py#L12-L177)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [faster_whisper.py:15-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L15-L262)
- [groq.py:16-124](file://py/provider_gateway/app/providers/llm/groq.py#L16-L124)
- [xtts.py:14-106](file://py/provider_gateway/app/providers/tts/xtts.py#L14-L106)

**Section sources**
- [provider-architecture.md:1-320](file://docs/provider-architecture.md#L1-L320)

## Core Components
- Go Provider Registry: Manages provider registration and resolution for ASR, LLM, TTS, and VAD, with tenant-scoped overrides and request-level precedence.
- Go Provider Interfaces: Define streaming recognition, generation, synthesis, cancellation, and capability retrieval for providers.
- Go gRPC Clients: Stub implementations of gRPC provider clients for ASR, LLM, and TTS, designed to integrate with the Python provider gateway.
- Python Provider Registry: Dynamic discovery and registration of providers, caching instances, and capability exposure.
- Python Provider Base Classes: Abstract interfaces for ASR, LLM, and TTS providers with standardized capability reporting.
- Python Provider Servicer: gRPC service exposing provider listing, capability queries, and health checks.
- Provider Implementations: Example providers (ASR Whisper, LLM Groq, TTS XTTS) demonstrating capability modeling and cancellation.

**Section sources**
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)
- [interfaces.go:21-97](file://go/pkg/providers/interfaces.go#L21-L97)
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [base_provider.py:12-177](file://py/provider_gateway/app/core/base_provider.py#L12-L177)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [faster_whisper.py:15-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L15-L262)
- [groq.py:16-124](file://py/provider_gateway/app/providers/llm/groq.py#L16-L124)
- [xtts.py:14-106](file://py/provider_gateway/app/providers/tts/xtts.py#L14-L106)

## Architecture Overview
The system uses a hybrid architecture:
- The Go orchestrator owns session orchestration and selects providers via a registry.
- Providers are implemented in Python and exposed via a provider gateway with a gRPC interface.
- The Go registry integrates with the Python gateway by registering gRPC clients that wrap provider calls.

```mermaid
sequenceDiagram
participant Orchestrator as "Orchestrator (Go)"
participant Registry as "Provider Registry (Go)"
participant GRPC as "gRPC Clients (Go)"
participant Gateway as "Provider Gateway (Python)"
participant RegistryPy as "Registry (Python)"
participant Provider as "Provider Impl (Python)"
Orchestrator->>Registry : ResolveForSession(session, overrides)
Registry-->>Orchestrator : SelectedProviders
Orchestrator->>GRPC : StreamRecognize/StreamGenerate/StreamSynthesize
GRPC->>Gateway : gRPC call (streaming)
Gateway->>RegistryPy : get_provider_capabilities()
RegistryPy-->>Gateway : ProviderCapability
Gateway->>Provider : stream_*()
Provider-->>Gateway : streaming results
Gateway-->>GRPC : streaming results
GRPC-->>Orchestrator : streaming results
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [registry.go:172-251](file://go/pkg/providers/registry.go#L172-L251)
- [grpc_client.go:62-247](file://go/pkg/providers/grpc_client.go#L62-L247)
- [provider_servicer.py:43-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L43-L190)
- [registry.py:182-204](file://py/provider_gateway/app/core/registry.py#L182-L204)

## Detailed Component Analysis

### Go Provider Registry
The Go registry maintains separate maps for ASR, LLM, TTS, and VAD providers, guarded by a mutex for thread safety. It supports:
- Registration of provider instances
- Lookup by name
- Listing provider names
- Resolution of providers for a session with priority: request → session → tenant → global defaults
- Validation that selected providers exist before returning selections

```mermaid
classDiagram
class ProviderRegistry {
-mu RWMutex
-asr map[string]ASRProvider
-llm map[string]LLMProvider
-tts map[string]TTSProvider
-vad map[string]VADProvider
-factories map[ProviderType]map[string]ProviderFactory
-config *registryConfig
+RegisterASR(name, provider)
+RegisterLLM(name, provider)
+RegisterTTS(name, provider)
+RegisterVAD(name, provider)
+GetASR(name) ASRProvider,error
+GetLLM(name) LLMProvider,error
+GetTTS(name) TTSProvider,error
+GetVAD(name) VADProvider,error
+ListASR() []string
+ListLLM() []string
+ListTTS() []string
+ListVAD() []string
+ResolveForSession(sess, requestProviders) SelectedProviders,error
+SetConfig(global, tenantOverrides)
}
```

**Diagram sources**
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)

**Section sources**
- [registry.go:14-262](file://go/pkg/providers/registry.go#L14-L262)

### Go Provider Interfaces and Options
Provider interfaces define streaming operations, cancellation, capability reporting, and naming. Options encapsulate provider-specific parameters for ASR, LLM, and TTS with builder-style helpers.

```mermaid
classDiagram
class ASRProvider {
+StreamRecognize(ctx, audioStream, opts) chan ASRResult,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class LLMProvider {
+StreamGenerate(ctx, messages, opts) chan LLMToken,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class TTSProvider {
+StreamSynthesize(ctx, text, opts) chan []byte,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
}
class ASROptions {
+WithLanguageHint(lang) ASROptions
+WithTimestamps(enable) ASROptions
+WithAudioFormat(format) ASROptions
+WithProviderOption(key,value) ASROptions
}
class LLMOptions {
+WithModel(model) LLMOptions
+WithMaxTokens(tokens) LLMOptions
+WithTemperature(temp) LLMOptions
+WithTopP(topP) LLMOptions
+WithStopSequences(sequences) LLMOptions
+WithSystemPrompt(prompt) LLMOptions
+WithProviderOption(key,value) LLMOptions
}
class TTSOptions {
+WithVoiceID(id) TTSOptions
+WithSpeed(speed) TTSOptions
+WithPitch(pitch) TTSOptions
+WithAudioFormat(format) TTSOptions
+WithSegmentIndex(index) TTSOptions
+WithProviderOption(key,value) TTSOptions
}
ASRProvider --> ASROptions : "uses"
LLMProvider --> LLMOptions : "uses"
TTSProvider --> TTSOptions : "uses"
```

**Diagram sources**
- [interfaces.go:21-97](file://go/pkg/providers/interfaces.go#L21-L97)
- [options.go:7-188](file://go/pkg/providers/options.go#L7-L188)

**Section sources**
- [interfaces.go:21-97](file://go/pkg/providers/interfaces.go#L21-L97)
- [options.go:7-188](file://go/pkg/providers/options.go#L7-L188)

### Go gRPC Provider Clients
The Go gRPC clients are placeholders that demonstrate the expected structure for connecting to the Python provider gateway. They implement the provider interfaces and forward calls to the gateway, with capability reporting aligned to the contract.

```mermaid
classDiagram
class GRPCASRProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamRecognize(ctx, audioStream, opts) chan ASRResult,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
+Close() error
}
class GRPCLLMProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamGenerate(ctx, messages, opts) chan LLMToken,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
+Close() error
}
class GRPCTTSProvider {
-name string
-config GRPCClientConfig
-conn *grpc.ClientConn
+StreamSynthesize(ctx, text, opts) chan []byte,error
+Cancel(ctx, sessionID) error
+Capabilities() ProviderCapability
+Name() string
+Close() error
}
GRPCASRProvider ..|> ASRProvider
GRPCLLMProvider ..|> LLMProvider
GRPCTTSProvider ..|> TTSProvider
```

**Diagram sources**
- [grpc_client.go:35-288](file://go/pkg/providers/grpc_client.go#L35-L288)

**Section sources**
- [grpc_client.go:14-288](file://go/pkg/providers/grpc_client.go#L14-L288)

### Python Provider Registry and Capability Model
The Python registry supports dynamic discovery, factory-based instantiation, and caching of provider instances keyed by name and configuration hash. It exposes capability queries and integrates with the gRPC provider service.

```mermaid
classDiagram
class ProviderRegistry {
-_asr_factories dict
-_llm_factories dict
-_tts_factories dict
-_asr_providers dict
-_llm_providers dict
-_tts_providers dict
+register_asr(name, factory)
+register_llm(name, factory)
+register_tts(name, factory)
+get_asr(name, **config) BaseASRProvider?
+get_llm(name, **config) BaseLLMProvider?
+get_tts(name, **config) BaseTTSProvider?
+list_asr_providers() list
+list_llm_providers() list
+list_tts_providers() list
+get_provider_capabilities(name, type) ProviderCapability?
+discover_and_register()
+load_provider_module(module_path)
}
class ProviderCapability {
+supports_streaming_input : bool
+supports_streaming_output : bool
+supports_word_timestamps : bool
+supports_voices : bool
+supports_interruptible_generation : bool
+preferred_sample_rates : list
+supported_codecs : list
}
ProviderRegistry --> ProviderCapability : "returns"
```

**Diagram sources**
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [capability.py:7-61](file://py/provider_gateway/app/core/capability.py#L7-L61)

**Section sources**
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [capability.py:7-61](file://py/provider_gateway/app/core/capability.py#L7-L61)

### Python Provider Base Classes and Implementations
Provider implementations inherit from base classes and expose capabilities and streaming operations. Example providers include:
- ASR: Faster Whisper with streaming input/output, word timestamps, and cancellation
- LLM: Groq adapter with streaming output and error normalization
- TTS: XTTS stub indicating server requirement

```mermaid
classDiagram
class BaseASRProvider {
<<abstract>>
+stream_recognize(audio_stream, options) AsyncIterator[ASRResponse]
+cancel(session_id) bool
+capabilities() ProviderCapability
+name() string
}
class BaseLLMProvider {
<<abstract>>
+stream_generate(messages, options) AsyncIterator[LLMResponse]
+cancel(session_id) bool
+capabilities() ProviderCapability
+name() string
}
class BaseTTSProvider {
<<abstract>>
+stream_synthesize(text, options) AsyncIterator[TTSResponse]
+cancel(session_id) bool
+capabilities() ProviderCapability
+name() string
}
class FasterWhisperProvider {
+stream_recognize(...)
+cancel(session_id) bool
+capabilities() ProviderCapability
+name() string
}
class GroqProvider {
+stream_generate(...)
+capabilities() ProviderCapability
+name() string
}
class XTTSProvider {
+stream_synthesize(...)
+cancel(session_id) bool
+capabilities() ProviderCapability
+name() string
}
BaseASRProvider <|-- FasterWhisperProvider
BaseLLMProvider <|-- GroqProvider
BaseTTSProvider <|-- XTTSProvider
```

**Diagram sources**
- [base_provider.py:39-177](file://py/provider_gateway/app/core/base_provider.py#L39-L177)
- [faster_whisper.py:15-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L15-L262)
- [groq.py:16-124](file://py/provider_gateway/app/providers/llm/groq.py#L16-L124)
- [xtts.py:14-106](file://py/provider_gateway/app/providers/tts/xtts.py#L14-L106)

**Section sources**
- [base_provider.py:12-177](file://py/provider_gateway/app/core/base_provider.py#L12-L177)
- [faster_whisper.py:15-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L15-L262)
- [groq.py:16-124](file://py/provider_gateway/app/providers/llm/groq.py#L16-L124)
- [xtts.py:14-106](file://py/provider_gateway/app/providers/tts/xtts.py#L14-L106)

### Provider Discovery and Registration Flow
Dynamic discovery in the Python registry imports provider modules and invokes their registration functions. Factories are stored and later instantiated on demand with configuration.

```mermaid
flowchart TD
Start(["Startup"]) --> Discover["discover_and_register()"]
Discover --> ImportASR["Import ASR register_providers"]
Discover --> ImportLLM["Import LLM register_providers"]
Discover --> ImportTTS["Import TTS register_providers"]
ImportASR --> RegisterASR["register_asr(name, factory)"]
ImportLLM --> RegisterLLM["register_llm(name, factory)"]
ImportTTS --> RegisterTTS["register_tts(name, factory)"]
RegisterASR --> Ready["Registry Ready"]
RegisterLLM --> Ready
RegisterTTS --> Ready
```

**Diagram sources**
- [registry.py:206-241](file://py/provider_gateway/app/core/registry.py#L206-L241)

**Section sources**
- [registry.py:206-241](file://py/provider_gateway/app/core/registry.py#L206-L241)

### Provider Capability-Based Selection
The Go registry’s resolution logic applies a strict priority order and validates provider availability before returning selections. The Python provider service surfaces capabilities for discovery and health checks.

```mermaid
flowchart TD
Sess["Session Start"] --> Resolve["ResolveForSession()"]
Resolve --> Global["Apply Global Defaults"]
Global --> Tenant["Apply Tenant Overrides"]
Tenant --> Session["Apply Session Overrides"]
Session --> Request["Apply Request Overrides"]
Request --> Validate{"Validate Providers Exist"}
Validate --> |OK| Selected["Return SelectedProviders"]
Validate --> |Fail| Error["Return Error"]
```

**Diagram sources**
- [registry.go:172-251](file://go/pkg/providers/registry.go#L172-L251)

**Section sources**
- [registry.go:172-251](file://go/pkg/providers/registry.go#L172-L251)
- [provider_servicer.py:43-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L43-L190)

### Provider Lifecycle Management, Health Checking, and Failover
- Lifecycle: Providers are lazily instantiated and cached by registry keys. Cancellation is supported via provider interfaces and gRPC clients.
- Health: The Python provider service exposes a health check endpoint returning a serving status.
- Failover: The Go orchestrator can select alternate providers based on tenant/session/request overrides; gRPC clients can be extended to retry and route around failures.

```mermaid
sequenceDiagram
participant Client as "Client"
participant Gateway as "Provider Service (Python)"
participant Registry as "Registry (Python)"
Client->>Gateway : HealthCheck(service_name)
Gateway->>Registry : get_provider_capabilities(name, type)
Registry-->>Gateway : ProviderCapability or None
Gateway-->>Client : HealthCheckResponse(status, version)
```

**Diagram sources**
- [provider_servicer.py:170-187](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L170-L187)

**Section sources**
- [provider_servicer.py:170-187](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L170-L187)
- [grpc_client.go:62-247](file://go/pkg/providers/grpc_client.go#L62-L247)

### Examples: Registering Custom Providers, Querying Capabilities, Managing Instances
- Registering a custom provider in Python:
  - Implement a provider class inheriting from the appropriate base class.
  - Export a function named register_providers(registry) that calls register_asr/register_llm/register_tts with a factory.
  - Use registry.load_provider_module(module_path) or rely on discover_and_register().
- Querying provider capabilities:
  - Use ProviderServicer.GetProviderInfo to retrieve ProviderInfo including capabilities.
  - Use ProviderRegistry.get_provider_capabilities(name, type) in Python.
- Managing provider instances:
  - Use get_asr/get_llm/get_tts with configuration; instances are cached by a composite key of name and hashed config.
  - Access capabilities via ProviderCapability attributes.

**Section sources**
- [registry.py:40-181](file://py/provider_gateway/app/core/registry.py#L40-L181)
- [provider_servicer.py:141-168](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L141-L168)
- [capability.py:7-61](file://py/provider_gateway/app/core/capability.py#L7-L61)

## Dependency Analysis
The orchestrator depends on the Go provider registry and gRPC clients to coordinate provider operations. The registry integrates with the Python provider gateway through gRPC, which in turn relies on the Python registry and provider implementations.

```mermaid
graph TB
Orchestrator["Orchestrator (Go)<br/>cmd/main.go"] --> RegistryGo["Registry (Go)<br/>registry.go"]
Orchestrator --> GRPCClients["gRPC Clients (Go)<br/>grpc_client.go"]
GRPCClients --> ProviderGateway["Provider Gateway (Python)<br/>provider_servicer.py"]
ProviderGateway --> RegistryPy["Registry (Python)<br/>registry.py"]
RegistryPy --> Providers["Provider Implementations<br/>faster_whisper.py, groq.py, xtts.py"]
```

**Diagram sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [registry.go:14-40](file://go/pkg/providers/registry.go#L14-L40)
- [grpc_client.go:35-288](file://go/pkg/providers/grpc_client.go#L35-L288)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)
- [faster_whisper.py:15-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L15-L262)
- [groq.py:16-124](file://py/provider_gateway/app/providers/llm/groq.py#L16-L124)
- [xtts.py:14-106](file://py/provider_gateway/app/providers/tts/xtts.py#L14-L106)

**Section sources**
- [main.go:195-257](file://go/orchestrator/cmd/main.go#L195-L257)
- [registry.go:14-40](file://go/pkg/providers/registry.go#L14-L40)
- [grpc_client.go:35-288](file://go/pkg/providers/grpc_client.go#L35-L288)
- [provider_servicer.py:28-190](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L28-L190)
- [registry.py:19-287](file://py/provider_gateway/app/core/registry.py#L19-L287)

## Performance Considerations
- Streaming-first design: Both Go and Python providers expose streaming operations to minimize latency and memory overhead.
- Capability-aware routing: Use ProviderCapability to validate audio formats and streaming support before dispatching work.
- Instance caching: Python registry caches provider instances keyed by configuration to reduce initialization costs.
- Asynchronous processing: Python providers offload heavy operations to thread pools to keep async streams responsive.
- Observability: Enable metrics and tracing in both Go and Python services to monitor latency, throughput, and error rates.

[No sources needed since this section provides general guidance]

## Troubleshooting Guide
Common issues and remedies:
- Provider not found during resolution:
  - Verify provider registration and that names match exactly.
  - Confirm tenant/session/request overrides do not reference missing providers.
- Capability mismatch:
  - Ensure audio formats and streaming flags align with provider capabilities.
- Health check failures:
  - Confirm the provider gateway is reachable and serving.
- Cancellation not working:
  - Ensure providers implement cancel and that session IDs are propagated correctly.
- gRPC connectivity:
  - Validate gRPC client configuration (address, timeouts, retries) and network access.

**Section sources**
- [registry.go:234-250](file://go/pkg/providers/registry.go#L234-L250)
- [provider_servicer.py:170-187](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L170-L187)
- [grpc_client.go:14-33](file://go/pkg/providers/grpc_client.go#L14-L33)

## Conclusion
The Provider Registry System cleanly separates provider implementations from orchestration logic, enabling dynamic registration, capability-driven selection, and scalable operation across ASR, LLM, and TTS domains. By leveraging streaming interfaces, capability models, and robust health and cancellation mechanisms, the system supports resilient, observable, and extensible AI service orchestration.

[No sources needed since this section summarizes without analyzing specific files]

## Appendices

### Provider Selection Hierarchy and Configuration
- Priority order: request-level overrides → session-level → tenant-level → global defaults
- Example configuration demonstrates default provider selection and per-provider settings.

**Section sources**
- [registry.go:172-251](file://go/pkg/providers/registry.go#L172-L251)
- [config-cloud.yaml:12-31](file://examples/config-cloud.yaml#L12-L31)

### Example Workflows

#### Provider Registration Workflow (Python)
```mermaid
flowchart TD
A["Implement Provider Class"] --> B["Export register_providers(registry)"]
B --> C["Call registry.register_*('name', factory)"]
C --> D["Optionally call discover_and_register()"]
D --> E["Providers Available via get_*()"]
```

**Diagram sources**
- [registry.py:40-84](file://py/provider_gateway/app/core/registry.py#L40-L84)
- [faster_whisper.py:256-262](file://py/provider_gateway/app/providers/asr/faster_whisper.py#L256-L262)

#### Capability Query Workflow (Python)
```mermaid
sequenceDiagram
participant Client as "Client"
participant Service as "ProviderServicer"
participant Registry as "Registry"
Client->>Service : GetProviderInfo(name, type)
Service->>Registry : get_provider_capabilities(name, type)
Registry-->>Service : ProviderCapability
Service-->>Client : ProviderInfo with capabilities
```

**Diagram sources**
- [provider_servicer.py:141-168](file://py/provider_gateway/app/grpc_api/provider_servicer.py#L141-L168)
- [registry.py:182-204](file://py/provider_gateway/app/core/registry.py#L182-L204)