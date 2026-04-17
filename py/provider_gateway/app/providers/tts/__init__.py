"""TTS providers module."""

from typing import TYPE_CHECKING

from app.providers.tts.google_tts import create_google_tts_provider
from app.providers.tts.mock_tts import create_mock_tts_provider
from app.providers.tts.xtts import create_xtts_provider

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """
    Register all TTS providers with the registry.

    Args:
        registry: The provider registry to register with.
    """
    # Register mock provider
    registry.register_tts("mock", create_mock_tts_provider)

    # Register Google TTS provider
    registry.register_tts("google_tts", create_google_tts_provider)

    # Register XTTS provider
    registry.register_tts("xtts", create_xtts_provider)


__all__ = [
    "create_google_tts_provider",
    "create_mock_tts_provider",
    "create_xtts_provider",
    "register_providers",
]
