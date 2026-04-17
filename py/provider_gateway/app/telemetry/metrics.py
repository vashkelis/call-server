"""Prometheus metrics for provider gateway."""

from typing import Optional

from prometheus_client import Counter, Histogram, start_http_server

# Provider request counter
provider_requests_total = Counter(
    "provider_requests_total",
    "Total number of provider requests",
    ["provider_name", "provider_type"],
)

# Provider request duration
provider_request_duration_seconds = Histogram(
    "provider_request_duration_seconds",
    "Provider request duration in seconds",
    ["provider_name", "provider_type"],
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0],
)

# Provider error counter
provider_errors_total = Counter(
    "provider_errors_total",
    "Total number of provider errors",
    ["provider_name", "provider_type", "error_code"],
)

# Active sessions gauge (optional, can be added if needed)


def record_request(provider_name: str, provider_type: str) -> None:
    """
    Record a provider request.

    Args:
        provider_name: Name of the provider.
        provider_type: Type of provider (asr, llm, tts).
    """
    provider_requests_total.labels(
        provider_name=provider_name,
        provider_type=provider_type,
    ).inc()


def record_error(
    provider_name: str,
    provider_type: str,
    error_code: str = "unknown",
) -> None:
    """
    Record a provider error.

    Args:
        provider_name: Name of the provider.
        provider_type: Type of provider (asr, llm, tts).
        error_code: Error code or type.
    """
    provider_errors_total.labels(
        provider_name=provider_name,
        provider_type=provider_type,
        error_code=error_code,
    ).inc()


def observe_duration(
    provider_name: str,
    provider_type: str,
    duration_seconds: float,
) -> None:
    """
    Observe a request duration.

    Args:
        provider_name: Name of the provider.
        provider_type: Type of provider (asr, llm, tts).
        duration_seconds: Duration in seconds.
    """
    provider_request_duration_seconds.labels(
        provider_name=provider_name,
        provider_type=provider_type,
    ).observe(duration_seconds)


def start_metrics_server(port: int = 9090) -> None:
    """
    Start the Prometheus metrics HTTP server.

    Args:
        port: Port to listen on.
    """
    start_http_server(port)
    import logging

    logging.getLogger(__name__).info(f"Metrics server started on port {port}")


__all__ = [
    "observe_duration",
    "provider_errors_total",
    "provider_request_duration_seconds",
    "provider_requests_total",
    "record_error",
    "record_request",
    "start_metrics_server",
]
