# Adding an ASR Provider

This guide walks you through adding a new Automatic Speech Recognition (ASR) provider to CloudApp.

## Overview

ASR providers convert audio streams into text transcripts. They must implement the `BaseASRProvider` abstract class and support streaming recognition with partial and final results.

## Step-by-Step Guide

### 1. Create Provider File

Create a new file in `/py/provider_gateway/app/providers/asr/`:

```bash
touch /py/provider_gateway/app/providers/asr/my_provider.py
```

### 2. Implement BaseASRProvider

```python
"""My Custom ASR Provider."""

import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseASRProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.asr import ASROptions, ASRResponse, WordTimestamp
from app.models.common import SessionContext, TimingMetadata

logger = logging.getLogger(__name__)


class MyASRProvider(BaseASRProvider):
    """Custom ASR provider implementation."""

    def __init__(
        self,
        api_key: Optional[str] = None,
        model: str = "default",
        language: str = "en-US",
    ) -> None:
        """
        Initialize the ASR provider.

        Args:
            api_key: API key for authentication.
            model: Model name to use.
            language: Default language code (e.g., "en-US").
        """
        self._api_key = api_key
        self._model = model
        self._language = language
        self._cancelled_sessions: set = set()

        # Initialize your ASR client here
        # self._client = MyASRClient(api_key=api_key)

        logger.info(f"MyASRProvider initialized with model={model}")

    def name(self) -> str:
        """Return the provider name."""
        return "my_asr_provider"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=True,      # Supports streaming audio input
            supports_streaming_output=True,     # Supports streaming transcripts
            supports_word_timestamps=True,      # Can provide word-level timestamps
            supports_voices=False,              # ASR doesn't use voices
            supports_interruptible_generation=True,  # Supports cancellation
            preferred_sample_rates=[16000, 8000],    # Supported sample rates
            supported_codecs=["pcm16", "opus"],      # Supported audio codecs
        )

    async def stream_recognize(
        self,
        audio_stream: AsyncIterator[bytes],
        options: Optional[ASROptions] = None,
    ) -> AsyncIterator[ASRResponse]:
        """
        Stream audio and receive transcript responses.

        Args:
            audio_stream: Async iterator of audio chunks (PCM16).
            options: ASR options including language hint.

        Yields:
            ASRResponse with transcript, partial/final flags, and metadata.
        """
        session_id = ""
        language = options.language_hint if options else self._language

        # Collect audio chunks
        audio_buffer = bytearray()
        async for chunk in audio_stream:
            # Check for cancellation
            if session_id and session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                logger.info(f"ASR session {session_id} cancelled")
                return

            audio_buffer.extend(chunk)

        # Generate session ID from audio hash
        import hashlib
        session_id = hashlib.md5(bytes(audio_buffer)).hexdigest()[:16]

        # Check cancellation before processing
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        try:
            # Process audio with your ASR service
            # This is where you integrate with your ASR API

            # Example: Yield partial transcripts as they arrive
            partial_transcript = ""
            for partial in self._process_audio_streaming(bytes(audio_buffer), language):
                # Check for cancellation
                if session_id in self._cancelled_sessions:
                    self._cancelled_sessions.discard(session_id)
                    return

                partial_transcript = partial
                yield ASRResponse(
                    session_context=session_context,
                    transcript=partial_transcript,
                    is_partial=True,           # This is an intermediate result
                    is_final=False,
                    confidence=0.85,           # Confidence score (0-1)
                    language=language,
                )

            # Yield final transcript
            final_transcript = partial_transcript
            word_timestamps = self._generate_word_timestamps(final_transcript)

            yield ASRResponse(
                session_context=session_context,
                transcript=final_transcript,
                is_partial=False,
                is_final=True,              # This is the final result
                confidence=0.92,
                language=language,
                word_timestamps=word_timestamps,
                timing=TimingMetadata(duration_ms=1500),
            )

        except Exception as e:
            logger.error(f"ASR processing failed: {e}")
            raise ProviderError(
                code=ProviderErrorCode.INTERNAL,
                message=f"ASR processing failed: {str(e)}",
                provider_name=self.name(),
            ) from e

    def _process_audio_streaming(
        self,
        audio_data: bytes,
        language: str,
    ) -> list[str]:
        """
        Process audio and return partial transcripts.

        Args:
            audio_data: Raw PCM16 audio data.
            language: Language code.

        Returns:
            List of partial transcripts.
        """
        # Implement your ASR logic here
        # This should integrate with your ASR service
        # and yield partial results as they become available
        pass

    def _generate_word_timestamps(self, text: str) -> list[WordTimestamp]:
        """
        Generate word timestamps for the transcript.

        Args:
            text: Final transcript text.

        Returns:
            List of word timestamps.
        """
        words = text.split()
        word_timestamps = []
        current_ms = 0

        for word in words:
            # Estimate duration based on word length
            # In production, use actual timestamps from your ASR service
            duration = max(100, len(word) * 50)
            word_timestamps.append(
                WordTimestamp(
                    word=word,
                    start_ms=current_ms,
                    end_ms=current_ms + duration,
                )
            )
            current_ms += duration + 50  # 50ms gap between words

        return word_timestamps

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing recognition.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        self._cancelled_sessions.add(session_id)
        logger.info(f"ASR cancellation requested for session {session_id}")
        return True


def create_my_asr_provider(**config) -> MyASRProvider:
    """Factory function for creating MyASRProvider."""
    return MyASRProvider(**config)
```

### 3. Register in `__init__.py`

Edit `/py/provider_gateway/app/providers/asr/__init__.py`:

```python
"""ASR providers module."""

from typing import TYPE_CHECKING

from app.providers.asr.faster_whisper import create_faster_whisper_provider
from app.providers.asr.google_speech import create_google_speech_provider
from app.providers.asr.mock_asr import create_mock_asr_provider
from app.providers.asr.my_provider import create_my_asr_provider  # Add this

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """Register all ASR providers with the registry."""
    registry.register_asr("mock", create_mock_asr_provider)
    registry.register_asr("faster_whisper", create_faster_whisper_provider)
    registry.register_asr("google_speech", create_google_speech_provider)
    registry.register_asr("my_provider", create_my_asr_provider)  # Add this


__all__ = [
    "create_faster_whisper_provider",
    "create_google_speech_provider",
    "create_mock_asr_provider",
    "create_my_asr_provider",  # Add this
    "register_providers",
]
```

### 4. Add Configuration

Add provider-specific configuration to your YAML config:

```yaml
providers:
  defaults:
    asr: "my_provider"  # Set as default

  asr:
    my_provider:
      api_key: "${MY_ASR_API_KEY}"
      model: "large-v3"
      language: "en-US"
```

### 5. Testing

Create a test file `/py/provider_gateway/app/tests/test_my_asr_provider.py`:

```python
"""Tests for MyASRProvider."""

import pytest
from app.providers.asr.my_provider import create_my_asr_provider


@pytest.fixture
def provider():
    return create_my_asr_provider(api_key="test-key")


@pytest.mark.asyncio
async def test_provider_name(provider):
    assert provider.name() == "my_asr_provider"


@pytest.mark.asyncio
async def test_provider_capabilities(provider):
    caps = provider.capabilities()
    assert caps.supports_streaming_input is True
    assert caps.supports_streaming_output is True


@pytest.mark.asyncio
async def test_stream_recognize(provider):
    # Create a mock audio stream
    async def mock_audio_stream():
        yield b"fake_audio_chunk_1"
        yield b"fake_audio_chunk_2"

    results = []
    async for response in provider.stream_recognize(mock_audio_stream()):
        results.append(response)

    assert len(results) > 0
    assert results[-1].is_final is True
```

Run the tests:

```bash
cd py/provider_gateway
python -m pytest app/tests/test_my_asr_provider.py -v
```

## Key Requirements

### Audio Format

- Input: PCM16 (16-bit signed little-endian)
- Sample rates: 8000 Hz or 16000 Hz (configurable)
- Channels: Mono (1 channel)

### Streaming Behavior

1. **Partial Results**: Yield `is_partial=True` transcripts as audio is processed
2. **Final Result**: Yield exactly one `is_final=True` result at the end
3. **Cancellation**: Check `self._cancelled_sessions` regularly and stop processing if cancelled

### Error Handling

- Raise `ProviderError` with appropriate error codes
- Set `retriable=True` for transient errors (timeouts, rate limits)
- Include meaningful error messages

### Performance Guidelines

- First partial transcript within 500ms of speech start
- Final transcript within 200ms of speech end
- Support cancellation within 100ms

## Example: OpenAI Whisper Integration

For reference, see the existing implementations:

- `/py/provider_gateway/app/providers/asr/mock_asr.py` — Simple mock for testing
- `/py/provider_gateway/app/providers/asr/faster_whisper.py` — Local Whisper inference
- `/py/provider_gateway/app/providers/asr/google_speech.py` — Cloud API example

## Troubleshooting

### Provider not found

Ensure the provider is registered in `__init__.py` and the registry has discovered providers:

```python
from app.core.registry import get_registry
registry = get_registry()
registry.discover_and_register()
print(registry.list_asr_providers())  # Should include your provider
```

### Audio format errors

Verify your provider's `capabilities()` returns correct `preferred_sample_rates` and `supported_codecs`.

### Cancellation not working

Ensure you check `self._cancelled_sessions` at appropriate points in your streaming loop.
