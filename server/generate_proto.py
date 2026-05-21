#!/usr/bin/env python3
"""
Script to generate Python gRPC code from proto files.

This script generates Python gRPC client/server code from .proto files
using the grpcio-tools package.
"""

import os
import subprocess
import sys


def generate_proto_files():
    """Generate Python code from proto files."""
    proto_dir = os.path.join(os.path.dirname(__file__), "..", "rpc", "proto")
    output_dir = os.path.dirname(__file__)

    proto_file = os.path.join(proto_dir, "task.proto")

    if not os.path.exists(proto_file):
        print(f"Error: Proto file not found: {proto_file}")
        sys.exit(1)

    print(f"Generating Python gRPC code from {proto_file}...")

    proto_include = subprocess.run(
        ["python3", "-m", "grpc_tools.protoc", "--include_include", "-I."],
        capture_output=True,
        text=True,
    ).stdout.strip()

    cmd = [
        sys.executable,
        "-m",
        "grpc_tools.protoc",
        f"--python_out={output_dir}",
        f"--grpc_python_out={output_dir}",
        f"-I{proto_dir}",
        f"-I{proto_include}" if proto_include else f"-I{proto_dir}",
        f"--proto_path={proto_dir}",
        f"--pyi_out={output_dir}",
        proto_file,
    ]

    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"Error generating proto files: {result.stderr}")
        sys.exit(1)

    import_task_pb2 = os.path.join(output_dir, "task_pb2.py")
    if os.path.exists(import_task_pb2):
        with open(import_task_pb2, "r", encoding="utf-8") as f:
            content = f.read()

        content = content.replace(
            "import task_pb2 as task_pb2",
            "from . import task_pb2",
        )

        with open(import_task_pb2, "w", encoding="utf-8") as f:
            f.write(content)

    print("Proto files generated successfully!")
    print(f"  - {os.path.join(output_dir, 'task_pb2.py')}")
    print(f"  - {os.path.join(output_dir, 'task_pb2_grpc.py')}")


if __name__ == "__main__":
    generate_proto_files()
