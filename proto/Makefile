# Makefile for generating Go and Python code from protobuf files

PROTO_DIR := .
GO_OUT := ../go/pkg/contracts/v1
PYTHON_OUT := ../py/provider_gateway/app/grpc_api/generated

# Protocol files
PROTO_FILES := common.proto asr.proto llm.proto tts.proto provider.proto

# Default target
.PHONY: all generate-all generate-go generate-python clean help

all: generate-all

generate-all: generate-go generate-python

# Generate Go code
generate-go:
	@echo "Generating Go code..."
	@mkdir -p $(GO_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
	@echo "Go code generated in $(GO_OUT)"

# Generate Python code
generate-python:
	@echo "Generating Python code..."
	@mkdir -p $(PYTHON_OUT)
	python -m grpc_tools.protoc \
		--proto_path=$(PROTO_DIR) \
		--python_out=$(PYTHON_OUT) \
		--pyi_out=$(PYTHON_OUT) \
		--grpc_python_out=$(PYTHON_OUT) \
		$(PROTO_FILES)
	@echo "Python code generated in $(PYTHON_OUT)"

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf $(GO_OUT)/*.go
	rm -rf $(PYTHON_OUT)/*_pb2*.py
	rm -rf $(PYTHON_OUT)/*_pb2*.pyi
	@echo "Cleaned generated files"

# Help
help:
	@echo "Available targets:"
	@echo "  generate-all     - Generate both Go and Python code"
	@echo "  generate-go      - Generate Go code only"
	@echo "  generate-python  - Generate Python code only"
	@echo "  clean            - Remove all generated files"
	@echo "  help             - Show this help message"
