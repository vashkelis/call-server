"""Groq LLM provider adapter."""

import logging
from typing import AsyncIterator, List, Optional

from app.core.base_provider import BaseLLMProvider
from app.core.capability import ProviderCapability
from app.core.errors import ProviderError, ProviderErrorCode
from app.models.common import SessionContext, TimingMetadata
from app.models.llm import ChatMessage, LLMOptions, LLMResponse, UsageMetadata
from app.providers.llm.openai_compatible import OpenAICompatibleLLMProvider

logger = logging.getLogger(__name__)


class GroqProvider(OpenAICompatibleLLMProvider):
    """
    Dedicated Groq LLM provider adapter.

    Extends OpenAI-compatible provider with Groq-specific defaults and error handling.
    """

    DEFAULT_BASE_URL = "https://api.groq.com/openai/v1"
    DEFAULT_MODEL = "llama3-8b-8192"

    def __init__(
        self,
        api_key: str,
        model: Optional[str] = None,
        timeout: float = 60.0,
    ) -> None:
        """
        Initialize Groq LLM provider.

        Args:
            api_key: Groq API key.
            model: Model name to use (default: llama3-8b-8192).
            timeout: Request timeout in seconds.
        """
        super().__init__(
            base_url=self.DEFAULT_BASE_URL,
            api_key=api_key,
            model=model or self.DEFAULT_MODEL,
            timeout=timeout,
        )
        logger.info(f"GroqProvider initialized with model={self._model}")

    def name(self) -> str:
        """Return the provider name."""
        return f"groq_{self._model}"

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
        Stream generate tokens from Groq.

        Args:
            messages: List of chat messages.
            options: LLM options for generation.

        Yields:
            LLMResponse with generated token and metadata.

        Raises:
            ProviderError: With Groq-specific error codes.
        """
        try:
            async for response in super().stream_generate(messages, options):
                yield response
        except ProviderError:
            raise
        except Exception as e:
            error_msg = str(e).lower()

            # Map Groq-specific errors
            if "rate limit" in error_msg or "rate_limit" in error_msg:
                raise ProviderError(
                    code=ProviderErrorCode.RATE_LIMITED,
                    message=f"Groq rate limit exceeded: {str(e)}",
                    provider_name=self.name(),
                    retriable=True,
                ) from e
            elif "quota" in error_msg:
                raise ProviderError(
                    code=ProviderErrorCode.QUOTA_EXCEEDED,
                    message=f"Groq quota exceeded: {str(e)}",
                    provider_name=self.name(),
                ) from e
            elif "invalid" in error_msg and "api key" in error_msg:
                raise ProviderError(
                    code=ProviderErrorCode.AUTHENTICATION,
                    message=f"Groq authentication failed: {str(e)}",
                    provider_name=self.name(),
                ) from e
            else:
                raise ProviderError(
                    code=ProviderErrorCode.INTERNAL,
                    message=f"Groq request failed: {str(e)}",
                    provider_name=self.name(),
                ) from e


def create_groq_provider(**config) -> GroqProvider:
    """Factory function for creating GroqProvider."""
    return GroqProvider(**config)


__all__ = ["create_groq_provider", "GroqProvider"]
