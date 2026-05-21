# API 参考

本文档详细描述 HDP Disk Inspect 的 gRPC API。

## 目录

1. [API 概述](#api-概述)
2. [Task 服务](#task-服务)
3. [数据结构](#数据结构)
4. [Go 客户端示例](#go-客户端示例)
5. [Python 客户端示例](#python-客户端示例)

---

## API 概述

HDP Disk Inspect 使用 gRPC 提供远程命令执行服务。

- **服务名称**: `rpc.Task`
- **协议**: gRPC/HTTP2
- **默认端口**: 58002
- **传输安全**: 可选 TLS 双向认证

---

## Task 服务

### Run 方法

执行 shell 命令并返回结果。

**RPC 定义**:

```proto
rpc Run(TaskRequest) returns (TaskResponse) {}
```

**请求参数**: `TaskRequest`
**响应结果**: `TaskResponse`

---

## 数据结构

### TaskRequest

执行任务的请求参数。

| 字段 | 类型 | 标签 | 描述 |
|------|------|------|------|
| `command` | string | 2 | 要执行的 shell 命令 |
| `timeout` | int32 | 3 | 任务执行超时时间（秒），默认 30 秒 |
| `id` | int64 | 4 | 任务唯一标识符，用于追踪日志 |

**示例 (JSON)**:

```json
{
  "command": "ls -la /tmp",
  "timeout": 30,
  "id": 12345
}
```

### TaskResponse

任务执行的响应结果。

| 字段 | 类型 | 标签 | 描述 |
|------|------|------|------|
| `output` | string | 1 | 标准输出 (stdout) |
| `stderr` | string | 5 | 标准错误输出 (stderr) |
| `error` | string | 2 | 执行错误信息（如果有） |
| `exit_code` | int32 | 3 | 命令退出码 |
| `start_time` | int64 | 6 | 开始执行时间戳（毫秒） |
| `end_time` | int64 | 7 | 结束执行时间戳（毫秒） |
| `duration_ms` | int64 | 4 | 执行耗时（毫秒） |

**成功响应示例 (JSON)**:

```json
{
  "output": "file1.txt\nfile2.txt\n",
  "stderr": "",
  "error": "",
  "exit_code": 0,
  "start_time": 1699999999123,
  "end_time": 1699999999456,
  "duration_ms": 333
}
```

**错误响应示例 (JSON)**:

```json
{
  "output": "",
  "stderr": "ls: cannot access '/non-existent': No such file or directory\n",
  "error": "",
  "exit_code": 2,
  "start_time": 1699999999123,
  "end_time": 1699999999234,
  "duration_ms": 111
}
```

**超时响应示例 (JSON)**:

```json
{
  "output": "",
  "stderr": "",
  "error": "timeout killed",
  "exit_code": -1,
  "start_time": 1699999999123,
  "end_time": 1699999999567,
  "duration_ms": 444
}
```

---

## Go 客户端示例

### 基础客户端

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/lynnyq/hdp-disk-inspect/rpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 连接服务器
	addr := "localhost:58002"
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	// 创建客户端
	client := pb.NewTaskClient(conn)

	// 执行命令
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.TaskRequest{
		Command: "ls -la /",
		Timeout: 30,
		Id:      1,
	}

	resp, err := client.Run(ctx, req)
	if err != nil {
		log.Fatalf("execute command failed: %v", err)
	}

	// 打印结果
	fmt.Printf("Exit Code: %d\n", resp.ExitCode)
	fmt.Printf("Duration: %d ms\n", resp.DurationMs)
	if resp.Output != "" {
		fmt.Printf("\nOutput:\n%s\n", resp.Output)
	}
	if resp.Stderr != "" {
		fmt.Printf("\nStderr:\n%s\n", resp.Stderr)
	}
	if resp.Error != "" {
		fmt.Printf("\nError: %s\n", resp.Error)
	}
}
```

### TLS 客户端

```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	pb "github.com/lynnyq/hdp-disk-inspect/rpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func loadTLSCredentials(caFile, certFile, keyFile string) (credentials.TransportCredentials, error) {
	// 加载 CA 证书
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate")
	}

	// 加载客户端证书
	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}

	return credentials.NewTLS(config), nil
}

func main() {
	// 加载 TLS 凭证
	creds, err := loadTLSCredentials(
		"certs/ca.crt",
		"certs/client/client.crt",
		"certs/client/client.key",
	)
	if err != nil {
		log.Fatalf("load TLS credentials failed: %v", err)
	}

	// 连接服务器
	addr := "localhost:58002"
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewTaskClient(conn)

	// 执行命令
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.TaskRequest{
		Command: "echo 'Hello from TLS client'",
		Timeout: 30,
		Id:      2,
	}

	resp, err := client.Run(ctx, req)
	if err != nil {
		log.Fatalf("execute command failed: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Output)
}
```

---

## Python 客户端示例

### 基础客户端

```python
import grpc

import task_pb2
import task_pb2_grpc


def run_command():
    # 连接服务器
    with grpc.insecure_channel('localhost:58002') as channel:
        stub = task_pb2_grpc.TaskStub(channel)

        # 执行命令
        request = task_pb2.TaskRequest(
            command="ls -la /",
            timeout=30,
            id=1
        )

        response = stub.Run(request)

        # 打印结果
        print(f"Exit Code: {response.exit_code}")
        print(f"Duration: {response.duration_ms} ms")
        if response.output:
            print(f"\nOutput:\n{response.output}")
        if response.stderr:
            print(f"\nStderr:\n{response.stderr}")
        if response.error:
            print(f"\nError: {response.error}")


if __name__ == '__main__':
    run_command()
```

### TLS 客户端

```python
import grpc
import ssl

import task_pb2
import task_pb2_grpc


def load_tls_credentials(ca_cert_file, client_cert_file, client_key_file):
    """加载 TLS 凭证"""
    # 加载 CA 证书
    with open(ca_cert_file, 'rb') as f:
        ca_cert = f.read()

    # 加载客户端证书
    with open(client_cert_file, 'rb') as f:
        client_cert = f.read()
    with open(client_key_file, 'rb') as f:
        client_key = f.read()

    credentials = grpc.ssl_channel_credentials(
        root_certificates=ca_cert,
        private_key=client_key,
        certificate_chain=client_cert
    )

    return credentials


def run_command():
    # 加载 TLS 凭证
    credentials = load_tls_credentials(
        'certs/ca.crt',
        'certs/client/client.crt',
        'certs/client/client.key'
    )

    # 连接服务器
    with grpc.secure_channel('localhost:58002', credentials) as channel:
        stub = task_pb2_grpc.TaskStub(channel)

        request = task_pb2.TaskRequest(
            command="echo 'Hello from TLS client'",
            timeout=30,
            id=2
        )

        response = stub.Run(request)
        print(f"Response: {response.output}")


if __name__ == '__main__':
    run_command()
```

---

## Prometheus 指标端点

除了 gRPC API，服务还暴露 HTTP 端点提供 Prometheus 指标。

### 端点

- **路径**: `/metrics`
- **方法**: GET
- **格式**: Prometheus text format

### 示例访问

```bash
curl http://localhost:58002/metrics
```

### Prometheus 配置

```yaml
scrape_configs:
  - job_name: 'hdp-disk-inspect'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:58002']
```

---

## 错误码

### 命令退出码

- `0`: 成功
- `1-255`: 命令执行失败，具体含义取决于命令
- `-1`: 超时被终止或内部错误

### gRPC 状态码

| 状态码 | 说明 |
|--------|------|
| `OK` | 成功 |
| `DEADLINE_EXCEEDED` | 请求超时 |
| `UNAVAILABLE` | 服务不可用 |
| `PERMISSION_DENIED` | 权限不足（TLS） |
| `UNAUTHENTICATED` | 未认证（TLS） |
