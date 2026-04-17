// Package events provides WebSocket event types for the real-time voice API.
package events

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// EventType represents the type of WebSocket event.
type EventType string

// Client -> Server event types
const (
	EventTypeSessionStart     EventType = "session.start"
	EventTypeAudioChunk       EventType = "audio.chunk"
	EventTypeSessionUpdate    EventType = "session.update"
	EventTypeSessionInterrupt EventType = "session.interrupt"
	EventTypeSessionStop      EventType = "session.stop"
)

// Server -> Client event types
const (
	EventTypeSessionStarted EventType = "session.started"
	EventTypeVAD            EventType = "vad.event"
	EventTypeASRPartial     EventType = "asr.partial"
	EventTypeASRFinal       EventType = "asr.final"
	EventTypeLLMPartialText EventType = "llm.partial_text"
	EventTypeTTSAudioChunk  EventType = "tts.audio_chunk"
	EventTypeTurn           EventType = "turn.event"
	EventTypeInterruption   EventType = "interruption.event"
	EventTypeError          EventType = "error"
	EventTypeSessionEnded   EventType = "session.ended"
)

// BaseEvent is the base structure for all events.
type BaseEvent struct {
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
}

// Now returns the current timestamp in milliseconds.
func Now() int64 {
	return time.Now().UnixMilli()
}

// NewBaseEvent creates a new base event.
func NewBaseEvent(eventType EventType, sessionID string) BaseEvent {
	return BaseEvent{
		Type:      eventType,
		Timestamp: Now(),
		SessionID: sessionID,
	}
}

// Event is the interface for all events.
type Event interface {
	GetType() EventType
	GetTimestamp() int64
	GetSessionID() string
}

// GetType returns the event type.
func (e BaseEvent) GetType() EventType {
	return e.Type
}

// GetTimestamp returns the event timestamp.
func (e BaseEvent) GetTimestamp() int64 {
	return e.Timestamp
}

// GetSessionID returns the session ID.
func (e BaseEvent) GetSessionID() string {
	return e.SessionID
}

// ParseEvent parses a JSON event and returns the appropriate type.
func ParseEvent(data []byte) (Event, error) {
	var base BaseEvent
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to unmarshal base event: %w", err)
	}

	switch base.Type {
	// Client -> Server
	case EventTypeSessionStart:
		var e SessionStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeAudioChunk:
		var e AudioChunkEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeSessionUpdate:
		var e SessionUpdateEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeSessionInterrupt:
		var e SessionInterruptEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeSessionStop:
		var e SessionStopEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil

	// Server -> Client
	case EventTypeSessionStarted:
		var e SessionStartedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeVAD:
		var e VADEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeASRPartial:
		var e ASRPartialEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeASRFinal:
		var e ASRFinalEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeLLMPartialText:
		var e LLMPartialTextEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeTTSAudioChunk:
		var e TTSAudioChunkEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeTurn:
		var e TurnEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeInterruption:
		var e InterruptionEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeError:
		var e ErrorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case EventTypeSessionEnded:
		var e SessionEndedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", base.Type)
	}
}

// MarshalEvent marshals an event to JSON.
func MarshalEvent(event Event) ([]byte, error) {
	return json.Marshal(event)
}

// MustMarshalEvent marshals an event to JSON, panicking on error.
func MustMarshalEvent(event Event) []byte {
	data, err := MarshalEvent(event)
	if err != nil {
		panic(err)
	}
	return data
}

// encodeAudio encodes audio bytes to base64 string.
func encodeAudio(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// decodeAudio decodes base64 string to audio bytes.
func decodeAudio(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
