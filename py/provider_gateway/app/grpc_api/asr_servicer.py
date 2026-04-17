"""ASR gRPC servicer implementation."""

import asyncio
import logging
from typing import AsyncIterator, Optional

from app.core.errors import ProviderError, normalize_error
from app.core.registry import ProviderRegistry
from app.grpc_api.generated.asr_pb2 import ASRRequest, ASRResponse, WordTimestamp
from app.grpc_api.generated.asr_pb2_grpc import ASRServiceServicer
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
from app.models.asr import ASROptions, ASRResponse as ASRResponseModel
from app.models.common import AudioFormat as AudioFormatModel, TimingMetadata as TimingMetadataModel
from app.telemetry import get_logger, record_error, record_request, start_span

logger = get_logger(__name__)


class ASRServicer(ASRServiceServicer):
    """ASR service servicer implementation."""

    def __init__(self, registry: ProviderRegistry) -> None:
        """
        Initialize ASR servicer.

        Args:
            registry: Provider registry for looking up ASR providers.
        """
        self._registry = registry
        self._active_sessions: dict = {}
        logger.info("ASRServicer initialized")

    async def StreamingRecognize(
        self,
        request_iterator: AsyncIterator[ASRRequest],
    ) -> AsyncIterator[ASRResponse]:
        """
        Bidirectional streaming for audio input and transcript output.

        Args:
            request_iterator: Async iterator of ASRRequest messages.

        Yields:
            ASRResponse messages with transcripts.
        """
        session_id: Optional[str] = None
        provider_name: Optional[str] = None

        try:
            # Get first request to determine session and provider
            first_request = await request_iterator.__anext__()

            if first_request.session_context:
                session_id = first_request.session_context.session_id
                provider_name = first_request.session_context.provider_name

            if not provider_name:
                provider_name = "mock"  # Default provider

            # Get provider from registry
            provider = self._registry.get_asr(provider_name)
            if provider is None:
                raise ProviderError(
                    message=f"ASR provider not found: {provider_name}",
                    provider_name=provider_name,
                )

            record_request(provider_name, "asr")

            # Track active session
            if session_id:
                self._active_sessions[session_id] = provider

            # Convert audio format
            audio_format = None
            if first_request.audio_format:
                audio_format = AudioFormatModel(
                    sample_rate=first_request.audio_format.sample_rate,
                    channels=first_request.audio_format.channels,
                    encoding=AudioEncoding(first_request.audio_format.encoding).name.lower(),
                )

            # Create ASR options
            options = ASROptions(
                language_hint=first_request.language_hint or "en-US",
                sample_rate=audio_format.sample_rate if audio_format else 16000,
            )

            # Create audio stream
            async def audio_stream() -> AsyncIterator[bytes]:
                """Yield audio chunks from requests."""
                yield first_request.audio_chunk
                async for request in request_iterator:
                    yield request.audio_chunk
                    if request.is_final:
                        break

            # Stream responses from provider
            with start_span("asr.streaming_recognize", {"provider": provider_name}):
                async for response in provider.stream_recognize(audio_stream(), options):
                    yield self._convert_response(response, session_id)

        except StopAsyncIteration:
            logger.warning("ASR stream ended without data")
        except Exception as e:
            error = normalize_error(e, provider_name or "unknown")
            record_error(provider_name or "unknown", "asr", error.code.name)
            logger.error(f"ASR streaming error: {error}")
            raise
        finally:
            if session_id and session_id in self._active_sessions:
                del self._active_sessions[session_id]

    def _convert_response(
        self,
        response: ASRResponseModel,
        session_id: Optional[str],
    ) -> ASRResponse:
        """Convert internal ASRResponse to gRPC ASRResponse."""
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

        # Build word timestamps
        word_timestamps = []
        for wt in response.word_timestamps:
            word_timestamps.append(
                WordTimestamp(
                    word=wt.word,
                    start_ms=wt.start_ms,
                    end_ms=wt.end_ms,
                )
            )

        return ASRResponse(
            session_context=session_context,
            transcript=response.transcript,
            is_partial=response.is_partial,
            is_final=response.is_final,
            confidence=response.confidence,
            language=response.language,
            word_timestamps=word_timestamps,
            timing=timing,
        )

    async def Cancel(self, request: CancelRequest) -> CancelResponse:
        """
        Cancel an ongoing recognition.

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
                logger.error(f"Error canceling ASR session {session_id}: {e}")

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
        provider = self._registry.get_asr(provider_name)

        if provider is None:
            raise ProviderError(
                message=f"ASR provider not found: {provider_name}",
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


__all__ = ["ASRServicer"]
