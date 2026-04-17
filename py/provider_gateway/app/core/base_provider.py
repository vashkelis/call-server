"""Abstract base classes for ASR, LLM, and TTS providers."""

from abc import ABC, abstractmethod
from typing import Any, AsyncIterator, Dict, Optional

from app.core.capability import ProviderCapability
from app.models.asr import ASROptions, ASRResponse
from app.models.llm import LLMOptions, LLMResponse
from app.models.tts import TTSOptions, TTSResponse


class BaseProvider(ABC):
    """Base class for all providers."""

    @abstractmethod
    def name(self) -> str:
        """Return the provider name."""
        raise NotImplementedError()

    @abstractmethod
    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        raise NotImplementedError()

    @abstractmethod
    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing operation.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        raise NotImplementedError()


class BaseASRProvider(BaseProvider):
    """Abstract base class for ASR (Automatic Speech Recognition) providers."""

    @abstractmethod
    async def stream_recognize(
        self,
        audio_stream: AsyncIterator[bytes],
        options: Optional[ASROptions] = None,
    ) -> AsyncIterator[ASRResponse]:
        """
        Stream audio and receive transcript responses.

        Args:
            audio_stream: Async iterator of audio chunks.
            options: ASR options for recognition.

        Yields:
            ASRResponse with transcript, partial/final flags, and metadata.
        """
        raise NotImplementedError()

    def name(self) -> str:
        """Return the provider name."""
        return self.__class__.__name__

    @abstractmethod
    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        raise NotImplementedError()

    @abstractmethod
    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing recognition.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        raise NotImplementedError()


class BaseLLMProvider(BaseProvider):
    """Abstract base class for LLM (Large Language Model) providers."""

    @abstractmethod
    async def stream_generate(
        self,
        messages: list,
        options: Optional[LLMOptions] = None,
    ) -> AsyncIterator[LLMResponse]:
        """
        Stream generate tokens from the LLM.

        Args:
            messages: List of chat messages.
            options: LLM options for generation.

        Yields:
            LLMResponse with generated token and metadata.
        """
        raise NotImplementedError()

    def name(self) -> str:
        """Return the provider name."""
        return self.__class__.__name__

    @abstractmethod
    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        raise NotImplementedError()

    @abstractmethod
    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing generation.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        raise NotImplementedError()


class BaseTTSProvider(BaseProvider):
    """Abstract base class for TTS (Text-to-Speech) providers."""

    @abstractmethod
    async def stream_synthesize(
        self,
        text: str,
        options: Optional[TTSOptions] = None,
    ) -> AsyncIterator[TTSResponse]:
        """
        Stream synthesize text to audio.

        Args:
            text: Text to synthesize.
            options: TTS options for synthesis.

        Yields:
            TTSResponse with audio chunk and metadata.
        """
        raise NotImplementedError()

    def name(self) -> str:
        """Return the provider name."""
        return self.__class__.__name__

    @abstractmethod
    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        raise NotImplementedError()

    @abstractmethod
    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing synthesis.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        raise NotImplementedError()


__all__ = [
    "BaseASRProvider",
    "BaseLLMProvider",
    "BaseProvider",
    "BaseTTSProvider",
]
