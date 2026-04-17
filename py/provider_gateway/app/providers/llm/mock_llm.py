"""Mock LLM provider for testing."""

import asyncio
import logging
from typing import AsyncIterator, List, Optional

from app.core.base_provider import BaseLLMProvider
from app.core.capability import ProviderCapability
from app.models.common import SessionContext, TimingMetadata
from app.models.llm import ChatMessage, LLMOptions, LLMResponse, UsageMetadata

logger = logging.getLogger(__name__)


class MockLLMProvider(BaseLLMProvider):
    """
    Mock LLM provider that produces canned streaming token responses.

    Simulates realistic token-by-token generation with configurable delay.
    Useful for testing and development.
    """

    # Default response text
    DEFAULT_RESPONSE = (
        "I understand your request. Let me help you with that. "
        "Is there anything else you'd like to know?"
    )

    def __init__(
        self,
        response_text: Optional[str] = None,
        token_delay_ms: float = 50.0,
        tokens_per_chunk: int = 1,
    ) -> None:
        """
        Initialize Mock LLM provider.

        Args:
            response_text: Custom response text (default: DEFAULT_RESPONSE).
            token_delay_ms: Delay between tokens in milliseconds.
            tokens_per_chunk: Number of tokens to yield per chunk.
        """
        self._response_text = response_text or self.DEFAULT_RESPONSE
        self._token_delay_ms = token_delay_ms
        self._tokens_per_chunk = tokens_per_chunk
        self._cancelled_sessions: set = set()
        logger.info("MockLLMProvider initialized")

    def name(self) -> str:
        """Return the provider name."""
        return "mock_llm"

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

        Produces canned streaming token responses with configurable delay.

        Args:
            messages: List of chat messages.
            options: LLM options for generation.

        Yields:
            LLMResponse with generated token and metadata.
        """
        # Generate session ID from messages
        import hashlib
        message_hash = hashlib.md5(
            "".join(m.content for m in messages).encode()
        ).hexdigest()[:16]
        session_id = f"mock_{message_hash}"

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"LLM session {session_id} cancelled")
            return

        # Create session context
        session_context = SessionContext(
            session_id=session_id,
            provider_name=self.name(),
        )

        # Tokenize response (simple word-based tokenization)
        tokens = self._tokenize(self._response_text)

        # Calculate usage
        prompt_tokens = sum(len(m.content.split()) for m in messages)
        completion_tokens = len(tokens)

        # Stream tokens
        current_chunk = ""
        tokens_in_chunk = 0

        for i, token in enumerate(tokens):
            # Check for cancellation
            if session_id in self._cancelled_sessions:
                self._cancelled_sessions.discard(session_id)
                logger.info(f"LLM session {session_id} cancelled during generation")

                # Yield final with stop reason
                yield LLMResponse(
                    session_context=session_context,
                    token=current_chunk,
                    is_final=True,
                    finish_reason="stop",
                    usage=UsageMetadata(
                        prompt_tokens=prompt_tokens,
                        completion_tokens=i + 1,
                        total_tokens=prompt_tokens + i + 1,
                    ),
                    timing=TimingMetadata(),
                )
                return

            current_chunk += token
            tokens_in_chunk += 1

            # Yield chunk when we have enough tokens
            if tokens_in_chunk >= self._tokens_per_chunk:
                is_last = i == len(tokens) - 1

                yield LLMResponse(
                    session_context=session_context,
                    token=current_chunk,
                    is_final=is_last,
                    finish_reason="stop" if is_last else "",
                    usage=None if not is_last else UsageMetadata(
                        prompt_tokens=prompt_tokens,
                        completion_tokens=completion_tokens,
                        total_tokens=prompt_tokens + completion_tokens,
                    ),
                    timing=None if not is_last else TimingMetadata(),
                )

                current_chunk = ""
                tokens_in_chunk = 0

                # Delay between chunks
                if not is_last:
                    await asyncio.sleep(self._token_delay_ms / 1000.0)

        # Yield any remaining tokens
        if current_chunk:
            yield LLMResponse(
                session_context=session_context,
                token=current_chunk,
                is_final=True,
                finish_reason="stop",
                usage=UsageMetadata(
                    prompt_tokens=prompt_tokens,
                    completion_tokens=completion_tokens,
                    total_tokens=prompt_tokens + completion_tokens,
                ),
                timing=TimingMetadata(),
            )

        logger.debug(f"Mock LLM generation completed for session {session_id}")

    def _tokenize(self, text: str) -> List[str]:
        """
        Simple word-based tokenization.

        Args:
            text: Text to tokenize.

        Returns:
            List of tokens (words with trailing spaces).
        """
        words = text.split()
        tokens = []
        for i, word in enumerate(words):
            # Add space after word except for last word or if word ends with punctuation
            if i < len(words) - 1 and not word[-1] in ".!?":
                tokens.append(word + " ")
            else:
                tokens.append(word)
        return tokens

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


def create_mock_llm_provider(**config) -> MockLLMProvider:
    """Factory function for creating MockLLMProvider."""
    return MockLLMProvider(**config)


__all__ = ["MockLLMProvider", "create_mock_llm_provider"]
