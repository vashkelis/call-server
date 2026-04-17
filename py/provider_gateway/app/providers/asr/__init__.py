"""ASR providers module."""

from typing import TYPE_CHECKING

from app.providers.asr.faster_whisper import create_faster_whisper_provider
from app.providers.asr.google_speech import create_google_speech_provider
from app.providers.asr.mock_asr import create_mock_asr_provider

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """
    Register all ASR providers with the registry.

    Args:
        registry: The provider registry to register with.
    """
    # Register mock provider
    registry.register_asr("mock", create_mock_asr_provider)

    # Register faster-whisper provider
    registry.register_asr("faster_whisper", create_faster_whisper_provider)

    # Register Google Speech provider
    registry.register_asr("google_speech", create_google_speech_provider)


__all__ = [
    "create_faster_whisper_provider",
    "create_google_speech_provider",
    "create_mock_asr_provider",
    "register_providers",
]
