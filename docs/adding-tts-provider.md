# Adding a TTS Provider

This guide walks you through adding a new Text-to-Speech (TTS) provider to CloudApp.

## Overview

TTS providers convert text into audio streams. They must implement the `BaseTTSProvider` abstract class and support streaming audio output in PCM16 format with cancellation support.

## Step-by-Step Guide

### 1. Create Provider File

Create a new file in `/py/provider_gateway/app/providers/tts/`:

```bash
touch /py/provider_gateway/app/providers/tts/my_tts_provider.py
```

### 2. Implement BaseTTSProvider

```python
"""My Custom TTS Provider."""

import asyncio
import logging
import struct
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseTTSProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.common import AudioFormat, AudioEncoding, SessionContext, TimingMetadata
from app.models.tts import TTSOptions, TTSResponse

logger = logging.getLogger(__name__)


class MyTTSProvider(BaseTTSProvider):
    """Custom TTS provider implementation."""

    # Audio output format
    SAMPLE_RATE = 24000  # Common for high-quality TTS
    CHANNELS = 1
    BYTES_PER_SAMPLE = 2  # PCM16 = 16-bit = 2 bytes

    def __init__(
        self,
        api_key: Optional[str] = None,
        base_url: str = "https://api.example.com/v1",
        voice_id: str = "default",
        chunk_delay_ms: float = 50.0,
    ) -> None:
        """
        Initialize the TTS provider.

        Args:
            api_key: API key for authentication.
            base_url: Base URL for the API endpoint.
            voice_id: Default voice ID to use.
            chunk_delay_ms: Delay between audio chunks for streaming simulation.
        """
        self._api_key = api_key
        self._base_url = base_url
        self._voice_id = voice_id
        self._chunk_delay_ms = chunk_delay_ms
        self._cancelled_sessions: set = set()

        logger.info(f"MyTTSProvider initialized with voice={voice_id}")

    def name(self) -> str:
        """Return the provider name."""
        return "my_tts_provider"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,     # TTS receives complete text
            supports_streaming_output=True,     # Supports streaming audio
            supports_word_timestamps=False,     # Not typically supported
            supports_voices=True,               # Supports multiple voices
            supports_interruptible_generation=True,  # Supports cancellation
            preferred_sample_rates=[24000, 16000, 22050],
            supported_codecs=["pcm16"],         # Required: PCM16 output
        )

    async def stream_synthesize(
        self,
        text: str,
        options: Optional[TTSOptions] = None,
    ) -> AsyncIterator[TTSResponse]:
        """
        Stream synthesize text to audio.

        Args:
            text: Text to synthesize.
            options: TTS options including voice_id and speed.

        Yields:
            TTSResponse with audio chunk and metadata.
        """
        # Generate session ID from text hash
        import hashlib
        text_hash = hashlib.md5(text.encode()).hexdigest()[:16]
        session_id = f"my_tts_{text_hash}"

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"TTS session {session_id} cancelled before start")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        # Audio format (must be PCM16)
        audio_format = AudioFormat(
            sample_rate=self.SAMPLE_RATE,
            channels=self.CHANNELS,
            encoding=AudioEncoding.PCM16,
        )

        # Get voice from options or use default
        voice_id = options.voice_id if options and options.voice_id else self._voice_id
        speed = options.speed if options and options.speed > 0 else 1.0

        try:
            # Synthesize audio with your TTS service
            # This example shows chunked streaming

            # Calculate expected duration (rough estimate)
            # Average: ~15 characters per second at normal speed
            chars_per_second = 15 * speed
            duration_seconds = max(1.0, len(text) / chars_per_second)

            # Number of chunks (100ms each)
            chunk_duration_ms = 100
            num_chunks = max(1, int(duration_seconds * 1000 / chunk_duration_ms))

            samples_per_chunk = int(
                self.SAMPLE_RATE * chunk_duration_ms / 1000
            )

            # Generate audio chunks
            for chunk_index in range(num_chunks):
                # Check for cancellation
                if session_id in self._cancelled_sessions:
                    self._cancelled_sessions.discard(session_id)
                    logger.info(f"TTS session {session_id} cancelled during synthesis")

                    # Yield final empty chunk
                    yield TTSResponse(
                        session_context=session_context,
                        audio_chunk=b"",
                        audio_format=audio_format,
                        segment_index=chunk_index,
                        is_final=True,
                        timing=TimingMetadata(),
                    )
                    return

                # Generate audio chunk
                # In production, this would come from your TTS API
                audio_chunk = await self._generate_audio_chunk(
                    text=text,
                    chunk_index=chunk_index,
                    total_chunks=num_chunks,
                    voice_id=voice_id,
                    speed=speed,
                )

                is_final = chunk_index == num_chunks - 1

                yield TTSResponse(
                    session_context=session_context,
                    audio_chunk=audio_chunk,
                    audio_format=audio_format,
                    segment_index=chunk_index,
                    is_final=is_final,
                    timing=TimingMetadata() if is_final else None,
                )

                # Delay between chunks (simulate streaming)
                if not is_final:
                    await asyncio.sleep(self._chunk_delay_ms / 1000.0)

            logger.debug(f"TTS synthesis completed for session {session_id}")

        except Exception as e:
            logger.error(f"TTS synthesis failed: {e}")
            raise ProviderError(
                code=ProviderErrorCode.INTERNAL,
                message=f"TTS synthesis failed: {str(e)}",
                provider_name=self.name(),
            ) from e

    async def _generate_audio_chunk(
        self,
        text: str,
        chunk_index: int,
        total_chunks: int,
        voice_id: str,
        speed: float,
    ) -> bytes:
        """
        Generate an audio chunk for the given text segment.

        Args:
            text: Full text being synthesized.
            chunk_index: Current chunk index.
            total_chunks: Total number of chunks.
            voice_id: Voice ID to use.
            speed: Speaking speed multiplier.

        Returns:
            PCM16 audio data as bytes.
        """
        # Implement your TTS API call here
        # This example generates a simple sine wave for demonstration

        samples_per_chunk = int(self.SAMPLE_RATE * 0.1)  # 100ms chunks
        frequency = 440.0  # A4 note

        samples = []
        start_sample = chunk_index * samples_per_chunk

        for i in range(samples_per_chunk):
            sample_index = start_sample + i
            time = sample_index / self.SAMPLE_RATE
            # Generate sine wave with envelope
            value = 32767 * 0.3 * self._envelope(chunk_index, total_chunks) * \
                    (1 if int(time * 10) % 2 == 0 else -1)  # Square-ish wave
            samples.append(int(value))

        # Pack as PCM16 (signed 16-bit little-endian)
        return struct.pack(f"<{len(samples)}h", *samples)

    def _envelope(self, chunk_index: int, total_chunks: int) -> float:
        """
        Generate amplitude envelope to avoid clicking at chunk boundaries.

        Args:
            chunk_index: Current chunk index.
            total_chunks: Total number of chunks.

        Returns:
            Amplitude multiplier (0.0 to 1.0).
        """
        # Simple fade in/out
        fade_chunks = 3
        if chunk_index < fade_chunks:
            return chunk_index / fade_chunks
        elif chunk_index >= total_chunks - fade_chunks:
            return (total_chunks - chunk_index) / fade_chunks
        return 1.0

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing synthesis.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        self._cancelled_sessions.add(session_id)
        logger.info(f"TTS cancellation requested for session {session_id}")
        return True


def create_my_tts_provider(**config) -> MyTTSProvider:
    """Factory function for creating MyTTSProvider."""
    return MyTTSProvider(**config)
```

### 3. Register in `__init__.py`

Edit `/py/provider_gateway/app/providers/tts/__init__.py`:

```python
"""TTS providers module."""

from typing import TYPE_CHECKING

from app.providers.tts.google_tts import create_google_tts_provider
from app.providers.tts.mock_tts import create_mock_tts_provider
from app.providers.tts.xtts import create_xtts_provider
from app.providers.tts.my_tts_provider import create_my_tts_provider  # Add this

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """Register all TTS providers with the registry."""
    registry.register_tts("mock", create_mock_tts_provider)
    registry.register_tts("google_tts", create_google_tts_provider)
    registry.register_tts("xtts", create_xtts_provider)
    registry.register_tts("my_provider", create_my_tts_provider)  # Add this


__all__ = [
    "create_google_tts_provider",
    "create_mock_tts_provider",
    "create_xtts_provider",
    "create_my_tts_provider",  # Add this
    "register_providers",
]
```

### 4. Add Configuration

Add provider-specific configuration to your YAML config:

```yaml
providers:
  defaults:
    tts: "my_provider"  # Set as default

  tts:
    my_provider:
      api_key: "${MY_TTS_API_KEY}"
      base_url: "https://api.example.com/v1"
      voice_id: "en-US-Standard-C"
      chunk_delay_ms: 50
```

### 5. Testing

Create a test file `/py/provider_gateway/app/tests/test_my_tts_provider.py`:

```python
"""Tests for MyTTSProvider."""

import pytest
from app.providers.tts.my_tts_provider import create_my_tts_provider


@pytest.fixture
def provider():
    return create_my_tts_provider(api_key="test-key")


@pytest.mark.asyncio
async def test_provider_name(provider):
    assert provider.name() == "my_tts_provider"


@pytest.mark.asyncio
async def test_provider_capabilities(provider):
    caps = provider.capabilities()
    assert caps.supports_streaming_output is True
    assert caps.supports_voices is True
    assert "pcm16" in caps.supported_codecs


@pytest.mark.asyncio
async def test_stream_synthesize(provider):
    text = "Hello, world!"

    results = []
    async for response in provider.stream_synthesize(text):
        results.append(response)
        assert response.audio_format.encoding == "PCM16"
        assert response.audio_format.sample_rate == 24000

    assert len(results) > 0
    assert results[-1].is_final is True
```

Run the tests:

```bash
cd py/provider_gateway
python -m pytest app/tests/test_my_tts_provider.py -v
```

## Audio Format Requirements

### Required Output Format

All TTS providers **must** output audio in the following format:

| Property | Value | Notes |
|----------|-------|-------|
| Encoding | PCM16 | Signed 16-bit little-endian |
| Sample Rate | 16000 or 24000 Hz | 16000 Hz is standard for telephony |
| Channels | 1 (Mono) | Stereo not supported |
| Byte Order | Little-endian | Standard for WAV/PCM |

### PCM16 Packing

```python
import struct

# Pack list of integers as PCM16
samples = [1000, -500, 0, 32767, -32768]
audio_bytes = struct.pack(f"<{len(samples)}h", *samples)

# Unpack PCM16 to integers
samples = struct.unpack(f"<{len(audio_bytes)//2}h", audio_bytes)
```

### Sample Rate Conversion

If your TTS API outputs a different sample rate, you must resample:

```python
import numpy as np
from scipy import signal

def resample_audio(audio_bytes: bytes, orig_rate: int, target_rate: int) -> bytes:
    """Resample PCM16 audio from orig_rate to target_rate."""
    # Convert bytes to numpy array
    samples = np.frombuffer(audio_bytes, dtype=np.int16)

    # Calculate resampling ratio
    ratio = target_rate / orig_rate
    num_samples = int(len(samples) * ratio)

    # Resample
    resampled = signal.resample(samples, num_samples)

    # Clip to int16 range and convert back to bytes
    resampled = np.clip(resampled, -32768, 32767).astype(np.int16)
    return resampled.tobytes()
```

## Reference Implementation

For a complete working example, see:

- `/py/provider_gateway/app/providers/tts/mock_tts.py` — Mock TTS with sine wave generation

## Key Requirements

### Streaming Behavior

1. **Chunked Output**: Stream audio in 50-100ms chunks for low latency
2. **Final Flag**: Set `is_final=True` on the last chunk
3. **Cancellation**: Check `self._cancelled_sessions` and yield empty final chunk if cancelled

### Audio Quality Guidelines

| Metric | Target |
|--------|--------|
| First audio chunk | < 500ms from text receipt |
| Chunk latency | 50-100ms between chunks |
| Audio quality | 16-bit, 16kHz+ sample rate |
| Volume normalization | -3dB peak to prevent clipping |

### Error Handling

Common TTS errors:

| Error | Code | Notes |
|-------|------|-------|
| Voice not found | `INVALID_REQUEST` | Invalid voice_id |
| Text too long | `INVALID_REQUEST` | Exceeds max length |
| Rate limited | `RATE_LIMITED` | API quota exceeded |
| Unsupported language | `UNSUPPORTED_FORMAT` | Language not available |

## Troubleshooting

### Audio plays too fast/slow

Ensure your `sample_rate` in `AudioFormat` matches the actual audio data sample rate.

### Audio has clicking/popping

Add fade-in/fade-out envelopes at chunk boundaries (see `_envelope()` example above).

### Cancellation not working

Ensure you're:
1. Checking `self._cancelled_sessions` in your synthesis loop
2. Yielding a final empty chunk before returning
3. Not blocking on I/O without async/await

### Audio format errors in client

Verify:
- `AudioFormat.encoding` is set to `AudioEncoding.PCM16`
- `audio_chunk` is properly packed PCM16 bytes
- Sample rate matches the configured audio profile
