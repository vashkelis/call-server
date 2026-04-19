package statemachine

import (
	"testing"

	"github.com/parlona/cloudapp/pkg/session"
)

func TestValidTransitions(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Verify initial state
	if fsm.Current() != session.StateIdle {
		t.Errorf("expected initial state Idle, got %v", fsm.Current())
	}

	// Idle -> Listening (via SpeechStartEvent)
	err := fsm.Transition(SpeechStartEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateListening {
		t.Errorf("expected state Listening, got %v", fsm.Current())
	}

	// Listening -> Processing (via SpeechEndEvent)
	err = fsm.Transition(SpeechEndEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateProcessing {
		t.Errorf("expected state Processing, got %v", fsm.Current())
	}

	// Processing -> Speaking (via FirstTTSAudioEvent)
	err = fsm.Transition(FirstTTSAudioEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateSpeaking {
		t.Errorf("expected state Speaking, got %v", fsm.Current())
	}

	// Speaking -> Idle (via BotFinishedEvent)
	err = fsm.Transition(BotFinishedEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateIdle {
		t.Errorf("expected state Idle, got %v", fsm.Current())
	}
}

func TestInvalidTransition(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Idle -> Speaking is invalid (FirstTTSAudioEvent from Idle is a no-op)
	err := fsm.Transition(FirstTTSAudioEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify state unchanged (no-op transition)
	if fsm.Current() != session.StateIdle {
		t.Errorf("expected state to remain Idle, got %v", fsm.Current())
	}

	// Idle -> Processing is invalid (ASRFinalEvent from Idle is a no-op)
	err = fsm.Transition(ASRFinalEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify state unchanged (no-op transition)
	if fsm.Current() != session.StateIdle {
		t.Errorf("expected state to remain Idle, got %v", fsm.Current())
	}
}

func TestInterruptionTransition(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Speaking state
	fsm.Transition(SpeechStartEvent)   // Idle -> Listening
	fsm.Transition(SpeechEndEvent)     // Listening -> Processing
	fsm.Transition(FirstTTSAudioEvent) // Processing -> Speaking

	if fsm.Current() != session.StateSpeaking {
		t.Fatalf("expected state Speaking, got %v", fsm.Current())
	}

	// Speaking -> Interrupted
	err := fsm.Transition(InterruptionEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateInterrupted {
		t.Errorf("expected state Interrupted, got %v", fsm.Current())
	}

	// Interrupted -> Listening
	err = fsm.Transition(SpeechStartEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateListening {
		t.Errorf("expected state Listening, got %v", fsm.Current())
	}
}

func TestOnEnterOnExitHooks(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	var enterCalls []session.SessionState
	var exitCalls []session.SessionState

	// Set up enter hook for Listening state
	fsm.SetOnEnter(session.StateListening, func(change StateChange) {
		enterCalls = append(enterCalls, change.To)
	})

	// Set up exit hook for Idle state
	fsm.SetOnExit(session.StateIdle, func(change StateChange) {
		exitCalls = append(exitCalls, change.From)
	})

	// Set up general transition hook
	var transitionCalls []StateChange
	fsm.SetOnTransition(func(change StateChange) {
		transitionCalls = append(transitionCalls, change)
	})

	// Transition Idle -> Listening
	err := fsm.Transition(SpeechStartEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hooks were called
	if len(enterCalls) != 1 || enterCalls[0] != session.StateListening {
		t.Errorf("expected enter hook called for Listening, got %v", enterCalls)
	}

	if len(exitCalls) != 1 || exitCalls[0] != session.StateIdle {
		t.Errorf("expected exit hook called for Idle, got %v", exitCalls)
	}

	if len(transitionCalls) != 1 {
		t.Errorf("expected 1 transition call, got %d", len(transitionCalls))
	}
	if transitionCalls[0].From != session.StateIdle || transitionCalls[0].To != session.StateListening {
		t.Errorf("expected transition Idle->Listening, got %v->%v",
			transitionCalls[0].From, transitionCalls[0].To)
	}
}

func TestCanTransition(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// From Idle, can transition to Listening
	if !fsm.CanTransition(session.StateListening) {
		t.Error("expected CanTransition to return true for Idle->Listening")
	}

	// From Idle, cannot transition to Speaking
	if fsm.CanTransition(session.StateSpeaking) {
		t.Error("expected CanTransition to return false for Idle->Speaking")
	}

	// Move to Speaking
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)

	// From Speaking, can transition to Interrupted
	if !fsm.CanTransition(session.StateInterrupted) {
		t.Error("expected CanTransition to return true for Speaking->Interrupted")
	}
}

func TestIsActive(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Idle is not active
	if fsm.IsActive() {
		t.Error("expected IsActive to be false for Idle")
	}

	// Move to Listening
	fsm.Transition(SpeechStartEvent)
	if !fsm.IsActive() {
		t.Error("expected IsActive to be true for Listening")
	}
}

func TestIsProcessing(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Idle is not processing
	if fsm.IsProcessing() {
		t.Error("expected IsProcessing to be false for Idle")
	}

	// Listening is not processing
	fsm.Transition(SpeechStartEvent)
	if fsm.IsProcessing() {
		t.Error("expected IsProcessing to be false for Listening")
	}

	// Processing is processing
	fsm.Transition(SpeechEndEvent)
	if !fsm.IsProcessing() {
		t.Error("expected IsProcessing to be true for Processing")
	}

	// Speaking is processing
	fsm.Transition(FirstTTSAudioEvent)
	if !fsm.IsProcessing() {
		t.Error("expected IsProcessing to be true for Speaking")
	}
}

func TestIsListening(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	if fsm.IsListening() {
		t.Error("expected IsListening to be false for Idle")
	}

	fsm.Transition(SpeechStartEvent)
	if !fsm.IsListening() {
		t.Error("expected IsListening to be true for Listening")
	}
}

func TestIsSpeaking(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	if fsm.IsSpeaking() {
		t.Error("expected IsSpeaking to be false for Idle")
	}

	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)

	if !fsm.IsSpeaking() {
		t.Error("expected IsSpeaking to be true for Speaking")
	}
}

func TestIsInterrupted(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	if fsm.IsInterrupted() {
		t.Error("expected IsInterrupted to be false for Idle")
	}

	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)
	fsm.Transition(InterruptionEvent)

	if !fsm.IsInterrupted() {
		t.Error("expected IsInterrupted to be true for Interrupted")
	}
}

func TestReset(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Move through some states
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)

	if fsm.Current() != session.StateSpeaking {
		t.Fatalf("expected state Speaking, got %v", fsm.Current())
	}

	// Reset
	fsm.Reset()

	if fsm.Current() != session.StateIdle {
		t.Errorf("expected state Idle after reset, got %v", fsm.Current())
	}
}

func TestString(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	if fsm.String() != "idle" {
		t.Errorf("expected string 'idle', got %s", fsm.String())
	}

	fsm.Transition(SpeechStartEvent)
	if fsm.String() != "listening" {
		t.Errorf("expected string 'listening', got %s", fsm.String())
	}
}

func TestSessionStopEvent(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Move to Speaking
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)

	// SessionStopEvent should transition to Idle from any state
	err := fsm.Transition(SessionStopEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateIdle {
		t.Errorf("expected state Idle, got %v", fsm.Current())
	}
}

func TestFSMManager(t *testing.T) {
	manager := NewFSMManager()
	eventSink := make(chan<- interface{}, 10)

	// Get or create FSM
	fsm1 := manager.GetOrCreate("session-1", eventSink)
	if fsm1 == nil {
		t.Fatal("expected non-nil FSM")
	}

	// Get same FSM again
	fsm2 := manager.GetOrCreate("session-1", eventSink)
	if fsm1 != fsm2 {
		t.Error("expected same FSM instance for same session")
	}

	// Get different FSM
	fsm3 := manager.GetOrCreate("session-2", eventSink)
	if fsm1 == fsm3 {
		t.Error("expected different FSM instance for different session")
	}

	// Get existing FSM
	existing, ok := manager.Get("session-1")
	if !ok {
		t.Error("expected to find existing FSM")
	}
	if existing != fsm1 {
		t.Error("expected retrieved FSM to match original")
	}

	// Get non-existent FSM
	_, ok = manager.Get("nonexistent")
	if ok {
		t.Error("expected not to find non-existent FSM")
	}

	// List sessions
	sessions := manager.List()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Remove FSM
	manager.Remove("session-1")
	_, ok = manager.Get("session-1")
	if ok {
		t.Error("expected FSM to be removed")
	}
}

func TestASRFinalEventTransition(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Idle -> Listening
	fsm.Transition(SpeechStartEvent)

	// Listening -> Processing (via ASRFinalEvent)
	err := fsm.Transition(ASRFinalEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateProcessing {
		t.Errorf("expected state Processing, got %v", fsm.Current())
	}
}

func TestSpeechStartFromSpeaking(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Speaking
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)

	// SpeechStartEvent from Speaking should go to Listening (barge-in)
	err := fsm.Transition(SpeechStartEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateListening {
		t.Errorf("expected state Listening after barge-in, got %v", fsm.Current())
	}
}

func TestSpeechStartFromInterrupted(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Interrupted
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)
	fsm.Transition(FirstTTSAudioEvent)
	fsm.Transition(InterruptionEvent)

	// SpeechStartEvent from Interrupted should go to Listening
	err := fsm.Transition(SpeechStartEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateListening {
		t.Errorf("expected state Listening, got %v", fsm.Current())
	}
}

func TestInterruptionFromProcessing(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Processing
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(SpeechEndEvent)

	// InterruptionEvent from Processing should go to Interrupted
	err := fsm.Transition(InterruptionEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateInterrupted {
		t.Errorf("expected state Interrupted, got %v", fsm.Current())
	}
}

func TestInterruptionFromListening(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Listening
	fsm.Transition(SpeechStartEvent)

	// InterruptionEvent from Listening should go to Interrupted
	err := fsm.Transition(InterruptionEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateInterrupted {
		t.Errorf("expected state Interrupted, got %v", fsm.Current())
	}
}

func TestInterruptedToProcessing(t *testing.T) {
	eventSink := make(chan<- interface{}, 10)
	fsm := NewSessionFSM("session-123", eventSink)

	// Go to Interrupted
	fsm.Transition(SpeechStartEvent)
	fsm.Transition(InterruptionEvent)

	// Interrupted -> Processing
	err := fsm.Transition(ASRFinalEvent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fsm.Current() != session.StateProcessing {
		t.Errorf("expected state Processing, got %v", fsm.Current())
	}
}
