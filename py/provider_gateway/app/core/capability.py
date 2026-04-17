"""Provider capability model matching proto definition."""

from dataclasses import dataclass, field
from typing import List


@dataclass
class ProviderCapability:
    """
    Provider capabilities dataclass matching proto definition.

    Attributes:
        supports_streaming_input: Whether provider supports streaming input.
        supports_streaming_output: Whether provider supports streaming output.
        supports_word_timestamps: Whether ASR provider supports word timestamps.
        supports_voices: Whether TTS provider supports multiple voices.
        supports_interruptible_generation: Whether generation can be interrupted.
        preferred_sample_rates: List of preferred audio sample rates.
        supported_codecs: List of supported audio codecs.
    """

    supports_streaming_input: bool = False
    supports_streaming_output: bool = False
    supports_word_timestamps: bool = False
    supports_voices: bool = False
    supports_interruptible_generation: bool = False
    preferred_sample_rates: List[int] = field(default_factory=list)
    supported_codecs: List[str] = field(default_factory=list)

    def to_proto(self) -> "ProviderCapability":  # type: ignore
        """Convert to proto message (when using real protobuf)."""
        from app.grpc_api.generated.common_pb2 import (
            ProviderCapability as ProtoCapability,
        )

        return ProtoCapability(
            supports_streaming_input=self.supports_streaming_input,
            supports_streaming_output=self.supports_streaming_output,
            supports_word_timestamps=self.supports_word_timestamps,
            supports_voices=self.supports_voices,
            supports_interruptible_generation=self.supports_interruptible_generation,
            preferred_sample_rates=self.preferred_sample_rates,
            supported_codecs=self.supported_codecs,
        )

    @classmethod
    def from_proto(cls, proto: "ProviderCapability") -> "ProviderCapability":  # type: ignore
        """Create from proto message."""
        return cls(
            supports_streaming_input=proto.supports_streaming_input,
            supports_streaming_output=proto.supports_streaming_output,
            supports_word_timestamps=proto.supports_word_timestamps,
            supports_voices=proto.supports_voices,
            supports_interruptible_generation=proto.supports_interruptible_generation,
            preferred_sample_rates=list(proto.preferred_sample_rates),
            supported_codecs=list(proto.supported_codecs),
        )


__all__ = ["ProviderCapability"]
