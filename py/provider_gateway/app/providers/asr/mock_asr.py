"""Mock ASR provider for testing."""

import asyncio
import hashlib
import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseASRProvider
from app.core.capability import ProviderCapability
from app.models.asr import ASROptions, ASRResponse, WordTimestamp
from app.models.common import SessionContext, TimingMetadata

logger = logging.getLogger(__name__)


class MockASRProvider(BaseASRProvider):
    """
    Mock ASR provider that produces deterministic transcript responses.

    Given audio chunks, produces realistic partial transcripts followed by
    a final transcript. Useful for testing and development.
    """

    # Default transcript to return
    DEFAULT_TRANSCRIPT = "Hello, this is a test transcription. How can I help you today?"

    # Partial transcript chunks (simulating real-time recognition)
    PARTIAL_CHUNKS = [
        "Hello",
        "Hello, this",
        "Hello, this is",
        "Hello, this is a",
        "Hello, this is a test",
        "Hello, this is a test transcription",
        "Hello, this is a test transcription. How",
        "Hello, this is a test transcription. How can",
        "Hello, this is a test transcription. How can I",
        "Hello, this is a test transcription. How can I help",
        "Hello, this is a test transcription. How can I help you",
        "Hello, this is a test transcription. How can I help you today?",
    ]

    def __init__(
        self,
        transcript: Optional[str] = None,
        chunk_delay_ms: float = 100.0,
        confidence: float = 0.95,
    ) -> None:
        """
        Initialize Mock ASR provider.

        Args:
            transcript: Custom transcript to return (default: DEFAULT_TRANSCRIPT).
            chunk_delay_ms: Delay between partial chunks in milliseconds.
            confidence: Confidence score to return (0-1).
        """
        self._transcript = transcript or self.DEFAULT_TRANSCRIPT
        self._chunk_delay_ms = chunk_delay_ms
        self._confidence = confidence
        self._cancelled_sessions: set = set()
        logger.info("MockASRProvider initialized")

    def name(self) -> str:
        """Return the provider name."""
        return "mock_asr"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=True,
            supports_streaming_output=True,
            supports_word_timestamps=True,
            supports_voices=False,
            supports_interruptible_generation=True,
            preferred_sample_rates=[16000, 8000],
            supported_codecs=["pcm16", "opus"],
        )

    async def stream_recognize(
        self,
        audio_stream: AsyncIterator[bytes],
        options: Optional[ASROptions] = None,
    ) -> AsyncIterator[ASRResponse]:
        """
        Stream audio and receive transcript responses.

        Produces deterministic partial transcripts followed by a final transcript.

        Args:
            audio_stream: Async iterator of audio chunks.
            options: ASR options for recognition.

        Yields:
            ASRResponse with transcript, partial/final flags, and metadata.
        """
        # Collect audio chunks (simulate processing)
        audio_data = bytearray()
        session_id = ""

        async for chunk in audio_stream:
            audio_data.extend(chunk)

            # Check if cancelled
            if session_id and session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                logger.info(f"ASR session {session_id} cancelled")
                return

        # Generate deterministic session ID from audio data
        session_id = hashlib.md5(bytes(audio_data)).hexdigest()[:16]

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"ASR session {session_id} cancelled")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        # Generate partial transcripts
        chunks = self._generate_chunks()

        for i, chunk_text in enumerate(chunks[:-1]):
            # Check for cancellation
            if session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                logger.info(f"ASR session {session_id} cancelled during streaming")
                return

            yield ASRResponse(
                session_context=session_context,
                transcript=chunk_text,
                is_partial=True,
                is_final=False,
                confidence=self._confidence * (0.8 + 0.2 * (i / len(chunks))),
                language=options.language_hint if options else "en-US",
            )

            # Simulate processing delay
            await asyncio.sleep(self._chunk_delay_ms / 1000.0)

        # Final transcript
        final_text = chunks[-1] if chunks else self._transcript

        # Generate word timestamps for final result
        word_timestamps = self._generate_word_timestamps(final_text)

        yield ASRResponse(
            session_context=session_context,
            transcript=final_text,
            is_partial=False,
            is_final=True,
            confidence=self._confidence,
            language=options.language_hint if options else "en-US",
            word_timestamps=word_timestamps,
            timing=TimingMetadata(duration_ms=len(chunks) * int(self._chunk_delay_ms)),
        )

        logger.debug(f"Mock ASR completed for session {session_id}")

    def _generate_chunks(self) -> list:
        """Generate partial transcript chunks."""
        if self._transcript == self.DEFAULT_TRANSCRIPT:
            return self.PARTIAL_CHUNKS

        # Generate chunks from custom transcript
        words = self._transcript.split()
        chunks = []
        for i in range(1, len(words) + 1):
            chunks.append(" ".join(words[:i]))
        return chunks if chunks else [self._transcript]

    def _generate_word_timestamps(self, text: str) -> list:
        """Generate word timestamps for the transcript."""
        words = text.split()
        word_timestamps = []
        current_ms = 0
        avg_word_duration = 200  # ms per word

        for word in words:
            # Clean word of punctuation for duration calculation
            clean_word = "".join(c for c in word if c.isalnum())
            duration = max(100, len(clean_word) * 50)  # Longer words take more time

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


def create_mock_asr_provider(**config) -> MockASRProvider:
    """Factory function for creating MockASRProvider."""
    return MockASRProvider(**config)


__all__ = ["MockASRProvider", "create_mock_asr_provider"]
