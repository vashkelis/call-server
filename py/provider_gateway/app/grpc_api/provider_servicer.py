"""Provider gRPC servicer implementation."""

import logging
from typing import Optional

from app.core.capability import ProviderCapability as ProviderCapabilityModel
from app.core.registry import ProviderRegistry
from app.grpc_api.generated.common_pb2 import (
    HealthCheckRequest,
    HealthCheckResponse,
    ProviderCapability,
    ProviderStatus,
    ProviderType,
    ServingStatus,
)
from app.grpc_api.generated.provider_pb2 import (
    GetProviderInfoRequest,
    ListProvidersRequest,
    ListProvidersResponse,
    ProviderInfo,
)
from app.grpc_api.generated.provider_pb2_grpc import ProviderServiceServicer
from app.telemetry import get_logger

logger = get_logger(__name__)


class ProviderServicer(ProviderServiceServicer):
    """Provider service servicer implementation."""

    def __init__(self, registry: ProviderRegistry, version: str = "0.1.0") -> None:
        """
        Initialize Provider servicer.

        Args:
            registry: Provider registry for looking up providers.
            version: Service version string.
        """
        self._registry = registry
        self._version = version
        logger.info("ProviderServicer initialized")

    async def ListProviders(
        self,
        request: ListProvidersRequest,
    ) -> ListProvidersResponse:
        """
        List available providers by type.

        Args:
            request: ListProvidersRequest with provider type filter.

        Returns:
            ListProvidersResponse with list of providers.
        """
        providers: list = []

        provider_type = request.provider_type

        # Map proto provider type to internal type
        if provider_type == ProviderType.PROVIDER_TYPE_UNSPECIFIED:
            # List all providers
            providers.extend(self._list_asr_providers())
            providers.extend(self._list_llm_providers())
            providers.extend(self._list_tts_providers())
        elif provider_type == ProviderType.ASR:
            providers.extend(self._list_asr_providers())
        elif provider_type == ProviderType.LLM:
            providers.extend(self._list_llm_providers())
        elif provider_type == ProviderType.TTS:
            providers.extend(self._list_tts_providers())

        return ListProvidersResponse(providers=providers)

    def _list_asr_providers(self) -> list:
        """List ASR providers."""
        providers = []
        for name in self._registry.list_asr_providers():
            caps = self._registry.get_provider_capabilities(name, "asr")
            providers.append(
                ProviderInfo(
                    name=name,
                    provider_type=ProviderType.ASR,
                    version=self._version,
                    capabilities=self._convert_capabilities(caps),
                    status=ProviderStatus.AVAILABLE,
                )
            )
        return providers

    def _list_llm_providers(self) -> list:
        """List LLM providers."""
        providers = []
        for name in self._registry.list_llm_providers():
            caps = self._registry.get_provider_capabilities(name, "llm")
            providers.append(
                ProviderInfo(
                    name=name,
                    provider_type=ProviderType.LLM,
                    version=self._version,
                    capabilities=self._convert_capabilities(caps),
                    status=ProviderStatus.AVAILABLE,
                )
            )
        return providers

    def _list_tts_providers(self) -> list:
        """List TTS providers."""
        providers = []
        for name in self._registry.list_tts_providers():
            caps = self._registry.get_provider_capabilities(name, "tts")
            providers.append(
                ProviderInfo(
                    name=name,
                    provider_type=ProviderType.TTS,
                    version=self._version,
                    capabilities=self._convert_capabilities(caps),
                    status=ProviderStatus.AVAILABLE,
                )
            )
        return providers

    def _convert_capabilities(
        self,
        caps: Optional[ProviderCapabilityModel],
    ) -> Optional[ProviderCapability]:
        """Convert internal ProviderCapability to proto ProviderCapability."""
        if caps is None:
            return None

        return ProviderCapability(
            supports_streaming_input=caps.supports_streaming_input,
            supports_streaming_output=caps.supports_streaming_output,
            supports_word_timestamps=caps.supports_word_timestamps,
            supports_voices=caps.supports_voices,
            supports_interruptible_generation=caps.supports_interruptible_generation,
            preferred_sample_rates=caps.preferred_sample_rates,
            supported_codecs=caps.supported_codecs,
        )

    async def GetProviderInfo(self, request: GetProviderInfoRequest) -> ProviderInfo:
        """
        Get detailed info about a specific provider.

        Args:
            request: GetProviderInfoRequest with provider name and type.

        Returns:
            ProviderInfo with provider details.
        """
        provider_name = request.provider_name
        provider_type = request.provider_type

        caps = None
        if provider_type == ProviderType.ASR:
            caps = self._registry.get_provider_capabilities(provider_name, "asr")
        elif provider_type == ProviderType.LLM:
            caps = self._registry.get_provider_capabilities(provider_name, "llm")
        elif provider_type == ProviderType.TTS:
            caps = self._registry.get_provider_capabilities(provider_name, "tts")

        return ProviderInfo(
            name=provider_name,
            provider_type=provider_type,
            version=self._version,
            capabilities=self._convert_capabilities(caps),
            status=ProviderStatus.AVAILABLE if caps else ProviderStatus.UNAVAILABLE,
        )

    async def HealthCheck(self, request: HealthCheckRequest) -> HealthCheckResponse:
        """
        Health check for provider service.

        Args:
            request: HealthCheckRequest with service name.

        Returns:
            HealthCheckResponse with serving status.
        """
        service_name = request.service_name or "provider-gateway"

        return HealthCheckResponse(
            status=ServingStatus.SERVING,
            service_name=service_name,
            version=self._version,
        )


__all__ = ["ProviderServicer"]
