# Configuration Reference

## Overview

CloudApp uses YAML configuration files with environment variable overrides. All configuration options are documented below.

## Configuration Loading

### File Location

Configuration is loaded from:

1. Path specified by `--config` flag
2. Path from `CLOUDAPP_CONFIG` environment variable
3. Default configuration (embedded)

```bash
# Via flag
./media-edge --config /path/to/config.yaml

# Via environment variable
export CLOUDAPP_CONFIG=/path/to/config.yaml
./media-edge
```

### Environment Variable Overrides

All configuration values can be overridden via environment variables with the `CLOUDAPP_` prefix:

```bash
# Override server port
export CLOUDAPP_SERVER_PORT=9090

# Override Redis address
export CLOUDAPP_REDIS_ADDR=redis.example.com:6379

# Override provider defaults
export CLOUDAPP_PROVIDERS_DEFAULTS_ASR=google_speech
export CLOUDAPP_PROVIDERS_DEFAULTS_LLM=groq
```

Environment variable format:
- Prefix: `CLOUDAPP_`
- Section separator: `_`
- Nested keys use `__` (double underscore) for provider configs

## Configuration Sections

### Server

HTTP and WebSocket server settings.

```yaml
server:
  host: "0.0.0.0"           # Bind address
  port: 8080                # HTTP/WebSocket port
  ws_path: "/ws"            # WebSocket endpoint path
  read_timeout: 30s         # Read timeout
  write_timeout: 30s        # Write timeout
  max_connections: 1000     # Maximum concurrent connections
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | string | `"0.0.0.0"` | Server bind address |
| `port` | int | `8080` | HTTP/WebSocket port |
| `ws_path` | string | `"/ws"` | WebSocket endpoint path |
| `read_timeout` | duration | `30s` | HTTP read timeout |
| `write_timeout` | duration | `30s` | HTTP write timeout |
| `max_connections` | int | `1000` | Max concurrent WebSocket connections |

**Environment Variables:**
- `CLOUDAPP_SERVER_HOST`
- `CLOUDAPP_SERVER_PORT`
- `CLOUDAPP_SERVER_WS_PATH`
- `CLOUDAPP_SERVER_READ_TIMEOUT`
- `CLOUDAPP_SERVER_WRITE_TIMEOUT`
- `CLOUDAPP_SERVER_MAX_CONNECTIONS`

### Redis

Redis connection for session storage and caching.

```yaml
redis:
  address: "localhost:6379"   # Redis server address
  password: ""                # Redis password (empty for no auth)
  db: 0                       # Redis database number
  key_prefix: "cloudapp:"     # Key prefix for all keys
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `address` | string | `"localhost:6379"` | Redis server address |
| `password` | string | `""` | Authentication password |
| `db` | int | `0` | Database number (0-15) |
| `key_prefix` | string | `"cloudapp:"` | Prefix for all Redis keys |

**Environment Variables:**
- `CLOUDAPP_REDIS_ADDR`
- `CLOUDAPP_REDIS_PASSWORD`
- `CLOUDAPP_REDIS_DB`
- `CLOUDAPP_REDIS_KEY_PREFIX`

### PostgreSQL

PostgreSQL connection for persistent storage.

```yaml
postgres:
  dsn: "postgres://voiceengine:voiceengine@localhost:5432/voiceengine?sslmode=disable"
  max_open_conns: 25          # Maximum open connections
  max_idle_conns: 5           # Maximum idle connections
  conn_max_lifetime: 5m       # Connection max lifetime
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `dsn` | string | `"postgres://localhost/cloudapp?sslmode=disable"` | Connection string |
| `max_open_conns` | int | `25` | Max open connections |
| `max_idle_conns` | int | `5` | Max idle connections |
| `conn_max_lifetime` | duration | `5m` | Connection max lifetime |

**Environment Variables:**
- `CLOUDAPP_POSTGRES_DSN`
- `CLOUDAPP_POSTGRES_MAX_OPEN_CONNS`
- `CLOUDAPP_POSTGRES_MAX_IDLE_CONNS`
- `CLOUDAPP_POSTGRES_CONN_MAX_LIFETIME`

### Providers

Provider selection and configuration.

```yaml
providers:
  defaults:
    asr: "mock"           # Default ASR provider
    llm: "mock"           # Default LLM provider
    tts: "mock"           # Default TTS provider
    vad: "mock"           # Default VAD provider

  # ASR provider configurations
  asr:
    faster_whisper:
      model_size: "base"
      device: "cpu"
      compute_type: "int8"
    google_speech:
      credentials_path: "/path/to/credentials.json"
      project_id: "my-project"
      language_code: "en-US"
      use_enhanced: "true"
      model: "latest_long"

  # LLM provider configurations
  llm:
    groq:
      api_key: "${GROQ_API_KEY}"
      model: "llama3-70b-8192"
      max_tokens: "1024"
      temperature: "0.7"
    openai_compatible:
      base_url: "http://localhost:8000/v1"
      api_key: ""
      model: "default"

  # TTS provider configurations
  tts:
    google_tts:
      credentials_path: "/path/to/credentials.json"
      project_id: "my-project"
      voice_name: "en-US-Neural2-F"
      speaking_rate: "1.0"
      pitch: "0.0"
    xtts:
      model_path: "/path/to/xtts/model"
      device: "cuda"

  # VAD provider configurations
  vad:
    energy:
      threshold: "0.01"
      min_speech_duration_ms: "250"
      min_silence_duration_ms: "500"
```

#### Provider Defaults

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `defaults.asr` | string | `"mock"` | Default ASR provider |
| `defaults.llm` | string | `"mock"` | Default LLM provider |
| `defaults.tts` | string | `"mock"` | Default TTS provider |
| `defaults.vad` | string | `"mock"` | Default VAD provider |

**Environment Variables:**
- `CLOUDAPP_PROVIDERS_DEFAULTS_ASR`
- `CLOUDAPP_PROVIDERS_DEFAULTS_LLM`
- `CLOUDAPP_PROVIDERS_DEFAULTS_TTS`
- `CLOUDAPP_PROVIDERS_DEFAULTS_VAD`

#### Provider-Specific Config

Provider configurations are passed directly to provider factories as keyword arguments. Each provider defines its own configuration schema.

**ASR Providers:**

| Provider | Config Options |
|----------|----------------|
| `mock` | `transcript`, `chunk_delay_ms`, `confidence` |
| `faster_whisper` | `model_size`, `device`, `compute_type` |
| `google_speech` | `credentials_path`, `project_id`, `language_code`, `use_enhanced`, `model` |

**LLM Providers:**

| Provider | Config Options |
|----------|----------------|
| `mock` | `response`, `chunk_delay_ms` |
| `groq` | `api_key`, `model`, `max_tokens`, `temperature` |
| `openai_compatible` | `base_url`, `api_key`, `model`, `timeout` |

**TTS Providers:**

| Provider | Config Options |
|----------|----------------|
| `mock` | `chunk_delay_ms`, `frequency` |
| `google_tts` | `credentials_path`, `project_id`, `voice_name`, `speaking_rate`, `pitch` |
| `xtts` | `model_path`, `device`, `language` |

### Audio

Audio format profiles.

```yaml
audio:
  default_input_profile: "telephony"   # Default input profile
  default_output_profile: "telephony"  # Default output profile

  profiles:
    telephony:
      sample_rate: 16000
      channels: 1
      encoding: "pcm16"

    telephony8k:
      sample_rate: 8000
      channels: 1
      encoding: "pcm16"

    webrtc:
      sample_rate: 48000
      channels: 1
      encoding: "pcm16"

    internal:
      sample_rate: 16000
      channels: 1
      encoding: "pcm16"
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `default_input_profile` | string | `"telephony"` | Default profile for input audio |
| `default_output_profile` | string | `"telephony"` | Default profile for output audio |

**Profile Options:**

| Option | Type | Description |
|--------|------|-------------|
| `sample_rate` | int | Sample rate in Hz (8000, 16000, 24000, 48000) |
| `channels` | int | Channel count (1 for mono, 2 for stereo) |
| `encoding` | string | Audio encoding (`"pcm16"`, `"opus"`, `"g711_ulaw"`, `"g711_alaw"`) |

**Environment Variables:**
- `CLOUDAPP_AUDIO_DEFAULT_INPUT_PROFILE`
- `CLOUDAPP_AUDIO_DEFAULT_OUTPUT_PROFILE`

### Observability

Logging, metrics, and tracing configuration.

```yaml
observability:
  log_level: "info"           # Log level: debug, info, warn, error
  log_format: "json"          # Log format: json, console
  metrics_port: 9090          # Prometheus metrics port
  otel_endpoint: "localhost:4317"  # OpenTelemetry collector endpoint
  enable_tracing: true        # Enable distributed tracing
  enable_metrics: true        # Enable Prometheus metrics
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `log_level` | string | `"info"` | Log level (`debug`, `info`, `warn`, `error`) |
| `log_format` | string | `"json"` | Log format (`json`, `console`) |
| `metrics_port` | int | `9090` | Prometheus metrics HTTP port |
| `otel_endpoint` | string | `"localhost:4317"` | OpenTelemetry OTLP endpoint |
| `enable_tracing` | bool | `true` | Enable distributed tracing |
| `enable_metrics` | bool | `true` | Enable Prometheus metrics |

**Environment Variables:**
- `CLOUDAPP_OBSERVABILITY_LOG_LEVEL`
- `CLOUDAPP_OBSERVABILITY_LOG_FORMAT`
- `CLOUDAPP_OBSERVABILITY_METRICS_PORT`
- `CLOUDAPP_OBSERVABILITY_OTEL_ENDPOINT`
- `CLOUDAPP_OBSERVABILITY_ENABLE_TRACING`
- `CLOUDAPP_OBSERVABILITY_ENABLE_METRICS`

### Security

Security and authentication settings.

```yaml
security:
  max_session_duration: 1h      # Maximum session duration
  max_chunk_size: 65536         # Maximum WebSocket message size (bytes)
  auth_enabled: false           # Enable authentication
  auth_token: ""                # Static auth token (if auth_enabled)
  allowed_origins:              # Allowed CORS origins
    - "*"
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_session_duration` | duration | `1h` | Maximum session duration |
| `max_chunk_size` | int | `65536` | Max WebSocket message size (64KB) |
| `auth_enabled` | bool | `false` | Enable bearer token authentication |
| `auth_token` | string | `""` | Static bearer token (if auth enabled) |
| `allowed_origins` | list | `["*"]` | Allowed CORS origins |

**Environment Variables:**
- `CLOUDAPP_SECURITY_MAX_SESSION_DURATION`
- `CLOUDAPP_SECURITY_MAX_CHUNK_SIZE`
- `CLOUDAPP_SECURITY_AUTH_ENABLED`
- `CLOUDAPP_SECURITY_AUTH_TOKEN`
- `CLOUDAPP_SECURITY_ALLOWED_ORIGINS` (comma-separated)

## Example Configurations

### Development (Mock Mode)

See [examples/config-mock.yaml](../examples/config-mock.yaml):

```yaml
server:
  host: "0.0.0.0"
  port: 8080

redis:
  address: "localhost:6379"

providers:
  defaults:
    asr: "mock"
    llm: "mock"
    tts: "mock"

observability:
  log_level: "debug"
  log_format: "console"

security:
  auth_enabled: false
```

### Production (Cloud Providers)

See [examples/config-cloud.yaml](../examples/config-cloud.yaml):

```yaml
server:
  host: "0.0.0.0"
  port: 8080

providers:
  defaults:
    asr: "google_speech"
    llm: "groq"
    tts: "google_tts"

  asr:
    google_speech:
      credentials_path: "/secrets/google-credentials.json"
      project_id: "my-project"

  llm:
    groq:
      api_key: "${GROQ_API_KEY}"
      model: "llama3-70b-8192"

observability:
  log_level: "warn"
  enable_tracing: true
  enable_metrics: true

security:
  auth_enabled: true
  auth_token: "${CLOUDAPP_AUTH_TOKEN}"
  max_session_duration: 30m
```

### Local vLLM Inference

See [examples/config-vllm.yaml](../examples/config-vllm.yaml):

```yaml
providers:
  defaults:
    asr: "faster_whisper"
    llm: "openai_compatible"
    tts: "mock"

  llm:
    openai_compatible:
      base_url: "http://localhost:8000/v1"
      model: "meta-llama/Meta-Llama-3-8B-Instruct"

  asr:
    faster_whisper:
      model_size: "base"
      device: "cuda"
```

## Configuration Validation

Configuration is validated on startup. Invalid configuration will cause the service to exit with an error message.

### Validation Rules

- `server.port` must be between 1 and 65535
- `redis.db` must be between 0 and 15
- `audio.profiles.*.sample_rate` must be 8000, 16000, 24000, or 48000
- `security.max_chunk_size` must be at least 1024
- `observability.log_level` must be one of: `debug`, `info`, `warn`, `error`

## Provider Gateway Configuration

The Python provider-gateway has its own configuration system using Pydantic Settings:

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PROVIDER_GATEWAY_CONFIG` | Path to YAML config file | - |
| `PROVIDER_GATEWAY_SERVER__HOST` | gRPC server host | `0.0.0.0` |
| `PROVIDER_GATEWAY_SERVER__PORT` | gRPC server port | `50051` |
| `PROVIDER_GATEWAY_TELEMETRY__LOG_LEVEL` | Log level | `INFO` |
| `PROVIDER_GATEWAY_TELEMETRY__METRICS_PORT` | Metrics port | `9090` |
| `PROVIDER_GATEWAY_PROVIDERS__ASR_DEFAULT` | Default ASR provider | `mock` |
| `PROVIDER_GATEWAY_PROVIDERS__LLM_DEFAULT` | Default LLM provider | `mock` |
| `PROVIDER_GATEWAY_PROVIDERS__TTS_DEFAULT` | Default TTS provider | `mock` |

Note: Provider-gateway uses `__` (double underscore) as nested key separator.
