// Package statemachine provides session state management and turn tracking.
package statemachine

import (
	"fmt"
	"sync"

	"github.com/parlona/cloudapp/pkg/events"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/session"
)

// EventType represents a state machine event.
type EventType string

const (
	// SpeechStartEvent is triggered when speech is detected.
	SpeechStartEvent EventType = "speech_start"
	// SpeechEndEvent is triggered when speech ends.
	SpeechEndEvent EventType = "speech_end"
	// ASRFinalEvent is triggered when ASR produces a final transcript.
	ASRFinalEvent EventType = "asr_final"
	// FirstTTSAudioEvent is triggered when the first TTS audio chunk is ready.
	FirstTTSAudioEvent EventType = "first_tts_audio"
	// InterruptionEvent is triggered when an interruption is detected.
	InterruptionEvent EventType = "interruption"
	// BotFinishedEvent is triggered when the bot finishes speaking.
	BotFinishedEvent EventType = "bot_finished"
	// SessionStopEvent is triggered when the session is stopped.
	SessionStopEvent EventType = "session_stop"
)

// StateChange represents a state transition.
type StateChange struct {
	From      session.SessionState
	To        session.SessionState
	Event     EventType
	Timestamp int64
}

// StateChangeHandler is called when the state changes.
type StateChangeHandler func(change StateChange)

// SessionFSM manages session state with valid transitions.
type SessionFSM struct {
	mu               sync.RWMutex
	currentState     session.SessionState
	validTransitions map[session.SessionState][]session.SessionState
	onEnter          map[session.SessionState]StateChangeHandler
	onExit           map[session.SessionState]StateChangeHandler
	onTransition     StateChangeHandler
	sessionID        string
	eventSink        chan<- interface{}
}

// NewSessionFSM creates a new session state machine starting in Idle state.
func NewSessionFSM(sessionID string, eventSink chan<- interface{}) *SessionFSM {
	fsm := &SessionFSM{
		currentState: session.StateIdle,
		sessionID:    sessionID,
		eventSink:    eventSink,
		validTransitions: map[session.SessionState][]session.SessionState{
			session.StateIdle: {
				session.StateListening,
			},
			session.StateListening: {
				session.StateProcessing,
				session.StateIdle,
				session.StateInterrupted,
			},
			session.StateProcessing: {
				session.StateSpeaking,
				session.StateIdle,
				session.StateInterrupted,
			},
			session.StateSpeaking: {
				session.StateListening,
				session.StateIdle,
				session.StateInterrupted,
			},
			session.StateInterrupted: {
				session.StateListening,
				session.StateIdle,
				session.StateProcessing,
			},
		},
		onEnter: make(map[session.SessionState]StateChangeHandler),
		onExit:  make(map[session.SessionState]StateChangeHandler),
	}

	return fsm
}

// Current returns the current state.
func (fsm *SessionFSM) Current() session.SessionState {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState
}

// Transition attempts to transition to a new state based on an event.
// Returns error on invalid transition.
func (fsm *SessionFSM) Transition(event EventType) error {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	targetState := fsm.resolveTargetState(event)
	if targetState == fsm.currentState {
		// No transition needed
		return nil
	}

	// Validate transition
	if !fsm.isValidTransition(fsm.currentState, targetState) {
		return fmt.Errorf("invalid transition from %s to %s on event %s",
			fsm.currentState, targetState, event)
	}

	// Execute exit handler for current state
	if handler, ok := fsm.onExit[fsm.currentState]; ok {
		handler(StateChange{
			From:      fsm.currentState,
			To:        targetState,
			Event:     event,
			Timestamp: events.Now(),
		})
	}

	// Perform transition
	fromState := fsm.currentState
	fsm.currentState = targetState

	change := StateChange{
		From:      fromState,
		To:        targetState,
		Event:     event,
		Timestamp: events.Now(),
	}

	// Execute enter handler for new state
	if handler, ok := fsm.onEnter[targetState]; ok {
		handler(change)
	}

	// Execute general transition handler
	if fsm.onTransition != nil {
		fsm.onTransition(change)
	}

	// Emit state change event
	if fsm.eventSink != nil {
		turnEvent := events.NewTurnEvent(fsm.sessionID, "assistant", targetState.String())
		select {
		case fsm.eventSink <- turnEvent:
		default:
			// Don't block if event sink is full
		}
	}

	return nil
}

// resolveTargetState determines the target state based on the event.
func (fsm *SessionFSM) resolveTargetState(event EventType) session.SessionState {
	switch event {
	case SpeechStartEvent:
		if fsm.currentState == session.StateIdle ||
			fsm.currentState == session.StateSpeaking ||
			fsm.currentState == session.StateInterrupted {
			return session.StateListening
		}

	case SpeechEndEvent, ASRFinalEvent:
		if fsm.currentState == session.StateListening {
			return session.StateProcessing
		}

	case FirstTTSAudioEvent:
		if fsm.currentState == session.StateProcessing {
			return session.StateSpeaking
		}

	case InterruptionEvent:
		if fsm.currentState == session.StateSpeaking ||
			fsm.currentState == session.StateProcessing ||
			fsm.currentState == session.StateListening {
			return session.StateInterrupted
		}

	case BotFinishedEvent:
		if fsm.currentState == session.StateSpeaking {
			return session.StateIdle
		}

	case SessionStopEvent:
		return session.StateIdle
	}

	return fsm.currentState
}

// isValidTransition checks if a transition is valid.
func (fsm *SessionFSM) isValidTransition(from, to session.SessionState) bool {
	if from == to {
		return true
	}

	allowed, ok := fsm.validTransitions[from]
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

// SetOnEnter sets a handler to be called when entering a state.
func (fsm *SessionFSM) SetOnEnter(state session.SessionState, handler StateChangeHandler) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.onEnter[state] = handler
}

// SetOnExit sets a handler to be called when exiting a state.
func (fsm *SessionFSM) SetOnExit(state session.SessionState, handler StateChangeHandler) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.onExit[state] = handler
}

// SetOnTransition sets a handler to be called on any state transition.
func (fsm *SessionFSM) SetOnTransition(handler StateChangeHandler) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.onTransition = handler
}

// CanTransition checks if a transition to the target state is possible.
func (fsm *SessionFSM) CanTransition(target session.SessionState) bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.isValidTransition(fsm.currentState, target)
}

// IsActive returns true if the session is in an active state (not idle).
func (fsm *SessionFSM) IsActive() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState != session.StateIdle
}

// IsProcessing returns true if the session is processing or speaking.
func (fsm *SessionFSM) IsProcessing() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState == session.StateProcessing ||
		fsm.currentState == session.StateSpeaking
}

// IsListening returns true if the session is listening.
func (fsm *SessionFSM) IsListening() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState == session.StateListening
}

// IsSpeaking returns true if the session is speaking.
func (fsm *SessionFSM) IsSpeaking() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState == session.StateSpeaking
}

// IsInterrupted returns true if the session is interrupted.
func (fsm *SessionFSM) IsInterrupted() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState == session.StateInterrupted
}

// Reset resets the state machine to idle.
func (fsm *SessionFSM) Reset() {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	oldState := fsm.currentState
	fsm.currentState = session.StateIdle

	if fsm.onTransition != nil {
		fsm.onTransition(StateChange{
			From:      oldState,
			To:        session.StateIdle,
			Event:     SessionStopEvent,
			Timestamp: events.Now(),
		})
	}
}

// String returns the string representation of the current state.
func (fsm *SessionFSM) String() string {
	return fsm.Current().String()
}

// FSMManager manages FSMs for multiple sessions.
type FSMManager struct {
	mu   sync.RWMutex
	fsms map[string]*SessionFSM
}

// NewFSMManager creates a new FSM manager.
func NewFSMManager() *FSMManager {
	return &FSMManager{
		fsms: make(map[string]*SessionFSM),
	}
}

// GetOrCreate gets an existing FSM or creates a new one.
func (m *FSMManager) GetOrCreate(sessionID string, eventSink chan<- interface{}) *SessionFSM {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fsm, ok := m.fsms[sessionID]; ok {
		return fsm
	}

	fsm := NewSessionFSM(sessionID, eventSink)
	m.fsms[sessionID] = fsm
	return fsm
}

// Get gets an existing FSM.
func (m *FSMManager) Get(sessionID string) (*SessionFSM, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fsm, ok := m.fsms[sessionID]
	return fsm, ok
}

// Remove removes an FSM.
func (m *FSMManager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fsms, sessionID)
}

// List returns all session IDs with FSMs.
func (m *FSMManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.fsms))
	for id := range m.fsms {
		ids = append(ids, id)
	}
	return ids
}

// ensure observability is imported
var _ = observability.NewTimestampTracker
