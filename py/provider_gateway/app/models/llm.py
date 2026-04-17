"""LLM Pydantic models for provider gateway."""

from typing import Dict, List, Optional

from pydantic import BaseModel, Field

from app.models.common import SessionContext, TimingMetadata


class ChatMessage(BaseModel):
    """Chat message for LLM conversation."""

    role: str = Field(default="user", description="Message role (system, user, assistant)")
    content: str = Field(default="", description="Message content")


class UsageMetadata(BaseModel):
    """Token usage metadata."""

    prompt_tokens: int = Field(default=0, description="Number of prompt tokens")
    completion_tokens: int = Field(default=0, description="Number of completion tokens")
    total_tokens: int = Field(default=0, description="Total number of tokens")


class LLMOptions(BaseModel):
    """LLM generation options."""

    max_tokens: int = Field(default=1024, description="Maximum tokens to generate")
    temperature: float = Field(default=0.7, description="Sampling temperature")
    top_p: float = Field(default=1.0, description="Nucleus sampling parameter")
    stop_sequences: List[str] = Field(
        default_factory=list, description="Sequences to stop generation"
    )
    model: Optional[str] = Field(default=None, description="Model name to use")
    provider_options: Dict[str, str] = Field(
        default_factory=dict, description="Provider-specific options"
    )


class LLMRequest(BaseModel):
    """LLM request containing conversation messages."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    messages: List[ChatMessage] = Field(default_factory=list, description="Chat messages")
    max_tokens: int = Field(default=0, description="Maximum tokens to generate")
    temperature: float = Field(default=0.0, description="Sampling temperature")
    top_p: float = Field(default=0.0, description="Nucleus sampling parameter")
    stop_sequences: List[str] = Field(
        default_factory=list, description="Sequences to stop generation"
    )
    provider_options: Dict[str, str] = Field(
        default_factory=dict, description="Provider-specific options"
    )


class LLMResponse(BaseModel):
    """LLM response containing generated tokens."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    token: str = Field(default="", description="Generated token")
    is_final: bool = Field(default=False, description="Whether this is the final token")
    finish_reason: str = Field(default="", description="Reason for finishing (stop, length, etc.)")
    usage: Optional[UsageMetadata] = Field(default=None, description="Token usage metadata")
    timing: Optional[TimingMetadata] = Field(default=None, description="Timing metadata")


__all__ = [
    "ChatMessage",
    "LLMOptions",
    "LLMRequest",
    "LLMResponse",
    "UsageMetadata",
]
