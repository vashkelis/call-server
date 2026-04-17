"""Provider error handling and normalization."""

from dataclasses import dataclass, field
from enum import IntEnum
from typing import Any, Dict, Optional


class ProviderErrorCode(IntEnum):
    """Error codes for provider errors."""

    UNSPECIFIED = 0
    INTERNAL = 1
    INVALID_REQUEST = 2
    RATE_LIMITED = 3
    QUOTA_EXCEEDED = 4
    TIMEOUT = 5
    SERVICE_UNAVAILABLE = 6
    AUTHENTICATION = 7
    AUTHORIZATION = 8
    UNSUPPORTED_FORMAT = 9
    CANCELED = 10


@dataclass
class ProviderError(Exception):
    """
    Exception class for provider errors.

    Attributes:
        code: Error code indicating the type of error.
        message: Human-readable error message.
        provider_name: Name of the provider that raised the error.
        retriable: Whether the operation can be retried.
        details: Additional error details as key-value pairs.
    """

    code: ProviderErrorCode = ProviderErrorCode.UNSPECIFIED
    message: str = ""
    provider_name: str = ""
    retriable: bool = False
    details: Dict[str, str] = field(default_factory=dict)

    def __post_init__(self) -> None:
        """Initialize exception with message."""
        super().__init__(self.message)

    def __str__(self) -> str:
        """Return string representation of error."""
        return (
            f"ProviderError[{self.code.name}]("
            f"provider={self.provider_name}, "
            f"message={self.message}, "
            f"retriable={self.retriable})"
        )

    def to_dict(self) -> Dict[str, Any]:
        """Convert error to dictionary."""
        return {
            "code": self.code.name,
            "message": self.message,
            "provider_name": self.provider_name,
            "retriable": self.retriable,
            "details": self.details,
        }


def normalize_error(
    error: Exception,
    provider_name: str = "",
    default_code: ProviderErrorCode = ProviderErrorCode.INTERNAL,
) -> ProviderError:
    """
    Normalize any exception into a ProviderError.

    Args:
        error: The exception to normalize.
        provider_name: Name of the provider that raised the error.
        default_code: Default error code if not a ProviderError.

    Returns:
        A normalized ProviderError instance.
    """
    if isinstance(error, ProviderError):
        if provider_name and not error.provider_name:
            error.provider_name = provider_name
        return error

    # Map common exception types to error codes
    error_code = default_code
    retriable = False

    error_type = type(error).__name__
    error_msg = str(error)

    if "timeout" in error_msg.lower() or "timeout" in error_type.lower():
        error_code = ProviderErrorCode.TIMEOUT
        retriable = True
    elif "connection" in error_msg.lower() or "connection" in error_type.lower():
        error_code = ProviderErrorCode.SERVICE_UNAVAILABLE
        retriable = True
    elif "auth" in error_msg.lower():
        if "unauthorized" in error_msg.lower():
            error_code = ProviderErrorCode.AUTHORIZATION
        else:
            error_code = ProviderErrorCode.AUTHENTICATION
    elif "rate" in error_msg.lower() and "limit" in error_msg.lower():
        error_code = ProviderErrorCode.RATE_LIMITED
        retriable = True
    elif "quota" in error_msg.lower():
        error_code = ProviderErrorCode.QUOTA_EXCEEDED
    elif "cancel" in error_msg.lower():
        error_code = ProviderErrorCode.CANCELED
    elif "format" in error_msg.lower() or "encoding" in error_msg.lower():
        error_code = ProviderErrorCode.UNSUPPORTED_FORMAT

    return ProviderError(
        code=error_code,
        message=error_msg or f"{error_type}: {error}",
        provider_name=provider_name,
        retriable=retriable,
        details={"original_error_type": error_type},
    )


def is_retriable(error: Exception) -> bool:
    """
    Check if an error is retriable.

    Args:
        error: The exception to check.

    Returns:
        True if the error is retriable.
    """
    if isinstance(error, ProviderError):
        return error.retriable

    normalized = normalize_error(error)
    return normalized.retriable


__all__ = [
    "ProviderError",
    "ProviderErrorCode",
    "is_retriable",
    "normalize_error",
]
