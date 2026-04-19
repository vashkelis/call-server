#!/usr/bin/env python3
"""
Session Simulator for Parlona Voice Engine

This script simulates a complete voice session lifecycle including:
- Session initialization
- Audio streaming (with configurable patterns)
- ASR transcript reception
- LLM response streaming
- TTS audio reception
- Interruption simulation
- Session teardown

Usage:
    python simulate-session.py --server ws://localhost:8080/ws --scenario full
    python simulate-session.py --server ws://localhost:8080/ws --scenario interrupt --interrupt-at 2.5

Requirements:
    pip install websockets
"""

import argparse
import asyncio
import base64
import json
import math
import random
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from enum import Enum, auto
from typing import Dict, List, Optional, Callable

import websockets
from websockets.exceptions import ConnectionClosed


class SessionState(Enum):
    """Session states."""
    IDLE = auto()
    CONNECTING = auto()
    STARTED = auto()
    LISTENING = auto()
    PROCESSING = auto()
    SPEAKING = auto()
    INTERRUPTED = auto()
    STOPPING = auto()
    STOPPED = auto()


@dataclass
class SessionMetrics:
    """Metrics collected during a session."""
    session_id: str
    start_time: Optional[float] = None
    end_time: Optional[float] = None
    audio_chunks_sent: int = 0
    audio_bytes_sent: int = 0
    asr_partials: int = 0
    asr_finals: int = 0
    llm_tokens: int = 0
    tts_chunks: int = 0
    tts_bytes: int = 0
    interruptions: int = 0
    errors: int = 0
    
    @property
    def duration(self) -> float:
        if self.start_time and self.end_time:
            return self.end_time - self.start_time
        return 0.0
    
    def print_summary(self):
        print("\n" + "=" * 50)
        print("SESSION METRICS SUMMARY")
        print("=" * 50)
        print(f"Session ID: {self.session_id}")
        print(f"Duration: {self.duration:.2f} seconds")
        print(f"Audio chunks sent: {self.audio_chunks_sent}")
        print(f"Audio bytes sent: {self.audio_bytes_sent}")
        print(f"ASR partials received: {self.asr_partials}")
        print(f"ASR finals received: {self.asr_finals}")
        print(f"LLM tokens received: {self.llm_tokens}")
        print(f"TTS chunks received: {self.tts_chunks}")
        print(f"TTS bytes received: {self.tts_bytes}")
        print(f"Interruptions: {self.interruptions}")
        print(f"Errors: {self.errors}")
        print("=" * 50)


class AudioPattern:
    """Base class for audio patterns."""
    
    def generate(self, sample_rate: int, duration_ms: int) -> bytes:
        """Generate audio data."""
        raise NotImplementedError


class SilencePattern(AudioPattern):
    """Generate silence."""
    
    def generate(self, sample_rate: int, duration_ms: int) -> bytes:
        num_samples = int(sample_rate * (duration_ms / 1000.0))
        return bytes(2 * num_samples)  # 16-bit samples


class SineWavePattern(AudioPattern):
    """Generate a sine wave."""
    
    def __init__(self, frequency: float = 440.0, amplitude: float = 0.3):
        self.frequency = frequency
        self.amplitude = amplitude
        self.phase = 0.0
    
    def generate(self, sample_rate: int, duration_ms: int) -> bytes:
        import struct
        
        num_samples = int(sample_rate * (duration_ms / 1000.0))
        samples = []
        
        for i in range(num_samples):
            sample = int(32767 * self.amplitude * 
                        math.sin(2 * math.pi * self.frequency * self.phase / sample_rate))
            samples.append(sample)
            self.phase += 1
        
        return struct.pack('<' + 'h' * len(samples), *samples)


class SpeechPattern(AudioPattern):
    """Generate synthetic speech-like audio (modulated noise)."""
    
    def __init__(self, amplitude: float = 0.5):
        self.amplitude = amplitude
        self.seed = random.randint(0, 10000)
    
    def generate(self, sample_rate: int, duration_ms: int) -> bytes:
        import struct
        
        num_samples = int(sample_rate * (duration_ms / 1000.0))
        samples = []
        
        random.seed(self.seed)
        
        # Generate modulated noise to simulate speech
        for i in range(num_samples):
            # Base noise
            noise = random.uniform(-1.0, 1.0)
            
            # Envelope to simulate syllables (5-10 Hz modulation)
            envelope = 0.5 + 0.5 * math.sin(2 * math.pi * 7 * i / sample_rate)
            
            # Apply envelope and amplitude
            sample = int(32767 * self.amplitude * envelope * noise)
            samples.append(sample)
        
        return struct.pack('<' + 'h' * len(samples), *samples)


class SessionSimulator:
    """Simulates a voice session."""
    
    def __init__(
        self,
        server_url: str,
        session_id: str,
        sample_rate: int = 16000,
        chunk_duration_ms: int = 100,
        verbose: bool = False
    ):
        self.server_url = server_url
        self.session_id = session_id
        self.sample_rate = sample_rate
        self.chunk_duration_ms = chunk_duration_ms
        self.verbose = verbose
        
        self.websocket: Optional[websockets.WebSocketClientProtocol] = None
        self.state = SessionState.IDLE
        self.metrics = SessionMetrics(session_id=session_id)
        self.state_handlers: Dict[SessionState, List[Callable]] = {}
        self.stop_event = asyncio.Event()
        self.interrupt_event = asyncio.Event()
        
    def on_state(self, state: SessionState, handler: Callable):
        """Register a handler for a state."""
        if state not in self.state_handlers:
            self.state_handlers[state] = []
        self.state_handlers[state].append(handler)
    
    def _set_state(self, new_state: SessionState):
        """Set the current state and trigger handlers."""
        old_state = self.state
        self.state = new_state
        
        if self.verbose:
            print(f"[State] {old_state.name} -> {new_state.name}")
        
        for handler in self.state_handlers.get(new_state, []):
            try:
                handler()
            except Exception as e:
                print(f"State handler error: {e}")
    
    def _create_event(self, event_type: str, **kwargs) -> dict:
        """Create a base event."""
        event = {
            "event": event_type,
            "session_id": self.session_id,
            "timestamp": datetime.now(timezone.utc).isoformat() + "Z",
            **kwargs
        }
        return event
    
    async def connect(self):
        """Connect to the WebSocket server."""
        self._set_state(SessionState.CONNECTING)
        self.websocket = await websockets.connect(self.server_url)
        self._set_state(SessionState.IDLE)
    
    async def start_session(self, system_prompt: str = "You are a helpful assistant."):
        """Start a new session."""
        event = self._create_event(
            "session.start",
            audio_format={
                "sample_rate": self.sample_rate,
                "channels": 1,
                "encoding": "PCM16"
            },
            system_prompt=system_prompt
        )
        await self.websocket.send(json.dumps(event))
        self._set_state(SessionState.STARTED)
        self.metrics.start_time = time.time()
    
    async def send_audio_chunk(self, audio_data: bytes, is_final: bool = False):
        """Send an audio chunk."""
        event = self._create_event(
            "audio.chunk",
            audio=base64.b64encode(audio_data).decode('utf-8'),
            is_final=is_final
        )
        await self.websocket.send(json.dumps(event))
        self.metrics.audio_chunks_sent += 1
        self.metrics.audio_bytes_sent += len(audio_data)
    
    async def send_interrupt(self):
        """Send an interruption event."""
        event = self._create_event("input.interrupt")
        await self.websocket.send(json.dumps(event))
        self.metrics.interruptions += 1
        self._set_state(SessionState.INTERRUPTED)
    
    async def stop_session(self):
        """Stop the session."""
        self._set_state(SessionState.STOPPING)
        event = self._create_event("session.stop")
        await self.websocket.send(json.dumps(event))
    
    async def stream_audio_pattern(
        self,
        pattern: AudioPattern,
        duration_seconds: float,
        interrupt_at: Optional[float] = None
    ):
        """Stream audio with a given pattern."""
        self._set_state(SessionState.LISTENING)
        
        num_chunks = int((duration_seconds * 1000) / self.chunk_duration_ms)
        
        for i in range(num_chunks):
            if self.stop_event.is_set():
                break
            
            # Check for interruption point
            if interrupt_at is not None:
                current_time = (i * self.chunk_duration_ms) / 1000.0
                if current_time >= interrupt_at and not self.interrupt_event.is_set():
                    print(f"[Simulator] Triggering interruption at {current_time:.2f}s")
                    await self.send_interrupt()
                    self.interrupt_event.set()
                    break
            
            audio_data = pattern.generate(self.sample_rate, self.chunk_duration_ms)
            await self.send_audio_chunk(audio_data)
            
            # Small delay to simulate real-time
            await asyncio.sleep(self.chunk_duration_ms / 1000.0)
    
    async def receive_events(self):
        """Receive and process events from the server."""
        try:
            async for message in self.websocket:
                try:
                    event = json.loads(message)
                    await self._handle_event(event)
                except json.JSONDecodeError:
                    print(f"[Raw] {message}")
        except ConnectionClosed:
            print("[Connection Closed]")
        finally:
            self.stop_event.set()
    
    async def _handle_event(self, event: dict):
        """Handle a server event."""
        event_type = event.get('event', 'unknown')
        
        if event_type == 'asr.partial':
            self.metrics.asr_partials += 1
            if self.verbose:
                print(f"[ASR Partial] {event.get('transcript', '')}")
        
        elif event_type == 'asr.final':
            self.metrics.asr_finals += 1
            transcript = event.get('transcript', '')
            print(f"[ASR Final] {transcript}")
            self._set_state(SessionState.PROCESSING)
        
        elif event_type == 'llm.partial':
            self.metrics.llm_tokens += 1
            if self.verbose:
                print(f"[LLM] {event.get('text', '')}", end='', flush=True)
        
        elif event_type == 'llm.final':
            if self.verbose:
                print()  # New line after streaming tokens
            print(f"[LLM Final] {event.get('text', '')}")
        
        elif event_type == 'tts.audio':
            self.metrics.tts_chunks += 1
            audio_data = event.get('audio', '')
            self.metrics.tts_bytes += len(audio_data)
            is_final = event.get('is_final', False)
            
            if is_final:
                print(f"[TTS] Final chunk received")
            elif self.verbose:
                chunk_index = event.get('chunk_index', 0)
                print(f"[TTS] Chunk {chunk_index}, {len(audio_data)} bytes")
            
            if self.state != SessionState.SPEAKING:
                self._set_state(SessionState.SPEAKING)
        
        elif event_type == 'turn':
            state = event.get('state', '')
            print(f"[Turn] {event.get('role')} -> {state}")
        
        elif event_type == 'error':
            self.metrics.errors += 1
            print(f"[Error] {event.get('message', 'Unknown error')}")
        
        elif event_type == 'session.stopped':
            print("[Session Stopped]")
            self._set_state(SessionState.STOPPED)
            self.stop_event.set()
    
    async def run_scenario(
        self,
        scenario: str,
        duration: float = 5.0,
        interrupt_at: Optional[float] = None
    ):
        """Run a simulation scenario."""
        print(f"\n{'=' * 50}")
        print(f"Running scenario: {scenario}")
        print(f"{'=' * 50}\n")
        
        # Connect
        await self.connect()
        
        # Start session
        await self.start_session()
        
        # Start receiving events in background
        receive_task = asyncio.create_task(self.receive_events())
        
        # Run scenario
        if scenario == 'full':
            # Full session: silence + speech + silence
            print("Phase 1: Silence (1s)")
            await self.stream_audio_pattern(SilencePattern(), 1.0)
            
            print("Phase 2: Speech pattern (3s)")
            await self.stream_audio_pattern(SpeechPattern(), 3.0)
            
            print("Phase 3: Silence (1s)")
            await self.stream_audio_pattern(SilencePattern(), 1.0)
        
        elif scenario == 'interrupt':
            # Interruption scenario
            print(f"Streaming speech with interruption at {interrupt_at}s")
            await self.stream_audio_pattern(
                SpeechPattern(),
                duration,
                interrupt_at=interrupt_at
            )
            
            # Wait a bit after interruption
            await asyncio.sleep(1.0)
            
            # Continue with more audio
            print("Continuing with more speech...")
            await self.stream_audio_pattern(SpeechPattern(), 2.0)
        
        elif scenario == 'sine':
            # Sine wave pattern
            print(f"Streaming sine wave ({duration}s)")
            await self.stream_audio_pattern(SineWavePattern(440, 0.3), duration)
        
        elif scenario == 'multi-turn':
            # Multiple turns
            for i in range(3):
                print(f"Turn {i + 1}: Speech pattern (2s)")
                await self.stream_audio_pattern(SpeechPattern(), 2.0)
                await asyncio.sleep(2.0)  # Wait for response
        
        else:
            print(f"Unknown scenario: {scenario}")
        
        # Stop session
        await self.stop_session()
        
        # Wait for completion
        try:
            await asyncio.wait_for(self.stop_event.wait(), timeout=10.0)
        except asyncio.TimeoutError:
            print("Timeout waiting for session to stop")
        
        # Cleanup
        receive_task.cancel()
        try:
            await receive_task
        except asyncio.CancelledError:
            pass
        
        self.metrics.end_time = time.time()
        self.metrics.print_summary()


async def main():
    parser = argparse.ArgumentParser(
        description='Session Simulator for Parlona Voice Engine',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Scenarios:
  full          - Complete session with silence, speech, and silence phases
  interrupt     - Session with interruption at specified time
  sine          - Session with sine wave audio
  multi-turn    - Multiple back-and-forth turns

Examples:
  # Run full scenario
  python simulate-session.py --server ws://localhost:8080/ws --scenario full

  # Simulate interruption at 2.5 seconds
  python simulate-session.py --server ws://localhost:8080/ws --scenario interrupt --interrupt-at 2.5

  # Custom duration sine wave
  python simulate-session.py --server ws://localhost:8080/ws --scenario sine --duration 10
        """
    )
    
    parser.add_argument(
        '--server', '-s',
        default='ws://localhost:8080/ws',
        help='WebSocket server URL (default: ws://localhost:8080/ws)'
    )
    
    parser.add_argument(
        '--session-id',
        default=None,
        help='Session ID (default: auto-generated)'
    )
    
    parser.add_argument(
        '--scenario',
        default='full',
        choices=['full', 'interrupt', 'sine', 'multi-turn'],
        help='Simulation scenario (default: full)'
    )
    
    parser.add_argument(
        '--duration',
        type=float,
        default=5.0,
        help='Session duration in seconds (default: 5.0)'
    )
    
    parser.add_argument(
        '--interrupt-at',
        type=float,
        default=None,
        help='Time in seconds to trigger interruption (for interrupt scenario)'
    )
    
    parser.add_argument(
        '--sample-rate',
        type=int,
        default=16000,
        help='Audio sample rate in Hz (default: 16000)'
    )
    
    parser.add_argument(
        '--chunk-duration',
        type=int,
        default=100,
        help='Audio chunk duration in ms (default: 100)'
    )
    
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Enable verbose output'
    )
    
    args = parser.parse_args()
    
    # Generate session ID if not provided
    if args.session_id is None:
        import uuid
        args.session_id = f"sim-{uuid.uuid4().hex[:8]}"
    
    # Validate arguments
    if args.scenario == 'interrupt' and args.interrupt_at is None:
        args.interrupt_at = args.duration / 2
        print(f"No --interrupt-at specified, using default: {args.interrupt_at}s")
    
    # Create simulator
    simulator = SessionSimulator(
        server_url=args.server,
        session_id=args.session_id,
        sample_rate=args.sample_rate,
        chunk_duration_ms=args.chunk_duration,
        verbose=args.verbose
    )
    
    # Run scenario
    try:
        await simulator.run_scenario(
            scenario=args.scenario,
            duration=args.duration,
            interrupt_at=args.interrupt_at
        )
    except KeyboardInterrupt:
        print("\nInterrupted by user")
        simulator.stop_event.set()
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == '__main__':
    asyncio.run(main())
