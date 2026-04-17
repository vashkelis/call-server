package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/parlona/cloudapp/pkg/session"
	"github.com/redis/go-redis/v9"
)

// RedisPersistence wraps RedisSessionStore with additional hot state helpers.
type RedisPersistence struct {
	store     *session.RedisSessionStore
	client    *redis.Client
	keyPrefix string
}

// NewRedisPersistence creates a new Redis persistence layer.
func NewRedisPersistence(client *redis.Client, keyPrefix string) *RedisPersistence {
	if keyPrefix == "" {
		keyPrefix = "cloudapp:"
	}

	store := session.NewRedisSessionStore(client,
		session.WithKeyPrefix(keyPrefix+"session:"),
		session.WithTTL(3600),
	)

	return &RedisPersistence{
		store:     store,
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// StoreTurnState stores the assistant turn state in Redis.
// Uses Redis hash fields for fine-grained updates.
func (r *RedisPersistence) StoreTurnState(
	ctx context.Context,
	sessionID string,
	turn *session.AssistantTurn,
) error {
	key := r.keyPrefix + "turn:" + sessionID

	// Clone the turn to avoid mutating the original and get a clean copy for serialization
	turnClone := turn.Clone()

	// Serialize turn to JSON
	data, err := json.Marshal(turnClone)
	if err != nil {
		return fmt.Errorf("failed to marshal turn state: %w", err)
	}

	// Store in Redis hash
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, "data", string(data))
	pipe.HSet(ctx, key, "updated_at", time.Now().UTC().Format(time.RFC3339))
	pipe.Expire(ctx, key, time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store turn state: %w", err)
	}

	return nil
}

// GetTurnState retrieves the assistant turn state from Redis.
func (r *RedisPersistence) GetTurnState(
	ctx context.Context,
	sessionID string,
) (*session.AssistantTurn, error) {
	key := r.keyPrefix + "turn:" + sessionID

	data, err := r.client.HGet(ctx, key, "data").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("%w: turn state not found for session %s", session.ErrSessionNotFound, sessionID)
		}
		return nil, fmt.Errorf("failed to get turn state: %w", err)
	}

	var turn session.AssistantTurn
	if err := json.Unmarshal([]byte(data), &turn); err != nil {
		return nil, fmt.Errorf("failed to unmarshal turn state: %w", err)
	}

	return &turn, nil
}

// SetBotSpeaking sets whether the bot is currently speaking.
func (r *RedisPersistence) SetBotSpeaking(
	ctx context.Context,
	sessionID string,
	speaking bool,
) error {
	key := r.keyPrefix + "speaking:" + sessionID

	value := "0"
	if speaking {
		value = "1"
	}

	if err := r.client.Set(ctx, key, value, time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set bot speaking state: %w", err)
	}

	return nil
}

// GetBotSpeaking retrieves whether the bot is currently speaking.
func (r *RedisPersistence) GetBotSpeaking(
	ctx context.Context,
	sessionID string,
) (bool, error) {
	key := r.keyPrefix + "speaking:" + sessionID

	value, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to get bot speaking state: %w", err)
	}

	return value == "1", nil
}

// StorePlayoutPosition stores the current playout position for a session.
func (r *RedisPersistence) StorePlayoutPosition(
	ctx context.Context,
	sessionID string,
	position int,
) error {
	key := r.keyPrefix + "playout:" + sessionID

	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, "position", position)
	pipe.HSet(ctx, key, "updated_at", time.Now().UTC().Format(time.RFC3339))
	pipe.Expire(ctx, key, time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store playout position: %w", err)
	}

	return nil
}

// GetPlayoutPosition retrieves the current playout position for a session.
func (r *RedisPersistence) GetPlayoutPosition(
	ctx context.Context,
	sessionID string,
) (int, error) {
	key := r.keyPrefix + "playout:" + sessionID

	position, err := r.client.HGet(ctx, key, "position").Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get playout position: %w", err)
	}

	return position, nil
}

// StoreGenerationID stores the active generation ID for a session.
func (r *RedisPersistence) StoreGenerationID(
	ctx context.Context,
	sessionID string,
	generationID string,
	providerType string,
) error {
	key := r.keyPrefix + "generation:" + sessionID

	field := providerType + "_id"
	if err := r.client.HSet(ctx, key, field, generationID).Err(); err != nil {
		return fmt.Errorf("failed to store generation ID: %w", err)
	}

	if err := r.client.Expire(ctx, key, time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set generation ID expiry: %w", err)
	}

	return nil
}

// GetGenerationID retrieves the active generation ID for a session.
func (r *RedisPersistence) GetGenerationID(
	ctx context.Context,
	sessionID string,
	providerType string,
) (string, error) {
	key := r.keyPrefix + "generation:" + sessionID

	field := providerType + "_id"
	generationID, err := r.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get generation ID: %w", err)
	}

	return generationID, nil
}

// ClearGenerationID clears the generation ID for a session.
func (r *RedisPersistence) ClearGenerationID(
	ctx context.Context,
	sessionID string,
	providerType string,
) error {
	key := r.keyPrefix + "generation:" + sessionID

	field := providerType + "_id"
	if err := r.client.HDel(ctx, key, field).Err(); err != nil {
		return fmt.Errorf("failed to clear generation ID: %w", err)
	}

	return nil
}

// StoreSessionState stores the session state in Redis.
func (r *RedisPersistence) StoreSessionState(
	ctx context.Context,
	sessionID string,
	state session.SessionState,
) error {
	key := r.keyPrefix + "state:" + sessionID

	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, "state", state.String())
	pipe.HSet(ctx, key, "updated_at", time.Now().UTC().Format(time.RFC3339))
	pipe.Expire(ctx, key, time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store session state: %w", err)
	}

	return nil
}

// GetSessionState retrieves the session state from Redis.
func (r *RedisPersistence) GetSessionState(
	ctx context.Context,
	sessionID string,
) (session.SessionState, error) {
	key := r.keyPrefix + "state:" + sessionID

	stateStr, err := r.client.HGet(ctx, key, "state").Result()
	if err != nil {
		if err == redis.Nil {
			return session.StateIdle, nil
		}
		return session.StateIdle, fmt.Errorf("failed to get session state: %w", err)
	}

	// Parse state string
	switch stateStr {
	case "idle":
		return session.StateIdle, nil
	case "listening":
		return session.StateListening, nil
	case "processing":
		return session.StateProcessing, nil
	case "speaking":
		return session.StateSpeaking, nil
	case "interrupted":
		return session.StateInterrupted, nil
	default:
		return session.StateIdle, nil
	}
}

// DeleteSession deletes all Redis data for a session.
func (r *RedisPersistence) DeleteSession(ctx context.Context, sessionID string) error {
	keys := []string{
		r.keyPrefix + "turn:" + sessionID,
		r.keyPrefix + "speaking:" + sessionID,
		r.keyPrefix + "playout:" + sessionID,
		r.keyPrefix + "generation:" + sessionID,
		r.keyPrefix + "state:" + sessionID,
	}

	pipe := r.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session data: %w", err)
	}

	return nil
}

// GetStore returns the underlying session store.
func (r *RedisPersistence) GetStore() *session.RedisSessionStore {
	return r.store
}

// Ping checks the Redis connection.
func (r *RedisPersistence) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (r *RedisPersistence) Close() error {
	return r.store.Close()
}
