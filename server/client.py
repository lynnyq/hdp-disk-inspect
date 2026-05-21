#!/usr/bin/env python3
"""
Example gRPC client for HDP Disk Inspect server.

This script demonstrates how to use the gRPC client to execute
shell commands on the server with optional TLS support.
"""

import argparse
import logging
import sys

import grpc

import task_pb2
import task_pb2_grpc

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


def load_tls_credentials(ca_cert: str, client_cert: str = None, client_key: str = None):
    """
    Load TLS credentials for gRPC client.

    Args:
        ca_cert: Path to CA certificate (for server verification)
        client_cert: Path to client certificate (for mTLS)
        client_key: Path to client private key (for mTLS)

    Returns:
        gRPC channel credentials
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

    if client_cert and client_key:
        try:
            with open(client_cert, "rb") as f:
                client_cert_data = f.read()
            with open(client_key, "rb") as f:
                client_key_data = f.read()

            credentials = grpc.ssl_channel_credentials(
                root_certificates=ca_cert_data,
                private_key=client_key_data,
                certificate_chain=client_cert_data,
            )
            logger.info("TLS credentials loaded with client certificate (mTLS)")
        except FileNotFoundError as e:
            logger.error(f"Certificate file not found: {e}")
            sys.exit(1)
        except Exception as e:
            logger.error(f"Failed to load client credentials: {e}")
            sys.exit(1)
    else:
        credentials = grpc.ssl_channel_credentials(
            root_certificates=ca_cert_data,
        )
        logger.info("TLS credentials loaded (server verification only)")

    return credentials


def run_command(
    server_address: str,
    command: str,
    timeout: int = 30,
    task_id: int = 1,
    use_tls: bool = False,
    ca_cert: str = None,
    client_cert: str = None,
    client_key: str = None,
):
    """
    Execute a command on the gRPC server.

    Args:
        server_address: Server address (host:port)
        command: Shell command to execute
        timeout: Command timeout in seconds
        task_id: Unique task identifier
        use_tls: Use TLS encryption
        ca_cert: Path to CA certificate
        client_cert: Path to client certificate
        client_key: Path to client private key

    Returns:
        TaskResponse with execution results
    """
    if use_tls:
        if not ca_cert:
            logger.error("TLS requires --ca-cert")
            sys.exit(1)

        credentials = load_tls_credentials(ca_cert, client_cert, client_key)
        channel = grpc.secure_channel(server_address, credentials)
        logger.info(f"Using TLS for connection to {server_address}")
    else:
        channel = grpc.insecure_channel(server_address)
        logger.warning(f"Using insecure connection to {server_address}")

    try:
        with channel:
            stub = task_pb2_grpc.TaskStub(channel)

            request = task_pb2.TaskRequest(
                command=command,
                timeout=timeout,
                id=task_id,
            )

            print(f"Executing command: {command}")
            print(f"Timeout: {timeout}s")
            print("-" * 50)

            response = stub.Run(request)

            print(f"Exit Code: {response.exit_code}")
            print(f"Duration: {response.duration_ms}ms")

            if response.output:
                print(f"\nStdout:\n{response.output}")

            if response.stderr:
                print(f"\nStderr:\n{response.stderr}")

            if response.error:
                print(f"\nError: {response.error}")

            return response
    except grpc.RpcError as e:
        logger.error(f"RPC failed: {e.code()}: {e.details()}")
        raise


def interactive_mode(
    server_address: str,
    use_tls: bool = False,
    ca_cert: str = None,
    client_cert: str = None,
    client_key: str = None,
):
    """
    Run the client in interactive mode.

    Args:
        server_address: Server address (host:port)
        use_tls: Use TLS encryption
        ca_cert: Path to CA certificate
        client_cert: Path to client certificate
        client_key: Path to client private key
    """
    print("Interactive Mode - Type 'exit' to quit")
    print("=" * 50)

    if use_tls:
        print("TLS Mode Enabled")
        if client_cert:
            print("Client Certificate: Required")
        else:
            print("Client Certificate: Not Required")

    task_id = 1

    while True:
        try:
            command = input("\n$ ")
            if command.strip().lower() in ("exit", "quit", "q"):
                print("Goodbye!")
                break

            if not command.strip():
                continue

            print()
            run_command(
                server_address,
                command,
                task_id=task_id,
                use_tls=use_tls,
                ca_cert=ca_cert,
                client_cert=client_cert,
                client_key=client_key,
            )
            task_id += 1

        except KeyboardInterrupt:
            print("\n\nGoodbye!")
            break
        except Exception as e:  # pylint: disable=broad-except
            print(f"Error: {e}")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="HDP Disk Inspect gRPC Client",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Run a single command without TLS
  python client.py "ls -la"

  # Run with TLS (server verification only)
  python client.py "ls -la" --use-tls --ca-cert=certs/ca/ca.crt

  # Run with TLS (mutual authentication)
  python client.py "ls -la" --use-tls \\
      --ca-cert=certs/ca/ca.crt \\
      --client-cert=certs/client/client.crt \\
      --client-key=certs/client/client.key

  # Interactive mode with TLS
  python client.py --interactive --use-tls \\
      --ca-cert=certs/ca/ca.crt \\
      --client-cert=certs/client/client.crt \\
      --client-key=certs/client/client.key

  # Run with custom timeout
  python client.py "sleep 60" --timeout 120
        """,
    )
    parser.add_argument(
        "command",
        nargs="?",
        help="Shell command to execute",
    )
    parser.add_argument(
        "-s",
        "--server",
        default="localhost:5921",
        help="gRPC server address (default: localhost:5921)",
    )
    parser.add_argument(
        "-t",
        "--timeout",
        type=int,
        default=30,
        help="Command timeout in seconds (default: 30)",
    )
    parser.add_argument(
        "-i",
        "--id",
        type=int,
        default=1,
        help="Task ID (default: 1)",
    )
    parser.add_argument(
        "--interactive",
        action="store_true",
        help="Run in interactive mode",
    )
    parser.add_argument(
        "--use-tls",
        action="store_true",
        help="Use TLS encryption",
    )
    parser.add_argument(
        "--ca-cert",
        type=str,
        default=None,
        help="Path to CA certificate (required for TLS)",
    )
    parser.add_argument(
        "--client-cert",
        type=str,
        default=None,
        help="Path to client certificate (optional, for mTLS)",
    )
    parser.add_argument(
        "--client-key",
        type=str,
        default=None,
        help="Path to client private key (optional, for mTLS)",
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

    if args.use_tls and not args.ca_cert:
        parser.error("--use-tls requires --ca-cert")

    if args.interactive:
        interactive_mode(
            args.server,
            use_tls=args.use_tls,
            ca_cert=args.ca_cert,
            client_cert=args.client_cert,
            client_key=args.client_key,
        )
    elif args.command:
        run_command(
            args.server,
            args.command,
            args.timeout,
            args.id,
            use_tls=args.use_tls,
            ca_cert=args.ca_cert,
            client_cert=args.client_cert,
            client_key=args.client_key,
        )
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
