"""gRPC server setup for provider gateway."""

import asyncio
import logging
import signal
from concurrent import futures
from typing import Optional

import grpc

from app.core.registry import ProviderRegistry
from app.grpc_api.asr_servicer import ASRServicer
from app.grpc_api.generated.asr_pb2_grpc import add_ASRServiceServicer_to_server
from app.grpc_api.generated.llm_pb2_grpc import add_LLMServiceServicer_to_server
from app.grpc_api.generated.provider_pb2_grpc import add_ProviderServiceServicer_to_server
from app.grpc_api.generated.tts_pb2_grpc import add_TTSServiceServicer_to_server
from app.grpc_api.llm_servicer import LLMServicer
from app.grpc_api.provider_servicer import ProviderServicer
from app.grpc_api.tts_servicer import TTSServicer
from app.telemetry import get_logger

logger = get_logger(__name__)


class GRPCServer:
    """gRPC server for provider gateway."""

    def __init__(
        self,
        registry: ProviderRegistry,
        host: str = "0.0.0.0",
        port: int = 50051,
        max_workers: int = 10,
        version: str = "0.1.0",
    ) -> None:
        """
        Initialize gRPC server.

        Args:
            registry: Provider registry.
            host: Host to bind to.
            port: Port to listen on.
            max_workers: Maximum number of worker threads.
            version: Service version.
        """
        self._registry = registry
        self._host = host
        self._port = port
        self._max_workers = max_workers
        self._version = version
        self._server: Optional[grpc.aio.Server] = None
        self._shutdown_event = asyncio.Event()

    async def start(self) -> None:
        """Start the gRPC server."""
        # Create server
        self._server = grpc.aio.server(
            futures.ThreadPoolExecutor(max_workers=self._max_workers),
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),  # 50MB
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),  # 50MB
            ],
        )

        # Add servicers
        add_ASRServiceServicer_to_server(
            ASRServicer(self._registry),
            self._server,
        )
        add_LLMServiceServicer_to_server(
            LLMServicer(self._registry),
            self._server,
        )
        add_TTSServiceServicer_to_server(
            TTSServicer(self._registry),
            self._server,
        )
        add_ProviderServiceServicer_to_server(
            ProviderServicer(self._registry, self._version),
            self._server,
        )

        # Bind to port
        listen_addr = f"{self._host}:{self._port}"
        self._server.add_insecure_port(listen_addr)

        # Start server
        await self._server.start()
        logger.info(f"gRPC server started on {listen_addr}")

        # Setup signal handlers
        self._setup_signal_handlers()

    def _setup_signal_handlers(self) -> None:
        """Setup signal handlers for graceful shutdown."""
        try:
            loop = asyncio.get_event_loop()
            for sig in (signal.SIGTERM, signal.SIGINT):
                loop.add_signal_handler(sig, lambda: asyncio.create_task(self.stop()))
        except NotImplementedError:
            # Windows doesn't support add_signal_handler
            pass

    async def stop(self, grace_period: Optional[float] = 5.0) -> None:
        """
        Stop the gRPC server gracefully.

        Args:
            grace_period: Grace period for graceful shutdown in seconds.
        """
        if self._server is None:
            return

        logger.info("Shutting down gRPC server...")
        await self._server.stop(grace_period)
        self._shutdown_event.set()
        logger.info("gRPC server stopped")

    async def wait_for_termination(self) -> None:
        """Wait for the server to terminate."""
        if self._server is None:
            raise RuntimeError("Server not started")

        try:
            await self._shutdown_event.wait()
        except asyncio.CancelledError:
            await self.stop()
            raise

    @property
    def is_running(self) -> bool:
        """Check if the server is running."""
        return self._server is not None and not self._shutdown_event.is_set()


async def create_and_start_server(
    registry: ProviderRegistry,
    host: str = "0.0.0.0",
    port: int = 50051,
    max_workers: int = 10,
    version: str = "0.1.0",
) -> GRPCServer:
    """
    Create and start a gRPC server.

    Args:
        registry: Provider registry.
        host: Host to bind to.
        port: Port to listen on.
        max_workers: Maximum number of worker threads.
        version: Service version.

    Returns:
        The started GRPCServer instance.
    """
    server = GRPCServer(
        registry=registry,
        host=host,
        port=port,
        max_workers=max_workers,
        version=version,
    )
    await server.start()
    return server


__all__ = [
    "create_and_start_server",
    "GRPCServer",
]
