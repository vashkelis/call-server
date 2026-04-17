# Adding an LLM Provider

This guide walks you through adding a new Large Language Model (LLM) provider to CloudApp.

## Overview

LLM providers generate text responses from conversation history. They must implement the `BaseLLMProvider` abstract class and support streaming token generation with cancellation support.

## Step-by-Step Guide

### 1. Create Provider File

Create a new file in `/py/provider_gateway/app/providers/llm/`:

```bash
touch /py/provider_gateway/app/providers/llm/my_llm_provider.py
```

### 2. Implement BaseLLMProvider

```python
"""My Custom LLM Provider."""

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


class MyLLMProvider(BaseLLMProvider):
    """Custom LLM provider implementation."""

    def __init__(
        self,
        api_key: Optional[str] = None,
        base_url: str = "https://api.example.com/v1",
        model: str = "default",
        timeout: float = 60.0,
    ) -> None:
        """
        Initialize the LLM provider.

        Args:
            api_key: API key for authentication.
            base_url: Base URL for the API endpoint.
            model: Model name to use.
            timeout: Request timeout in seconds.
        """
        self._api_key = api_key
        self._base_url = base_url.rstrip("/")
        self._model = model
        self._timeout = timeout
        self._client: Optional[httpx.AsyncClient] = None
        self._cancelled_sessions: set = set()

        logger.info(f"MyLLMProvider initialized with model={model}")

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
        return f"my_llm_{self._model}"

    def capabilities(self) -> ProviderCapability:
        """Return the provider capabilities."""
        return ProviderCapability(
            supports_streaming_input=False,     # LLM receives complete messages
            supports_streaming_output=True,     # Supports streaming tokens
            supports_word_timestamps=False,     # Not applicable for LLM
            supports_voices=False,              # Not applicable for LLM
            supports_interruptible_generation=True,  # Supports cancellation
            preferred_sample_rates=[],          # Not applicable
            supported_codecs=[],                # Not applicable
        )

    async def stream_generate(
        self,
        messages: List[ChatMessage],
        options: Optional[LLMOptions] = None,
    ) -> AsyncIterator[LLMResponse]:
        """
        Stream generate tokens from the LLM.

        Args:
            messages: List of chat messages with role and content.
            options: LLM options for generation (temperature, max_tokens, etc.).

        Yields:
            LLMResponse with generated token and metadata.
        """
        # Generate session ID from message hash
        import hashlib
        message_hash = hashlib.md5(
            "".join(m.content for m in messages).encode()
        ).hexdigest()[:16]
        session_id = f"my_llm_{message_hash}"

        # Check for cancellation
        if session_id in self._cancelled_sessions:
            self._cancelled_sessions.discard(session_id)
            logger.info(f"LLM session {session_id} cancelled before start")
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
            "stream": True,  # Enable streaming
        }

        # Add optional parameters
        if opts.max_tokens > 0:
            payload["max_tokens"] = opts.max_tokens
        if opts.temperature >= 0:
            payload["temperature"] = opts.temperature
        if opts.top_p > 0:
            payload["top_p"] = opts.top_p
        if opts.stop_sequences:
            payload["stop"] = opts.stop_sequences

        client = self._get_client()
        prompt_tokens = 0
        completion_tokens = 0

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

                async for line in response.aiter_lines():
                    # Check for cancellation
                    if session_id in self._cancelled_sessions:
                        self._cancelled_sessions.discard(session_id)
                        logger.info(f"LLM session {session_id} cancelled during streaming")

                        # Yield final response with usage
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

                    # Parse SSE line (OpenAI-compatible format)
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
                            usage=None,  # Only include in final response
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

                logger.debug(f"LLM generation completed for session {session_id}")

        except httpx.ConnectError as e:
            raise ProviderError(
                code=ProviderErrorCode.SERVICE_UNAVAILABLE,
                message=f"Connection error: {str(e)}",
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


def create_my_llm_provider(**config) -> MyLLMProvider:
    """Factory function for creating MyLLMProvider."""
    return MyLLMProvider(**config)
```

### 3. Register in `__init__.py`

Edit `/py/provider_gateway/app/providers/llm/__init__.py`:

```python
"""LLM providers module."""

from typing import TYPE_CHECKING

from app.providers.llm.groq import create_groq_provider
from app.providers.llm.mock_llm import create_mock_llm_provider
from app.providers.llm.openai_compatible import create_openai_compatible_provider
from app.providers.llm.my_llm_provider import create_my_llm_provider  # Add this

if TYPE_CHECKING:
    from app.core.registry import ProviderRegistry


def register_providers(registry: "ProviderRegistry") -> None:
    """Register all LLM providers with the registry."""
    registry.register_llm("mock", create_mock_llm_provider)
    registry.register_llm("openai_compatible", create_openai_compatible_provider)
    registry.register_llm("groq", create_groq_provider)
    registry.register_llm("my_provider", create_my_llm_provider)  # Add this


__all__ = [
    "create_groq_provider",
    "create_mock_llm_provider",
    "create_openai_compatible_provider",
    "create_my_llm_provider",  # Add this
    "register_providers",
]
```

### 4. Add Configuration

Add provider-specific configuration to your YAML config:

```yaml
providers:
  defaults:
    llm: "my_provider"  # Set as default

  llm:
    my_provider:
      api_key: "${MY_LLM_API_KEY}"
      base_url: "https://api.example.com/v1"
      model: "gpt-4"
      timeout: 60.0
```

### 5. Testing

Create a test file `/py/provider_gateway/app/tests/test_my_llm_provider.py`:

```python
"""Tests for MyLLMProvider."""

import pytest
from app.models.llm import ChatMessage
from app.providers.llm.my_llm_provider import create_my_llm_provider


@pytest.fixture
def provider():
    return create_my_llm_provider(api_key="test-key")


@pytest.mark.asyncio
async def test_provider_name(provider):
    assert "my_llm" in provider.name()


@pytest.mark.asyncio
async def test_provider_capabilities(provider):
    caps = provider.capabilities()
    assert caps.supports_streaming_output is True
    assert caps.supports_interruptible_generation is True


@pytest.mark.asyncio
async def test_stream_generate(provider):
    messages = [
        ChatMessage(role="system", content="You are a helpful assistant."),
        ChatMessage(role="user", content="Hello!"),
    ]

    # Note: This would need mocking for actual testing
    # results = []
    # async for response in provider.stream_generate(messages):
    #     results.append(response)
    #
    # assert len(results) > 0
    # assert results[-1].is_final is True
```

Run the tests:

```bash
cd py/provider_gateway
python -m pytest app/tests/test_my_llm_provider.py -v
```

## Reference Implementation

For a complete working example, see:

- `/py/provider_gateway/app/providers/llm/openai_compatible.py` — OpenAI-compatible API adapter

This implementation supports:
- vLLM local inference
- Groq API
- Any OpenAI-compatible endpoint

## Key Requirements

### Message Format

Messages follow the standard chat format:

```python
class ChatMessage:
    role: str      # "system", "user", or "assistant"
    content: str   # Message text
```

### Streaming Behavior

1. **Token Streaming**: Yield tokens as they arrive from the LLM
2. **Final Response**: Yield `is_final=True` with usage metadata
3. **Cancellation**: Check `self._cancelled_sessions` and stop if cancelled

### Error Handling

Common error scenarios:

| Error | Code | Retriable |
|-------|------|-----------|
| Connection failed | `SERVICE_UNAVAILABLE` | Yes |
| Timeout | `TIMEOUT` | Yes |
| Rate limited | `RATE_LIMITED` | Yes |
| Invalid API key | `AUTHENTICATION` | No |
| Model not found | `INVALID_REQUEST` | No |

### Performance Guidelines

- First token within 500ms
- Token streaming latency < 100ms between tokens
- Support cancellation within 50ms

## Troubleshooting

### Tokens not streaming

Ensure your API request includes `stream: true` and you're parsing SSE (Server-Sent Events) format correctly.

### Cancellation not working

Check that you're:
1. Checking `self._cancelled_sessions` in your streaming loop
2. Yielding a final response before returning
3. Not blocking on I/O without cancellation checks

### Usage metadata missing

Some APIs only return usage in the final chunk. Ensure you're extracting it and including it in the final `LLMResponse`.
