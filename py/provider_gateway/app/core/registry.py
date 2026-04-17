"""Provider registry with dynamic loading and thread-safe registration."""

import importlib
import inspect
import logging
from typing import Any, Callable, Dict, List, Optional, Type, TypeVar

from app.core.base_provider import BaseASRProvider, BaseLLMProvider, BaseTTSProvider
from app.core.capability import ProviderCapability

logger = logging.getLogger(__name__)

T = TypeVar("T", BaseASRProvider, BaseLLMProvider, BaseTTSProvider)

# Provider type mapping
ProviderType = str


class ProviderRegistry:
    """
    Thread-safe provider registry for managing provider instances.

    Supports dynamic loading and auto-discovery of built-in providers.
    """

    def __init__(self) -> None:
        """Initialize the provider registry."""
        # Factory functions that create provider instances
        self._asr_factories: Dict[str, Callable[..., BaseASRProvider]] = {}
        self._llm_factories: Dict[str, Callable[..., BaseLLMProvider]] = {}
        self._tts_factories: Dict[str, Callable[..., BaseTTSProvider]] = {}

        # Cached provider instances
        self._asr_providers: Dict[str, BaseASRProvider] = {}
        self._llm_providers: Dict[str, BaseLLMProvider] = {}
        self._tts_providers: Dict[str, BaseTTSProvider] = {}

        logger.debug("ProviderRegistry initialized")

    def register_asr(
        self,
        name: str,
        factory: Callable[..., BaseASRProvider],
    ) -> None:
        """
        Register an ASR provider factory.

        Args:
            name: Unique name for the provider.
            factory: Factory function that creates the provider.
        """
        self._asr_factories[name] = factory
        logger.info(f"Registered ASR provider: {name}")

    def register_llm(
        self,
        name: str,
        factory: Callable[..., BaseLLMProvider],
    ) -> None:
        """
        Register an LLM provider factory.

        Args:
            name: Unique name for the provider.
            factory: Factory function that creates the provider.
        """
        self._llm_factories[name] = factory
        logger.info(f"Registered LLM provider: {name}")

    def register_tts(
        self,
        name: str,
        factory: Callable[..., BaseTTSProvider],
    ) -> None:
        """
        Register a TTS provider factory.

        Args:
            name: Unique name for the provider.
            factory: Factory function that creates the provider.
        """
        self._tts_factories[name] = factory
        logger.info(f"Registered TTS provider: {name}")

    def get_asr(self, name: str, **config: Any) -> Optional[BaseASRProvider]:
        """
        Get an ASR provider by name.

        Args:
            name: The provider name.
            **config: Configuration to pass to the factory.

        Returns:
            The provider instance or None if not found.
        """
        # Return cached instance if no config changes
        cache_key = f"{name}_{hash(str(sorted(config.items())))}"
        if cache_key in self._asr_providers:
            return self._asr_providers[cache_key]

        factory = self._asr_factories.get(name)
        if factory is None:
            logger.warning(f"ASR provider not found: {name}")
            return None

        try:
            provider = factory(**config)
            self._asr_providers[cache_key] = provider
            return provider
        except Exception as e:
            logger.error(f"Failed to create ASR provider {name}: {e}")
            return None

    def get_llm(self, name: str, **config: Any) -> Optional[BaseLLMProvider]:
        """
        Get an LLM provider by name.

        Args:
            name: The provider name.
            **config: Configuration to pass to the factory.

        Returns:
            The provider instance or None if not found.
        """
        cache_key = f"{name}_{hash(str(sorted(config.items())))}"
        if cache_key in self._llm_providers:
            return self._llm_providers[cache_key]

        factory = self._llm_factories.get(name)
        if factory is None:
            logger.warning(f"LLM provider not found: {name}")
            return None

        try:
            provider = factory(**config)
            self._llm_providers[cache_key] = provider
            return provider
        except Exception as e:
            logger.error(f"Failed to create LLM provider {name}: {e}")
            return None

    def get_tts(self, name: str, **config: Any) -> Optional[BaseTTSProvider]:
        """
        Get a TTS provider by name.

        Args:
            name: The provider name.
            **config: Configuration to pass to the factory.

        Returns:
            The provider instance or None if not found.
        """
        cache_key = f"{name}_{hash(str(sorted(config.items())))}"
        if cache_key in self._tts_providers:
            return self._tts_providers[cache_key]

        factory = self._tts_factories.get(name)
        if factory is None:
            logger.warning(f"TTS provider not found: {name}")
            return None

        try:
            provider = factory(**config)
            self._tts_providers[cache_key] = provider
            return provider
        except Exception as e:
            logger.error(f"Failed to create TTS provider {name}: {e}")
            return None

    def list_asr_providers(self) -> List[str]:
        """Return list of registered ASR provider names."""
        return list(self._asr_factories.keys())

    def list_llm_providers(self) -> List[str]:
        """Return list of registered LLM provider names."""
        return list(self._llm_factories.keys())

    def list_tts_providers(self) -> List[str]:
        """Return list of registered TTS provider names."""
        return list(self._tts_factories.keys())

    def get_provider_capabilities(self, name: str, provider_type: str) -> Optional[ProviderCapability]:
        """
        Get capabilities for a provider.

        Args:
            name: The provider name.
            provider_type: The type of provider (asr, llm, tts).

        Returns:
            ProviderCapability or None if provider not found.
        """
        provider: Optional[Any] = None

        if provider_type == "asr":
            provider = self.get_asr(name)
        elif provider_type == "llm":
            provider = self.get_llm(name)
        elif provider_type == "tts":
            provider = self.get_tts(name)

        if provider:
            return provider.capabilities()
        return None

    def discover_and_register(self) -> None:
        """
        Auto-discover and register all built-in providers.

        This method imports provider modules and registers any providers found.
        """
        logger.info("Discovering and registering built-in providers...")

        # ASR providers
        try:
            from app.providers.asr import register_providers as register_asr
            register_asr(self)
        except Exception as e:
            logger.warning(f"Could not register ASR providers: {e}")

        # LLM providers
        try:
            from app.providers.llm import register_providers as register_llm
            register_llm(self)
        except Exception as e:
            logger.warning(f"Could not register LLM providers: {e}")

        # TTS providers
        try:
            from app.providers.tts import register_providers as register_tts
            register_tts(self)
        except Exception as e:
            logger.warning(f"Could not register TTS providers: {e}")

        logger.info(
            f"Provider discovery complete. "
            f"ASR: {len(self._asr_factories)}, "
            f"LLM: {len(self._llm_factories)}, "
            f"TTS: {len(self._tts_factories)}"
        )

    def load_provider_module(self, module_path: str) -> None:
        """
        Dynamically load a provider module.

        Args:
            module_path: Python path to the module (e.g., 'my_package.my_provider').
        """
        try:
            module = importlib.import_module(module_path)

            # Look for register function
            if hasattr(module, "register_providers"):
                register_func = getattr(module, "register_providers")
                if callable(register_func):
                    register_func(self)
                    logger.info(f"Loaded provider module: {module_path}")
            else:
                logger.warning(f"Module {module_path} has no register_providers function")
        except Exception as e:
            logger.error(f"Failed to load provider module {module_path}: {e}")


# Singleton registry instance
_registry: Optional[ProviderRegistry] = None


def get_registry() -> ProviderRegistry:
    """Get the singleton provider registry instance."""
    global _registry
    if _registry is None:
        _registry = ProviderRegistry()
    return _registry


def reset_registry() -> None:
    """Reset the singleton registry (useful for testing)."""
    global _registry
    _registry = None


__all__ = [
    "get_registry",
    "ProviderRegistry",
    "reset_registry",
]
