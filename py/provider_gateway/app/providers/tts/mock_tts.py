"""Mock TTS provider for testing."""

import asyncio
import logging
import math
import struct
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseTTSProvider
from app.core.capability import ProviderCapability
from app.models.common import AudioFormat, AudioEncoding, SessionContext, TimingMetadata
from app.models.tts import TTSOptions, TTSResponse

logger = logging.getLogger(__name__)


class MockTTSProvider(BaseTTSProvider):
    """
    Mock TTS provider that produces PCM16 audio chunks.

    Generates simple sine wave tones at 440Hz (A4 note).
    Streams chunks with configurable delay.
    """

    # Audio generation constants
    SAMPLE_RATE = 16000
    CHANNELS = 1
    FREQUENCY = 440.0  # A4 note
    CHUNK_DURATION_MS = 100  # Duration of each audio chunk
    BYTES_PER_SAMPLE = 2  # 16-bit

    def __init__(
        self,
        chunk_delay_ms: float = 50.0,
        frequency: Optional[float] = None,
    ) -> None:
        """
        Initialize Mock TTS provider.

        Args:
            chunk_delay_ms: Delay between audio chunks in milliseconds.
            frequency: Sine wave frequency in Hz (default: 440.0).
        """
        self._chunk_delay_ms = chunk_delay_ms
        self._frequency = frequency or self.FREQUENCY
        self._cancelled_sessions: set = set()
        logger.info("MockTTSProvider initialized")

    def name(self) -> str:
        """Return the provider name."""
        return "mock_tts"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,
            supports_streaming_output=True,
            supports_word_timestamps=False,
            supports_voices=True,
            supports_interruptible_generation=True,
            preferred_sample_rates=[16000, 22050, 24000, 48000],
            supported_codecs=["pcm16"],
        )

    async def stream_synthesize(
        self,
        text: str,
        options: Optional[TTSOptions] = None,
    ) -> AsyncIterator[TTSResponse]:
        """
        Stream synthesize text to audio.

        Generates PCM16 sine wave audio chunks.

        Args:
            text: Text to synthesize.
            options: TTS options for synthesis.

        Yields:
            TTSResponse with audio chunk and metadata.
        """
        # Generate session ID from text
        import hashlib
        text_hash = hashlib.md5(text.encode()).hexdigest()[:16]
        session_id = f"mock_tts_{text_hash}"

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"TTS session {session_id} cancelled")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        # Audio format
        audio_format = AudioFormat(
            sample_rate=self.SAMPLE_RATE,
            channels=self.CHANNELS,
            encoding=AudioEncoding.PCM16,
        )

        # Calculate number of chunks based on text length
        # Rough estimate: ~5 characters per second of speech
        chars_per_second = 15
        duration_seconds = max(1.0, len(text) / chars_per_second)
        num_chunks = max(1, int(duration_seconds * 1000 / self.CHUNK_DURATION_MS))

        samples_per_chunk = int(
            self.SAMPLE_RATE * self.CHUNK_DURATION_MS / 1000
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

            # Generate sine wave samples
            start_sample = chunk_index * samples_per_chunk
            audio_chunk = self._generate_sine_wave(
                start_sample, samples_per_chunk
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

            # Delay between chunks
            if not is_final:
                await asyncio.sleep(self._chunk_delay_ms / 1000.0)

        logger.debug(f"Mock TTS synthesis completed for session {session_id}")

    def _generate_sine_wave(
        self,
        start_sample: int,
        num_samples: int,
    ) -> bytes:
        """
        Generate sine wave audio data.

        Args:
            start_sample: Starting sample index.
            num_samples: Number of samples to generate.

        Returns:
            PCM16 audio data as bytes.
        """
        samples = []
        amplitude = 32767 * 0.3  # 30% amplitude to avoid clipping

        for i in range(num_samples):
            sample_index = start_sample + i
            time = sample_index / self.SAMPLE_RATE
            # Generate sine wave with slight envelope to avoid clicking
            value = amplitude * math.sin(2 * math.pi * self._frequency * time)
            samples.append(int(value))

        # Pack as PCM16 (signed 16-bit little-endian)
        return struct.pack(f"<{len(samples)}h", *samples)

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


def create_mock_tts_provider(**config) -> MockTTSProvider:
    """Factory function for creating MockTTSProvider."""
    return MockTTSProvider(**config)


__all__ = ["create_mock_tts_provider", "MockTTSProvider"]
