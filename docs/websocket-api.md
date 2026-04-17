# WebSocket API Reference

## Overview

The WebSocket API is the primary interface for real-time voice interactions. Clients connect via WebSocket and exchange JSON messages for control and base64-encoded audio data.

## Connection

```
ws://host:8080/ws
```

### Connection Headers

| Header | Description |
|--------|-------------|
| `Origin` | Validated against `security.allowed_origins` config |
| `Authorization` | Bearer token (if `security.auth_enabled` is true) |

## Message Format

All messages are JSON with a `type` field identifying the message type. Audio data is base64-encoded within JSON fields.

## Client → Server Messages

### `session.start`

Initiates a new voice session.

```json
{
  "type": "session.start",
  "timestamp": 1704067200000,
  "session_id": "sess_abc123",
  "audio_profile": {
    "sample_rate": 16000,
    "channels": 1,
    "encoding": "pcm16"
  },
  "voice_profile": {
    "voice_id": "en-US-Standard-C",
    "speed": 1.0,
    "pitch": 0.0
  },
  "providers": {
    "asr": "mock",
    "llm": "mock",
    "tts": "mock",
    "vad": "mock"
  },
  "system_prompt": "You are a helpful voice assistant. Be concise and friendly.",
  "model_options": {
    "model_name": "gpt-4",
    "max_tokens": 150,
    "temperature": 0.7,
    "top_p": 1.0,
    "stop_sequences": []
  },
  "tenant_id": "tenant_123"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"session.start"` |
| `timestamp` | int64 | No | Unix timestamp in milliseconds |
| `session_id` | string | No | Auto-generated if not provided |
| `audio_profile` | object | Yes | Audio format configuration |
| `audio_profile.sample_rate` | int | Yes | Sample rate (8000 or 16000) |
| `audio_profile.channels` | int | Yes | Channel count (1 for mono) |
| `audio_profile.encoding` | string | Yes | `"pcm16"`, `"opus"`, `"g711_ulaw"`, `"g711_alaw"` |
| `voice_profile` | object | No | TTS voice configuration |
| `voice_profile.voice_id` | string | No | Voice identifier |
| `voice_profile.speed` | float | No | Speaking speed (0.5-2.0) |
| `voice_profile.pitch` | float | No | Pitch adjustment (-10 to 10) |
| `providers` | object | No | Provider selection |
| `providers.asr` | string | No | ASR provider name |
| `providers.llm` | string | No | LLM provider name |
| `providers.tts` | string | No | TTS provider name |
| `providers.vad` | string | No | VAD provider name |
| `system_prompt` | string | No | System prompt for LLM |
| `model_options` | object | No | LLM generation options |
| `tenant_id` | string | No | Tenant identifier for multi-tenancy |

### `audio.chunk`

Sends audio data to the server.

```json
{
  "type": "audio.chunk",
  "timestamp": 1704067200100,
  "session_id": "sess_abc123",
  "audio_data": "<base64-encoded-pcm16-audio>",
  "is_final": false,
  "sequence": 42
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"audio.chunk"` |
| `timestamp` | int64 | No | Unix timestamp in milliseconds |
| `session_id` | string | Yes | Session identifier |
| `audio_data` | string | Yes | Base64-encoded PCM16 audio |
| `is_final` | bool | No | True if this is the final chunk |
| `sequence` | int | No | Sequence number for ordering |

**Audio Format:**
- Encoding: PCM16 (signed 16-bit little-endian)
- Sample rate: Must match `audio_profile.sample_rate` from session.start
- Channels: Must match `audio_profile.channels` from session.start
- Chunk size: 10-100ms recommended (160-1600 samples at 16kHz)

### `session.update`

Updates session configuration mid-conversation.

```json
{
  "type": "session.update",
  "timestamp": 1704067201000,
  "session_id": "sess_abc123",
  "system_prompt": "You are now a pirate. Speak like one!",
  "voice_profile": {
    "voice_id": "en-US-Standard-D",
    "speed": 1.2
  },
  "model_options": {
    "temperature": 0.9
  },
  "providers": {
    "llm": "groq"
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"session.update"` |
| `timestamp` | int64 | No | Unix timestamp in milliseconds |
| `session_id` | string | Yes | Session identifier |
| `system_prompt` | string | No | New system prompt |
| `voice_profile` | object | No | Updated voice profile |
| `model_options` | object | No | Updated model options |
| `providers` | object | No | Updated provider selection |

### `session.interrupt`

Manually interrupts the current assistant response.

```json
{
  "type": "session.interrupt",
  "timestamp": 1704067202000,
  "session_id": "sess_abc123",
  "reason": "user_request"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"session.interrupt"` |
| `timestamp` | int64 | No | Unix timestamp in milliseconds |
| `session_id` | string | Yes | Session identifier |
| `reason` | string | No | Reason for interruption |

### `session.stop`

Ends the session.

```json
{
  "type": "session.stop",
  "timestamp": 1704067205000,
  "session_id": "sess_abc123",
  "reason": "user_disconnect"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"session.stop"` |
| `timestamp` | int64 | No | Unix timestamp in milliseconds |
| `session_id` | string | Yes | Session identifier |
| `reason` | string | No | Reason for stopping |

## Server → Client Messages

### `session.started`

Sent when a session is successfully started.

```json
{
  "type": "session.started",
  "timestamp": 1704067200050,
  "session_id": "sess_abc123",
  "audio_profile": {
    "sample_rate": 16000,
    "channels": 1,
    "encoding": "pcm16"
  },
  "server_time": 1704067200050
}
```

### `vad.event`

Voice Activity Detection events.

```json
{
  "type": "vad.event",
  "timestamp": 1704067201000,
  "session_id": "sess_abc123",
  "event": "speech_start",
  "confidence": 0.95,
  "position": 1000
}
```

```json
{
  "type": "vad.event",
  "timestamp": 1704067203500,
  "session_id": "sess_abc123",
  "event": "speech_end",
  "confidence": 0.92,
  "position": 3500
}
```

**Event Types:**
- `speech_start` — User started speaking
- `speech_end` — User stopped speaking

### `asr.partial`

Partial (interim) transcription result.

```json
{
  "type": "asr.partial",
  "timestamp": 1704067201500,
  "session_id": "sess_abc123",
  "transcript": "Hello, I need help with",
  "confidence": 0.85,
  "language": "en-US"
}
```

### `asr.final`

Final transcription result.

```json
{
  "type": "asr.final",
  "timestamp": 1704067203600,
  "session_id": "sess_abc123",
  "transcript": "Hello, I need help with my account.",
  "confidence": 0.94,
  "language": "en-US",
  "word_timestamps": [
    {"word": "Hello", "start_ms": 0, "end_ms": 350},
    {"word": "I", "start_ms": 400, "end_ms": 450},
    {"word": "need", "start_ms": 500, "end_ms": 700},
    {"word": "help", "start_ms": 750, "end_ms": 950},
    {"word": "with", "start_ms": 1000, "end_ms": 1150},
    {"word": "my", "start_ms": 1200, "end_ms": 1350},
    {"word": "account", "start_ms": 1400, "end_ms": 1800}
  ]
}
```

### `llm.partial_text`

Partial LLM token (streaming response).

```json
{
  "type": "llm.partial_text",
  "timestamp": 1704067204000,
  "session_id": "sess_abc123",
  "text": "I'd be happy",
  "is_complete": false
}
```

```json
{
  "type": "llm.partial_text",
  "timestamp": 1704067204200,
  "session_id": "sess_abc123",
  "text": " to help you with your account!",
  "is_complete": true
}
```

### `tts.audio_chunk`

Synthesized audio chunk.

```json
{
  "type": "tts.audio_chunk",
  "timestamp": 1704067204100,
  "session_id": "sess_abc123",
  "audio_data": "<base64-encoded-pcm16-audio>",
  "segment_index": 0,
  "is_final": false
}
```

```json
{
  "type": "tts.audio_chunk",
  "timestamp": 1704067204500,
  "session_id": "sess_abc123",
  "audio_data": "<base64-encoded-pcm16-audio>",
  "segment_index": 5,
  "is_final": true
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `audio_data` | string | Base64-encoded PCM16 audio |
| `segment_index` | int32 | Chunk sequence number |
| `is_final` | bool | True if this is the final chunk |

### `turn.event`

Turn state change events.

```json
{
  "type": "turn.event",
  "timestamp": 1704067203600,
  "session_id": "sess_abc123",
  "turn_type": "assistant",
  "event": "started",
  "generation_id": "gen_xyz789"
}
```

```json
{
  "type": "turn.event",
  "timestamp": 1704067204600,
  "session_id": "sess_abc123",
  "turn_type": "assistant",
  "event": "completed",
  "text": "I'd be happy to help you with your account!"
}
```

**Event Types:**
- `started` — Turn started
- `completed` — Turn completed successfully
- `cancelled` — Turn was interrupted/cancelled

### `interruption.event`

Sent when an interruption occurs (barge-in).

```json
{
  "type": "interruption.event",
  "timestamp": 1704067205000,
  "session_id": "sess_abc123",
  "reason": "user_speech",
  "spoken_text": "I'd be happy to help",
  "unspoken_text": " you with your account!",
  "playout_position_ms": 1200
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `reason` | string | Reason for interruption (`"user_speech"`, `"manual"`) |
| `spoken_text` | string | Text that was actually spoken before interruption |
| `unspoken_text` | string | Text that was generated but not spoken |
| `playout_position_ms` | int64 | Audio playout position in milliseconds |

### `error`

Error event.

```json
{
  "type": "error",
  "timestamp": 1704067206000,
  "session_id": "sess_abc123",
  "code": "PROVIDER_ERROR",
  "message": "ASR service unavailable",
  "details": "Connection timeout after 30s",
  "retriable": true
}
```

**Error Codes:**

| Code | Description | Retriable |
|------|-------------|-----------|
| `INVALID_MESSAGE` | Malformed message | No |
| `SESSION_NOT_FOUND` | Session doesn't exist | No |
| `SESSION_EXISTS` | Session already started | No |
| `PROVIDER_ERROR` | Provider service error | Depends |
| `RATE_LIMITED` | Rate limit exceeded | Yes |
| `AUTHENTICATION` | Authentication failed | No |
| `AUTHORIZATION` | Permission denied | No |
| `INTERNAL_ERROR` | Internal server error | Yes |

### `session.ended`

Sent when a session ends.

```json
{
  "type": "session.ended",
  "timestamp": 1704067205500,
  "session_id": "sess_abc123",
  "reason": "user_disconnect",
  "duration_ms": 5500
}
```

## REST Endpoints

In addition to WebSocket, the media-edge service exposes REST endpoints:

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok"
}
```

**Status Codes:**
- `200 OK` — Service is healthy

### GET /ready

Readiness probe for Kubernetes.

**Response:**
```json
{
  "status": "ready"
}
```

```json
{
  "status": "not_ready",
  "reason": "redis_unavailable"
}
```

**Status Codes:**
- `200 OK` — Service is ready
- `503 Service Unavailable` — Service is not ready

### GET /metrics

Prometheus metrics endpoint (if enabled).

**Response:** Prometheus exposition format

```
# HELP websocket_connections_active Current number of active WebSocket connections
# TYPE websocket_connections_active gauge
websocket_connections_active 42

# HELP session_duration_seconds Session duration in seconds
# TYPE session_duration_seconds histogram
session_duration_seconds_bucket{le="1"} 5
session_duration_seconds_bucket{le="5"} 25
session_duration_seconds_bucket{le="+Inf"} 30
```

## Example Session Flow

```
Client                                          Server
  |                                               |
  |─── session.start ────────────────────────────>|
  |                                               |
  |<── session.started ───────────────────────────|
  |                                               |
  |─── audio.chunk (user speaks "Hello") ────────>|
  |                                               |
  |<── vad.event (speech_start) ──────────────────|
  |                                               |
  |─── audio.chunk ──────────────────────────────>|
  |─── audio.chunk ──────────────────────────────>|
  |                                               |
  |<── asr.partial ("Hello") ─────────────────────|
  |<── asr.partial ("Hello, I need") ─────────────|
  |                                               |
  |─── audio.chunk (speech ends) ────────────────>|
  |                                               |
  |<── vad.event (speech_end) ────────────────────|
  |<── asr.final ("Hello, I need help") ──────────|
  |                                               |
  |<── turn.event (assistant started) ────────────|
  |<── llm.partial_text ("I") ────────────────────|
  |<── llm.partial_text (" can") ─────────────────|
  |<── llm.partial_text (" help!") ──────────────|
  |                                               |
  |<── tts.audio_chunk (audio data) ─────────────|
  |<── tts.audio_chunk (audio data) ─────────────|
  |<── tts.audio_chunk (final) ─────────────────|
  |                                               |
  |<── turn.event (assistant completed) ──────────|
  |                                               |
  |─── session.stop ─────────────────────────────>|
  |                                               |
  |<── session.ended ─────────────────────────────|
```

## Code Examples

### JavaScript Client

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  // Start session
  ws.send(JSON.stringify({
    type: 'session.start',
    audio_profile: {
      sample_rate: 16000,
      channels: 1,
      encoding: 'pcm16'
    },
    system_prompt: 'You are a helpful assistant.'
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  switch (msg.type) {
    case 'asr.final':
      console.log('User said:', msg.transcript);
      break;
    case 'tts.audio_chunk':
      playAudio(msg.audio_data);
      break;
    case 'error':
      console.error('Error:', msg.message);
      break;
  }
};

// Send audio
function sendAudioChunk(audioBytes) {
  const base64 = btoa(String.fromCharCode(...audioBytes));
  ws.send(JSON.stringify({
    type: 'audio.chunk',
    audio_data: base64
  }));
}
```

### Python Client

```python
import asyncio
import base64
import json
import websockets

async def client():
    async with websockets.connect('ws://localhost:8080/ws') as ws:
        # Start session
        await ws.send(json.dumps({
            'type': 'session.start',
            'audio_profile': {
                'sample_rate': 16000,
                'channels': 1,
                'encoding': 'pcm16'
            }
        }))

        # Receive responses
        async for message in ws:
            msg = json.loads(message)

            if msg['type'] == 'asr.final':
                print(f"User said: {msg['transcript']}")
            elif msg['type'] == 'tts.audio_chunk':
                audio = base64.b64decode(msg['audio_data'])
                play_audio(audio)
            elif msg['type'] == 'error':
                print(f"Error: {msg['message']}")

asyncio.run(client())
```
