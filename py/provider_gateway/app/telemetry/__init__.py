"""Telemetry module for provider gateway."""

from app.telemetry.logging import get_logger, JSONFormatter, setup_logging
from app.telemetry.metrics import (
    observe_duration,
    provider_errors_total,
    provider_request_duration_seconds,
    provider_requests_total,
    record_error,
    record_request,
    start_metrics_server,
)
from app.telemetry.tracing import (
    add_event,
    get_current_span,
    get_tracer,
    record_exception,
    set_attribute,
    setup_tracing,
    start_span,
)

__all__ = [
    # Logging
    "get_logger",
    "JSONFormatter",
    "setup_logging",
    # Metrics
    "observe_duration",
    "provider_errors_total",
    "provider_request_duration_seconds",
    "provider_requests_total",
    "record_error",
    "record_request",
    "start_metrics_server",
    # Tracing
    "add_event",
    "get_current_span",
    "get_tracer",
    "record_exception",
    "set_attribute",
    "setup_tracing",
    "start_span",
]
