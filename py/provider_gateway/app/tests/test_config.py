"""Tests for configuration loading."""

import os
import tempfile
import pytest
from pathlib import Path

from app.config.settings import (
    Settings,
    ServerConfig,
    TelemetryConfig,
    ProviderConfig,
    get_settings,
    reset_settings,
    reload_settings,
)


class TestSettings:
    """Test cases for Settings."""

    def setup_method(self):
        """Reset settings before each test."""
        reset_settings()
        # Clear environment variables that might affect tests
        for key in list(os.environ.keys()):
            if key.startswith("PROVIDER_GATEWAY_") or key.startswith("SERVER_"):
                del os.environ[key]

    def teardown_method(self):
        """Reset settings after each test."""
        reset_settings()
        # Clean up environment variables
        for key in list(os.environ.keys()):
            if key.startswith("PROVIDER_GATEWAY_") or key.startswith("SERVER_"):
                del os.environ[key]

    def test_default_settings(self):
        """Test that default settings load correctly."""
        settings = Settings()

        # Server defaults
        assert settings.server.host == "0.0.0.0"
        assert settings.server.port == 50051
        assert settings.server.max_workers == 10

        # Telemetry defaults
        assert settings.telemetry.log_level == "INFO"
        assert settings.telemetry.metrics_port == 9090
        assert settings.telemetry.otel_endpoint is None

        # Provider defaults
        assert settings.providers.asr_default == "mock"
        assert settings.providers.llm_default == "mock"
        assert settings.providers.tts_default == "mock"

    def test_env_override(self):
        """Test that environment variables override defaults."""
        os.environ["SERVER_HOST"] = "127.0.0.1"
        os.environ["SERVER_PORT"] = "8080"
        os.environ["TELEMETRY_LOG_LEVEL"] = "DEBUG"
        os.environ["PROVIDER_GATEWAY_PROVIDERS__ASR_DEFAULT"] = "google"

        settings = Settings()

        assert settings.server.host == "127.0.0.1"
        assert settings.server.port == 8080
        assert settings.telemetry.log_level == "DEBUG"
        assert settings.providers.asr_default == "google"

    def test_from_yaml(self):
        """Test loading settings from YAML file."""
        yaml_content = """
server:
  host: "192.168.1.1"
  port: 9090
  max_workers: 20

telemetry:
  log_level: "WARNING"
  metrics_port: 8080
  otel_endpoint: "otel:4317"

providers:
  asr_default: "google"
  llm_default: "openai"
  tts_default: "elevenlabs"
  configs:
    google:
      api_key: "test-key"
      region: "us-east1"
"""

        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            f.write(yaml_content)
            temp_path = f.name

        try:
            settings = Settings.from_yaml(temp_path)

            assert settings.server.host == "192.168.1.1"
            assert settings.server.port == 9090
            assert settings.server.max_workers == 20
            assert settings.telemetry.log_level == "WARNING"
            assert settings.telemetry.metrics_port == 8080
            assert settings.telemetry.otel_endpoint == "otel:4317"
            assert settings.providers.asr_default == "google"
            assert settings.providers.llm_default == "openai"
            assert settings.providers.tts_default == "elevenlabs"
            assert settings.providers.configs["google"]["api_key"] == "test-key"
        finally:
            os.unlink(temp_path)

    def test_from_yaml_file_not_found(self):
        """Test that loading from non-existent YAML file raises error."""
        with pytest.raises(FileNotFoundError):
            Settings.from_yaml("/nonexistent/path/config.yaml")

    def test_load_with_config_env_var(self):
        """Test loading settings from YAML specified in env var."""
        yaml_content = """
server:
  port: 7777
telemetry:
  log_level: "ERROR"
"""

        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            f.write(yaml_content)
            temp_path = f.name

        try:
            os.environ["PROVIDER_GATEWAY_CONFIG"] = temp_path
            reset_settings()

            settings = get_settings()

            assert settings.server.port == 7777
            assert settings.telemetry.log_level == "ERROR"
        finally:
            os.unlink(temp_path)

    def test_get_provider_config(self):
        """Test getting provider-specific configuration."""
        yaml_content = """
providers:
  configs:
    openai:
      api_key: "sk-test"
      model: "gpt-4"
    google:
      credentials_path: "/path/to/creds.json"
"""

        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            f.write(yaml_content)
            temp_path = f.name

        try:
            settings = Settings.from_yaml(temp_path)

            openai_config = settings.get_provider_config("openai")
            assert openai_config["api_key"] == "sk-test"
            assert openai_config["model"] == "gpt-4"

            google_config = settings.get_provider_config("google")
            assert google_config["credentials_path"] == "/path/to/creds.json"

            # Non-existent provider returns empty dict
            empty_config = settings.get_provider_config("nonexistent")
            assert empty_config == {}
        finally:
            os.unlink(temp_path)

    def test_telemetry_log_level_validation(self):
        """Test that invalid log levels are rejected."""
        with pytest.raises(ValueError):
            TelemetryConfig(log_level="INVALID")

    def test_telemetry_log_level_normalization(self):
        """Test that log levels are normalized to uppercase."""
        config = TelemetryConfig(log_level="debug")
        assert config.log_level == "DEBUG"

        config = TelemetryConfig(log_level="Warning")
        assert config.log_level == "WARNING"


class TestSingletonSettings:
    """Test cases for singleton settings."""

    def setup_method(self):
        """Reset settings before each test."""
        reset_settings()

    def teardown_method(self):
        """Reset settings after each test."""
        reset_settings()

    def test_get_settings_returns_same_instance(self):
        """Test that get_settings returns the same instance."""
        settings1 = get_settings()
        settings2 = get_settings()
        assert settings1 is settings2

    def test_reset_settings_creates_new_instance(self):
        """Test that reset_settings creates a new instance."""
        settings1 = get_settings()
        reset_settings()
        settings2 = get_settings()
        assert settings1 is not settings2

    def test_reload_settings(self):
        """Test that reload_settings reloads from environment."""
        settings1 = get_settings()

        # Change environment
        os.environ["SERVER_PORT"] = "9999"

        settings2 = reload_settings()

        assert settings1 is not settings2
        assert settings2.server.port == 9999

        # Clean up
        del os.environ["SERVER_PORT"]


class TestServerConfig:
    """Test cases for ServerConfig."""

    def test_default_values(self):
        """Test ServerConfig default values."""
        config = ServerConfig()
        assert config.host == "0.0.0.0"
        assert config.port == 50051
        assert config.max_workers == 10

    def test_custom_values(self):
        """Test ServerConfig with custom values."""
        config = ServerConfig(host="127.0.0.1", port=8080, max_workers=20)
        assert config.host == "127.0.0.1"
        assert config.port == 8080
        assert config.max_workers == 20


class TestProviderConfig:
    """Test cases for ProviderConfig."""

    def test_default_values(self):
        """Test ProviderConfig default values."""
        config = ProviderConfig()
        assert config.asr_default == "mock"
        assert config.llm_default == "mock"
        assert config.tts_default == "mock"
        assert config.configs == {}

    def test_extra_fields_allowed(self):
        """Test that ProviderConfig allows extra fields."""
        config = ProviderConfig(custom_field="value", another_field=123)
        # Should not raise validation error
        assert config.asr_default == "mock"
