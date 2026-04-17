"""Shared pytest fixtures for provider gateway tests."""

import pytest
import pytest_asyncio
from app.core.registry import ProviderRegistry, get_registry, reset_registry
from app.config.settings import Settings, get_settings, reset_settings
from app.providers.asr.mock_asr import create_mock_asr_provider
from app.providers.llm.mock_llm import create_mock_llm_provider
from app.providers.tts.mock_tts import create_mock_tts_provider


@pytest.fixture
def registry():
    """Create a fresh provider registry for testing.
    
    This fixture creates a new ProviderRegistry instance that is
    isolated from the singleton registry.
    """
    return ProviderRegistry()


@pytest.fixture
def registry_with_mocks():
    """Create a provider registry with mock providers registered.
    
    This fixture creates a new ProviderRegistry and registers
    mock ASR, LLM, and TTS providers.
    """
    registry = ProviderRegistry()
    registry.register_asr("mock", create_mock_asr_provider)
    registry.register_llm("mock", create_mock_llm_provider)
    registry.register_tts("mock", create_mock_tts_provider)
    return registry


@pytest.fixture(scope="function")
def reset_singletons():
    """Reset all singleton instances before and after test.
    
    This ensures tests don't interfere with each other through
    shared singleton state.
    """
    reset_registry()
    reset_settings()
    yield
    reset_registry()
    reset_settings()


@pytest.fixture
def settings():
    """Create fresh settings for testing.
    
    This fixture creates a new Settings instance with test values.
    """
    return Settings(
        server={
            "host": "127.0.0.1",
            "port": 50051,
            "max_workers": 5,
        },
        telemetry={
            "log_level": "DEBUG",
            "metrics_port": 9090,
        },
        providers={
            "asr_default": "mock",
            "llm_default": "mock",
            "tts_default": "mock",
        },
    )


@pytest.fixture
def test_config_with_yaml(tmp_path):
    """Create a temporary YAML config file for testing.
    
    Returns the path to a temporary YAML config file with test settings.
    """
    config_content = """
server:
  host: "127.0.0.1"
  port: 50051
  max_workers: 10

telemetry:
  log_level: "DEBUG"
  metrics_port: 9090

providers:
  asr_default: "mock"
  llm_default: "mock"
  tts_default: "mock"
  configs:
    mock:
      test_mode: true
"""
    config_file = tmp_path / "test_config.yaml"
    config_file.write_text(config_content)
    return str(config_file)


@pytest.fixture
def mock_asr_provider():
    """Create a mock ASR provider instance."""
    return create_mock_asr_provider()


@pytest.fixture
def mock_llm_provider():
    """Create a mock LLM provider instance."""
    return create_mock_llm_provider()


@pytest.fixture
def mock_tts_provider():
    """Create a mock TTS provider instance."""
    return create_mock_tts_provider()


@pytest.fixture
def mock_audio_stream():
    """Create a mock async audio stream for testing ASR.
    
    Yields PCM16 audio chunks.
    """
    async def stream():
        # Yield some dummy PCM16 audio chunks
        for _ in range(5):
            yield b"\x00\x01\x02\x03" * 80  # 320 bytes = 160 samples
    return stream()


@pytest.fixture
def mock_chat_messages():
    """Create mock chat messages for testing LLM."""
    from app.models.llm import ChatMessage
    return [
        ChatMessage(role="system", content="You are a helpful assistant."),
        ChatMessage(role="user", content="Hello!"),
    ]


@pytest_asyncio.fixture
async def async_cleanup():
    """Provide a cleanup mechanism for async tests.
    
    Use this fixture to ensure async resources are properly cleaned up.
    """
    resources = []
    
    def register(coro):
        """Register a coroutine for cleanup."""
        resources.append(coro)
        return coro
    
    yield register
    
    # Cleanup
    for coro in resources:
        try:
            await coro
        except Exception:
            pass  # Ignore cleanup errors


# Pytest configuration
def pytest_configure(config):
    """Configure pytest."""
    # Add custom markers
    config.addinivalue_line(
        "markers", "slow: marks tests as slow (deselect with '-m \"not slow\"')"
    )
    config.addinivalue_line(
        "markers", "integration: marks tests as integration tests"
    )
    config.addinivalue_line(
        "markers", "asyncio: marks tests as async tests"
    )


def pytest_collection_modifyitems(config, items):
    """Modify test collection."""
    # Add asyncio marker to async tests automatically
    for item in items:
        if hasattr(item, "obj") and hasattr(item.obj, "__code__"):
            import inspect
            if inspect.iscoroutinefunction(item.obj):
                item.add_marker(pytest.mark.asyncio)
