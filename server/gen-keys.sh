#!/bin/bash
set -e
mkdir -p certs
cd certs

# 1. CA
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt \
-subj "/C=CN/ST=GD/L=SZ/O=GRPC/OU=CA/CN=grpc-ca"

# 2. Server
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr \
-subj "/C=CN/ST=GD/L=SZ/O=GRPC/OU=SERVER/CN=localhost"
cat > server.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
-out server.crt -days 1825 -sha256 -extfile server.ext

# 3. Client
openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr \
-subj "/C=CN/ST=GD/L=SZ/O=GRPC/OU=CLIENT/CN=grpc-client"
cat > client.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
EOF
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAserial ca.srl \
-out client.crt -days 1825 -sha256 -extfile client.ext

# 清理临时文件
rm -f *.csr *.ext ca.srl
echo "证书生成完成，目录：$(pwd)"
ls -l