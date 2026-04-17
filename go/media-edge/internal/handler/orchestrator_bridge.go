// Package handler provides WebSocket handlers and session management.
package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/events"
)

// OrchestratorBridge defines the interface for communication with the orchestrator.
type OrchestratorBridge interface {
	// StartSession notifies the orchestrator of a new session.
	StartSession(ctx context.Context, sessionID string, config SessionConfig) error

	// SendAudio sends audio data to the orchestrator for ASR processing.
	SendAudio(ctx context.Context, sessionID string, audio []byte) error

	// SendUserUtterance sends a user transcript to the orchestrator.
	SendUserUtterance(ctx context.Context, sessionID string, transcript string) error

	// ReceiveEvents returns a channel for receiving events from the orchestrator.
	ReceiveEvents(ctx context.Context, sessionID string) (<-chan events.Event, error)

	// Interrupt triggers an interruption for the given session.
	Interrupt(ctx context.Context, sessionID string) error

	// StopSession notifies the orchestrator to stop a session.
	StopSession(ctx context.Context, sessionID string) error
}

// SessionConfig contains configuration for a new session.
type SessionConfig struct {
	SessionID    string
	TenantID     string
	SystemPrompt string
	VoiceProfile events.VoiceProfileConfig
	ModelOptions events.ModelOptionsConfig
	Providers    events.ProviderConfig
	AudioProfile events.AudioProfileConfig
}

// ChannelBridge implements OrchestratorBridge using Go channels for in-process communication.
// This is used for MVP when media-edge and orchestrator run in the same process.
type ChannelBridge struct {
	mu sync.RWMutex

	// Session channels
	sessions map[string]*SessionChannels

	// Global event channel for orchestrator to receive
	eventCh chan BridgeEvent

	// Closed flag
	closed bool
}

// SessionChannels holds the communication channels for a session.
type SessionChannels struct {
	sessionID   string
	audioCh     chan []byte
	utteranceCh chan string
	eventCh     chan events.Event
	interruptCh chan struct{}
	stopCh      chan struct{}
	closed      bool
	mu          sync.Mutex
}

// BridgeEvent represents an event sent through the bridge.
type BridgeEvent struct {
	SessionID string
	Type      BridgeEventType
	Data      interface{}
}

// BridgeEventType represents the type of bridge event.
type BridgeEventType int

const (
	EventTypeSessionStart BridgeEventType = iota
	EventTypeAudio
	EventTypeUtterance
	EventTypeInterrupt
	EventTypeStop
)

// NewChannelBridge creates a new channel-based orchestrator bridge.
func NewChannelBridge() *ChannelBridge {
	return &ChannelBridge{
		sessions: make(map[string]*SessionChannels),
		eventCh:  make(chan BridgeEvent, 1000),
	}
}

// StartSession creates channels for a new session.
func (b *ChannelBridge) StartSession(ctx context.Context, sessionID string, config SessionConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("bridge is closed")
	}

	if _, exists := b.sessions[sessionID]; exists {
		return fmt.Errorf("session %s already exists", sessionID)
	}

	sessionCh := &SessionChannels{
		sessionID:   sessionID,
		audioCh:     make(chan []byte, 100),
		utteranceCh: make(chan string, 10),
		eventCh:     make(chan events.Event, 100),
		interruptCh: make(chan struct{}, 1),
		stopCh:      make(chan struct{}, 1),
	}

	b.sessions[sessionID] = sessionCh

	// Send session start event
	select {
	case b.eventCh <- BridgeEvent{
		SessionID: sessionID,
		Type:      EventTypeSessionStart,
		Data:      config,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// SendAudio sends audio data to the orchestrator.
func (b *ChannelBridge) SendAudio(ctx context.Context, sessionID string, audio []byte) error {
	b.mu.RLock()
	sessionCh, exists := b.sessions[sessionID]
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sessionCh.mu.Lock()
	if sessionCh.closed {
		sessionCh.mu.Unlock()
		return fmt.Errorf("session %s is closed", sessionID)
	}
	sessionCh.mu.Unlock()

	// Copy audio data to prevent modification
	audioCopy := make([]byte, len(audio))
	copy(audioCopy, audio)

	select {
	case sessionCh.audioCh <- audioCopy:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel full, drop oldest
		select {
		case <-sessionCh.audioCh:
		default:
		}
		select {
		case sessionCh.audioCh <- audioCopy:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SendUserUtterance sends a user transcript to the orchestrator.
func (b *ChannelBridge) SendUserUtterance(ctx context.Context, sessionID string, transcript string) error {
	b.mu.RLock()
	sessionCh, exists := b.sessions[sessionID]
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sessionCh.mu.Lock()
	if sessionCh.closed {
		sessionCh.mu.Unlock()
		return fmt.Errorf("session %s is closed", sessionID)
	}
	sessionCh.mu.Unlock()

	select {
	case sessionCh.utteranceCh <- transcript:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReceiveEvents returns a channel for receiving events from the orchestrator.
func (b *ChannelBridge) ReceiveEvents(ctx context.Context, sessionID string) (<-chan events.Event, error) {
	b.mu.RLock()
	sessionCh, exists := b.sessions[sessionID]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return sessionCh.eventCh, nil
}

// Interrupt triggers an interruption for the given session.
func (b *ChannelBridge) Interrupt(ctx context.Context, sessionID string) error {
	b.mu.RLock()
	sessionCh, exists := b.sessions[sessionID]
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sessionCh.mu.Lock()
	if sessionCh.closed {
		sessionCh.mu.Unlock()
		return fmt.Errorf("session %s is closed", sessionID)
	}
	sessionCh.mu.Unlock()

	select {
	case sessionCh.interruptCh <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil // Already signaled
	}
}

// StopSession stops a session and cleans up its channels.
func (b *ChannelBridge) StopSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	sessionCh, exists := b.sessions[sessionID]
	if exists {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sessionCh.mu.Lock()
	if !sessionCh.closed {
		sessionCh.closed = true
		close(sessionCh.audioCh)
		close(sessionCh.utteranceCh)
		close(sessionCh.eventCh)
		close(sessionCh.interruptCh)
		close(sessionCh.stopCh)
	}
	sessionCh.mu.Unlock()

	return nil
}

// GetSessionChannels returns the channels for a session (for orchestrator side).
func (b *ChannelBridge) GetSessionChannels(sessionID string) (*SessionChannels, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ch, ok := b.sessions[sessionID]
	return ch, ok
}

// EventChannel returns the global event channel (for orchestrator side).
func (b *ChannelBridge) EventChannel() <-chan BridgeEvent {
	return b.eventCh
}

// SendEventToSession sends an event to a specific session (for orchestrator side).
func (b *ChannelBridge) SendEventToSession(sessionID string, event events.Event) error {
	b.mu.RLock()
	sessionCh, exists := b.sessions[sessionID]
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sessionCh.mu.Lock()
	if sessionCh.closed {
		sessionCh.mu.Unlock()
		return fmt.Errorf("session %s is closed", sessionID)
	}
	sessionCh.mu.Unlock()

	select {
	case sessionCh.eventCh <- event:
		return nil
	default:
		// Channel full, drop event
		return fmt.Errorf("event channel full for session %s", sessionID)
	}
}

// Close closes the bridge and all session channels.
func (b *ChannelBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Close all sessions
	for sessionID, sessionCh := range b.sessions {
		sessionCh.mu.Lock()
		if !sessionCh.closed {
			sessionCh.closed = true
			close(sessionCh.audioCh)
			close(sessionCh.utteranceCh)
			close(sessionCh.eventCh)
			close(sessionCh.interruptCh)
			close(sessionCh.stopCh)
		}
		sessionCh.mu.Unlock()
		delete(b.sessions, sessionID)
	}

	close(b.eventCh)

	return nil
}

// ListSessions returns a list of active session IDs.
func (b *ChannelBridge) ListSessions() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sessions := make([]string, 0, len(b.sessions))
	for id := range b.sessions {
		sessions = append(sessions, id)
	}
	return sessions
}

// GRPCBridge is a placeholder for gRPC-based inter-service communication.
// This will be implemented when media-edge and orchestrator run as separate services.
type GRPCBridge struct {
	endpoint string
	mu       sync.RWMutex
	closed   bool
}

// NewGRPCBridge creates a new gRPC bridge (placeholder).
func NewGRPCBridge(endpoint string) *GRPCBridge {
	return &GRPCBridge{
		endpoint: endpoint,
	}
}

// StartSession implements OrchestratorBridge.
func (b *GRPCBridge) StartSession(ctx context.Context, sessionID string, config SessionConfig) error {
	// TODO: Implement gRPC call to orchestrator
	return fmt.Errorf("gRPC bridge not implemented")
}

// SendAudio implements OrchestratorBridge.
func (b *GRPCBridge) SendAudio(ctx context.Context, sessionID string, audio []byte) error {
	// TODO: Implement gRPC streaming to orchestrator
	return fmt.Errorf("gRPC bridge not implemented")
}

// SendUserUtterance implements OrchestratorBridge.
func (b *GRPCBridge) SendUserUtterance(ctx context.Context, sessionID string, transcript string) error {
	// TODO: Implement gRPC call to orchestrator
	return fmt.Errorf("gRPC bridge not implemented")
}

// ReceiveEvents implements OrchestratorBridge.
func (b *GRPCBridge) ReceiveEvents(ctx context.Context, sessionID string) (<-chan events.Event, error) {
	// TODO: Implement gRPC streaming from orchestrator
	return nil, fmt.Errorf("gRPC bridge not implemented")
}

// Interrupt implements OrchestratorBridge.
func (b *GRPCBridge) Interrupt(ctx context.Context, sessionID string) error {
	// TODO: Implement gRPC call to orchestrator
	return fmt.Errorf("gRPC bridge not implemented")
}

// StopSession implements OrchestratorBridge.
func (b *GRPCBridge) StopSession(ctx context.Context, sessionID string) error {
	// TODO: Implement gRPC call to orchestrator
	return fmt.Errorf("gRPC bridge not implemented")
}

// Close closes the gRPC bridge.
func (b *GRPCBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	return nil
}

// BridgeStats contains statistics about the bridge.
type BridgeStats struct {
	ActiveSessions int
	PendingEvents  int
}

// Stats returns current bridge statistics.
func (b *ChannelBridge) Stats() BridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BridgeStats{
		ActiveSessions: len(b.sessions),
		PendingEvents:  len(b.eventCh),
	}
}

// WaitForSession waits for a session to be created with timeout.
func (b *ChannelBridge) WaitForSession(ctx context.Context, sessionID string, timeout time.Duration) (*SessionChannels, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			b.mu.RLock()
			sessionCh, exists := b.sessions[sessionID]
			b.mu.RUnlock()
			if exists {
				return sessionCh, nil
			}
		}
	}
}
