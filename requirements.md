Build a production-oriented MVP of a real-time AI voice engine backend with a pluggable provider architecture for ASR, LLM, and TTS.

The system must be designed so we can easily swap providers and run experiments, for example:
- ASR: Faster-Whisper, Whisper API, Google Speech-to-Text
- LLM: vLLM with local Llama 3 8B, Groq API, OpenAI-compatible APIs
- TTS: XTTSv2, OpenVoice, Google Cloud TTS

We want a complete runnable app, not a toy script.

==================================================
1. PRODUCT GOAL
==================================================

Build the “black box” voice engine core that:
- accepts streaming audio input plus metadata
- performs VAD-aware turn-taking
- sends audio to ASR
- sends finalized user utterances to LLM
- streams LLM output into TTS incrementally
- returns streaming bot audio output
- supports interruption / barge-in
- tracks only actually spoken assistant text in conversation state
- supports both telephony and WebRTC/game profiles
- is architected for low latency and future scaling

The system should be suitable for two use cases:
1. SMB voice receptionist via SIP/telephony
2. Game/NPC voice backend via WebRTC/API

==================================================
2. HARD REQUIREMENTS
==================================================

Implement the system as a modular monorepo with:
- Go services for realtime orchestration and media/session handling
- Python services for inference providers and adapters
- protobuf/gRPC contracts between Go and Python services
- Redis for hot session state
- Postgres for durable session logs/transcripts/config
- Prometheus metrics
- OpenTelemetry tracing
- Docker-based local deployment
- docker-compose for local full-stack run
- optional Kubernetes manifests stub for future deployment

Must support pluggable providers for:
- ASR
- LLM
- TTS
- VAD optionally too

The app must be designed around interfaces/adapters, not hardcoded providers.

==================================================
3. ARCHITECTURE TO BUILD
==================================================

Create the following components.

A. go/media-edge
Responsibilities:
- websocket realtime audio API for clients
- accept audio chunks from client
- normalize audio into canonical internal PCM format
- apply VAD
- detect speech start / speech end
- detect interruption while bot is speaking
- stream audio frames to orchestrator
- stream synthesized audio back to client
- track outbound playout progress
- expose health and metrics endpoints

For MVP, websocket is enough.
Design it so SIP/WebRTC adapters can be added later.

B. go/orchestrator
Responsibilities:
- own session state machine
- coordinate ASR -> LLM -> TTS pipeline
- maintain session state in memory + Redis
- handle turn-taking
- assemble prompt context
- manage assistant pending text vs spoken text
- handle cancellation/interruption logic
- dispatch to chosen providers
- emit events to media-edge
- persist transcript/events to Postgres
- expose health and metrics endpoints

C. py/provider-gateway
Responsibilities:
- host pluggable provider implementations
- expose gRPC endpoints for ASR, LLM, TTS
- load provider config dynamically from environment or config files
- support switching providers per session or per request
- include mock providers for local testing
- include at least one concrete provider implementation in each category

D. infra
Responsibilities:
- docker-compose for all services
- Redis
- Postgres
- Prometheus
- Grafana optional
- migrations
- example env files
- seed config

==================================================
4. PLUGGABLE PROVIDER SYSTEM
==================================================

This is the most important design constraint.

Implement a provider abstraction so that each pipeline stage is swappable.

Define clear contracts for:
- ASRProvider
- LLMProvider
- TTSProvider
- optionally VADProvider

Requirements:
- provider chosen by config
- provider can be selected globally, per tenant, per session, or per request
- provider capabilities discoverable at runtime
- provider-specific parameters supported without polluting core interfaces
- provider errors normalized into shared error model
- each provider reports latency/cost/capability metadata

Examples of provider combos we want to support:
- ASR=faster_whisper, LLM=vllm_llama3, TTS=xttsv2
- ASR=google_stt, LLM=groq, TTS=google_tts
- ASR=mock, LLM=mock, TTS=mock

Create the code so new providers can be added by implementing interfaces and registering them.

==================================================
5. PROVIDER INTERFACES
==================================================

Design strong internal contracts.

ASR provider interface must support:
- streaming audio input
- partial transcripts
- final transcript event
- language hint
- timestamps if available
- cancellation
- sample rate metadata
- session metadata passthrough

LLM provider interface must support:
- prompt/context input
- streaming token output
- stop/cancel
- max tokens / temperature / top_p / stop sequences
- provider-specific options map
- usage metadata
- timing metadata

TTS provider interface must support:
- incremental text segment synthesis
- streaming audio chunks out
- voice selection
- audio format selection
- cancellation
- per-segment metadata
- timing metadata

Capability model should include things like:
- supports_streaming_input
- supports_streaming_output
- supports_word_timestamps
- supports_voices
- supports_interruptible_generation
- preferred_sample_rates
- supported_codecs

==================================================
6. INITIAL PROVIDERS TO IMPLEMENT
==================================================

Implement at least these providers:

ASR:
1. MockASRProvider
2. FasterWhisperProvider or a clean stub with real adapter structure
3. GoogleSpeechProvider stub/interface-ready implementation

LLM:
1. MockLLMProvider
2. OpenAICompatibleLLMProvider supporting:
   - local vLLM endpoint
   - Groq if OpenAI-compatible mode is available through config
3. GroqProvider dedicated adapter if needed

TTS:
1. MockTTSProvider
2. GoogleTTSProvider stub/interface-ready implementation
3. XTTSProvider stub/interface-ready implementation

Important:
- If a provider cannot be fully run locally without secrets or heavy model weights, still implement the adapter cleanly with configuration, request/response models, and graceful fallback.
- Mock providers must be fully functional for end-to-end local testing.

==================================================
7. REPO STRUCTURE
==================================================

Create a clean monorepo like this:

/README.md
/docs/
/proto/
/go/
  /media-edge/
  /orchestrator/
  /pkg/
    /contracts/
    /session/
    /audio/
    /events/
    /providers/
    /config/
    /observability/
/py/
  /provider_gateway/
    /app/
      /providers/
        /asr/
        /llm/
        /tts/
        /vad/
      /grpc_api/
      /core/
      /config/
      /models/
      /telemetry/
      /tests/
/infra/
  /docker/
  /compose/
  /k8s/
  /prometheus/
  /migrations/
/scripts/
/examples/

==================================================
8. SESSION AND STATE MODEL
==================================================

Implement robust session state.

Each session should include:
- session_id
- tenant_id optional
- transport_type
- selected providers
- audio profile
- voice profile
- system prompt id or raw prompt
- model options
- active turn state
- bot speaking flag
- interruption flag
- timestamps
- trace metadata

For each assistant turn track separately:
- generated_text
- queued_for_tts_text
- spoken_text
- interrupted boolean
- playout cursor
- generation_id

Critical rule:
Only spoken_text may be committed into final dialogue history.
Never commit unspoken generated tail after interruption.

Use Redis for hot state and Postgres for durable history.

==================================================
9. INTERRUPTION / BARGE-IN LOGIC
==================================================

Implement real interruption flow.

Behavior:
- if user starts speaking while bot is speaking, detect interruption
- immediately stop outbound playout
- cancel active LLM generation if still running
- cancel active TTS synthesis
- trim assistant turn so only actually spoken text is committed
- start processing new user utterance cleanly

Model this with:
- generation_id
- explicit cancel signals
- idempotent cancellation
- playout tracking
- assistant pending vs spoken buffers

Include tests for this.

==================================================
10. AUDIO AND TRANSPORT
==================================================

Internal canonical format:
- PCM16 mono 16kHz for internal ASR/VAD pipeline

Support input/output profiles:
1. telephony profile:
   - 8kHz or 16kHz mono
   - G.711-compatible conceptually
   - for MVP use PCM transport if codec implementation complicates things, but structure code for codec adapters
2. gamedev/webrtc profile:
   - 48kHz mono
   - PCM/Opus-ready abstraction
   - for MVP websocket PCM is acceptable

Implement:
- audio normalization
- resampling abstraction
- chunking
- basic jitter-safe buffering abstraction
- playout cursor accounting

Do not overcomplicate codec implementation in MVP, but structure it so RTP/WebRTC/SIP modules can be added later.

==================================================
11. API TO IMPLEMENT
==================================================

Implement websocket realtime API.

Client -> media-edge messages:
- session.start
- audio.chunk
- session.update
- session.interrupt optional
- session.stop

Server -> client messages:
- session.started
- vad.event
- asr.partial
- asr.final
- llm.partial_text optional
- tts.audio_chunk
- turn.event
- interruption.event
- error
- session.ended

Use JSON for control events and binary or base64 for audio chunks.
Keep API simple but well documented.

Also implement:
- REST health endpoints
- REST readiness endpoints
- metrics endpoint

==================================================
12. gRPC CONTRACTS
==================================================

Define protobuf contracts for:
- ASR streaming
- LLM streaming
- TTS streaming
- provider metadata/capabilities
- cancellation
- health
- worker info

Design them carefully.
Include:
- session_id
- turn_id
- generation_id
- tenant_id optional
- trace_id
- timestamps
- options map
- provider name
- model name

Generate code for Go and Python.

==================================================
13. OBSERVABILITY
==================================================

Instrument everything.

Implement:
- structured JSON logs
- Prometheus metrics
- OpenTelemetry spans

Must track timestamps for:
- vad_end
- asr_final
- llm_dispatch
- llm_first_token
- first_speakable_segment
- tts_dispatch
- tts_first_chunk
- first_audio_sent
- interruption_detected
- llm_cancel_ack
- tts_cancel_ack

Expose metrics like:
- sessions_active
- turns_total
- asr_latency_ms
- llm_ttft_ms
- tts_first_chunk_ms
- server_ttfa_ms
- interruption_stop_ms
- provider_errors_total
- provider_requests_total
- provider_request_duration_ms
- websocket_connections_active

==================================================
14. CONFIGURATION
==================================================

Implement configuration with:
- environment variables
- YAML config file support
- per-provider config sections
- per-tenant overrides
- sensible defaults

Need config examples for:
A. local mock mode
B. local vLLM + mock ASR/TTS
C. google ASR/TTS + groq LLM
D. all-local experimental mode

Provide config schema and examples.

==================================================
15. SECURITY AND RESILIENCE
==================================================

Implement MVP-grade safety and reliability:
- input validation
- timeouts
- bounded queues
- backpressure handling
- max session duration
- max audio chunk size
- auth hook placeholder for tenants/api keys
- graceful shutdown
- worker drain behavior
- retry only where safe
- circuit breaker or basic fail-fast logic for providers
- normalized error responses

Do not add enterprise auth complexity, but leave extension points.

==================================================
16. TESTING REQUIREMENTS
==================================================

Create substantial tests, not minimal ones.

Need:
- unit tests for provider registry
- unit tests for session state machine
- unit tests for interruption trimming logic
- unit tests for prompt assembly
- unit tests for audio profile conversion abstraction
- unit tests for config loading
- integration tests for mock end-to-end pipeline
- integration tests for websocket session flow
- integration tests for provider switching
- integration tests for interruption during bot speech
- golden tests for state commit behavior

Must include a local test mode where everything runs with mock providers and deterministic outputs.

==================================================
17. DEMO / LOCAL RUN EXPERIENCE
==================================================

Make it easy to run.

Provide:
- docker-compose up path
- one command to run local dev mode
- one example websocket client script
- one example CLI script that simulates a session
- example config for switching providers
- seed prompts and voices
- sample transcript logs

Local demo should work even with only mock providers.

==================================================
18. IMPLEMENTATION STYLE
==================================================

Engineering requirements:
- production-quality structure
- clean architecture
- readable code
- avoid overengineering where unnecessary
- use interfaces and dependency injection
- keep core orchestration provider-agnostic
- use comments only where helpful
- strong typing
- error wrapping
- idempotent state transitions
- deterministic tests where possible

For Go:
- idiomatic Go
- context propagation everywhere
- small packages with clear responsibilities

For Python:
- FastAPI or gRPC server as appropriate
- pydantic/dataclasses for config and models
- provider classes with shared base abstractions
- asyncio where it makes sense

==================================================
19. IMPLEMENTATION PRIORITIES
==================================================

Build in this order:

Phase 1:
- monorepo
- protobuf contracts
- mock providers
- websocket media-edge
- orchestrator
- Redis/Postgres integration
- basic end-to-end mock pipeline

Phase 2:
- provider registry
- openai-compatible LLM provider
- faster-whisper adapter scaffold
- google ASR/TTS adapter scaffold
- metrics and tracing
- interruption trimming

Phase 3:
- load-test-friendly improvements
- better audio format abstraction
- docker polish
- docs
- example configs
- kubernetes stubs

==================================================
20. DOCUMENTATION TO WRITE
==================================================

Write:
- top-level README with architecture overview
- quickstart
- provider architecture guide
- how to add a new ASR provider
- how to add a new LLM provider
- how to add a new TTS provider
- websocket API docs
- config reference
- session/interruption behavior doc
- deployment notes
- testing guide

==================================================
21. DELIVERABLE FORMAT
==================================================

Produce:
1. Full codebase
2. Protobuf files
3. Dockerfiles
4. docker-compose.yml
5. README and docs
6. example configs
7. migrations
8. test suite
9. sample client
10. notes on what is fully implemented vs scaffolded

When done, provide:
- final repo tree
- setup instructions
- assumptions
- limitations
- next recommended production steps

==================================================
22. IMPORTANT PRODUCT/ARCHITECTURE CONSTRAINTS
==================================================

Do not hardwire the system to a single vendor.
Do not let provider-specific SDK code leak into orchestration logic.
Do not commit unspoken assistant text into session memory.
Do not make the only path depend on external cloud credentials.
Do not make the mock mode unrealistic; it must be useful for real end-to-end local development.
Do not collapse everything into one service.

==================================================
23. OPTIONAL STRETCH GOALS
==================================================

If reasonable, include:
- provider capability discovery endpoint
- admin endpoint to list available providers
- per-session provider override
- feature flagging for experiments
- A/B routing hooks
- benchmark script for latency tracing
- basic Grafana dashboard JSON
- Kubernetes manifests for future GPU worker deployment

==================================================
24. START WORK
==================================================

Start by:
1. designing the protobuf contracts
2. creating the monorepo structure
3. implementing mock providers
4. implementing orchestrator session state machine
5. implementing websocket media-edge
6. wiring end-to-end mock flow
7. then adding real provider adapters/scaffolds
8. then adding infra/docs/tests

Be explicit in code comments and README where something is production-ready MVP versus scaffold/stub.

Return the implementation as if you are creating the repository from scratch.