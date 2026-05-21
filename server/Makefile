.PHONY: help install gen clean run test certs certs-clean certs-list

PYTHON := python3
PIP := pip3
OPENSSL := openssl
SERVER_DIR := $(shell pwd)
PROTO_DIR := $(SERVER_DIR)/../rpc/proto
CERTS_DIR := $(SERVER_DIR)/certs

help:
	@echo "HDP Disk Inspect Python gRPC Server"
	@echo ""
	@echo "Available targets:"
	@echo "  install       - Install Python dependencies"
	@echo "  gen           - Generate gRPC code from proto files"
	@echo "  clean         - Remove generated files"
	@echo "  run            - Run the gRPC server"
	@echo "  run-tls        - Run the gRPC server with TLS"
	@echo "  test           - Run basic tests"
	@echo "  certs          - Generate TLS certificates"
	@echo "  certs-server   - Generate server certificate with IPs/hosts"
	@echo "  certs-client   - Generate client certificate"
	@echo "  certs-clean    - Remove all certificates"
	@echo "  certs-list     - List generated certificates"
	@echo ""
	@echo "Certificate options:"
	@echo "  IPs=127.0.0.1,192.168.1.100"
	@echo "  HOSTS=localhost,server.example.com"
	@echo ""

install:
	$(PIP) install -r requirements.txt

gen:
	$(PYTHON) generate_proto.py

clean:
	rm -f $(SERVER_DIR)/*_pb2*.py
	find $(SERVER_DIR) -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find $(SERVER_DIR) -name "*.pyc" -delete

run:
	$(PYTHON) server.py

run-tls:
	@if [ ! -f "$(CERTS_DIR)/server/server.crt" ]; then \
		echo "Certificates not found. Run 'make certs' first."; \
		exit 1; \
	fi
	$(PYTHON) server.py --enable-tls \
		--ca-cert=$(CERTS_DIR)/ca/ca.crt \
		--server-cert=$(CERTS_DIR)/server/server.crt \
		--server-key=$(CERTS_DIR)/server/server.key

test:
	@echo "Running basic import tests..."
	$(PYTHON) -c "import task_pb2; import task_pb2_grpc; print('Imports OK')" 2>/dev/null || echo "Run 'make gen' first to generate proto files"

# Certificate generation targets
certs:
	@echo "Generating TLS certificates..."
	@chmod +x $(SERVER_DIR)/generate_certs.sh
	@$(SERVER_DIR)/generate_certs.sh all
	@echo ""
	@echo "Certificates generated in $(CERTS_DIR)/"
	@echo "Run 'make certs-list' to see generated files"

certs-server:
	@echo "Generating server certificate..."
	@chmod +x $(SERVER_DIR)/generate_certs.sh
	@$(SERVER_DIR)/generate_certs.sh server -i 127.0.0.1 $(if $(IPs),$(foreach ip,$(shell echo $(IPs) | tr ',' ' '),-i $(ip)))$(if $(HOSTS),$(foreach host,$(shell echo $(HOSTS) | tr ',' ' '),-h $(host)))
	@echo ""
	@echo "Server certificate generated in $(CERTS_DIR)/server/"

certs-client:
	@echo "Generating client certificate..."
	@chmod +x $(SERVER_DIR)/generate_certs.sh
	@$(SERVER_DIR)/generate_certs.sh client
	@echo ""
	@echo "Client certificate generated in $(CERTS_DIR)/client/"

certs-clean:
	@echo "Cleaning up certificates..."
	@rm -rf $(CERTS_DIR)
	@echo "Certificates cleaned successfully!"

certs-list:
	@echo "Generated certificates:"
	@echo ""
	@if [ -d "$(CERTS_DIR)" ]; then \
		echo "CA Directory ($(CERTS_DIR)/ca):"; \
		ls -la $(CERTS_DIR)/ca 2>/dev/null || echo "  [Not generated]"; \
		echo ""; \
		echo "Server Directory ($(CERTS_DIR)/server):"; \
		ls -la $(CERTS_DIR)/server 2>/dev/null || echo "  [Not generated]"; \
		echo ""; \
		echo "Client Directory ($(CERTS_DIR)/client):"; \
		ls -la $(CERTS_DIR)/client 2>/dev/null || echo "  [Not generated]"; \
	else \
		echo "No certificates generated yet. Run 'make certs' first."; \
	fi
