#!/bin/bash
#
# CloudApp Protocol Buffer Generator
# Generates Go and Python stubs from .proto files
#
# Prerequisites:
#   - Go: protoc-gen-go and protoc-gen-go-grpc
#     go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#     go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   - Python: grpcio-tools
#     pip install grpcio-tools
#
# Usage:
#   ./scripts/generate-proto.sh           # Generate all stubs
#   ./scripts/generate-proto.sh --go      # Generate Go only
#   ./scripts/generate-proto.sh --python  # Generate Python only
#   ./scripts/generate-proto.sh --clean   # Clean generated files first
#
# To make this script executable:
#   chmod +x scripts/generate-proto.sh

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Directories
PROTO_DIR="${PROJECT_ROOT}/proto"
GO_OUT="${PROJECT_ROOT}/go/pkg/contracts"
PYTHON_OUT="${PROJECT_ROOT}/py/provider_gateway/app/grpc_api/generated"

# Generation flags
GEN_GO=true
GEN_PYTHON=true
CLEAN=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --go)
            GEN_GO=true
            GEN_PYTHON=false
            shift
            ;;
        --python)
            GEN_GO=false
            GEN_PYTHON=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --go       Generate Go stubs only"
            echo "  --python   Generate Python stubs only"
            echo "  --clean    Clean generated files before regenerating"
            echo "  --help     Show this help message"
            echo ""
            echo "Prerequisites:"
            echo "  Go:      go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
            echo "           go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
            echo "  Python:  pip install grpcio-tools"
            exit 0
            ;;
    esac
done

echo "========================================"
echo "CloudApp Protocol Buffer Generator"
echo "========================================"
echo ""

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed"
    echo "Download from: https://github.com/protocolbuffers/protobuf/releases"
    echo "Or install with: brew install protobuf (macOS)"
    exit 1
fi

echo "Using protoc version: $(protoc --version)"
echo ""

# Clean generated files if requested
if [[ "$CLEAN" == true ]]; then
    echo "Cleaning generated files..."
    if [[ "$GEN_GO" == true ]] && [[ -d "$GO_OUT" ]]; then
        rm -f "$GO_OUT"/*.pb.go
        echo "  ✓ Cleaned Go stubs"
    fi
    if [[ "$GEN_PYTHON" == true ]] && [[ -d "$PYTHON_OUT" ]]; then
        rm -f "$PYTHON_OUT"/*_pb2*.py
        echo "  ✓ Cleaned Python stubs"
    fi
    echo ""
fi

# Create output directories
mkdir -p "$GO_OUT"
mkdir -p "$PYTHON_OUT"

# Find all proto files
PROTO_FILES=$(find "$PROTO_DIR" -name "*.proto" -not -name "*.git*")

if [[ -z "$PROTO_FILES" ]]; then
    echo "Error: No .proto files found in $PROTO_DIR"
    exit 1
fi

echo "Found proto files:"
for file in $PROTO_FILES; do
    echo "  - $(basename "$file")"
done
echo ""

# Generate Go stubs
if [[ "$GEN_GO" == true ]]; then
    echo "Generating Go stubs..."
    echo "  Output: $GO_OUT"
    
    # Check for Go plugins
    if ! command -v protoc-gen-go &> /dev/null; then
        echo "Error: protoc-gen-go is not installed"
        echo "Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        exit 1
    fi
    
    if ! command -v protoc-gen-go-grpc &> /dev/null; then
        echo "Error: protoc-gen-go-grpc is not installed"
        echo "Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        exit 1
    fi
    
    for file in $PROTO_FILES; do
        filename=$(basename "$file")
        echo "  Processing: $filename"
        
        protoc \
            --proto_path="$PROTO_DIR" \
            --go_out="$GO_OUT" \
            --go_opt=paths=source_relative \
            --go-grpc_out="$GO_OUT" \
            --go-grpc_opt=paths=source_relative \
            "$file"
    done
    
    echo "  ✓ Go stubs generated"
    echo ""
fi

# Generate Python stubs
if [[ "$GEN_PYTHON" == true ]]; then
    echo "Generating Python stubs..."
    echo "  Output: $PYTHON_OUT"
    
    # Check for Python grpc_tools
    if ! python -c "import grpc_tools" 2>/dev/null; then
        echo "Error: grpcio-tools is not installed"
        echo "Install with: pip install grpcio-tools"
        exit 1
    fi
    
    for file in $PROTO_FILES; do
        filename=$(basename "$file")
        echo "  Processing: $filename"
        
        python -m grpc_tools.protoc \
            --proto_path="$PROTO_DIR" \
            --python_out="$PYTHON_OUT" \
            --grpc_python_out="$PYTHON_OUT" \
            "$file"
    done
    
    # Fix Python imports (relative imports in generated files)
    echo "  Fixing Python imports..."
    for file in "$PYTHON_OUT"/*_pb2*.py; do
        if [[ -f "$file" ]]; then
            # Replace absolute imports with relative imports
            sed -i '' 's/^import \(.*_pb2\)/from . import \1/g' "$file" 2>/dev/null || \
            sed -i 's/^import \(.*_pb2\)/from . import \1/g' "$file" 2>/dev/null || true
        fi
    done
    
    echo "  ✓ Python stubs generated"
    echo ""
fi

echo "========================================"
echo "Protocol buffer generation complete!"
echo "========================================"

# Print summary
if [[ "$GEN_GO" == true ]]; then
    echo ""
    echo "Go stubs location:"
    ls -la "$GO_OUT"/*.pb.go 2>/dev/null || echo "  (no files)"
fi

if [[ "$GEN_PYTHON" == true ]]; then
    echo ""
    echo "Python stubs location:"
    ls -la "$PYTHON_OUT"/*_pb2*.py 2>/dev/null || echo "  (no files)"
fi
