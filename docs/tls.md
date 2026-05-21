# TLS 双向认证配置指南

本指南介绍如何配置和使用 TLS 双向认证（mTLS）来保护 HDP Disk Inspect 的 gRPC 通信。

## 目录

1. [概述](#概述)
2. [快速开始](#快速开始)
3. [证书生成](#证书生成)
4. [手动生成证书](#手动生成证书)
5. [Go 服务器配置](#go-服务器配置)
6. [Go 客户端配置](#go-客户端配置)
7. [Python 服务器/客户端](#python-服务器客户端)
8. [安全最佳实践](#安全最佳实践)
9. [故障排除](#故障排除)

---

## 概述

TLS 双向认证提供以下安全保障：
- **加密传输**: 所有通信都经过 TLS 加密
- **服务器身份验证**: 客户端验证服务器证书
- **客户端身份验证**: 服务器验证客户端证书
- **保护指标端点**: 同时保护 Prometheus metrics 的安全

---

## 快速开始

### 1. 准备证书脚本

```bash
# 复制证书生成脚本到项目根目录（如果还没有）
cp server/generate_certs.sh .
chmod +x generate_certs.sh
```

### 2. 生成所有证书

```bash
# 使用默认配置生成（包含多个 IP 和主机名）
./generate_certs.sh all -i 127.0.0.1 -i 192.168.1.100 -h localhost -h server.example.com

# 或者使用更简单的方式
./generate_certs.sh all
```

### 3. 使用 TLS 启动 Go 服务器

```bash
./hdp-disk-inspect -enable-tls -ca-file certs/ca/ca.crt -cert-file certs/server/server.crt -key-file certs/server/server.key
```

### 4. 测试 TLS 连接

查看 [API 参考文档](api.md) 中的 Go 和 Python TLS 客户端示例。

---

## 证书生成

### 使用自动化脚本

```bash
# 查看帮助
./generate_certs.sh --help
```

#### 生成 CA

```bash
./generate_certs.sh ca
```

#### 生成服务器证书（支持多个 IP/主机名）

```bash
# 仅 IP 地址
./generate_certs.sh server -i 127.0.0.1 -i 192.168.1.100

# 仅主机名
./generate_certs.sh server -h localhost -h server.example.com

# 同时包含 IP 和主机名
./generate_certs.sh server \
    -i 127.0.0.1 \
    -i 192.168.1.100 \
    -h localhost \
    -h server.example.com
```

#### 生成客户端证书

```bash
./generate_certs.sh client
```

#### 完整生成所有证书

```bash
./generate_certs.sh all -i 127.0.0.1 -i 192.168.1.100 -h localhost -h server.example.com
```

#### 其他命令

```bash
# 列出生成的证书
./generate_certs.sh list

# 清理所有证书
./generate_certs.sh clean
```

---

## 手动生成证书

以下是使用 OpenSSL 手动生成证书的详细步骤。

### 1. 创建目录和配置

```bash
# 创建目录结构
mkdir -p certs/{ca,server,client}
chmod 700 certs

# 创建临时的 OpenSSL 配置文件
cat > server_ext.cnf << 'EOF'
[ req ]
default_bits        = 4096
distinguished_name  = req_distinguished_name
string_mask         = utf8only
default_md          = sha256
x509_extensions     = v3_ca

[ req_distinguished_name ]
countryName                     = Country Name (2 letter code)
stateOrProvinceName             = State or Province Name
localityName                    = Locality Name
organizationName                = Organization Name
organizationalUnitName          = Organizational Unit Name
emailAddress                    = Email Address

[ v3_ca ]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true, pathlen:0
keyUsage = critical, digitalSignature, cRLSign, keyCertSign

[ server_cert_ext ]
basicConstraints = CA:FALSE
nsCertType = server
nsComment = "OpenSSL Generated Server Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer:always
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth

[ client_cert_ext ]
basicConstraints = CA:FALSE
nsCertType = client, email
nsComment = "OpenSSL Generated Client Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer:always
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, emailProtection
EOF
```

### 2. 生成 CA（证书颁发机构）

```bash
# CA 密码（生产环境请使用强密码）
CA_PASSWORD="changeit"

# 生成 CA 私钥（加密的）
openssl genrsa -aes256 -passout pass:${CA_PASSWORD} -out certs/ca/ca.key.pem 4096

# 生成 CA 自签名证书
openssl req -config server_ext.cnf \
    -key certs/ca/ca.key.pem \
    -passin pass:${CA_PASSWORD} \
    -new -x509 -days 3650 \
    -sha256 -extensions v3_ca \
    -out certs/ca/ca.crt.pem \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=HDP/OU=DiskInspect/CN=HDP-CA/emailAddress=admin@example.com"

# 导出未加密的私钥（可选，用于方便）
openssl rsa -passin pass:${CA_PASSWORD} -in certs/ca/ca.key.pem -out certs/ca/ca.key 2>/dev/null || true

# 导出纯 PEM 格式的证书
openssl x509 -in certs/ca/ca.crt.pem -out certs/ca/ca.crt -outform PEM

# 导出 PKCS12 格式（用于浏览器）
openssl pkcs12 -export -passout pass:${CA_PASSWORD} \
    -in certs/ca/ca.crt.pem \
    -inkey certs/ca/ca.key.pem \
    -passin pass:${CA_PASSWORD} \
    -out certs/ca/ca.p12

# 设置权限
chmod 400 certs/ca/ca.key.pem certs/ca/ca.key
chmod 444 certs/ca/ca.crt.pem certs/ca/ca.crt

# 验证 CA 证书
openssl x509 -in certs/ca/ca.crt.pem -text -noout
```

### 3. 生成服务器证书（支持多 IP/主机名）

```bash
# 服务器密码
SERVER_PASSWORD="server123"

# 创建带有 Subject Alternative Name (SAN) 的扩展配置
cat > server_ext.cnf.tmp << 'EOF'
[ req ]
default_bits        = 4096
distinguished_name  = req_distinguished_name
string_mask         = utf8only
default_md          = sha256

[ req_distinguished_name ]
countryName                     = Country Name (2 letter code)
stateOrProvinceName             = State or Province Name
localityName                    = Locality Name
organizationName                = Organization Name
organizationalUnitName          = Organizational Unit Name
emailAddress                    = Email Address

[ server_cert_ext ]
basicConstraints = CA:FALSE
nsCertType = server
nsComment = "OpenSSL Generated Server Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer:always
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[ alt_names ]
IP.1 = 127.0.0.1
IP.2 = 192.168.1.100
DNS.1 = localhost
DNS.2 = server.example.com
EOF

# 生成服务器私钥
openssl genrsa -aes256 -passout pass:${SERVER_PASSWORD} -out certs/server/server.key.pem 4096

# 生成服务器 CSR（证书签名请求）
openssl req -config server_ext.cnf.tmp \
    -key certs/server/server.key.pem \
    -passin pass:${SERVER_PASSWORD} \
    -new -sha256 \
    -out certs/server/server.csr.pem \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=HDP/OU=DiskInspect/CN=server"

# 使用 CA 签发服务器证书
openssl x509 -req -passin pass:${CA_PASSWORD} \
    -CA certs/ca/ca.crt.pem \
    -CAkey certs/ca/ca.key.pem \
    -in certs/server/server.csr.pem \
    -out certs/server/server.crt.pem \
    -days 365 \
    -sha256 \
    -extensions server_cert_ext \
    -extfile server_ext.cnf.tmp \
    -CAcreateserial

# 导出未加密的私钥
openssl rsa -passin pass:${SERVER_PASSWORD} -in certs/server/server.key.pem -out certs/server/server.key 2>/dev/null || true

# 导出纯 PEM 格式的证书
openssl x509 -in certs/server/server.crt.pem -out certs/server/server.crt -outform PEM

# 导出 PKCS12 格式
openssl pkcs12 -export -passout pass:${SERVER_PASSWORD} \
    -in certs/server/server.crt.pem \
    -inkey certs/server/server.key.pem \
    -passin pass:${SERVER_PASSWORD} \
    -out certs/server/server.p12 \
    -name "server"

# 设置权限
chmod 400 certs/server/server.key.pem certs/server/server.key
chmod 444 certs/server/server.crt.pem certs/server/server.crt

# 验证服务器证书
openssl verify -CAfile certs/ca/ca.crt certs/server/server.crt

# 检查 SAN（Subject Alternative Name）
openssl x509 -in certs/server/server.crt.pem -noout -text | grep -A 10 "Subject Alternative Name"

# 清理临时文件
rm -f server_ext.cnf.tmp certs/server/server.csr.pem
```

### 4. 生成客户端证书

```bash
# 客户端密码
CLIENT_PASSWORD="client123"

# 生成客户端私钥
openssl genrsa -aes256 -passout pass:${CLIENT_PASSWORD} -out certs/client/client.key.pem 4096

# 生成客户端 CSR
openssl req -config server_ext.cnf \
    -key certs/client/client.key.pem \
    -passin pass:${CLIENT_PASSWORD} \
    -new -sha256 \
    -out certs/client/client.csr.pem \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=HDP/OU=DiskInspect/CN=client/emailAddress=client@example.com"

# 使用 CA 签发客户端证书
openssl x509 -req -passin pass:${CA_PASSWORD} \
    -CA certs/ca/ca.crt.pem \
    -CAkey certs/ca/ca.key.pem \
    -in certs/client/client.csr.pem \
    -out certs/client/client.crt.pem \
    -days 365 \
    -sha256 \
    -extensions client_cert_ext \
    -extfile server_ext.cnf \
    -CAcreateserial

# 导出未加密的私钥
openssl rsa -passin pass:${CLIENT_PASSWORD} -in certs/client/client.key.pem -out certs/client/client.key 2>/dev/null || true

# 导出纯 PEM 格式的证书
openssl x509 -in certs/client/client.crt.pem -out certs/client/client.crt -outform PEM

# 导出 PKCS12 格式
openssl pkcs12 -export -passout pass:${CLIENT_PASSWORD} \
    -in certs/client/client.crt.pem \
    -inkey certs/client/client.key.pem \
    -passin pass:${CLIENT_PASSWORD} \
    -out certs/client/client.p12 \
    -name "client"

# 设置权限
chmod 400 certs/client/client.key.pem certs/client/client.key
chmod 444 certs/client/client.crt.pem certs/client/client.crt

# 验证客户端证书
openssl verify -CAfile certs/ca/ca.crt certs/client/client.crt

# 清理临时文件
rm -f server_ext.cnf certs/client/client.csr.pem
```

### 证书文件结构

```
certs/
├── ca/
│   ├── ca.crt.pem          # CA 证书（PEM 格式，与客户端共享）
│   ├── ca.crt              # CA 证书（纯 PEM 格式，推荐使用）
│   ├── ca.key.pem          # CA 私钥（加密，保密！）
│   ├── ca.key              # CA 私钥（未加密）
│   └── ca.p12              # PKCS12 格式（用于浏览器）
│
├── server/
│   ├── server.crt.pem      # 服务器证书（PEM）
│   ├── server.crt          # 服务器证书（纯 PEM）
│   ├── server.key.pem      # 服务器私钥（加密）
│   ├── server.key          # 服务器私钥（未加密）
│   └── server.p12          # PKCS12 格式
│
└── client/
    ├── client.crt.pem      # 客户端证书（PEM）
    ├── client.crt          # 客户端证书（纯 PEM）
    ├── client.key.pem      # 客户端私钥（加密）
    ├── client.key          # 客户端私钥（未加密）
    └── client.p12          # PKCS12 格式
```

### 验证证书

```bash
# 验证 CA 证书
openssl x509 -in certs/ca/ca.crt -text -noout

# 验证服务器证书是否由 CA 签发
openssl verify -CAfile certs/ca/ca.crt certs/server/server.crt

# 验证客户端证书是否由 CA 签发
openssl verify -CAfile certs/ca/ca.crt certs/client/client.crt

# 查看证书有效期
openssl x509 -in certs/server/server.crt -noout -dates

# 查看 Subject Alternative Name (SAN)
openssl x509 -in certs/server/server.crt -noout -text | grep -A 10 "Subject Alternative Name"

# 查看证书指纹
openssl x509 -in certs/server/server.crt -noout -fingerprint -sha256
```

---

## Go 服务器配置

### 基本启动命令

```bash
./hdp-disk-inspect -enable-tls -ca-file certs/ca/ca.crt -cert-file certs/server/server.crt -key-file certs/server/server.key
```

### 完整参数说明

| 参数 | 描述 |
|------|------|
| `-enable-tls` | 启用 TLS 双向认证 |
| `-ca-file` | CA 证书文件路径 |
| `-cert-file` | 服务器证书文件路径 |
| `-key-file` | 服务器私钥文件路径 |
| `-s` | 服务器监听地址（默认：0.0.0.0:58002） |
| `-log-level` | 日志级别（debug/info/warn/error） |

### 部署配置示例

创建目录和部署证书：

```bash
# 创建专用用户
sudo useradd -r -s /usr/sbin/nologin -d /opt/hdp-disk-inspect hdp-disk-inspect

# 创建证书目录
sudo mkdir -p /opt/hdp-disk-inspect/certs
sudo cp -r certs/ca /opt/hdp-disk-inspect/certs/
sudo cp -r certs/server /opt/hdp-disk-inspect/certs/

# 设置权限
sudo chown -R hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/certs
sudo chmod 700 /opt/hdp-disk-inspect/certs
sudo chmod 400 /opt/hdp-disk-inspect/certs/server/server.key
sudo chmod 444 /opt/hdp-disk-inspect/certs/server/server.crt
sudo chmod 444 /opt/hdp-disk-inspect/certs/ca/ca.crt

# 复制二进制文件
sudo cp hdp-disk-inspect /opt/hdp-disk-inspect/
sudo chown hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/hdp-disk-inspect
sudo chmod 750 /opt/hdp-disk-inspect/hdp-disk-inspect
```

### systemd 配置（带 TLS）

编辑 `/etc/systemd/system/hdp-disk-inspect.service`:

```ini
[Unit]
Description=HDP Disk Inspect (TLS)
Documentation=https://github.com/lynnyq/hdp-disk-inspect
After=network.target

[Service]
Type=simple
User=hdp-disk-inspect
Group=hdp-disk-inspect
WorkingDirectory=/opt/hdp-disk-inspect
ExecStart=/opt/hdp-disk-inspect/hdp-disk-inspect \
    -enable-tls \
    -ca-file /opt/hdp-disk-inspect/certs/ca/ca.crt \
    -cert-file /opt/hdp-disk-inspect/certs/server/server.crt \
    -key-file /opt/hdp-disk-inspect/certs/server/server.key \
    -s 0.0.0.0:58002 \
    -log-level info
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# 资源限制
MemoryLimit=256M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

### 启动和验证服务

```bash
# 重载 systemd
sudo systemctl daemon-reload

# 启用并启动服务
sudo systemctl enable hdp-disk-inspect --now

# 查看服务状态
sudo systemctl status hdp-disk-inspect

# 查看日志
sudo journalctl -u hdp-disk-inspect -n 50 -f
```

---

## Go 客户端配置

### 完整的客户端代码示例

参见 [API 参考文档](api.md) 的 Go 客户端部分。

### 简单客户端示例

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

func main() {
	// 1. 加载证书
	caCert, err := os.ReadFile("certs/ca/ca.crt")
	if err != nil {
		log.Fatalf("load CA cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("failed to add CA certificate")
	}

	clientCert, err := tls.LoadX509KeyPair("certs/client/client.crt", "certs/client/client.key")
	if err != nil {
		log.Fatalf("load client cert: %v", err)
	}

	// 2. 配置 TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}
	creds := credentials.NewTLS(tlsConfig)

	// 3. 连接服务器
	conn, err := grpc.Dial("localhost:58002", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	// 4. 执行命令
	client := pb.NewTaskClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Run(ctx, &pb.TaskRequest{
		Command: "echo 'Hello TLS!'",
		Timeout: 30,
		Id:      1,
	})
	if err != nil {
		log.Fatalf("execute: %v", err)
	}

	fmt.Printf("Exit Code: %d\n", resp.ExitCode)
	fmt.Printf("Duration: %d ms\n", resp.DurationMs)
	if resp.Output != "" {
		fmt.Printf("\nOutput:\n%s\n", resp.Output)
	}
}
```

---

## Python 服务器/客户端

项目还提供 Python 版本的服务器和客户端，主要用于测试和调试。

### Python 服务器

```bash
cd server
pip install -r requirements.txt
python generate_proto.py

# 生成证书（使用脚本）
./generate_certs.sh all

# 启动 Python 服务器（TLS）
python server.py --enable-tls \
    --ca-cert certs/ca/ca.crt \
    --server-cert certs/server/server.crt \
    --server-key certs/server/server.key
```

### Python 客户端

```bash
# 执行命令（TLS）
python client.py "echo 'Hello Python TLS'" \
    --use-tls \
    --ca-cert certs/ca/ca.crt \
    --client-cert certs/client/client.crt \
    --client-key certs/client/client.key

# 交互模式
python client.py --interactive \
    --use-tls \
    --ca-cert certs/ca/ca.crt \
    --client-cert certs/client/client.crt \
    --client-key certs/client/client.key
```

---

## 安全最佳实践

### 1. 证书管理

- **保密私钥**: 永远不要将私钥提交到版本控制系统
- **加密私钥**: 使用强密码加密私钥
- **文件权限**: 设置正确的文件权限

```bash
chmod 700 certs/
chmod 400 certs/ca/ca.key certs/server/server.key certs/client/client.key
chmod 444 certs/ca/ca.crt certs/server/server.crt certs/client/client.crt
```

### 2. 证书有效期

- **CA 证书**: 长期有效（如 10 年）
- **服务器/客户端证书**: 短期有效（如 1 年）
- **轮换计划**: 建立证书轮换流程

```bash
# 重新生成服务器证书（更新有效期）
./generate_certs.sh server -i 127.0.0.1 -i 192.168.1.100
```

### 3. 限制访问

- **防火墙**: 使用防火墙限制允许访问的 IP 范围
- **客户端证书**: 只授权给需要的客户端
- **网络隔离**: 将管理接口放在隔离网络

```bash
# UFW 防火墙示例
sudo ufw allow from 192.168.1.0/24 to any port 58002

# firewalld 示例
sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="192.168.1.0/24" port protocol="tcp" port="58002" accept'
sudo firewall-cmd --reload
```

### 4. 监控和日志

- **启用详细日志**: 生产环境也应记录 TLS 握手信息
- **监控证书有效期**: 提前告警证书即将过期
- **异常访问**: 监控 TLS 握手失败的情况

```bash
# 检查证书过期时间（剩余天数）
cert_file="certs/server/server.crt"
expiry=$(openssl x509 -enddate -noout -in "$cert_file" | cut -d= -f2)
expiry_epoch=$(date -d "$expiry" +%s)
now_epoch=$(date +%s)
days_remaining=$(( ($expiry_epoch - $now_epoch) / 86400 ))
echo "证书剩余天数: $days_remaining"

# 检查即将过期的证书（少于 30 天）
if [ $days_remaining -lt 30 ]; then
    echo "警告: 证书即将过期！"
fi
```

### 5. TLS 安全配置

HDP Disk Inspect 使用 Go 默认的安全 TLS 配置，包括：
- TLS 1.2+
- 安全的密码套件
- 证书链验证

### 6. 凭证轮换

```bash
# 定期更新客户端证书
./generate_certs.sh client

# 分发新证书到客户端
# 建议使用配置管理工具（Ansible、Puppet、Chef 等）
```

### 7. 多环境证书管理

为不同环境使用独立的 CA：

```bash
# 开发环境
./generate_certs.sh all -i 127.0.0.1 -i 192.168.1.100 -h localhost -h dev.server.example.com

# 测试环境
./generate_certs.sh all -i 127.0.0.1 -i 192.168.2.100 -h localhost -h test.server.example.com

# 生产环境
./generate_certs.sh all -i 192.168.100.100 -i 192.168.100.101 -h prod-server1.example.com -h prod-server2.example.com
```

---

## 故障排除

### TLS 握手失败

#### 症状
```
rpc error: code = Unavailable desc = connection closed before server preface received
```

#### 排查步骤

```bash
# 1. 验证服务器证书有正确的 SAN（包含连接使用的 IP/主机名）
openssl x509 -in certs/server/server.crt -noout -text | grep -A 10 "Subject Alternative Name"

# 2. 检查系统时间是否正确
date

# 3. 查看服务器日志
sudo journalctl -u hdp-disk-inspect -n 50 -f

# 4. 验证证书有效性
openssl verify -CAfile certs/ca/ca.crt certs/server/server.crt
openssl verify -CAfile certs/ca/ca.crt certs/client/client.crt

# 5. 使用 OpenSSL s_client 测试连接
openssl s_client -connect localhost:58002 \
    -CAfile certs/ca/ca.crt \
    -cert certs/client/client.crt \
    -key certs/client/client.key

# 6. 检查文件权限
ls -la certs/
```

#### 常见原因

1. **SAN 不匹配** - 连接使用的 IP/主机名不在证书的 SAN 列表中
2. **证书过期** - 检查证书有效期
3. **时间不同步** - 系统时间差异过大导致证书验证失败
4. **文件权限** - 证书/密钥文件权限不正确
5. **CA 不匹配** - 客户端使用的 CA 证书不是签发服务器证书的 CA

### 证书过期

```bash
# 检查证书有效期
openssl x509 -in certs/server/server.crt -noout -dates

# 重新生成证书
./generate_certs.sh all -i 127.0.0.1 -i 192.168.1.100
```

### 权限被拒绝

```
rpc error: code = PermissionDenied desc = ...
```

检查：
- 服务器配置了 `RequireAndVerifyClientCert`
- 客户端提供了有效的客户端证书
- CA 证书正确，能验证客户端证书

### 更多帮助

参见 [故障排除文档](troubleshooting.md)。

---

## 相关文档

- [API 参考](api.md) - gRPC API 文档
- [部署指南](deployment.md) - 生产环境部署
- [故障排除](troubleshooting.md) - 问题排查
