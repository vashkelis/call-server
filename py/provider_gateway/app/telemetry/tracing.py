"""OpenTelemetry tracing setup for provider gateway."""

from contextlib import contextmanager
from typing import Generator, Optional

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource, SERVICE_NAME
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

# Global tracer provider
_tracer_provider: Optional[TracerProvider] = None
_tracer: Optional[trace.Tracer] = None


def setup_tracing(
    service_name: str = "provider-gateway",
    otel_endpoint: Optional[str] = None,
) -> TracerProvider:
    """
    Set up OpenTelemetry tracing.

    Args:
        service_name: Name of the service.
        otel_endpoint: OTLP endpoint URL (optional).

    Returns:
        The configured TracerProvider.
    """
    global _tracer_provider, _tracer

    # Create resource
    resource = Resource.create({SERVICE_NAME: service_name})

    # Create provider
    _tracer_provider = TracerProvider(resource=resource)

    # Add OTLP exporter if endpoint is provided
    if otel_endpoint:
        exporter = OTLPSpanExporter(endpoint=otel_endpoint)
        processor = BatchSpanProcessor(exporter)
        _tracer_provider.add_span_processor(processor)

    # Set as global provider
    trace.set_tracer_provider(_tracer_provider)

    # Create tracer
    _tracer = trace.get_tracer(service_name)

    return _tracer_provider


def get_tracer() -> Optional[trace.Tracer]:
    """Get the global tracer instance."""
    global _tracer
    return _tracer


def get_current_span() -> Optional[trace.Span]:
    """Get the current active span."""
    return trace.get_current_span()


def set_attribute(key: str, value: str) -> None:
    """
    Set an attribute on the current span.

    Args:
        key: Attribute key.
        value: Attribute value.
    """
    span = get_current_span()
    if span:
        span.set_attribute(key, value)


def add_event(name: str, attributes: Optional[dict] = None) -> None:
    """
    Add an event to the current span.

    Args:
        name: Event name.
        attributes: Optional event attributes.
    """
    span = get_current_span()
    if span:
        span.add_event(name, attributes)


@contextmanager
def start_span(
    name: str,
    attributes: Optional[dict] = None,
) -> Generator[trace.Span, None, None]:
    """
    Start a new span context manager.

    Args:
        name: Span name.
        attributes: Optional span attributes.

    Yields:
        The created span.
    """
    tracer = get_tracer()
    if tracer is None:
        # Return a dummy span if tracing not set up
        class DummySpan:
            def set_attribute(self, key: str, value: str) -> None:
                pass

            def add_event(self, name: str, attributes: Optional[dict] = None) -> None:
                pass

            def record_exception(self, exception: Exception) -> None:
                pass

            def __enter__(self):
                return self

            def __exit__(self, *args):
                pass

        yield DummySpan()  # type: ignore
    else:
        with tracer.start_as_current_span(name, attributes=attributes) as span:
            yield span


def record_exception(exception: Exception) -> None:
    """
    Record an exception on the current span.

    Args:
        exception: The exception to record.
    """
    span = get_current_span()
    if span:
        span.record_exception(exception)


__all__ = [
    "add_event",
    "get_current_span",
    "get_tracer",
    "record_exception",
    "set_attribute",
    "setup_tracing",
    "start_span",
]
