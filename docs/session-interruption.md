# Session Interruption (Barge-in)

## Overview

Session interruption (also called "barge-in") allows users to interrupt the AI assistant while it's speaking. This is a critical feature for natural voice conversations.

## Session Lifecycle

```
┌─────────┐     session.start      ┌─────────┐
│  Idle   │───────────────────────►│ Active  │
└─────────┘                        └────┬────┘
     ▲                                  │
     │                                  │
     │ session.ended                    │
     │ or disconnect                    │
     │                                  │
     └──────────────────────────────────┘
```

Sessions progress through states during their lifetime:

1. **Created** — Session object initialized
2. **Active** — WebSocket connected, audio flowing
3. **Ended** — Session stopped or connection closed

## Session State Machine

Within an active session, the conversation follows a state machine:

```
                    ┌──────────┐
         ┌─────────►│  Idle    │◄────────┐
         │          └────┬─────┘         │
         │               │               │
         │    audio      │    session    │
         │    received   │    ended      │
         │               ▼               │
         │          ┌──────────┐         │
         │          │Listening │─────────┘
         │          └────┬─────┘  interruption
         │               │
         │    speech     │
         │    ended      │
         │               ▼
         │          ┌──────────┐
         │          │Processing│
         │          └────┬─────┘
         │               │
         │    LLM        │
         │    token      │
         │               ▼
         │          ┌──────────┐
         │          │ Speaking │◄──────┐
         │          └────┬─────┘       │
         │               │             │
         │    TTS        │    user     │
         │    complete   │    speaks   │
         │               ▼             │
         │          ┌──────────┐       │
         └──────────┤Interrupted├───────┘
                    └──────────┘
```

### State Descriptions

| State | Description |
|-------|-------------|
| **Idle** | Session created, waiting for audio |
| **Listening** | Receiving audio, VAD active |
| **Processing** | ASR complete, waiting for LLM/TTS |
| **Speaking** | AI is speaking (TTS audio streaming) |
| **Interrupted** | User interrupted, cleaning up |

### Valid State Transitions

```go
var validTransitions = map[SessionState][]SessionState{
    StateIdle: {
        StateListening,
    },
    StateListening: {
        StateProcessing,
        StateIdle,
        StateInterrupted,
    },
    StateProcessing: {
        StateSpeaking,
        StateIdle,
        StateInterrupted,
    },
    StateSpeaking: {
        StateListening,
        StateIdle,
        StateInterrupted,
    },
    StateInterrupted: {
        StateListening,
        StateIdle,
        StateProcessing,
    },
}
```

## Turn Model

The conversation is organized into turns:

### User Turn

- Starts when VAD detects speech
- Ends when VAD detects silence
- ASR produces transcript
- Transcript added to conversation history

### Assistant Turn

- Starts when LLM begins generating
- Tracks three text states:
  1. **Generated Text** — Full text from LLM
  2. **Queued for TTS** — Text sent to TTS
  3. **Spoken Text** — Text actually played to user

```go
type AssistantTurn struct {
    GenerationID       string
    GeneratedText      string  // Full LLM output
    QueuedForTTSText   string  // Sent to TTS
    SpokenText         string  // Actually played
    Interrupted        bool
    PlayoutCursor      int     // Bytes sent
}
```

## The Critical Rule

> **Only `spoken_text` is committed to conversation history.**

This is the key principle of interruption handling. When a user interrupts:

1. The assistant turn is truncated to what was actually spoken
2. Unspoken generated text is discarded
3. Only the spoken portion is added to history

This prevents the AI from "remembering" things it never actually said.

## Interruption Flow

### Step-by-Step Process

```
1. User speaks while bot is speaking
        │
        ▼
2. VAD detects speech start
   (Media-edge: internal/vad/vad.go)
        │
        ▼
3. Media-edge stops outbound audio playout
   (Clears output buffer, resets playout tracker)
        │
        ▼
4. Media-edge sends interrupt to orchestrator
   (via gRPC/bridge)
        │
        ▼
5. Orchestrator cancels active LLM generation
   (Provider gateway: cancel() method)
        │
        ▼
6. Orchestrator cancels active TTS synthesis
   (Provider gateway: cancel() method)
        │
        ▼
7. Assistant turn trimmed to spoken text
   (TurnManager.CommitSpokenText())
        │
        ▼
8. Trimmed text committed to conversation history
   (History.AppendAssistantMessage())
        │
        ▼
9. New user utterance processed
   (Normal ASR → LLM → TTS flow)
```

### Code Flow

**Media-Edge (handler/session_handler.go):**

```go
func (sh *SessionHandler) handleInterruption() {
    // Stop playout
    sh.outputBuffer.Clear()
    sh.playoutTracker.Reset()

    // Notify orchestrator
    sh.bridge.Interrupt(sh.ctx, sh.sessionID)

    // Send interruption event to client
    interruptionEvent := events.NewInterruptionEvent(sh.sessionID, "user_speech")
    sh.sendEventToClient(interruptionEvent)

    // Transition back to listening
    sh.state = session.StateListening
}
```

**Orchestrator (pipeline/engine.go):**

```go
func (e *Engine) HandleInterruption(ctx context.Context, sessionID string) error {
    // Cancel LLM generation
    if generationID := sessionCtx.TurnManager.GetGenerationID(); generationID != "" {
        e.llmStage.Cancel(ctx, sessionID, generationID)
    }

    // Cancel TTS synthesis
    e.ttsStage.Cancel(ctx, sessionID)

    // Get playout position and handle interruption
    playoutPosition := sessionCtx.TurnManager.GetCurrentPosition()
    sessionCtx.TurnManager.HandleInterruption(playoutPosition)

    // Commit only spoken text to history
    committedMsg := sessionCtx.TurnManager.CommitTurn()
    if committedMsg.Content != "" {
        sessionCtx.History.AppendAssistantMessage(committedMsg.Content)
    }

    return nil
}
```

## Playout Tracking

The system tracks audio playout to determine what was actually spoken:

### Playout Tracker

```go
type PlayoutTracker struct {
    sampleRate    int
    channels      int
    bytesPerSample int
    position      int  // Total bytes sent
}

func (pt *PlayoutTracker) Advance(bytes int) {
    pt.position += bytes
}

func (pt *PlayoutTracker) CurrentPosition() time.Duration {
    samples := pt.position / pt.bytesPerSample
    seconds := float64(samples) / float64(pt.sampleRate)
    return time.Duration(seconds * float64(time.Second))
}
```

### Text-to-Audio Mapping

To determine how much text corresponds to played audio:

```go
func (t *AssistantTurn) CommitSpokenText() string {
    // Calculate duration from playout cursor
    spokenDuration := t.calculateDurationFromBytes(t.PlayoutCursor)

    // Estimate characters at ~15 chars per second
    spokenChars := int(spokenDuration.Seconds() * 15)

    if spokenChars > len(t.GeneratedText) {
        spokenChars = len(t.GeneratedText)
    }

    committed := t.GeneratedText[:spokenChars]
    t.SpokenText = committed

    // Trim generated text to only what was spoken
    t.GeneratedText = t.GeneratedText[spokenChars:]

    return committed
}
```

## Generation ID

Each assistant turn has a unique `generation_id` for idempotent cancellation:

```go
generationID := generateID()  // e.g., "gen_abc123xyz"
turn := sessionCtx.TurnManager.StartTurn(generationID, 16000)
```

The generation ID is used to:
1. Track which generation is active
2. Cancel the correct generation (avoiding race conditions)
3. Correlate events across the pipeline

### Cancellation with Generation ID

```python
# Provider implementation
class MyLLMProvider(BaseLLMProvider):
    def __init__(self):
        self._cancelled_sessions: set = set()

    async def stream_generate(self, messages, options=None):
        session_id = self._generate_session_id(messages)

        for token in self._generate_tokens(messages):
            # Check if this specific session was cancelled
            if session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                return  # Stop generation

            yield token

    async def cancel(self, session_id: str) -> bool:
        self._cancelled_sessions.add(session_id)
        return True
```

## Interruption Events

### Client Notification

When an interruption occurs, the client receives:

```json
{
  "type": "interruption.event",
  "timestamp": 1704067205000,
  "session_id": "sess_abc123",
  "reason": "user_speech",
  "spoken_text": "I'd be happy to help",
  "unspoken_text": " you with your account! What seems to be the problem?",
  "playout_position_ms": 1200
}
```

### Turn Events

Turn events indicate state changes:

```json
// Turn started
{
  "type": "turn.event",
  "turn_type": "assistant",
  "event": "started",
  "generation_id": "gen_xyz789"
}

// Turn cancelled (interrupted)
{
  "type": "turn.event",
  "turn_type": "assistant",
  "event": "cancelled",
  "text": "I'd be happy to help"
}
```

## Configuration

Interruption behavior can be configured:

```yaml
# In pipeline engine config
engine:
  enable_interruptions: true
  interruption_sensitivity: 0.5  # VAD sensitivity
```

## Performance Considerations

### Target Latencies

| Operation | Target Latency |
|-----------|---------------|
| VAD speech detection | < 100ms |
| Audio playout stop | < 50ms |
| LLM cancellation | < 100ms |
| TTS cancellation | < 50ms |
| Total interruption latency | < 300ms |

### Optimization Strategies

1. **VAD Tuning**: Balance between false positives and detection delay
2. **Buffer Management**: Clear output buffers immediately on interruption
3. **Async Cancellation**: Don't wait for provider acknowledgment
4. **Cursor Tracking**: Maintain accurate playout position

## Testing Interruption

### Manual Testing

```bash
# Start a session
python scripts/ws-client.py --server ws://localhost:8080/ws

# While the AI is speaking, send an interrupt
# Or use the synthetic audio with interruption simulation
```

### Automated Testing

```python
async def test_interruption():
    async with websockets.connect('ws://localhost:8080/ws') as ws:
        # Start session
        await ws.send(json.dumps({
            'type': 'session.start',
            # ... config
        }))

        # Send audio to trigger response
        await ws.send(json.dumps({
            'type': 'audio.chunk',
            'audio_data': base64_audio
        }))

        # Wait for AI to start speaking
        await wait_for_turn_started(ws)

        # Send interrupt
        await ws.send(json.dumps({
            'type': 'session.interrupt'
        }))

        # Verify interruption event received
        msg = await ws.recv()
        assert json.loads(msg)['type'] == 'interruption.event'
```

## Common Issues

### Interruption Not Working

Check:
1. VAD is detecting speech during bot speech
2. `enable_interruptions` is true in config
3. Provider supports cancellation (`supports_interruptible_generation`)

### Partial Text Committed

If too much or too little text is committed:
1. Check playout cursor accuracy
2. Verify text-to-duration estimation
3. Review audio buffer clearing timing

### Audio Continues After Interruption

If audio keeps playing after interruption:
1. Ensure output buffer is cleared immediately
2. Check for race conditions in buffer access
3. Verify client-side audio queue is cleared
