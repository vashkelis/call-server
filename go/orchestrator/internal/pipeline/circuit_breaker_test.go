package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	// Circuit should start closed
	if cb.State() != StateClosed {
		t.Errorf("expected initial state Closed, got %v", cb.State())
	}

	// Successful calls should keep circuit closed
	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error on call %d: %v", i, err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed after successful calls, got %v", cb.State())
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Fail enough times to open circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func() error {
			return testErr
		})
		if err != testErr {
			t.Errorf("expected test error on call %d, got %v", i, err)
		}
	}

	// Circuit should be open
	if cb.State() != StateOpen {
		t.Errorf("expected state Open after failures, got %v", cb.State())
	}

	// Next call should fail fast with ErrCircuitOpen
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          50 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state Open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Next call should transition to half-open
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected state HalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreakerRecovers(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          50 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Successful calls in half-open should close the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error on call %d: %v", i, err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          50 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Failure in half-open should reopen circuit
	err := cb.Execute(context.Background(), func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("expected test error, got %v", err)
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state Open after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenLimit(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          50 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Use up half-open calls
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return nil
		})
	}

	// Third call in half-open should fail with limit reached
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err == nil || !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen (half-open limit), got %v", err)
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	// Initial stats
	stats := cb.Stats()
	if stats.State != StateClosed {
		t.Errorf("expected initial state Closed, got %v", stats.State)
	}
	if stats.FailureCount != 0 {
		t.Errorf("expected initial failure count 0, got %d", stats.FailureCount)
	}

	// After failures
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	stats = cb.Stats()
	if stats.FailureCount != 2 {
		t.Errorf("expected failure count 2, got %d", stats.FailureCount)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state Open, got %v", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed after reset, got %v", cb.State())
	}

	stats := cb.Stats()
	if stats.FailureCount != 0 {
		t.Errorf("expected failure count 0 after reset, got %d", stats.FailureCount)
	}

	// Should be able to execute again
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error after reset: %v", err)
	}
}

func TestCircuitBreakerString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("state %v: expected %q, got %q", tt.state, tt.expected, got)
		}
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.FailureThreshold != 5 {
		t.Errorf("expected failure threshold 5, got %d", config.FailureThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", config.Timeout)
	}
	if config.HalfOpenMaxCalls != 3 {
		t.Errorf("expected half-open max calls 3, got %d", config.HalfOpenMaxCalls)
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	registry := NewCircuitBreakerRegistry(config)

	// Get circuit breaker
	cb1 := registry.Get("provider-1")
	if cb1 == nil {
		t.Fatal("expected non-nil circuit breaker")
	}

	// Get same instance
	cb2 := registry.Get("provider-1")
	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance")
	}

	// Get different instance
	cb3 := registry.Get("provider-2")
	if cb1 == cb3 {
		t.Error("expected different circuit breaker instance")
	}

	// Get all
	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 circuit breakers, got %d", len(all))
	}

	// Reset all
	cb1.Execute(context.Background(), func() error {
		return errors.New("test")
	})
	for i := 0; i < 5; i++ {
		cb1.Execute(context.Background(), func() error {
			return errors.New("test")
		})
	}

	if cb1.State() != StateOpen {
		t.Fatalf("expected state Open, got %v", cb1.State())
	}

	registry.Reset()

	if cb1.State() != StateClosed {
		t.Errorf("expected state Closed after reset, got %v", cb1.State())
	}
}

func TestCircuitBreakerSuccessResetsFailureCount(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker("test", config)

	testErr := errors.New("test error")

	// Fail twice
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	stats := cb.Stats()
	if stats.FailureCount != 2 {
		t.Errorf("expected failure count 2, got %d", stats.FailureCount)
	}

	// Success should reset failure count
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stats = cb.Stats()
	if stats.FailureCount != 0 {
		t.Errorf("expected failure count 0 after success, got %d", stats.FailureCount)
	}
}
