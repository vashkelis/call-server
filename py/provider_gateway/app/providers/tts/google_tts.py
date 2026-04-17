"""Google Text-to-Speech provider stub."""

import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseTTSProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.tts import TTSOptions, TTSResponse

logger = logging.getLogger(__name__)


class GoogleTTSProvider(BaseTTSProvider):
    """
    Google Text-to-Speech provider stub.

    Full request/response model is in place, but methods raise NotImplementedError
    with a clear message about requiring google-cloud-texttospeech credentials.
    """

    def __init__(
        self,
        credentials_path: Optional[str] = None,
        voice_name: str = "en-US-Standard-A",
        language_code: str = "en-US",
    ) -> None:
        """
        Initialize Google TTS provider.

        Args:
            credentials_path: Path to Google Cloud credentials JSON file.
            voice_name: Voice name to use.
            language_code: Language code for synthesis.
        """
        self._credentials_path = credentials_path
        self._voice_name = voice_name
        self._language_code = language_code
        self._client = None
        logger.info("GoogleTTSProvider initialized (stub)")

    def name(self) -> str:
        """Return the provider name."""
        return "google_tts"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,
            supports_streaming_output=True,
            supports_word_timestamps=False,
            supports_voices=True,
            supports_interruptible_generation=True,
            preferred_sample_rates=[24000],
            supported_codecs=["pcm16", "opus", "mp3"],
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
            NotImplementedError: Always raises with clear message about credentials.
        """
        raise NotImplementedError(
            "Google TTS provider requires google-cloud-texttospeech credentials. "
            "Please install google-cloud-texttospeech and provide valid credentials. "
            "Install with: pip install google-cloud-texttospeech"
        )

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing synthesis.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.

        Raises:
            NotImplementedError: Always raises with clear message about credentials.
        """
        raise NotImplementedError(
            "Google TTS provider requires google-cloud-texttospeech credentials. "
            "Please install google-cloud-texttospeech and provide valid credentials."
        )


def create_google_tts_provider(**config) -> GoogleTTSProvider:
    """Factory function for creating GoogleTTSProvider."""
    return GoogleTTSProvider(**config)


__all__ = ["create_google_tts_provider", "GoogleTTSProvider"]
