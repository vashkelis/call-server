"""XTTS (Coqui) Text-to-Speech provider stub."""

import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseTTSProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.tts import TTSOptions, TTSResponse

logger = logging.getLogger(__name__)


class XTTSProvider(BaseTTSProvider):
    """
    XTTS (Coqui) Text-to-Speech provider stub.

    Full request/response model is in place, but methods raise NotImplementedError
    with a clear message about requiring XTTS server.
    """

    def __init__(
        self,
        server_url: str = "http://localhost:8020",
        voice_id: Optional[str] = None,
        language: str = "en",
    ) -> None:
        """
        Initialize XTTS provider.

        Args:
            server_url: URL of the XTTS server API.
            voice_id: Voice ID to use (optional).
            language: Language code for synthesis.
        """
        self._server_url = server_url
        self._voice_id = voice_id
        self._language = language
        logger.info("XTTSProvider initialized (stub)")

    def name(self) -> str:
        """Return the provider name."""
        return "xtts"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,
            supports_streaming_output=True,
            supports_word_timestamps=False,
            supports_voices=True,
            supports_interruptible_generation=True,
            preferred_sample_rates=[24000],
            supported_codecs=["pcm16", "wav"],
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
            options: TTS options for synthesis.

        Yields:
            TTSResponse with audio chunk and metadata.

        Raises:
            NotImplementedError: Always raises with clear message about XTTS server.
        """
        raise NotImplementedError(
            "XTTS provider requires an XTTS server. "
            "Please set up an XTTS server and provide the server URL. "
            "See: https://github.com/coqui-ai/TTS"
        )

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing synthesis.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.

        Raises:
            NotImplementedError: Always raises with clear message about XTTS server.
        """
        raise NotImplementedError(
            "XTTS provider requires an XTTS server. "
            "Please set up an XTTS server and provide the server URL."
        )


def create_xtts_provider(**config) -> XTTSProvider:
    """Factory function for creating XTTSProvider."""
    return XTTSProvider(**config)


__all__ = ["create_xtts_provider", "XTTSProvider"]
