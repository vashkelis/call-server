// Package pipeline provides the ASR->LLM->TTS pipeline stages and orchestration.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// StateClosed means the circuit is closed and requests flow normally.
	StateClosed CircuitState = iota
	// StateOpen means the circuit is open and requests fail fast.
	StateOpen
	// StateHalfOpen means the circuit is testing if the service has recovered.
	StateHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig contains configuration for a circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening the circuit.
	FailureThreshold int
	// Timeout is the duration to keep the circuit open before trying again.
	Timeout time.Duration
	// HalfOpenMaxCalls is the maximum number of calls to allow in half-open state.
	HalfOpenMaxCalls int
}

// DefaultCircuitBreakerConfig returns a default configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		Timeout:          30 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern for provider calls.
type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	config          CircuitBreakerConfig
	name            string
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: config,
		name:   name,
	}
}

// ErrCircuitOpen is returned when the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Execute runs the given function if the circuit allows it.
// If the circuit is open, it returns ErrCircuitOpen immediately.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.beforeCall(); err != nil {
		return err
	}

	err := fn()
	cb.afterCall(err)
	return err
}

// beforeCall checks if the call should proceed based on circuit state.
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.failureCount = 0
			cb.successCount = 0
			return nil
		}
		return fmt.Errorf("%w: %s", ErrCircuitOpen, cb.name)

	case StateHalfOpen:
		// Allow limited calls in half-open state
		if cb.successCount+cb.failureCount >= cb.config.HalfOpenMaxCalls {
			return fmt.Errorf("%w: %s (half-open limit reached)", ErrCircuitOpen, cb.name)
		}
		return nil

	default:
		return nil
	}
}

// afterCall updates the circuit state based on the call result.
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failure and potentially opens the circuit.
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}

	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = StateOpen
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// recordSuccess records a success and potentially closes the circuit.
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		// If we've had enough successes, close the circuit
		if cb.successCount >= cb.config.HalfOpenMaxCalls {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
		}

	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns the current circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitStats{
		State:        cb.state,
		FailureCount: cb.failureCount,
		SuccessCount: cb.successCount,
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// CircuitStats contains circuit breaker statistics.
type CircuitStats struct {
	State        CircuitState
	FailureCount int
	SuccessCount int
}

// CircuitBreakerRegistry manages circuit breakers for multiple providers.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a new circuit breaker registry.
func NewCircuitBreakerRegistry(config CircuitBreakerConfig) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// Get returns a circuit breaker for the given provider, creating one if needed.
func (r *CircuitBreakerRegistry) Get(providerName string) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok := r.breakers[providerName]; ok {
		return cb
	}

	cb := NewCircuitBreaker(providerName, r.config)
	r.breakers[providerName] = cb
	return cb
}

// GetAll returns all circuit breakers.
func (r *CircuitBreakerRegistry) GetAll() map[string]*CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CircuitBreaker, len(r.breakers))
	for k, v := range r.breakers {
		result[k] = v
	}
	return result
}

// Reset resets all circuit breakers.
func (r *CircuitBreakerRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cb := range r.breakers {
		cb.Reset()
	}
}
