"""Tests for the provider registry."""

import pytest
from app.core.registry import ProviderRegistry, get_registry, reset_registry
from app.providers.asr.mock_asr import MockASRProvider, create_mock_asr_provider
from app.providers.llm.mock_llm import MockLLMProvider, create_mock_llm_provider
from app.providers.tts.mock_tts import MockTTSProvider, create_mock_tts_provider


class TestProviderRegistry:
    """Test cases for ProviderRegistry."""

    @pytest.fixture
    def registry(self):
        """Create a fresh registry for each test."""
        return ProviderRegistry()

    def test_register_and_get_asr_provider(self, registry):
        """Test registering and retrieving an ASR provider."""
        registry.register_asr("mock", create_mock_asr_provider)

        provider = registry.get_asr("mock")
        assert provider is not None
        assert isinstance(provider, MockASRProvider)
        assert provider.name() == "mock_asr"

    def test_register_and_get_llm_provider(self, registry):
        """Test registering and retrieving an LLM provider."""
        registry.register_llm("mock", create_mock_llm_provider)

        provider = registry.get_llm("mock")
        assert provider is not None
        assert isinstance(provider, MockLLMProvider)
        assert provider.name() == "mock_llm"

    def test_register_and_get_tts_provider(self, registry):
        """Test registering and retrieving a TTS provider."""
        registry.register_tts("mock", create_mock_tts_provider)

        provider = registry.get_tts("mock")
        assert provider is not None
        assert isinstance(provider, MockTTSProvider)
        assert provider.name() == "mock_tts"

    def test_provider_not_found(self, registry):
        """Test that requesting an unknown provider returns None."""
        asr = registry.get_asr("nonexistent")
        assert asr is None

        llm = registry.get_llm("nonexistent")
        assert llm is None

        tts = registry.get_tts("nonexistent")
        assert tts is None

    def test_list_asr_providers(self, registry):
        """Test listing registered ASR providers."""
        registry.register_asr("mock1", create_mock_asr_provider)
        registry.register_asr("mock2", create_mock_asr_provider)

        providers = registry.list_asr_providers()
        assert len(providers) == 2
        assert "mock1" in providers
        assert "mock2" in providers

    def test_list_llm_providers(self, registry):
        """Test listing registered LLM providers."""
        registry.register_llm("mock1", create_mock_llm_provider)
        registry.register_llm("mock2", create_mock_llm_provider)

        providers = registry.list_llm_providers()
        assert len(providers) == 2
        assert "mock1" in providers
        assert "mock2" in providers

    def test_list_tts_providers(self, registry):
        """Test listing registered TTS providers."""
        registry.register_tts("mock1", create_mock_tts_provider)
        registry.register_tts("mock2", create_mock_tts_provider)

        providers = registry.list_tts_providers()
        assert len(providers) == 2
        assert "mock1" in providers
        assert "mock2" in providers

    def test_duplicate_registration_overwrites(self, registry):
        """Test that duplicate registration overwrites the previous provider."""
        registry.register_asr("mock", create_mock_asr_provider)

        # Create a custom factory that returns a provider with different config
        def custom_factory(**config):
            return MockASRProvider(transcript="Custom transcript")

        registry.register_asr("mock", custom_factory)

        provider = registry.get_asr("mock")
        assert provider is not None

    def test_get_provider_with_config(self, registry):
        """Test getting a provider with configuration."""
        registry.register_asr("mock", create_mock_asr_provider)

        provider = registry.get_asr("mock", transcript="Custom transcript")
        assert provider is not None
        assert isinstance(provider, MockASRProvider)

    def test_get_provider_capabilities(self, registry):
        """Test getting provider capabilities."""
        registry.register_asr("mock", create_mock_asr_provider)

        caps = registry.get_provider_capabilities("mock", "asr")
        assert caps is not None
        assert caps.supports_streaming_input is True
        assert caps.supports_streaming_output is True

    def test_get_capabilities_unknown_provider(self, registry):
        """Test getting capabilities for unknown provider returns None."""
        caps = registry.get_provider_capabilities("nonexistent", "asr")
        assert caps is None

    def test_get_capabilities_unknown_type(self, registry):
        """Test getting capabilities for unknown provider type returns None."""
        registry.register_asr("mock", create_mock_asr_provider)

        caps = registry.get_provider_capabilities("mock", "unknown")
        assert caps is None

    def test_empty_registry_lists(self, registry):
        """Test that empty registry returns empty lists."""
        assert registry.list_asr_providers() == []
        assert registry.list_llm_providers() == []
        assert registry.list_tts_providers() == []

    def test_provider_caching(self, registry):
        """Test that providers are cached."""
        registry.register_asr("mock", create_mock_asr_provider)

        # Get provider twice with same config
        provider1 = registry.get_asr("mock")
        provider2 = registry.get_asr("mock")

        # Should be same instance due to caching
        assert provider1 is provider2

    def test_provider_different_config_not_cached(self, registry):
        """Test that providers with different configs are not cached together."""
        registry.register_asr("mock", create_mock_asr_provider)

        # Get provider with different configs
        provider1 = registry.get_asr("mock", transcript="Transcript 1")
        provider2 = registry.get_asr("mock", transcript="Transcript 2")

        # Should be different instances
        assert provider1 is not provider2


class TestSingletonRegistry:
    """Test cases for the singleton registry."""

    def setup_method(self):
        """Reset singleton before each test."""
        reset_registry()

    def teardown_method(self):
        """Reset singleton after each test."""
        reset_registry()

    def test_get_registry_returns_same_instance(self):
        """Test that get_registry returns the same instance."""
        reg1 = get_registry()
        reg2 = get_registry()
        assert reg1 is reg2

    def test_reset_registry_creates_new_instance(self):
        """Test that reset_registry creates a new instance."""
        reg1 = get_registry()
        reset_registry()
        reg2 = get_registry()
        assert reg1 is not reg2
