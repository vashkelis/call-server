"""LLM gRPC servicer implementation."""

import logging
from typing import AsyncIterator, Optional

from app.core.errors import ProviderError, normalize_error
from app.core.registry import ProviderRegistry
from app.grpc_api.generated.common_pb2 import (
    CancelRequest,
    CancelResponse,
    CapabilityRequest,
    ProviderCapability,
    SessionContext,
    TimingMetadata,
)
from app.grpc_api.generated.llm_pb2 import ChatMessage, LLMRequest, LLMResponse, UsageMetadata
from app.grpc_api.generated.llm_pb2_grpc import LLMServiceServicer
from app.models.llm import ChatMessage as ChatMessageModel, LLMOptions, LLMResponse as LLMResponseModel
from app.telemetry import get_logger, record_error, record_request, start_span

logger = get_logger(__name__)


class LLMServicer(LLMServiceServicer):
    """LLM service servicer implementation."""

    def __init__(self, registry: ProviderRegistry) -> None:
        """
        Initialize LLM servicer.

        Args:
            registry: Provider registry for looking up LLM providers.
        """
        self._registry = registry
        self._active_sessions: dict = {}
        logger.info("LLMServicer initialized")

    async def StreamGenerate(
        self,
        request: LLMRequest,
    ) -> AsyncIterator[LLMResponse]:
        """
        Server streaming for prompt input and token output.

        Args:
            request: LLMRequest with messages.

        Yields:
            LLMResponse messages with generated tokens.
        """
        session_id: Optional[str] = None
        provider_name: Optional[str] = None

        try:
            # Get session and provider info
            if request.session_context:
                session_id = request.session_context.session_id
                provider_name = request.session_context.provider_name

            if not provider_name:
                provider_name = "mock"  # Default provider

            # Get provider from registry
            provider = self._registry.get_llm(provider_name)
            if provider is None:
                raise ProviderError(
                    message=f"LLM provider not found: {provider_name}",
                    provider_name=provider_name,
                )

            record_request(provider_name, "llm")

            # Track active session
            if session_id:
                self._active_sessions[session_id] = provider

            # Convert messages
            messages = [
                ChatMessageModel(role=msg.role, content=msg.content)
                for msg in request.messages
            ]

            # Create LLM options
            options = LLMOptions(
                max_tokens=request.max_tokens or 1024,
                temperature=request.temperature or 0.7,
                top_p=request.top_p or 1.0,
                stop_sequences=list(request.stop_sequences),
                provider_options=dict(request.provider_options),
            )

            # Stream responses from provider
            with start_span("llm.stream_generate", {"provider": provider_name}):
                async for response in provider.stream_generate(messages, options):
                    yield self._convert_response(response, session_id)

        except Exception as e:
            error = normalize_error(e, provider_name or "unknown")
            record_error(provider_name or "unknown", "llm", error.code.name)
            logger.error(f"LLM streaming error: {error}")
            raise
        finally:
            if session_id and session_id in self._active_sessions:
                del self._active_sessions[session_id]

    def _convert_response(
        self,
        response: LLMResponseModel,
        session_id: Optional[str],
    ) -> LLMResponse:
        """Convert internal LLMResponse to gRPC LLMResponse."""
        # Build session context
        session_context = None
        if response.session_context:
            sc = response.session_context
            session_context = SessionContext(
                session_id=sc.session_id,
                turn_id=sc.turn_id,
                generation_id=sc.generation_id,
                tenant_id=sc.tenant_id,
                trace_id=sc.trace_id,
                provider_name=sc.provider_name,
                model_name=sc.model_name,
            )
        elif session_id:
            session_context = SessionContext(session_id=session_id)

        # Build timing metadata
        timing = None
        if response.timing:
            timing = TimingMetadata(
                duration_ms=response.timing.duration_ms,
            )

        # Build usage metadata
        usage = None
        if response.usage:
            usage = UsageMetadata(
                prompt_tokens=response.usage.prompt_tokens,
                completion_tokens=response.usage.completion_tokens,
                total_tokens=response.usage.total_tokens,
            )

        return LLMResponse(
            session_context=session_context,
            token=response.token,
            is_final=response.is_final,
            finish_reason=response.finish_reason,
            usage=usage,
            timing=timing,
        )

    async def Cancel(self, request: CancelRequest) -> CancelResponse:
        """
        Cancel an ongoing generation.

        Args:
            request: CancelRequest with session context.

        Returns:
            CancelResponse indicating acknowledgment.
        """
        session_id = None
        generation_id = ""

        if request.session_context:
            session_id = request.session_context.session_id
            generation_id = request.session_context.generation_id

        if session_id and session_id in self._active_sessions:
            provider = self._active_sessions[session_id]
            try:
                acknowledged = await provider.cancel(session_id)
                return CancelResponse(
                    acknowledged=acknowledged,
                    generation_id=generation_id,
                )
            except Exception as e:
                logger.error(f"Error canceling LLM session {session_id}: {e}")

        return CancelResponse(
            acknowledged=False,
            generation_id=generation_id,
        )

    async def GetCapabilities(self, request: CapabilityRequest) -> ProviderCapability:
        """
        Get provider capabilities.

        Args:
            request: CapabilityRequest with provider name.

        Returns:
            ProviderCapability with capability flags.
        """
        provider_name = request.provider_name or "mock"
        provider = self._registry.get_llm(provider_name)

        if provider is None:
            raise ProviderError(
                message=f"LLM provider not found: {provider_name}",
                provider_name=provider_name,
            )

        caps = provider.capabilities()
        return ProviderCapability(
            supports_streaming_input=caps.supports_streaming_input,
            supports_streaming_output=caps.supports_streaming_output,
            supports_word_timestamps=caps.supports_word_timestamps,
            supports_voices=caps.supports_voices,
            supports_interruptible_generation=caps.supports_interruptible_generation,
            preferred_sample_rates=caps.preferred_sample_rates,
            supported_codecs=caps.supported_codecs,
        )


__all__ = ["LLMServicer"]
