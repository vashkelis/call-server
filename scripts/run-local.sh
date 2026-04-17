#!/bin/bash
#
# CloudApp Local Development Runner
# One-command script to start the full stack in mock mode
#
# Usage:
#   ./scripts/run-local.sh          # Start with mock providers
#   ./scripts/run-local.sh --vllm   # Start with vLLM configuration
#   ./scripts/run-local.sh --cloud  # Start with cloud providers (requires API keys)
#   ./scripts/run-local.sh --build  # Force rebuild of images
#
# To make this script executable:
#   chmod +x scripts/run-local.sh

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Default environment file
ENV_FILE="${PROJECT_ROOT}/infra/compose/.env.mock"
COMPOSE_FILE="${PROJECT_ROOT}/infra/compose/docker-compose.yml"
BUILD_FLAG=""

# Parse arguments
for arg in "$@"; do
    case $arg in
        --vllm)
            ENV_FILE="${PROJECT_ROOT}/infra/compose/.env.vllm"
            echo "Using vLLM configuration"
            shift
            ;;
        --cloud)
            ENV_FILE="${PROJECT_ROOT}/infra/compose/.env.cloud"
            echo "Using cloud providers configuration"
            echo "Make sure GROQ_API_KEY is set in your environment!"
            shift
            ;;
        --build)
            BUILD_FLAG="--build"
            echo "Will force rebuild of images"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --vllm    Use vLLM configuration for local LLM inference"
            echo "  --cloud   Use cloud providers (Google Speech, Groq)"
            echo "  --build   Force rebuild of Docker images"
            echo "  --help    Show this help message"
            exit 0
            ;;
    esac
done

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed"
    exit 1
fi

# Check if environment file exists
if [[ ! -f "$ENV_FILE" ]]; then
    echo "Error: Environment file not found: $ENV_FILE"
    exit 1
fi

echo "========================================"
echo "CloudApp Voice Engine - Local Development"
echo "========================================"
echo ""
echo "Compose file: $COMPOSE_FILE"
echo "Environment:  $ENV_FILE"
echo ""

# Check if already running
if docker-compose -f "$COMPOSE_FILE" ps | grep -q "Up"; then
    echo "Services are already running. Stopping first..."
    docker-compose -f "$COMPOSE_FILE" down
    echo ""
fi

echo "Starting services..."
echo ""

# Start the stack
docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up $BUILD_FLAG

# Cleanup on exit
echo ""
echo "Shutting down..."
docker-compose -f "$COMPOSE_FILE" down
