"""Common Pydantic models for provider gateway."""

from datetime import datetime
from enum import Enum
from typing import Any, Dict, Optional

from pydantic import BaseModel, Field


class AudioEncoding(str, Enum):
    """Audio encoding formats."""

    PCM16 = "pcm16"
    OPUS = "opus"
    G711_ULAW = "g711_ulaw"
    G711_ALAW = "g711_alaw"


class AudioFormat(BaseModel):
    """Audio format specification."""

    sample_rate: int = Field(default=16000, description="Sample rate in Hz")
    channels: int = Field(default=1, description="Number of audio channels")
    encoding: AudioEncoding = Field(default=AudioEncoding.PCM16, description="Audio encoding")


class SessionContext(BaseModel):
    """Session context shared across all services."""

    session_id: str = Field(default="", description="Unique session identifier")
    turn_id: str = Field(default="", description="Turn identifier within session")
    generation_id: str = Field(default="", description="Unique generation identifier")
    tenant_id: Optional[str] = Field(default=None, description="Tenant identifier")
    trace_id: str = Field(default="", description="Distributed trace identifier")
    created_at: Optional[datetime] = Field(default=None, description="Session creation time")
    updated_at: Optional[datetime] = Field(default=None, description="Last update time")
    options: Dict[str, str] = Field(default_factory=dict, description="Additional options")
    provider_name: str = Field(default="", description="Selected provider name")
    model_name: str = Field(default="", description="Selected model name")


class TimingMetadata(BaseModel):
    """Timing metadata for tracking operation duration."""

    start_time: Optional[datetime] = Field(default=None, description="Operation start time")
    end_time: Optional[datetime] = Field(default=None, description="Operation end time")
    duration_ms: int = Field(default=0, description="Operation duration in milliseconds")


class ProviderInfo(BaseModel):
    """Provider information."""

    name: str = Field(default="", description="Provider name")
    provider_type: str = Field(default="", description="Provider type (asr, llm, tts)")
    version: str = Field(default="", description="Provider version")
    status: str = Field(default="available", description="Provider status")
    metadata: Dict[str, str] = Field(default_factory=dict, description="Additional metadata")


__all__ = [
    "AudioEncoding",
    "AudioFormat",
    "ProviderInfo",
    "SessionContext",
    "TimingMetadata",
]
