"""TTS gRPC servicer implementation."""

import logging
from typing import AsyncIterator, Optional

from app.core.errors import ProviderError, normalize_error
from app.core.registry import ProviderRegistry
from app.grpc_api.generated.common_pb2 import (
    AudioEncoding,
    AudioFormat,
    CancelRequest,
    CancelResponse,
    CapabilityRequest,
    ProviderCapability,
    SessionContext,
    TimingMetadata,
)
from app.grpc_api.generated.tts_pb2 import TTSRequest, TTSResponse
from app.grpc_api.generated.tts_pb2_grpc import TTSServiceServicer
from app.models.common import AudioFormat as AudioFormatModel
from app.models.tts import TTSOptions, TTSResponse as TTSResponseModel
from app.telemetry import get_logger, record_error, record_request, start_span

logger = get_logger(__name__)


class TTSServicer(TTSServiceServicer):
    """TTS service servicer implementation."""

    def __init__(self, registry: ProviderRegistry) -> None:
        """
        Initialize TTS servicer.

        Args:
            registry: Provider registry for looking up TTS providers.
        """
        self._registry = registry
        self._active_sessions: dict = {}
        logger.info("TTSServicer initialized")

    async def StreamSynthesize(
        self,
        request: TTSRequest,
    ) -> AsyncIterator[TTSResponse]:
        """
        Server streaming for text input and audio output.

        Args:
            request: TTSRequest with text to synthesize.

        Yields:
            TTSResponse messages with audio chunks.
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
            provider = self._registry.get_tts(provider_name)
            if provider is None:
                raise ProviderError(
                    message=f"TTS provider not found: {provider_name}",
                    provider_name=provider_name,
                )

            record_request(provider_name, "tts")

            # Track active session
            if session_id:
                self._active_sessions[session_id] = provider

            # Convert audio format
            audio_format = None
            if request.audio_format:
                audio_format = AudioFormatModel(
                    sample_rate=request.audio_format.sample_rate,
                    channels=request.audio_format.channels,
                    encoding=AudioEncoding(request.audio_format.encoding).name.lower(),
                )

            # Create TTS options
            options = TTSOptions(
                voice_id=request.voice_id or "default",
                audio_format=audio_format,
                provider_options=dict(request.provider_options),
            )

            # Stream responses from provider
            with start_span("tts.stream_synthesize", {"provider": provider_name}):
                async for response in provider.stream_synthesize(request.text, options):
                    yield self._convert_response(response, session_id)

        except Exception as e:
            error = normalize_error(e, provider_name or "unknown")
            record_error(provider_name or "unknown", "tts", error.code.name)
            logger.error(f"TTS streaming error: {error}")
            raise
        finally:
            if session_id and session_id in self._active_sessions:
                del self._active_sessions[session_id]

    def _convert_response(
        self,
        response: TTSResponseModel,
        session_id: Optional[str],
    ) -> TTSResponse:
        """Convert internal TTSResponse to gRPC TTSResponse."""
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

        # Build audio format
        audio_format = None
        if response.audio_format:
            encoding_value = AudioEncoding.PCM16
            try:
                encoding_value = AudioEncoding[response.audio_format.encoding.upper()]
            except (KeyError, AttributeError):
                pass

            audio_format = AudioFormat(
                sample_rate=response.audio_format.sample_rate,
                channels=response.audio_format.channels,
                encoding=encoding_value,
            )

        return TTSResponse(
            session_context=session_context,
            audio_chunk=response.audio_chunk,
            audio_format=audio_format,
            segment_index=response.segment_index,
            is_final=response.is_final,
            timing=timing,
        )

    async def Cancel(self, request: CancelRequest) -> CancelResponse:
        """
        Cancel an ongoing synthesis.

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
                logger.error(f"Error canceling TTS session {session_id}: {e}")

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
        provider = self._registry.get_tts(provider_name)

        if provider is None:
            raise ProviderError(
                message=f"TTS provider not found: {provider_name}",
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


__all__ = ["TTSServicer"]
