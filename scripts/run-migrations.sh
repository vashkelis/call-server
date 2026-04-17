#!/bin/bash
#
# CloudApp Database Migration Runner
# Runs PostgreSQL migrations using psql
#
# Usage:
#   ./scripts/run-migrations.sh              # Run migrations on local database
#   ./scripts/run-migrations.sh --up         # Run up migrations (default)
#   ./scripts/run-migrations.sh --down       # Run down migrations (rollback)
#   ./scripts/run-migrations.sh --docker     # Run against Docker Compose postgres
#
# To make this script executable:
#   chmod +x scripts/run-migrations.sh

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Default settings
MIGRATIONS_DIR="${PROJECT_ROOT}/infra/migrations"
ACTION="up"
DOCKER_MODE=false

# Database connection defaults
DB_HOST="localhost"
DB_PORT="5432"
DB_NAME="voiceengine"
DB_USER="voiceengine"
DB_PASSWORD="voiceengine"

# Parse arguments
for arg in "$@"; do
    case $arg in
        --up)
            ACTION="up"
            shift
            ;;
        --down)
            ACTION="down"
            echo "WARNING: Running DOWN migrations will DELETE DATA!"
            read -p "Are you sure? (yes/no): " confirm
            if [[ "$confirm" != "yes" ]]; then
                echo "Aborted."
                exit 0
            fi
            shift
            ;;
        --docker)
            DOCKER_MODE=true
            shift
            ;;
        --host)
            DB_HOST="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --up       Run up migrations (default)"
            echo "  --down     Run down migrations (rollback)"
            echo "  --docker   Run against Docker Compose postgres"
            echo "  --host     Database host (default: localhost)"
            echo "  --help     Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  DATABASE_URL    Full PostgreSQL connection string"
            exit 0
            ;;
    esac
done

# Check if DATABASE_URL is set
if [[ -n "$DATABASE_URL" ]]; then
    echo "Using DATABASE_URL from environment"
    DSN="$DATABASE_URL"
elif [[ "$DOCKER_MODE" == true ]]; then
    # Check if postgres container is running
    if ! docker ps | grep -q "cloudapp-postgres"; then
        echo "Error: Postgres container is not running"
        echo "Start it with: docker-compose -f infra/compose/docker-compose.yml up -d postgres"
        exit 1
    fi
    DSN="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
else
    DSN="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
fi

echo "========================================"
echo "CloudApp Database Migrations"
echo "========================================"
echo ""
echo "Action:      $ACTION"
echo "Migrations:  $MIGRATIONS_DIR"
echo ""

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo "Error: psql is not installed"
    echo "Install PostgreSQL client: https://www.postgresql.org/download/"
    exit 1
fi

# Find migration files
if [[ "$ACTION" == "up" ]]; then
    MIGRATION_FILES=$(find "$MIGRATIONS_DIR" -name "*.up.sql" | sort)
else
    MIGRATION_FILES=$(find "$MIGRATIONS_DIR" -name "*.down.sql" | sort -r)
fi

if [[ -z "$MIGRATION_FILES" ]]; then
    echo "No migration files found!"
    exit 1
fi

echo "Found migration files:"
echo "$MIGRATION_FILES"
echo ""

# Run migrations
for file in $MIGRATION_FILES; do
    filename=$(basename "$file")
    echo "Running: $filename"
    
    if PGPASSWORD="$DB_PASSWORD" psql "$DSN" -f "$file" -v ON_ERROR_STOP=1; then
        echo "  ✓ Success"
    else
        echo "  ✗ Failed"
        exit 1
    fi
done

echo ""
echo "========================================"
echo "Migrations completed successfully!"
echo "========================================"
