package session

import (
	"errors"
	"fmt"
)

// SessionState represents the current state of a session.
type SessionState int

const (
	StateIdle SessionState = iota
	StateListening
	StateProcessing
	StateSpeaking
	StateInterrupted
)

// String returns the string representation of the state.
func (s SessionState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateProcessing:
		return "processing"
	case StateSpeaking:
		return "speaking"
	case StateInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// ValidTransitions defines which state transitions are allowed.
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

// IsValidTransition checks if a state transition is valid.
func IsValidTransition(from, to SessionState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}

// ErrInvalidStateTransition is returned when an invalid state transition is attempted.
var ErrInvalidStateTransition = errors.New("invalid state transition")

// StateMachine manages session state transitions.
type StateMachine struct {
	currentState SessionState
	onTransition func(from, to SessionState)
}

// NewStateMachine creates a new state machine starting in the idle state.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		currentState: StateIdle,
	}
}

// SetOnTransition sets a callback for state transitions.
func (sm *StateMachine) SetOnTransition(fn func(from, to SessionState)) {
	sm.onTransition = fn
}

// Current returns the current state.
func (sm *StateMachine) Current() SessionState {
	return sm.currentState
}

// Transition attempts to transition to a new state.
func (sm *StateMachine) Transition(to SessionState) error {
	if !IsValidTransition(sm.currentState, to) {
		return fmt.Errorf("cannot transition from %s to %s: %w", sm.currentState, to, ErrInvalidStateTransition)
	}

	from := sm.currentState
	sm.currentState = to

	if sm.onTransition != nil {
		sm.onTransition(from, to)
	}

	return nil
}

// CanTransition checks if a transition to the given state is possible.
func (sm *StateMachine) CanTransition(to SessionState) bool {
	return IsValidTransition(sm.currentState, to)
}

// Reset resets the state machine to idle.
func (sm *StateMachine) Reset() {
	oldState := sm.currentState
	sm.currentState = StateIdle
	if sm.onTransition != nil {
		sm.onTransition(oldState, StateIdle)
	}
}

// IsActive returns true if the session is in an active state (not idle).
func (sm *StateMachine) IsActive() bool {
	return sm.currentState != StateIdle
}

// IsProcessing returns true if the session is processing or speaking.
func (sm *StateMachine) IsProcessing() bool {
	return sm.currentState == StateProcessing || sm.currentState == StateSpeaking
}

// IsListening returns true if the session is listening.
func (sm *StateMachine) IsListening() bool {
	return sm.currentState == StateListening
}

// IsSpeaking returns true if the session is speaking.
func (sm *StateMachine) IsSpeaking() bool {
	return sm.currentState == StateSpeaking
}
