"""Base VAD (Voice Activity Detection) provider interface."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import AsyncIterator, Optional


@dataclass
class VADSegment:
    """VAD-detected speech segment."""

    start_sample: int
    end_sample: int
    is_speech: bool
    confidence: float = 1.0


class VADProvider(ABC):
    """Abstract base class for VAD (Voice Activity Detection) providers."""

    @abstractmethod
    def name(self) -> str:
        """Return the provider name."""
        raise NotImplementedError()

    @abstractmethod
    async def detect(
        self,
        audio_stream: AsyncIterator[bytes],
        sample_rate: int = 16000,
        frame_duration_ms: int = 30,
    ) -> AsyncIterator[VADSegment]:
        """
        Detect voice activity in audio stream.

        Args:
            audio_stream: Async iterator of audio chunks.
            sample_rate: Audio sample rate in Hz.
            frame_duration_ms: Frame duration in milliseconds.

        Yields:
            VADSegment with speech detection results.
        """
        raise NotImplementedError()

    @abstractmethod
    async def process_buffer(
        self,
        audio_buffer: bytes,
        sample_rate: int = 16000,
    ) -> list[VADSegment]:
        """
        Process an audio buffer for voice activity.

        Args:
            audio_buffer: Audio data buffer.
            sample_rate: Audio sample rate in Hz.

        Returns:
            List of detected VAD segments.
        """
        raise NotImplementedError()


__all__ = ["VADProvider", "VADSegment"]
