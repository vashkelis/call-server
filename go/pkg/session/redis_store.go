package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSessionStore implements SessionStore using Redis.
type RedisSessionStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
	metrics   StoreMetrics
}

// NewRedisSessionStore creates a new Redis-backed session store.
func NewRedisSessionStore(client *redis.Client, opts ...StoreOption) *RedisSessionStore {
	options := &storeOptions{
		keyPrefix: "session:",
		ttl:       3600, // 1 hour
	}

	for _, opt := range opts {
		opt(options)
	}

	return &RedisSessionStore{
		client:    client,
		keyPrefix: options.keyPrefix,
		ttl:       time.Duration(options.ttl) * time.Second,
	}
}

// Get retrieves a session by ID from Redis.
func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	key := ComposeSessionKey(s.keyPrefix, sessionID)

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		s.metrics.Errors++
		return nil, fmt.Errorf("failed to get session from redis: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		s.metrics.Errors++
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	s.metrics.Gets++
	return &session, nil
}

// Save persists a session to Redis.
func (s *RedisSessionStore) Save(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	key := ComposeSessionKey(s.keyPrefix, session.SessionID)

	// Update timestamp before saving
	session.Touch()

	data, err := json.Marshal(session)
	if err != nil {
		s.metrics.Errors++
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		s.metrics.Errors++
		return fmt.Errorf("failed to save session to redis: %w", err)
	}

	s.metrics.Saves++
	return nil
}

// Delete removes a session from Redis.
func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	key := ComposeSessionKey(s.keyPrefix, sessionID)

	result, err := s.client.Del(ctx, key).Result()
	if err != nil {
		s.metrics.Errors++
		return fmt.Errorf("failed to delete session from redis: %w", err)
	}

	if result == 0 {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	s.metrics.Deletes++
	return nil
}

// UpdateTurn updates the active turn for a session.
func (s *RedisSessionStore) UpdateTurn(ctx context.Context, sessionID string, turn *AssistantTurn) error {
	// Get the current session
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update the turn
	session.SetActiveTurn(turn)

	// Save back
	if err := s.Save(ctx, session); err != nil {
		return fmt.Errorf("failed to save session after turn update: %w", err)
	}

	s.metrics.UpdateTurns++
	return nil
}

// List returns all active session IDs from Redis.
func (s *RedisSessionStore) List(ctx context.Context) ([]string, error) {
	pattern := s.keyPrefix + "*"
	var sessionIDs []string

	iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Remove prefix to get session ID
		sessionID := key[len(s.keyPrefix):]
		sessionIDs = append(sessionIDs, sessionID)
	}

	if err := iter.Err(); err != nil {
		s.metrics.Errors++
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessionIDs, nil
}

// Close closes the Redis client.
func (s *RedisSessionStore) Close() error {
	return s.client.Close()
}

// Ping checks the Redis connection.
func (s *RedisSessionStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// ExtendTTL extends the TTL of a session.
func (s *RedisSessionStore) ExtendTTL(ctx context.Context, sessionID string) error {
	key := ComposeSessionKey(s.keyPrefix, sessionID)
	return s.client.Expire(ctx, key, s.ttl).Err()
}

// GetMetrics returns the store metrics.
func (s *RedisSessionStore) GetMetrics() StoreMetrics {
	return s.metrics
}
