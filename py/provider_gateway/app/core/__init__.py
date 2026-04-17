"""Core framework for provider gateway."""

from app.core.base_provider import (
    BaseASRProvider,
    BaseLLMProvider,
    BaseProvider,
    BaseTTSProvider,
)
from app.core.capability import ProviderCapability
from app.core.errors import (
    ProviderError,
    ProviderErrorCode,
    is_retriable,
    normalize_error,
)
from app.core.registry import (
    ProviderRegistry,
    get_registry,
    reset_registry,
)

__all__ = [
    # Base providers
    "BaseASRProvider",
    "BaseLLMProvider",
    "BaseProvider",
    "BaseTTSProvider",
    # Capabilities
    "ProviderCapability",
    # Errors
    "ProviderError",
    "ProviderErrorCode",
    "is_retriable",
    "normalize_error",
    # Registry
    "ProviderRegistry",
    "get_registry",
    "reset_registry",
]
