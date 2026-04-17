"""Simple energy-based VAD provider."""

import logging
import struct
from typing import AsyncIterator, List, Optional

import numpy as np

from app.providers.vad.base import VADProvider, VADSegment

logger = logging.getLogger(__name__)


class EnergyVADProvider(VADProvider):
    """
    Simple energy-based VAD provider.

    Detects speech based on audio energy levels.
    This is a basic implementation suitable for testing and simple use cases.
    """

    def __init__(
        self,
        energy_threshold: float = 0.01,
        min_speech_duration_ms: int = 200,
        min_silence_duration_ms: int = 300,
    ) -> None:
        """
        Initialize Energy VAD provider.

        Args:
            energy_threshold: Energy threshold for speech detection (0-1).
            min_speech_duration_ms: Minimum speech duration in milliseconds.
            min_silence_duration_ms: Minimum silence duration in milliseconds.
        """
        self._energy_threshold = energy_threshold
        self._min_speech_duration_ms = min_speech_duration_ms
        self._min_silence_duration_ms = min_silence_duration_ms
        logger.info("EnergyVADProvider initialized")

    def name(self) -> str:
        """Return the provider name."""
        return "energy_vad"

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
        # Collect all audio data
        audio_buffer = bytearray()
        async for chunk in audio_stream:
            audio_buffer.extend(chunk)

        # Process buffer
        segments = await self.process_buffer(bytes(audio_buffer), sample_rate)

        for segment in segments:
            yield segment

    async def process_buffer(
        self,
        audio_buffer: bytes,
        sample_rate: int = 16000,
    ) -> List[VADSegment]:
        """
        Process an audio buffer for voice activity.

        Args:
            audio_buffer: Audio data buffer (PCM16 format).
            sample_rate: Audio sample rate in Hz.

        Returns:
            List of detected VAD segments.
        """
        # Convert bytes to numpy array (assuming PCM16)
        if len(audio_buffer) % 2 != 0:
            # Pad with zero if odd number of bytes
            audio_buffer = audio_buffer + b"\x00"

        audio_data = np.frombuffer(audio_buffer, dtype=np.int16).astype(np.float32)
        audio_data = audio_data / 32768.0  # Normalize to -1 to 1

        # Frame size in samples
        frame_size = int(sample_rate * 30 / 1000)  # 30ms frames

        segments = []
        current_sample = 0
        is_speech = False
        speech_start = 0

        min_speech_samples = int(sample_rate * self._min_speech_duration_ms / 1000)
        min_silence_samples = int(sample_rate * self._min_silence_duration_ms / 1000)

        while current_sample + frame_size <= len(audio_data):
            frame = audio_data[current_sample:current_sample + frame_size]
            energy = np.sqrt(np.mean(frame ** 2))

            if energy > self._energy_threshold:
                if not is_speech:
                    # Speech start
                    is_speech = True
                    speech_start = current_sample
            else:
                if is_speech:
                    # Speech end
                    speech_duration = current_sample - speech_start
                    if speech_duration >= min_speech_samples:
                        segments.append(
                            VADSegment(
                                start_sample=speech_start,
                                end_sample=current_sample,
                                is_speech=True,
                                confidence=min(1.0, energy * 10),
                            )
                        )
                    is_speech = False

            current_sample += frame_size

        # Handle ongoing speech at end of buffer
        if is_speech:
            speech_duration = current_sample - speech_start
            if speech_duration >= min_speech_samples:
                segments.append(
                    VADSegment(
                        start_sample=speech_start,
                        end_sample=current_sample,
                        is_speech=True,
                        confidence=0.8,
                    )
                )

        return segments

    def _calculate_energy(self, audio_data: bytes) -> float:
        """
        Calculate RMS energy of audio data.

        Args:
            audio_data: PCM16 audio data.

        Returns:
            RMS energy value.
        """
        if len(audio_data) < 2:
            return 0.0

        # Unpack as 16-bit integers
        num_samples = len(audio_data) // 2
        samples = struct.unpack(f"<{num_samples}h", audio_data[:num_samples * 2])

        # Calculate RMS
        sum_squares = sum(s * s for s in samples)
        rms = (sum_squares / num_samples) ** 0.5 if num_samples > 0 else 0.0

        # Normalize to 0-1 range
        return rms / 32768.0


def create_energy_vad_provider(**config) -> EnergyVADProvider:
    """Factory function for creating EnergyVADProvider."""
    return EnergyVADProvider(**config)


__all__ = ["create_energy_vad_provider", "EnergyVADProvider"]
