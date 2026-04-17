"""LLM providers module."""

from typing import TYPE_CHECKING

from app.providers.llm.groq import create_groq_provider
from app.providers.llm.mock_llm import create_mock_llm_provider
from app.providers.llm.openai_compatible import create_openai_compatible_provider

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """
    Register all LLM providers with the registry.

    Args:
        registry: The provider registry to register with.
    """
    # Register mock provider
    registry.register_llm("mock", create_mock_llm_provider)

    # Register OpenAI-compatible provider (for vLLM)
    registry.register_llm("openai_compatible", create_openai_compatible_provider)

    # Register Groq provider
    registry.register_llm("groq", create_groq_provider)


__all__ = [
    "create_groq_provider",
    "create_mock_llm_provider",
    "create_openai_compatible_provider",
    "register_providers",
]
