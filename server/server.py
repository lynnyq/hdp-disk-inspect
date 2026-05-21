#!/usr/bin/env python3
"""
HDP Disk Inspect gRPC Server

A gRPC server that executes shell commands on behalf of clients.
"""

import argparse
import logging
import os
import signal
import sys
import time
from concurrent import futures

import grpc

import task_pb2
import task_pb2_grpc

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


class TaskServicer(task_pb2_grpc.TaskServicer):
    """gRPC service implementation for task execution."""

    def Run(self, request, context):
        """
        Execute a shell command and return the result.

        Args:
            request: TaskRequest containing command and timeout
            context: gRPC context

        Returns:
            TaskResponse with execution results
        """
        start_time = int(time.time() * 1000)

        logger.info(f"Execute cmd start: [id={request.id} cmd={request.command}]")

        response = task_pb2.TaskResponse()

        try:
            timeout = request.timeout if request.timeout > 0 else 30

            process = __import__("subprocess").Popen(
                request.command,
                shell=True,
                stdout=__import__("subprocess").PIPE,
                stderr=__import__("subprocess").PIPE,
                text=True,
            )

            try:
                stdout, stderr = process.communicate(timeout=timeout)
                exit_code = process.returncode
            except __import__("subprocess").TimeoutExpired:
                process.kill()
                stdout, stderr = process.communicate()
                exit_code = -1
                response.error = "timeout killed"

            response.output = stdout
            response.stderr = stderr
            response.exit_code = exit_code

        except Exception as e:  # pylint: disable=broad-except
            logger.error(f"Command execution failed: {e}")
            response.error = str(e)
            response.exit_code = -1

        end_time = int(time.time() * 1000)

        response.start_time = start_time
        response.end_time = end_time
        response.duration_ms = end_time - start_time

        logger.info(
            f"Execute cmd end: [id={request.id} "
            f"cmd={request.command} exit_code={response.exit_code} "
            f"duration_ms={response.duration_ms}]"
        )

        return response


def load_tls_credentials(ca_cert: str, server_cert: str, server_key: str):
    """
    Load TLS credentials for mutual authentication.

    Args:
        ca_cert: Path to CA certificate
        server_cert: Path to server certificate
        server_key: Path to server private key

    Returns:
        gRPC server credentials
    """
    try:
        with open(ca_cert, "rb") as f:
            ca_cert_data = f.read()
    except FileNotFoundError:
        logger.error(f"CA certificate not found: {ca_cert}")
        sys.exit(1)
    except Exception as e:
        logger.error(f"Failed to read CA certificate: {e}")
        sys.exit(1)

    try:
        with open(server_cert, "rb") as f:
            server_cert_data = f.read()
    except FileNotFoundError:
        logger.error(f"Server certificate not found: {server_cert}")
        sys.exit(1)
    except Exception as e:
        logger.error(f"Failed to read server certificate: {e}")
        sys.exit(1)

    try:
        with open(server_key, "rb") as f:
            server_key_data = f.read()
    except FileNotFoundError:
        logger.error(f"Server key not found: {server_key}")
        sys.exit(1)
    except Exception as e:
        logger.error(f"Failed to read server key: {e}")
        sys.exit(1)

    try:
        credentials = grpc.ssl_server_credentials(
            [(server_key_data, server_cert_data)],
            root_certificates=ca_cert_data,
            require_client_cert=True,
        )
        logger.info("TLS credentials loaded successfully")
        return credentials
    except Exception as e:
        logger.error(f"Failed to create TLS credentials: {e}")
        sys.exit(1)


def serve(
    address: str,
    port: int,
    enable_tls: bool = False,
    max_workers: int = 10,
    ca_cert: str = None,
    server_cert: str = None,
    server_key: str = None,
):
    """
    Start the gRPC server.

    Args:
        address: Server bind address
        port: Server port
        enable_tls: Enable TLS encryption
        max_workers: Maximum number of worker threads
        ca_cert: Path to CA certificate (for TLS)
        server_cert: Path to server certificate (for TLS)
        server_key: Path to server private key (for TLS)
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))

    task_pb2_grpc.add_TaskServicer_to_server(TaskServicer(), server)

    server_address = f"{address}:{port}"

    if enable_tls:
        if not all([ca_cert, server_cert, server_key]):
            logger.error("TLS requires --ca-cert, --server-cert, and --server-key")
            logger.info("Generating self-signed certificates for testing...")
            logger.info("For production, use certificates from generate_certs.sh")
            sys.exit(1)

        if not os.path.exists(ca_cert):
            logger.error(f"CA certificate not found: {ca_cert}")
            sys.exit(1)
        if not os.path.exists(server_cert):
            logger.error(f"Server certificate not found: {server_cert}")
            sys.exit(1)
        if not os.path.exists(server_key):
            logger.error(f"Server key not found: {server_key}")
            sys.exit(1)

        credentials = load_tls_credentials(ca_cert, server_cert, server_key)
        server.add_secure_port(server_address, credentials)
        logger.info(f"TLS enabled with mutual authentication")
        logger.info(f"CA Certificate: {ca_cert}")
        logger.info(f"Server Certificate: {server_cert}")
    else:
        server.add_insecure_port(server_address)
        logger.warning("TLS is disabled, connection is not encrypted")

    server.start()
    logger.info(f"Server listening on {server_address}")

    def handle_signal(signum, frame):
        logger.info("Received shutdown signal, stopping server...")
        server.stop(grace=5)
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    server.wait_for_termination()


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="HDP Disk Inspect gRPC Server",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Run without TLS
  python server.py

  # Run with TLS
  python server.py --enable-tls \\
      --ca-cert=certs/ca/ca.crt \\
      --server-cert=certs/server/server.crt \\
      --server-key=certs/server/server.key

  # Run with custom settings
  python server.py -a 0.0.0.0 -p 8080 -w 20 -v
        """,
    )
    parser.add_argument(
        "-a",
        "--address",
        default="0.0.0.0",
        help="Server bind address (default: 0.0.0.0)",
    )
    parser.add_argument(
        "-p",
        "--port",
        type=int,
        default=5921,
        help="Server port (default: 5921)",
    )
    parser.add_argument(
        "--enable-tls",
        action="store_true",
        help="Enable TLS encryption with mutual authentication",
    )
    parser.add_argument(
        "--ca-cert",
        type=str,
        default=None,
        help="Path to CA certificate (required for TLS)",
    )
    parser.add_argument(
        "--server-cert",
        type=str,
        default=None,
        help="Path to server certificate (required for TLS)",
    )
    parser.add_argument(
        "--server-key",
        type=str,
        default=None,
        help="Path to server private key (required for TLS)",
    )
    parser.add_argument(
        "-w",
        "--workers",
        type=int,
        default=10,
        help="Maximum number of worker threads (default: 10)",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Enable verbose logging",
    )

    args = parser.parse_args()

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

    if args.enable_tls:
        if not args.ca_cert or not args.server_cert or not args.server_key:
            parser.error("--enable-tls requires --ca-cert, --server-cert, and --server-key")

    logger.info(
        f"Starting server on {args.address}:{args.port} "
        f"(workers={args.workers}, tls={args.enable_tls})"
    )

    serve(
        args.address,
        args.port,
        args.enable_tls,
        args.workers,
        args.ca_cert,
        args.server_cert,
        args.server_key,
    )


if __name__ == "__main__":
    main()
