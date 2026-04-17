"""OpenAI-compatible LLM provider adapter (supports vLLM and Groq)."""

import json
import logging
from typing import AsyncIterator, List, Optional

import httpx

from app.core.base_provider import BaseLLMProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.common import SessionContext, TimingMetadata
from app.models.llm import ChatMessage, LLMOptions, LLMResponse, UsageMetadata

logger = logging.getLogger(__name__)


class OpenAICompatibleLLMProvider(BaseLLMProvider):
    """
    OpenAI-compatible LLM provider adapter.

    Supports both local vLLM and remote endpoints like Groq via configuration.
    Uses httpx for async HTTP requests and handles SSE streaming responses.
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8000/v1",
        api_key: Optional[str] = None,
        model: str = "default",
        timeout: float = 60.0,
    ) -> None:
        """
        Initialize OpenAI-compatible LLM provider.

        Args:
            base_url: Base URL for the API (e.g., http://localhost:8000/v1 for vLLM
                     or https://api.groq.com/openai/v1 for Groq).
            api_key: API key for authentication (optional for local vLLM).
            model: Model name to use.
            timeout: Request timeout in seconds.
        """
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._model = model
        self._timeout = timeout
        self._client: Optional[httpx.AsyncClient] = None
        self._cancelled_sessions: set = set()

        logger.info(
            f"OpenAICompatibleLLMProvider initialized with base_url={base_url}, model={model}"
        )

    def _get_client(self) -> httpx.AsyncClient:
        """Get or create HTTP client."""
        if self._client is None:
            headers = {
                "Content-Type": "application/json",
                "Accept": "text/event-stream",
            }
            if self._api_key:
                headers["Authorization"] = f"Bearer {self._api_key}"

            self._client = httpx.AsyncClient(
                base_url=self._base_url,
                headers=headers,
                timeout=self._timeout,
            )
        return self._client

    def name(self) -> str:
        """Return the provider name."""
        return f"openai_compatible_{self._model}"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,
            supports_streaming_output=True,
            supports_word_timestamps=False,
            supports_voices=False,
            supports_interruptible_generation=True,
            preferred_sample_rates=[],
            supported_codecs=[],
        )

    async def stream_generate(
        self,
        messages: List[ChatMessage],
        options: Optional[LLMOptions] = None,
    ) -> AsyncIterator[LLMResponse]:
        """
        Stream generate tokens from the LLM.

        Makes streaming POST to /v1/chat/completions with stream=true,
        parses SSE responses.

        Args:
            messages: List of chat messages.
            options: LLM options for generation.

        Yields:
            LLMResponse with generated token and metadata.
        """
        # Generate session ID
        import hashlib
        message_hash = hashlib.md5(
            "".join(m.content for m in messages).encode()
        ).hexdigest()[:16]
        session_id = f"openai_{message_hash}"

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"LLM session {session_id} cancelled")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
            model_name=self._model,
        )

        # Build request payload
        opts = options or LLMOptions()
        payload = {
            "model": opts.model or self._model,
            "messages": [{"role": m.role, "content": m.content} for m in messages],
            "stream": True,
        }

        # Add optional parameters
        if opts.max_tokens > 0:
            payload["max_tokens"] = opts.max_tokens
        if opts.temperature > 0:
            payload["temperature"] = opts.temperature
        if opts.top_p > 0:
            payload["top_p"] = opts.top_p
        if opts.stop_sequences:
            payload["stop"] = opts.stop_sequences

        # Merge provider options
        for key, value in opts.provider_options.items():
            if key not in payload:
                payload[key] = value

        client = self._get_client()

        try:
            async with client.stream(
                "POST",
                "/chat/completions",
                json=payload,
            ) as response:
                if response.status_code != 200:
                    error_text = await response.aread()
                    raise ProviderError(
                        code=ProviderErrorCode.SERVICE_UNAVAILABLE,
                        message=f"HTTP {response.status_code}: {error_text.decode()}",
                        provider_name=self.name(),
                    )

                prompt_tokens = 0
                completion_tokens = 0

                async for line in response.aiter_lines():
                    # Check for cancellation
                    if session_id in self._cancelled_sessions:
                        self._cancelled_sessions.discard(session_id)
                        logger.info(f"LLM session {session_id} cancelled during streaming")

                        yield LLMResponse(
                            session_context=session_context,
                            token="",
                            is_final=True,
                            finish_reason="stop",
                            usage=UsageMetadata(
                                prompt_tokens=prompt_tokens,
                                completion_tokens=completion_tokens,
                                total_tokens=prompt_tokens + completion_tokens,
                            ),
                            timing=TimingMetadata(),
                        )
                        return

                    # Parse SSE line
                    if not line.startswith("data: "):
                        continue

                    data = line[6:]  # Remove "data: " prefix

                    if data == "[DONE]":
                        break

                    try:
                        chunk = json.loads(data)
                    except json.JSONDecodeError:
                        continue

                    # Extract content from delta
                    delta = chunk.get("choices", [{}])[0].get("delta", {})
                    content = delta.get("content", "")

                    # Extract usage if available
                    usage = chunk.get("usage")
                    if usage:
                        prompt_tokens = usage.get("prompt_tokens", 0)
                        completion_tokens = usage.get("completion_tokens", 0)

                    # Extract finish reason
                    finish_reason = chunk.get("choices", [{}])[0].get("finish_reason", "")

                    if content or finish_reason:
                        yield LLMResponse(
                            session_context=session_context,
                            token=content,
                            is_final=finish_reason is not None and finish_reason != "",
                            finish_reason=finish_reason or "",
                            usage=None,
                            timing=None,
                        )

                # Final response with usage
                yield LLMResponse(
                    session_context=session_context,
                    token="",
                    is_final=True,
                    finish_reason="stop",
                    usage=UsageMetadata(
                        prompt_tokens=prompt_tokens,
                        completion_tokens=completion_tokens,
                        total_tokens=prompt_tokens + completion_tokens,
                    ),
                    timing=TimingMetadata(),
                )

                logger.debug(f"OpenAI-compatible LLM generation completed for session {session_id}")

        except httpx.ConnectError as e:
            raise ProviderError(
                code=ProviderErrorCode.SERVICE_UNAVAILABLE,
                message=f"Connection error: {str(e)}. Check that the LLM server is running.",
                provider_name=self.name(),
                retriable=True,
            ) from e
        except httpx.TimeoutException as e:
            raise ProviderError(
                code=ProviderErrorCode.TIMEOUT,
                message=f"Request timeout: {str(e)}",
                provider_name=self.name(),
                retriable=True,
            ) from e
        except Exception as e:
            raise ProviderError(
                code=ProviderErrorCode.INTERNAL,
                message=f"Request failed: {str(e)}",
                provider_name=self.name(),
            ) from e

    async def cancel(self, session_id: str) -> bool:
        """
        Cancel an ongoing generation.

        Args:
            session_id: The session ID to cancel.

        Returns:
            True if cancellation was acknowledged.
        """
        self._cancelled_sessions.add(session_id)
        logger.info(f"LLM cancellation requested for session {session_id}")
        return True

    async def close(self) -> None:
        """Close the HTTP client."""
        if self._client:
            await self._client.aclose()
            self._client = None


def create_openai_compatible_provider(**config) -> OpenAICompatibleLLMProvider:
    """Factory function for creating OpenAICompatibleLLMProvider."""
    return OpenAICompatibleLLMProvider(**config)


__all__ = ["create_openai_compatible_provider", "OpenAICompatibleLLMProvider"]
