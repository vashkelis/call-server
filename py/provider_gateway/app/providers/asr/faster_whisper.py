"""Faster Whisper ASR provider adapter."""

import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseASRProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.asr import ASROptions, ASRResponse, WordTimestamp
from app.models.common import SessionContext, TimingMetadata

logger = logging.getLogger(__name__)


class FasterWhisperProvider(BaseASRProvider):
    """
    Faster Whisper ASR provider adapter.

    This provider uses the faster-whisper library for efficient speech recognition.
    Falls back gracefully if faster-whisper is not installed.
    """

    def __init__(
        self,
        model_size: str = "base",
        device: str = "cpu",
        compute_type: str = "int8",
        language: str = "en",
    ) -> None:
        """
        Initialize Faster Whisper provider.

        Args:
            model_size: Model size (tiny, base, small, medium, large-v1, large-v2, large-v3).
            device: Device to use (cpu, cuda).
            compute_type: Compute type (int8, int8_float16, float16, float32).
            language: Default language code.
        """
        self._model_size = model_size
        self._device = device
        self._compute_type = compute_type
        self._language = language
        self._model = None
        self._cancelled_sessions: set = set()

        # Check if faster-whisper is available
        self._check_dependency()

    def _check_dependency(self) -> None:
        """Check if faster-whisper is installed."""
        try:
            import faster_whisper
            self._faster_whisper = faster_whisper
            logger.info(
                f"FasterWhisperProvider initialized with model={self._model_size}, "
                f"device={self._device}, compute_type={self._compute_type}"
            )
        except ImportError:
            self._faster_whisper = None
            logger.warning(
                "faster-whisper not installed. Provider will raise errors on use. "
                "Install with: pip install faster-whisper"
            )

    def _load_model(self):
        """Lazy load the faster-whisper model."""
        if self._faster_whisper is None:
            raise ProviderError(
                code=ProviderErrorCode.SERVICE_UNAVAILABLE,
                message=(
                    "faster-whisper is not installed. "
                    "Please install it with: pip install faster-whisper"
                ),
                provider_name=self.name(),
            )

        if self._model is None:
            logger.info(f"Loading faster-whisper model: {self._model_size}")
            self._model = self._faster_whisper.WhisperModel(
                self._model_size,
                device=self._device,
                compute_type=self._compute_type,
            )
            logger.info("faster-whisper model loaded successfully")

        return self._model

    def name(self) -> str:
        """Return the provider name."""
        return f"faster_whisper_{self._model_size}"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=True,
            supports_streaming_output=True,
            supports_word_timestamps=True,
            supports_voices=False,
            supports_interruptible_generation=True,
            preferred_sample_rates=[16000],
            supported_codecs=["pcm16"],
        )

    async def stream_recognize(
        self,
        audio_stream: AsyncIterator[bytes],
        options: Optional[ASROptions] = None,
    ) -> AsyncIterator[ASRResponse]:
        """
        Stream audio and receive transcript responses.

        Accumulates audio chunks and runs inference on VAD-detected segments.

        Args:
            audio_stream: Async iterator of audio chunks.
            options: ASR options for recognition.

        Yields:
            ASRResponse with transcript, partial/final flags, and metadata.
        """
        # Check dependency
        if self._faster_whisper is None:
            raise ProviderError(
                code=ProviderErrorCode.SERVICE_UNAVAILABLE,
                message=(
                    "faster-whisper is not installed. "
                    "Please install it with: pip install faster-whisper"
                ),
                provider_name=self.name(),
            )

        # Collect audio data
        audio_buffer = bytearray()
        session_id = ""

        async for chunk in audio_stream:
            audio_buffer.extend(chunk)

            # Check for cancellation
            if session_id and session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                logger.info(f"FasterWhisper session {session_id} cancelled")
                return

        # Generate session ID
        import hashlib
        session_id = hashlib.md5(bytes(audio_buffer)).hexdigest()[:16]

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"FasterWhisper session {session_id} cancelled")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        # Run inference (in thread pool to not block)
        import asyncio
        loop = asyncio.get_event_loop()

        try:
            segments, info = await loop.run_in_executor(
                None,
                self._transcribe,
                bytes(audio_buffer),
                options.language_hint if options else self._language,
            )

            # Yield results
            full_text = ""
            word_timestamps = []

            for segment in segments:
                # Check for cancellation
                if session_id in self._cancelled_sessions:
                    self._cancelled_sessions.discard(session_id)
                    logger.info(f"FasterWhisper session {session_id} cancelled during inference")
                    return

                full_text += segment.text + " "

                # Extract word timestamps if available
                if hasattr(segment, "words") and segment.words:
                    for word in segment.words:
                        word_timestamps.append(
                            WordTimestamp(
                                word=word.word,
                                start_ms=int(word.start * 1000),
                                end_ms=int(word.end * 1000),
                            )
                        )

            # Final response
            yield ASRResponse(
                session_context=session_context,
                transcript=full_text.strip(),
                is_partial=False,
                is_final=True,
                confidence=info.language_probability if hasattr(info, "language_probability") else 0.9,
                language=info.language if hasattr(info, "language") else (options.language_hint if options else self._language),
                word_timestamps=word_timestamps,
                timing=TimingMetadata(),
            )

            logger.debug(f"FasterWhisper transcription completed for session {session_id}")

        except Exception as e:
            logger.error(f"FasterWhisper transcription error: {e}")
            raise ProviderError(
                code=ProviderErrorCode.INTERNAL,
                message=f"Transcription failed: {str(e)}",
                provider_name=self.name(),
            ) from e

    def _transcribe(self, audio_data: bytes, language: str):
        """Run transcription (called in thread pool)."""
        model = self._load_model()

        # Convert bytes to numpy array
        import io
        import numpy as np

        # Assuming PCM16 input
        audio_np = np.frombuffer(audio_data, dtype=np.int16).astype(np.float32) / 32768.0

        segments, info = model.transcribe(
            audio_np,
            language=language,
            word_timestamps=True,
        )

        # Consume generator to list
        segments_list = list(segments)

        return segments_list, info

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing recognition.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        self._cancelled_sessions.add(session_id)
        logger.info(f"FasterWhisper cancellation requested for session {session_id}")
        return True


def create_faster_whisper_provider(**config) -> FasterWhisperProvider:
    """Factory function for creating FasterWhisperProvider."""
    return FasterWhisperProvider(**config)


__all__ = ["FasterWhisperProvider", "create_faster_whisper_provider"]
