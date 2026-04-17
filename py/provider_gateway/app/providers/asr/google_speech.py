"""Google Speech-to-Text ASR provider stub."""

import logging
from typing import AsyncIterator, Optional

from app.core.base_provider import BaseASRProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.asr import ASROptions, ASRResponse
from app.models.common import SessionContext

logger = logging.getLogger(__name__)


class GoogleSpeechProvider(BaseASRProvider):
    """
    Google Speech-to-Text ASR provider stub.

    Full request/response model is in place, but methods raise NotImplementedError
    with a clear message about requiring google-cloud-speech credentials.
    """

    def __init__(
        self,
        credentials_path: Optional[str] = None,
        language_code: str = "en-US",
        model: Optional[str] = None,
    ) -> None:
        """
        Initialize Google Speech provider.

        Args:
            credentials_path: Path to Google Cloud credentials JSON file.
            language_code: Language code for recognition.
            model: Model to use (latest, command_and_search, phone_call, video, default).
        """
        self._credentials_path = credentials_path
        self._language_code = language_code
        self._model = model
        self._client = None
        logger.info("GoogleSpeechProvider initialized (stub)")

    def name(self) -> str:
        """Return the provider name."""
        return "google_speech"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=True,
            supports_streaming_output=True,
            supports_word_timestamps=True,
            supports_voices=False,
            supports_interruptible_generation=True,
            preferred_sample_rates=[16000, 8000, 24000, 48000],
            supported_codecs=["pcm16", "opus", "g711_ulaw", "g711_alaw"],
        )

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

        Raises:
            NotImplementedError: Always raises with clear message about credentials.
        """
        raise NotImplementedError(
            "Google Speech provider requires google-cloud-speech credentials. "
            "Please install google-cloud-speech and provide valid credentials. "
            "Install with: pip install google-cloud-speech"
        )

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing recognition.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.

        Raises:
            NotImplementedError: Always raises with clear message about credentials.
        """
        raise NotImplementedError(
            "Google Speech provider requires google-cloud-speech credentials. "
            "Please install google-cloud-speech and provide valid credentials."
        )


def create_google_speech_provider(**config) -> GoogleSpeechProvider:
    """Factory function for creating GoogleSpeechProvider."""
    return GoogleSpeechProvider(**config)


__all__ = ["GoogleSpeechProvider", "create_google_speech_provider"]
