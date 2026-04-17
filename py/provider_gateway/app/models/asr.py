"""ASR Pydantic models for provider gateway."""

from typing import List, Optional

from pydantic import BaseModel, Field

from app.models.common import AudioFormat, SessionContext, TimingMetadata


class WordTimestamp(BaseModel):
    """Word-level timestamp."""

    word: str = Field(default="", description="The word")
    start_ms: int = Field(default=0, description="Start time in milliseconds")
    end_ms: int = Field(default=0, description="End time in milliseconds")


class ASROptions(BaseModel):
    """ASR recognition options."""

    language_hint: str = Field(default="en-US", description="Language code hint")
    enable_word_timestamps: bool = Field(default=False, description="Enable word timestamps")
    enable_automatic_punctuation: bool = Field(
        default=True, description="Enable automatic punctuation"
    )
    model: Optional[str] = Field(default=None, description="Model name to use")
    sample_rate: int = Field(default=16000, description="Audio sample rate")


class ASRRequest(BaseModel):
    """ASR request containing audio chunks."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    audio_chunk: bytes = Field(default=b"", description="Audio data chunk")
    audio_format: Optional[AudioFormat] = Field(default=None, description="Audio format")
    language_hint: str = Field(default="", description="Language code hint")
    is_final: bool = Field(default=False, description="Whether this is the final chunk")


class ASRResponse(BaseModel):
    """ASR response containing transcripts."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    transcript: str = Field(default="", description="Transcribed text")
    is_partial: bool = Field(default=False, description="Whether this is a partial result")
    is_final: bool = Field(default=False, description="Whether this is the final result")
    confidence: float = Field(default=0.0, description="Confidence score (0-1)")
    language: str = Field(default="", description="Detected language")
    word_timestamps: List[WordTimestamp] = Field(
        default_factory=list, description="Word-level timestamps"
    )
    timing: Optional[TimingMetadata] = Field(default=None, description="Timing metadata")


__all__ = [
    "ASROptions",
    "ASRRequest",
    "ASRResponse",
    "WordTimestamp",
]
