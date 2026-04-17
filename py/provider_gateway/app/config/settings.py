"""Pydantic Settings configuration for provider gateway."""

import os
from pathlib import Path
from typing import Any, Dict, Optional

import yaml
from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class ServerConfig(BaseSettings):
    """Server configuration."""

    model_config = SettingsConfigDict(env_prefix="SERVER_")

    host: str = "0.0.0.0"
    port: int = 50051
    max_workers: int = 10


class TelemetryConfig(BaseSettings):
    """Telemetry configuration."""

    model_config = SettingsConfigDict(env_prefix="TELEMETRY_")

    log_level: str = "INFO"
    metrics_port: int = 9090
    otel_endpoint: Optional[str] = None

    @field_validator("log_level")
    @classmethod
    def validate_log_level(cls, v: str) -> str:
        """Validate log level."""
        valid_levels = ["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"]
        v_upper = v.upper()
        if v_upper not in valid_levels:
            raise ValueError(f"Invalid log level: {v}. Must be one of {valid_levels}")
        return v_upper


class ProviderConfig(BaseSettings):
    """Provider-specific configuration."""

    model_config = SettingsConfigDict(extra="allow")

    asr_default: str = "mock"
    llm_default: str = "mock"
    tts_default: str = "mock"
    configs: Dict[str, Dict[str, Any]] = Field(default_factory=dict)


class Settings(BaseSettings):
    """
    Main application settings.

    Loads from YAML file (PROVIDER_GATEWAY_CONFIG env var) with env var overrides.
    """

    model_config = SettingsConfigDict(
        env_prefix="PROVIDER_GATEWAY_",
        env_nested_delimiter="__",
        extra="ignore",
    )

    server: ServerConfig = Field(default_factory=ServerConfig)
    telemetry: TelemetryConfig = Field(default_factory=TelemetryConfig)
    providers: ProviderConfig = Field(default_factory=ProviderConfig)

    @classmethod
    def from_yaml(cls, yaml_path: str) -> "Settings":
        """
        Load settings from YAML file.

        Args:
            yaml_path: Path to YAML configuration file.

        Returns:
            Settings instance loaded from YAML.
        """
        path = Path(yaml_path)
        if not path.exists():
            raise FileNotFoundError(f"Config file not found: {yaml_path}")

        with open(path, "r") as f:
            config_data = yaml.safe_load(f) or {}

        return cls(**config_data)

    @classmethod
    def load(cls) -> "Settings":
        """
        Load settings from environment.

        First checks for PROVIDER_GATEWAY_CONFIG env var to load YAML file,
        then applies environment variable overrides.

        Returns:
            Settings instance.
        """
        config_path = os.getenv("PROVIDER_GATEWAY_CONFIG")

        if config_path:
            try:
                settings = cls.from_yaml(config_path)
            except FileNotFoundError:
                # Fall back to env-only config
                settings = cls()
        else:
            settings = cls()

        return settings

    def get_provider_config(self, provider_name: str) -> Dict[str, Any]:
        """
        Get configuration for a specific provider.

        Args:
            provider_name: Name of the provider.

        Returns:
            Provider configuration dictionary.
        """
        return self.providers.configs.get(provider_name, {})


# Singleton settings instance
_settings: Optional[Settings] = None


def get_settings() -> Settings:
    """Get the singleton settings instance."""
    global _settings
    if _settings is None:
        _settings = Settings.load()
    return _settings


def reset_settings() -> None:
    """Reset the singleton settings (useful for testing)."""
    global _settings
    _settings = None


def reload_settings() -> Settings:
    """Reload settings from environment/config file."""
    global _settings
    _settings = Settings.load()
    return _settings


__all__ = [
    "get_settings",
    "reload_settings",
    "reset_settings",
    "ServerConfig",
    "Settings",
    "TelemetryConfig",
    "ProviderConfig",
]
