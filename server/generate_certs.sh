#!/usr/bin/env bash
#
# TLS Mutual Authentication Certificate Generation Script
#
# This script generates certificates for mutual TLS (mTLS) authentication
# between server and clients. Supports multiple IP addresses and hostnames.
#

set -euo pipefail

# Configuration
COUNTRY="CN"
STATE="Beijing"
CITY="Beijing"
ORGANIZATION="HDP"
ORGANIZATIONAL_UNIT="DiskInspect"
EMAIL="admin@example.com"
CA_PASSWORD="changeit"
SERVER_PASSWORD="server123"
CLIENT_PASSWORD="client123"

# Certificate validity (in days)
CA_VALIDITY=3650
SERVER_VALIDITY=365
CLIENT_VALIDITY=365

# Output directories
CERTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_DIR="${CERTS_DIR}/ca"
SERVER_DIR="${CERTS_DIR}/server"
CLIENT_DIR="${CERTS_DIR}/client"

# OpenSSL configuration template
OPENSSL_CA_CONFIG="
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

[ v3_intermediate_ca ]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true, pathlen:0
keyUsage = critical, digitalSignature, cRLSign, keyCertSign

[ server_cert ]
basicConstraints = CA:FALSE
nsCertType = server
nsComment = \"OpenSSL Generated Server Certificate\"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer:always
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth

[ client_cert ]
basicConstraints = CA:FALSE
nsCertType = client, email
nsComment = \"OpenSSL Generated Client Certificate\"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, emailProtection
"

# Function to display usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS] COMMAND

Commands:
    all         Generate all certificates (CA, Server, Client)
    ca          Generate CA certificate only
    server      Generate server certificate
    client      Generate client certificate
    clean       Remove all generated certificates
    list        List generated certificates

Options:
    -d, --days          Certificate validity in days (default: 365)
    -i, --ip            Add IP address to server certificate (can be specified multiple times)
    -h, --host          Add hostname to server certificate (can be specified multiple times)
    --ca-password       CA private key password
    --server-password   Server private key password
    --client-password   Client private key password
    -o, --output        Output directory (default: current directory)
    --help              Show this help message

Examples:
    # Generate all certificates with default settings
    $0 all

    # Generate server certificate with multiple IPs
    $0 server -i 127.0.0.1 -i 192.168.1.100 -i 10.0.0.1

    # Generate server certificate with hostname and IP
    $0 server -h localhost -h server.example.com -i 127.0.0.1

    # Generate with custom validity
    $0 all -d 730
EOF
}

# Arrays to store IPs and hosts
SERVER_IPS=()
SERVER_HOSTS=()

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        all|ca|server|client|clean|list)
            COMMAND="$1"
            shift
            ;;
        -d|--days)
            VALIDITY="$2"
            shift 2
            ;;
        -i|--ip)
            SERVER_IPS+=("$2")
            shift 2
            ;;
        -h|--host)
            SERVER_HOSTS+=("$2")
            shift 2
            ;;
        --ca-password)
            CA_PASSWORD="$2"
            shift 2
            ;;
        --server-password)
            SERVER_PASSWORD="$2"
            shift 2
            ;;
        --client-password)
            CLIENT_PASSWORD="$2"
            shift 2
            ;;
        -o|--output)
            CERTS_DIR="$2"
            CA_DIR="${CERTS_DIR}/ca"
            SERVER_DIR="${CERTS_DIR}/server"
            CLIENT_DIR="${CERTS_DIR}/client"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Set default command
COMMAND="${COMMAND:-all}"
VALIDITY="${VALIDITY:-365}"

# Create directories
create_directories() {
    mkdir -p "${CA_DIR}" "${SERVER_DIR}" "${CLIENT_DIR}"
    chmod 700 "${CA_DIR}" "${SERVER_DIR}" "${CLIENT_DIR}"
}

# Generate OpenSSL config file
generate_openssl_config() {
    local config_file="$1"
    local is_server="${2:-false}"

    cat > "${config_file}" <<< "${OPENSSL_CA_CONFIG}"

    if [[ "${is_server}" == "true" ]] && [[ ${#SERVER_IPS[@]} -gt 0 || ${#SERVER_HOSTS[@]} -gt 0 ]]; then
        cat >> "${config_file}" <<EOF

[ server_cert_ext ]
subjectAltName = @alt_names

[ alt_names ]
EOF
        local index=1
        for ip in "${SERVER_IPS[@]}"; do
            echo "IP.${index} = ${ip}" >> "${config_file}"
            ((index++))
        done
        for host in "${SERVER_HOSTS[@]}"; do
            echo "DNS.${index} = ${host}" >> "${config_file}"
            ((index++))
        done
    fi
}

# Generate CA certificate
generate_ca() {
    echo "==> Generating CA certificate..."

    # Generate CA private key
    openssl genrsa -aes256 -passout "pass:${CA_PASSWORD}" \
        -out "${CA_DIR}/ca.key.pem" 4096

    # Generate CA certificate
    openssl req -config <(cat <<< "${OPENSSL_CA_CONFIG}") \
        -key "${CA_DIR}/ca.key.pem" \
        -passin "pass:${CA_PASSWORD}" \
        -new -x509 -days "${CA_VALIDITY}" \
        -sha256 -extensions v3_ca \
        -out "${CA_DIR}/ca.crt.pem" \
        -subj "/C=${COUNTRY}/ST=${STATE}/L=${CITY}/O=${ORGANIZATION}/OU=${ORGANIZATIONAL_UNIT}/CN=${ORGANIZATION}-CA/emailAddress=${EMAIL}"

    # Verify CA certificate
    echo "==> Verifying CA certificate..."
    openssl x509 -noout -text -in "${CA_DIR}/ca.crt.pem" | head -20

    # Export CA to PKCS12 format for distribution
    openssl pkcs12 -export -passout "pass:${CA_PASSWORD}" \
        -in "${CA_DIR}/ca.crt.pem" \
        -inkey "${CA_DIR}/ca.key.pem" \
        -passin "pass:${CA_PASSWORD}" \
        -out "${CA_DIR}/ca.p12"

    # Export CA certificate to PEM format
    openssl x509 -in "${CA_DIR}/ca.crt.pem" -out "${CA_DIR}/ca.crt" -outform PEM

    # Export CA private key to unencrypted PEM (for signing)
    openssl rsa -passin "pass:${CA_PASSWORD}" \
        -in "${CA_DIR}/ca.key.pem" \
        -out "${CA_DIR}/ca.key" 2>/dev/null || true

    chmod 400 "${CA_DIR}/ca.key.pem"
    chmod 444 "${CA_DIR}/ca.crt.pem"

    echo "==> CA certificate generated successfully!"
    echo "    CA Certificate: ${CA_DIR}/ca.crt.pem"
    echo "    CA Private Key: ${CA_DIR}/ca.key.pem"
    echo "    CA PKCS12: ${CA_DIR}/ca.p12"
}

# Generate Server certificate
generate_server() {
    echo "==> Generating Server certificate..."

    local server_key="${SERVER_DIR}/server.key.pem"
    local server_csr="${SERVER_DIR}/server.csr.pem"
    local server_cert="${SERVER_DIR}/server.crt.pem"
    local server_p12="${SERVER_DIR}/server.p12"

    # Generate server private key
    openssl genrsa -aes256 -passout "pass:${SERVER_PASSWORD}" \
        -out "${server_key}" 4096

    # Generate server CSR
    local subject="/C=${COUNTRY}/ST=${STATE}/L=${CITY}/O=${ORGANIZATION}/OU=${ORGANIZATIONAL_UNIT}/CN=server"

    # If SAN config exists, use it
    local ext_section="server_cert"
    local ext_file="${SERVER_DIR}/server_ext.cnf"

    if [[ ${#SERVER_IPS[@]} -gt 0 || ${#SERVER_HOSTS[@]} -gt 0 ]]; then
        generate_openssl_config "${ext_file}" true
        ext_section="server_cert_ext"
    fi

    openssl req -config "${ext_file:-/dev/null}" \
        -key "${server_key}" \
        -passin "pass:${SERVER_PASSWORD}" \
        -new -sha256 \
        -out "${server_csr}" \
        -subj "${subject}"

    # Sign server certificate with CA
    openssl x509 -req -passin "pass:${CA_PASSWORD}" \
        -CA "${CA_DIR}/ca.crt.pem" \
        -CAkey "${CA_DIR}/ca.key.pem" \
        -in "${server_csr}" \
        -out "${server_cert}" \
        -days "${VALIDITY}" \
        -sha256 \
        -extensions "${ext_section}" \
        $([ -f "${ext_file}" ] && echo "-extfile ${ext_file}") \
        -CAcreateserial

    # Verify server certificate
    echo "==> Verifying server certificate..."
    openssl verify -CAfile "${CA_DIR}/ca.crt.pem" "${server_cert}"

    # Show certificate details
    echo "==> Server certificate details:"
    openssl x509 -noout -text -in "${server_cert}" | grep -A2 "Subject Alternative Name" || true

    # Export server to PKCS12 format
    openssl pkcs12 -export -passout "pass:${SERVER_PASSWORD}" \
        -in "${server_cert}" \
        -inkey "${server_key}" \
        -passin "pass:${SERVER_PASSWORD}" \
        -out "${server_p12}" \
        -name "server"

    # Export to PEM format
    openssl x509 -in "${server_cert}" -out "${SERVER_DIR}/server.crt" -outform PEM
    openssl rsa -passin "pass:${SERVER_PASSWORD}" \
        -in "${server_key}" \
        -out "${SERVER_DIR}/server.key" 2>/dev/null || true

    chmod 400 "${server_key}"
    chmod 444 "${server_cert}"

    # Cleanup
    rm -f "${server_csr}" "${ext_file}" 2>/dev/null || true

    echo "==> Server certificate generated successfully!"
    echo "    Server Certificate: ${SERVER_DIR}/server.crt.pem"
    echo "    Server Private Key: ${SERVER_DIR}/server.key.pem"
    echo "    Server PKCS12: ${SERVER_DIR}/server.p12"
}

# Generate Client certificate
generate_client() {
    echo "==> Generating Client certificate..."

    local client_key="${CLIENT_DIR}/client.key.pem"
    local client_csr="${CLIENT_DIR}/client.csr.pem"
    local client_cert="${CLIENT_DIR}/client.crt.pem"
    local client_p12="${CLIENT_DIR}/client.p12"
    local ext_file="${CLIENT_DIR}/client_ext.cnf"

    generate_openssl_config "${ext_file}" false

    # Generate client private key
    openssl genrsa -aes256 -passout "pass:${CLIENT_PASSWORD}" \
        -out "${client_key}" 4096

    # Generate client CSR
    openssl req -config "${ext_file}" \
        -key "${client_key}" \
        -passin "pass:${CLIENT_PASSWORD}" \
        -new -sha256 \
        -out "${client_csr}" \
        -subj "/C=${COUNTRY}/ST=${STATE}/L=${CITY}/O=${ORGANIZATION}/OU=${ORGANIZATIONAL_UNIT}/CN=client/emailAddress=${EMAIL}"

    # Sign client certificate with CA
    openssl x509 -req -passin "pass:${CA_PASSWORD}" \
        -CA "${CA_DIR}/ca.crt.pem" \
        -CAkey "${CA_DIR}/ca.key.pem" \
        -in "${client_csr}" \
        -out "${client_cert}" \
        -days "${VALIDITY}" \
        -sha256 \
        -extensions client_cert \
        -extfile "${ext_file}" \
        -CAcreateserial

    # Verify client certificate
    echo "==> Verifying client certificate..."
    openssl verify -CAfile "${CA_DIR}/ca.crt.pem" "${client_cert}"

    # Export client to PKCS12 format
    openssl pkcs12 -export -passout "pass:${CLIENT_PASSWORD}" \
        -in "${client_cert}" \
        -inkey "${client_key}" \
        -passin "pass:${CLIENT_PASSWORD}" \
        -out "${client_p12}" \
        -name "client"

    # Export to PEM format
    openssl x509 -in "${client_cert}" -out "${CLIENT_DIR}/client.crt" -outform PEM
    openssl rsa -passin "pass:${CLIENT_PASSWORD}" \
        -in "${client_key}" \
        -out "${CLIENT_DIR}/client.key" 2>/dev/null || true

    chmod 400 "${client_key}"
    chmod 444 "${client_cert}"

    # Cleanup
    rm -f "${client_csr}" "${ext_file}" 2>/dev/null || true

    echo "==> Client certificate generated successfully!"
    echo "    Client Certificate: ${CLIENT_DIR}/client.crt.pem"
    echo "    Client Private Key: ${CLIENT_DIR}/client.key.pem"
    echo "    Client PKCS12: ${CLIENT_DIR}/client.p12"
}

# Clean generated certificates
clean_certs() {
    echo "==> Cleaning up certificates..."
    rm -rf "${CA_DIR}" "${SERVER_DIR}" "${CLIENT_DIR}"
    echo "==> Certificates cleaned successfully!"
}

# List generated certificates
list_certs() {
    echo "==> Generated certificates:"
    echo ""
    echo "CA Directory (${CA_DIR}):"
    ls -la "${CA_DIR}" 2>/dev/null || echo "  [Not generated]"
    echo ""
    echo "Server Directory (${SERVER_DIR}):"
    ls -la "${SERVER_DIR}" 2>/dev/null || echo "  [Not generated]"
    echo ""
    echo "Client Directory (${CLIENT_DIR}):"
    ls -la "${CLIENT_DIR}" 2>/dev/null || echo "  [Not generated]"
}

# Generate README
generate_readme() {
    local readme_file="${CERTS_DIR}/README.md"
    cat > "${readme_file}" <<'EOF'
# TLS Mutual Authentication Certificates

This directory contains certificates for mutual TLS (mTLS) authentication.

## Directory Structure

```
certs/
├── ca/
│   ├── ca.crt.pem      # CA certificate (share with clients)
│   ├── ca.crt          # CA certificate in PEM format
│   ├── ca.key.pem      # CA private key (keep secret)
│   └── ca.p12          # CA in PKCS12 format (for browsers)
├── server/
│   ├── server.crt.pem  # Server certificate
│   ├── server.crt      # Server certificate in PEM format
│   ├── server.key.pem  # Server private key
│   └── server.p12      # Server certificate in PKCS12 format
└── client/
    ├── client.crt.pem  # Client certificate
    ├── client.crt      # Client certificate in PEM format
    ├── client.key.pem  # Client private key
    └── client.p12      # Client certificate in PKCS12 format
```

## Certificate Usage

### Server Certificate (Go)

```go
// Load server certificate
cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
if err != nil {
    log.Fatal(err)
}

// Configure TLS
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    caCertPool,
}

// Use with gRPC server
```

### Client Certificate (Python)

```python
import grpc
import ssl

# Load client certificate
credentials = grpc.ssl_channel_credentials(
    root_certificates='ca.crt',      # Server's CA certificate
    private_key='client.key',         # Client private key
    certificate_chain='client.crt'   # Client certificate
)

# Create secure channel
channel = grpc.secure_channel('localhost:5921', credentials)
```

### Client Certificate (Go)

```go
// Load client certificate
cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
if err != nil {
    log.Fatal(err)
}

// Configure TLS
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    RootCAs:     caCertPool,
}

// Use with gRPC client
```

## Certificate Passwords

Default passwords (change these in production!):
- CA Password: `changeit`
- Server Password: `server123`
- Client Password: `client123`

## Certificate Renewal

When certificates expire, regenerate them using:

```bash
# Regenerate all certificates
./generate_certs.sh all

# Regenerate only server certificate
./generate_certs.sh server -i 192.168.1.100 -h myserver.com
```

## Security Notes

1. Keep private keys secure and never commit them to version control
2. Use strong passwords for production certificates
3. Set appropriate file permissions: `chmod 400 *.key.pem`
4. Rotate certificates regularly
5. Use short validity periods for production certificates

EOF
    echo "==> README generated: ${readme_file}"
}

# Main function
main() {
    echo "=========================================="
    echo "  TLS Mutual Authentication Certificate Generator"
    echo "=========================================="
    echo ""
    echo "Output Directory: ${CERTS_DIR}"
    echo "Certificate Validity: ${VALIDITY} days"
    echo ""

    case "${COMMAND}" in
        all)
            create_directories
            generate_ca
            generate_server
            generate_client
            generate_readme
            echo ""
            echo "=========================================="
            echo "  All certificates generated successfully!"
            echo "=========================================="
            ;;
        ca)
            create_directories
            generate_ca
            ;;
        server)
            create_directories
            generate_server
            ;;
        client)
            create_directories
            generate_client
            ;;
        clean)
            clean_certs
            ;;
        list)
            list_certs
            ;;
        *)
            echo "Unknown command: ${COMMAND}"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main
