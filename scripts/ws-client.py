#!/usr/bin/env python3
"""
WebSocket Client Example for Parlona Voice Engine

This script demonstrates how to connect to the voice engine WebSocket endpoint,
stream audio, and receive transcription and TTS responses.

Usage:
    python ws-client.py --server ws://localhost:8080/ws --audio-file input.wav
    python ws-client.py --server ws://localhost:8080/ws --synthetic

Requirements:
    pip install websockets
"""

import argparse
import asyncio
import base64
import json
import sys
import wave
from datetime import datetime
from typing import Optional

import websockets
from websockets.exceptions import ConnectionClosed


def create_session_start_event(session_id: str) -> dict:
    """Create a session.start event."""
    return {
        "event": "session.start",
        "session_id": session_id,
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "audio_format": {
            "sample_rate": 16000,
            "channels": 1,
            "encoding": "PCM16"
        },
        "system_prompt": "You are a helpful voice assistant."
    }


def create_audio_chunk_event(session_id: str, audio_data: bytes, is_final: bool = False) -> dict:
    """Create an audio.chunk event with base64-encoded audio."""
    return {
        "event": "audio.chunk",
        "session_id": session_id,
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "audio": base64.b64encode(audio_data).decode('utf-8'),
        "is_final": is_final
    }


def create_session_stop_event(session_id: str) -> dict:
    """Create a session.stop event."""
    return {
        "event": "session.stop",
        "session_id": session_id,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }


def create_interrupt_event(session_id: str) -> dict:
    """Create an input.interrupt event."""
    return {
        "event": "input.interrupt",
        "session_id": session_id,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }


async def read_audio_file(file_path: str, chunk_duration_ms: int = 100):
    """Read a WAV file and yield audio chunks."""
    with wave.open(file_path, 'rb') as wav_file:
        sample_rate = wav_file.getframerate()
        channels = wav_file.getnchannels()
        sample_width = wav_file.getsampwidth()
        
        print(f"Audio file: {file_path}")
        print(f"  Sample rate: {sample_rate} Hz")
        print(f"  Channels: {channels}")
        print(f"  Sample width: {sample_width} bytes")
        print(f"  Duration: {wav_file.getnframes() / sample_rate:.2f} seconds")
        
        # Calculate chunk size for the desired duration
        frames_per_chunk = int(sample_rate * (chunk_duration_ms / 1000.0))
        chunk_size = frames_per_chunk * channels * sample_width
        
        while True:
            data = wav_file.readframes(frames_per_chunk)
            if not data:
                break
            yield data


async def generate_synthetic_audio(duration_seconds: float = 5.0, chunk_duration_ms: int = 100):
    """Generate synthetic audio (sine wave) for testing."""
    import math
    
    sample_rate = 16000
    frequency = 440  # A4 note
    
    total_frames = int(sample_rate * duration_seconds)
    frames_per_chunk = int(sample_rate * (chunk_duration_ms / 1000.0))
    
    frames_generated = 0
    while frames_generated < total_frames:
        chunk_frames = min(frames_per_chunk, total_frames - frames_generated)
        
        # Generate sine wave samples (16-bit PCM)
        samples = []
        for i in range(chunk_frames):
            sample = int(32767 * 0.3 * math.sin(2 * math.pi * frequency * (frames_generated + i) / sample_rate))
            samples.append(sample)
        
        # Convert to bytes
        import struct
        audio_data = struct.pack('<' + 'h' * len(samples), *samples)
        
        frames_generated += chunk_frames
        yield audio_data


async def receive_events(websocket, session_id: str, stop_event: asyncio.Event):
    """Receive and print events from the server."""
    try:
        async for message in websocket:
            try:
                event = json.loads(message)
                event_type = event.get('event', 'unknown')
                
                if event_type == 'asr.partial':
                    print(f"[ASR Partial] {event.get('transcript', '')}")
                elif event_type == 'asr.final':
                    print(f"[ASR Final] {event.get('transcript', '')}")
                elif event_type == 'llm.partial':
                    print(f"[LLM Partial] {event.get('text', '')}", end='', flush=True)
                elif event_type == 'llm.final':
                    print(f"\n[LLM Final] {event.get('text', '')}")
                elif event_type == 'tts.audio':
                    chunk_index = event.get('chunk_index', 0)
                    is_final = event.get('is_final', False)
                    audio_data = event.get('audio', '')
                    print(f"[TTS Audio] Chunk {chunk_index}, size: {len(audio_data)} bytes, final: {is_final}")
                elif event_type == 'turn':
                    print(f"[Turn] Role: {event.get('role')}, State: {event.get('state')}")
                elif event_type == 'error':
                    print(f"[Error] {event.get('message', 'Unknown error')}")
                elif event_type == 'session.stopped':
                    print(f"[Session Stopped] {event.get('session_id')}")
                    stop_event.set()
                else:
                    print(f"[{event_type}] {json.dumps(event, indent=2)}")
                    
            except json.JSONDecodeError:
                print(f"[Raw Message] {message}")
    except ConnectionClosed:
        print("[Connection Closed]")
        stop_event.set()


async def stream_audio(websocket, session_id: str, audio_source, stop_event: asyncio.Event):
    """Stream audio chunks to the server."""
    chunk_count = 0
    async for audio_chunk in audio_source:
        if stop_event.is_set():
            break
            
        event = create_audio_chunk_event(session_id, audio_chunk)
        await websocket.send(json.dumps(event))
        chunk_count += 1
        
        # Small delay to simulate real-time streaming
        await asyncio.sleep(0.01)
    
    print(f"Sent {chunk_count} audio chunks")


async def run_client(
    server_url: str,
    session_id: str,
    audio_file: Optional[str],
    synthetic_duration: float,
    verbose: bool = False
):
    """Run the WebSocket client."""
    print(f"Connecting to {server_url}...")
    
    try:
        async with websockets.connect(server_url) as websocket:
            print(f"Connected! Session ID: {session_id}")
            
            # Send session start
            start_event = create_session_start_event(session_id)
            await websocket.send(json.dumps(start_event))
            print("Sent session.start event")
            
            # Create stop event for signaling
            stop_event = asyncio.Event()
            
            # Create audio source
            if audio_file:
                audio_source = read_audio_file(audio_file)
            else:
                audio_source = generate_synthetic_audio(synthetic_duration)
                print(f"Generating synthetic audio ({synthetic_duration}s)...")
            
            # Start receiving and streaming concurrently
            receive_task = asyncio.create_task(
                receive_events(websocket, session_id, stop_event)
            )
            stream_task = asyncio.create_task(
                stream_audio(websocket, session_id, audio_source, stop_event)
            )
            
            # Wait for streaming to complete
            await stream_task
            print("Audio streaming complete")
            
            # Send session stop
            stop_event_msg = create_session_stop_event(session_id)
            await websocket.send(json.dumps(stop_event_msg))
            print("Sent session.stop event")
            
            # Wait a bit for final events
            await asyncio.wait_for(stop_event.wait(), timeout=5.0)
            
            # Cancel receive task
            receive_task.cancel()
            try:
                await receive_task
            except asyncio.CancelledError:
                pass
                
    except websockets.exceptions.InvalidURI:
        print(f"Error: Invalid WebSocket URI: {server_url}")
        sys.exit(1)
    except websockets.exceptions.InvalidHandshake:
        print(f"Error: Could not connect to {server_url}")
        print("Make sure the server is running and the URL is correct.")
        sys.exit(1)
    except asyncio.TimeoutError:
        print("Error: Connection timed out")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description='WebSocket client for Parlona Voice Engine',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Connect with synthetic audio
  python ws-client.py --server ws://localhost:8080/ws

  # Connect with a WAV file
  python ws-client.py --server ws://localhost:8080/ws --audio-file input.wav

  # Custom session ID and longer synthetic audio
  python ws-client.py --server ws://localhost:8080/ws --session-id my-session --synthetic-duration 10
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
        '--audio-file', '-a',
        default=None,
        help='Path to WAV audio file to stream'
    )
    
    parser.add_argument(
        '--synthetic-duration',
        type=float,
        default=3.0,
        help='Duration of synthetic audio in seconds (default: 3.0)'
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
        args.session_id = f"ws-client-{uuid.uuid4().hex[:8]}"
    
    # Validate arguments
    if args.audio_file and not args.audio_file.endswith('.wav'):
        print("Warning: Audio file should be a WAV file")
    
    # Run the client
    try:
        asyncio.run(run_client(
            server_url=args.server,
            session_id=args.session_id,
            audio_file=args.audio_file,
            synthetic_duration=args.synthetic_duration,
            verbose=args.verbose
        ))
    except KeyboardInterrupt:
        print("\nInterrupted by user")
        sys.exit(0)


if __name__ == '__main__':
    main()
