"""Entry point for provider gateway."""

import asyncio
import logging
import sys

from app.config import get_settings
from app.core.registry import get_registry
from app.grpc_api import create_and_start_server
from app.telemetry import get_logger, setup_logging, start_metrics_server, setup_tracing

logger = get_logger(__name__)


async def main() -> int:
    """
    Main entry point for provider gateway.

    Loads config, initializes telemetry, creates registry, registers all providers,
    and starts the gRPC server.

    Returns:
        Exit code (0 for success, 1 for error).
    """
    try:
        # Load configuration
        settings = get_settings()

        # Setup logging
        setup_logging(level=settings.telemetry.log_level)
        logger.info("Starting Provider Gateway")

        # Setup metrics
        start_metrics_server(port=settings.telemetry.metrics_port)

        # Setup tracing
        if settings.telemetry.otel_endpoint:
            setup_tracing(
                service_name="provider-gateway",
                otel_endpoint=settings.telemetry.otel_endpoint,
            )
            logger.info(f"Tracing enabled with endpoint: {settings.telemetry.otel_endpoint}")

        # Create registry and discover providers
        registry = get_registry()
        registry.discover_and_register()

        # Log registered providers
        logger.info(f"ASR providers: {registry.list_asr_providers()}")
        logger.info(f"LLM providers: {registry.list_llm_providers()}")
        logger.info(f"TTS providers: {registry.list_tts_providers()}")

        # Create and start gRPC server
        server = await create_and_start_server(
            registry=registry,
            host=settings.server.host,
            port=settings.server.port,
            max_workers=settings.server.max_workers,
        )

        # Wait for termination
        await server.wait_for_termination()

        return 0

    except KeyboardInterrupt:
        logger.info("Interrupted by user")
        return 0
    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        return 1


if __name__ == "__main__":
    try:
        exit_code = asyncio.run(main())
        sys.exit(exit_code)
    except KeyboardInterrupt:
        sys.exit(0)
