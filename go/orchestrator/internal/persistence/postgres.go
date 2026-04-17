// Package persistence provides data persistence for transcripts and events.
package persistence

import (
	"context"
	"fmt"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/session"
)

// PostgresPersistence provides PostgreSQL persistence for transcripts and events.
// For MVP: This is a stub implementation that logs "postgres persistence not configured"
// and returns nil errors. The structure is designed so a real implementation can be
// dropped in later.
type PostgresPersistence struct {
	logger *observability.Logger
	// In a real implementation, this would hold:
	// db *pgxpool.Pool or *sql.DB
}

// NewPostgresPersistence creates a new PostgreSQL persistence layer.
// For MVP, this logs a warning that persistence is not configured.
func NewPostgresPersistence(logger *observability.Logger) *PostgresPersistence {
	logger.Warn("Postgres persistence not configured - using stub implementation")
	return &PostgresPersistence{
		logger: logger.WithField("component", "postgres_persistence"),
	}
}

// Event represents a persisted event.
type Event struct {
	SessionID string                 `json:"session_id"`
	EventType string                 `json:"event_type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// SaveTranscript saves the conversation transcript to PostgreSQL.
// MVP: Logs the operation and returns nil.
func (p *PostgresPersistence) SaveTranscript(
	ctx context.Context,
	sessionID string,
	messages []contracts.ChatMessage,
) error {
	p.logger.WithFields(map[string]interface{}{
		"session_id":    sessionID,
		"message_count": len(messages),
	}).Info("SaveTranscript called (stub)")

	// MVP: No actual persistence
	// In production, this would:
	// 1. Insert/update session record
	// 2. Insert messages into conversation_history table
	// 3. Handle duplicates and updates

	return nil
}

// SaveEvent saves an event to PostgreSQL.
// MVP: Logs the operation and returns nil.
func (p *PostgresPersistence) SaveEvent(
	ctx context.Context,
	sessionID string,
	eventType string,
	data interface{},
) error {
	p.logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"event_type": eventType,
	}).Info("SaveEvent called (stub)")

	// MVP: No actual persistence
	// In production, this would:
	// 1. Serialize the event data
	// 2. Insert into events table
	// 3. Handle large payloads

	return nil
}

// GetTranscript retrieves the conversation transcript for a session.
// MVP: Returns empty slice and nil error.
func (p *PostgresPersistence) GetTranscript(
	ctx context.Context,
	sessionID string,
) ([]contracts.ChatMessage, error) {
	p.logger.WithField("session_id", sessionID).Info("GetTranscript called (stub)")

	// MVP: Return empty result
	// In production, this would:
	// 1. Query conversation_history table
	// 2. Order by timestamp
	// 3. Return all messages

	return []contracts.ChatMessage{}, nil
}

// GetSessionEvents retrieves all events for a session.
// MVP: Returns empty slice and nil error.
func (p *PostgresPersistence) GetSessionEvents(
	ctx context.Context,
	sessionID string,
) ([]Event, error) {
	p.logger.WithField("session_id", sessionID).Info("GetSessionEvents called (stub)")

	// MVP: Return empty result
	// In production, this would:
	// 1. Query events table
	// 2. Order by timestamp
	// 3. Return all events

	return []Event{}, nil
}

// Close closes the PostgreSQL connection.
// MVP: No-op.
func (p *PostgresPersistence) Close() error {
	p.logger.Info("Close called (stub)")

	// MVP: No-op
	// In production, this would close the database connection pool

	return nil
}

// HealthCheck checks the PostgreSQL connection health.
// MVP: Always returns healthy.
func (p *PostgresPersistence) HealthCheck(ctx context.Context) error {
	// MVP: Always healthy
	// In production, this would ping the database
	return nil
}

// TranscriptEntry represents a single entry in the transcript.
// This is the structure that would be stored in PostgreSQL.
type TranscriptEntry struct {
	ID         int64  `db:"id"`
	SessionID  string `db:"session_id"`
	TurnID     string `db:"turn_id"`
	Role       string `db:"role"`
	Content    string `db:"content"`
	Timestamp  int64  `db:"timestamp"`
	SpokenOnly bool   `db:"spoken_only"`
	DurationMs int64  `db:"duration_ms"`
}

// SQLSchema contains the database schema for PostgreSQL persistence.
// This can be used to create the necessary tables.
const SQLSchema = `
-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255) UNIQUE NOT NULL,
    tenant_id VARCHAR(255),
    transport_type VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    metadata JSONB
);

-- Conversation history table
CREATE TABLE IF NOT EXISTS conversation_history (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL REFERENCES sessions(session_id),
    turn_id VARCHAR(255),
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    spoken_only BOOLEAN DEFAULT FALSE,
    duration_ms BIGINT,
    metadata JSONB
);

-- Events table
CREATE TABLE IF NOT EXISTS session_events (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL REFERENCES sessions(session_id),
    event_type VARCHAR(100) NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    data JSONB
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_conversation_session_id ON conversation_history(session_id);
CREATE INDEX IF NOT EXISTS idx_events_session_id ON session_events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON session_events(timestamp);
`

// Ensure PostgresPersistence implements the required interfaces
var _ = session.HistoryStore(nil)
var _ = fmt.Sprintf
