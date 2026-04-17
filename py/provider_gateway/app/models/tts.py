"""TTS Pydantic models for provider gateway."""

from typing import Dict, Optional

from pydantic import BaseModel, Field

from app.models.common import AudioFormat, SessionContext, TimingMetadata


class TTSOptions(BaseModel):
    """TTS synthesis options."""

    voice_id: str = Field(default="default", description="Voice identifier")
    audio_format: Optional[AudioFormat] = Field(default=None, description="Output audio format")
    speed: float = Field(default=1.0, description="Speech speed multiplier")
    pitch: float = Field(default=1.0, description="Pitch multiplier")
    volume: float = Field(default=1.0, description="Volume multiplier")
    provider_options: Dict[str, str] = Field(
        default_factory=dict, description="Provider-specific options"
    )


class TTSRequest(BaseModel):
    """TTS request containing text to synthesize."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    text: str = Field(default="", description="Text to synthesize")
    voice_id: str = Field(default="", description="Voice identifier")
    audio_format: Optional[AudioFormat] = Field(default=None, description="Output audio format")
    segment_index: int = Field(default=0, description="Segment index for multi-part synthesis")
    provider_options: Dict[str, str] = Field(
        default_factory=dict, description="Provider-specific options"
    )


class TTSResponse(BaseModel):
    """TTS response containing audio chunks."""

    session_context: Optional[SessionContext] = Field(
        default=None, description="Session context"
    )
    audio_chunk: bytes = Field(default=b"", description="Audio data chunk")
    audio_format: Optional[AudioFormat] = Field(default=None, description="Audio format")
    segment_index: int = Field(default=0, description="Segment index")
    is_final: bool = Field(default=False, description="Whether this is the final chunk")
    timing: Optional[TimingMetadata] = Field(default=None, description="Timing metadata")


__all__ = [
    "TTSOptions",
    "TTSRequest",
    "TTSResponse",
]
