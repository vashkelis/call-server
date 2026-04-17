package session

import (
	"context"
	"errors"
	"fmt"
)

// Common errors for session store operations.
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExists   = errors.New("session already exists")
	ErrStoreClosed     = errors.New("session store is closed")
)

// SessionStore defines the interface for session persistence.
type SessionStore interface {
	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Save persists a session.
	Save(ctx context.Context, session *Session) error

	// Delete removes a session.
	Delete(ctx context.Context, sessionID string) error

	// UpdateTurn updates the active turn for a session.
	UpdateTurn(ctx context.Context, sessionID string, turn *AssistantTurn) error

	// List returns all active session IDs.
	List(ctx context.Context) ([]string, error)

	// Close closes the store.
	Close() error
}

// StoreConfig contains configuration for session stores.
type StoreConfig struct {
	KeyPrefix     string
	DefaultTTL    int // seconds
	MaxSessions   int
	EnableHistory bool
}

// Validate validates the store configuration.
func (c *StoreConfig) Validate() error {
	if c.KeyPrefix == "" {
		c.KeyPrefix = "session:"
	}
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = 3600 // 1 hour default
	}
	if c.MaxSessions <= 0 {
		c.MaxSessions = 10000
	}
	return nil
}

// StoreMetrics tracks store operation metrics.
type StoreMetrics struct {
	Gets        int64
	Saves       int64
	Deletes     int64
	UpdateTurns int64
	Errors      int64
}

// StoreOption is a functional option for configuring stores.
type StoreOption func(*storeOptions)

type storeOptions struct {
	keyPrefix string
	ttl       int
}

// WithKeyPrefix sets the key prefix for the store.
func WithKeyPrefix(prefix string) StoreOption {
	return func(o *storeOptions) {
		o.keyPrefix = prefix
	}
}

// WithTTL sets the default TTL for sessions.
func WithTTL(ttl int) StoreOption {
	return func(o *storeOptions) {
		o.ttl = ttl
	}
}

// ComposeSessionKey creates a session key with the given prefix.
func ComposeSessionKey(prefix, sessionID string) string {
	return fmt.Sprintf("%s%s", prefix, sessionID)
}

// SessionEvent represents an event in the session lifecycle.
type SessionEvent struct {
	SessionID string `json:"session_id"`
	EventType string `json:"event_type"`
	Timestamp int64  `json:"timestamp"`
	Data      []byte `json:"data,omitempty"`
}

// EventStore defines the interface for event persistence.
type EventStore interface {
	// AppendEvent appends an event to the session history.
	AppendEvent(ctx context.Context, sessionID string, event SessionEvent) error

	// GetEvents retrieves events for a session.
	GetEvents(ctx context.Context, sessionID string, limit int) ([]SessionEvent, error)

	// Close closes the event store.
	Close() error
}
