# Testing Guide

## Overview

This guide covers testing strategies for CloudApp, including unit tests, integration tests, and manual testing approaches.

## Go Unit Tests

### Running All Tests

```bash
cd go

# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Running Specific Package Tests

```bash
# Session package tests
go test -v ./pkg/session/...

# Events package tests
go test -v ./pkg/events/...

# Audio package tests
go test -v ./pkg/audio/...

# Media-edge handler tests
go test -v ./media-edge/internal/handler/...

# Orchestrator pipeline tests
go test -v ./orchestrator/internal/pipeline/...
```

### Test Configuration

Some tests may require environment variables:

```bash
# Set test Redis (uses mock if not set)
export CLOUDAPP_TEST_REDIS_ADDR=localhost:6379

# Set test PostgreSQL (uses mock if not set)
export CLOUDAPP_TEST_POSTGRES_DSN="postgres://test:test@localhost:5432/test?sslmode=disable"

# Run tests
go test ./...
```

### Writing Go Tests

Example test structure:

```go
// pkg/session/session_test.go
package session

import (
    "testing"
    "time"
)

func TestNewSession(t *testing.T) {
    sess := NewSession("test-id", "trace-123", TransportTypeWebSocket)

    if sess.SessionID != "test-id" {
        t.Errorf("expected session ID 'test-id', got %s", sess.SessionID)
    }

    if sess.CurrentState != StateIdle {
        t.Errorf("expected initial state Idle, got %s", sess.CurrentState)
    }
}

func TestSessionStateTransitions(t *testing.T) {
    sess := NewSession("test-id", "trace-123", TransportTypeWebSocket)

    // Valid transition: Idle -> Listening
    err := sess.SetState(StateListening)
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    // Invalid transition: Listening -> Idle (not in validTransitions)
    err = sess.SetState(StateIdle)
    if err == nil {
        t.Error("expected error for invalid transition")
    }
}
```

## Python Unit Tests

### Running All Tests

```bash
cd py/provider_gateway

# Install test dependencies
pip install -r requirements.txt
pip install pytest pytest-asyncio

# Run all tests
python -m pytest

# Run with verbose output
python -m pytest -v

# Run with coverage
python -m pytest --cov=app --cov-report=html

# Run specific test file
python -m pytest app/tests/test_registry.py -v
```

### Running Specific Test Categories

```bash
# Provider tests
python -m pytest app/tests/test_mock_providers.py -v

# Registry tests
python -m pytest app/tests/test_registry.py -v

# Config tests
python -m pytest app/tests/test_config.py -v
```

### Writing Python Tests

Example test structure:

```python
# app/tests/test_my_provider.py
import pytest
from app.providers.asr.my_provider import create_my_asr_provider
from app.models.asr import ASROptions


@pytest.fixture
def provider():
    return create_my_asr_provider(api_key="test-key")


@pytest.mark.asyncio
async def test_provider_name(provider):
    assert provider.name() == "my_provider"


@pytest.mark.asyncio
async def test_provider_capabilities(provider):
    caps = provider.capabilities()
    assert caps.supports_streaming_input is True
    assert caps.supports_streaming_output is True


@pytest.mark.asyncio
async def test_stream_recognize(provider):
    async def mock_audio_stream():
        yield b"fake_audio_chunk_1"
        yield b"fake_audio_chunk_2"

    results = []
    async for response in provider.stream_recognize(mock_audio_stream()):
        results.append(response)

    assert len(results) > 0
    assert results[-1].is_final is True
```

## Running with Mock Providers

Mock providers are useful for testing without external API dependencies.

### Docker Compose with Mocks

```bash
cd infra/compose

# Start with mock configuration
docker-compose --env-file .env.mock up -d

# Verify services are healthy
docker-compose ps

# Check logs
docker-compose logs -f media-edge
docker-compose logs -f orchestrator
docker-compose logs -f provider-gateway
```

### Local Development with Mocks

```bash
# Terminal 1: Start Redis
docker run -p 6379:6379 redis:7-alpine

# Terminal 2: Start provider-gateway with mock config
cd py/provider_gateway
export PROVIDER_GATEWAY_PROVIDERS__ASR_DEFAULT=mock
export PROVIDER_GATEWAY_PROVIDERS__LLM_DEFAULT=mock
export PROVIDER_GATEWAY_PROVIDERS__TTS_DEFAULT=mock
python main.py

# Terminal 3: Start orchestrator
cd go/orchestrator
go run cmd/main.go --config ../../examples/config-mock.yaml

# Terminal 4: Start media-edge
cd go/media-edge
go run cmd/main.go --config ../../examples/config-mock.yaml
```

## Integration Testing

### WebSocket Client Test

Use the provided WebSocket client for end-to-end testing:

```bash
# Install dependencies
pip install websockets

# Run with synthetic audio
python scripts/ws-client.py \
  --server ws://localhost:8080/ws \
  --synthetic-duration 5

# Run with WAV file
python scripts/ws-client.py \
  --server ws://localhost:8080/ws \
  --audio-file test-audio.wav

# Run with custom session ID
python scripts/ws-client.py \
  --server ws://localhost:8080/ws \
  --session-id test-session-001 \
  --synthetic-duration 3
```

### Session Simulator

```bash
# Run session simulation
python scripts/simulate-session.py \
  --server ws://localhost:8080/ws \
  --num-turns 5 \
  --delay 2
```

### Manual Testing Checklist

- [ ] Connect to WebSocket endpoint
- [ ] Send `session.start` message
- [ ] Receive `session.started` confirmation
- [ ] Send audio chunks
- [ ] Receive `vad.event` (speech_start)
- [ ] Receive `asr.partial` transcripts
- [ ] Receive `vad.event` (speech_end)
- [ ] Receive `asr.final` transcript
- [ ] Receive `turn.event` (assistant started)
- [ ] Receive `llm.partial_text` tokens
- [ ] Receive `tts.audio_chunk` audio
- [ ] Send `session.interrupt` during TTS
- [ ] Receive `interruption.event`
- [ ] Send `session.stop`
- [ ] Receive `session.ended`

## Test Coverage Expectations

### Minimum Coverage Targets

| Package | Target Coverage |
|---------|-----------------|
| `pkg/session` | 80% |
| `pkg/events` | 75% |
| `pkg/audio` | 70% |
| `pkg/config` | 75% |
| `pkg/providers` | 70% |
| `media-edge/internal/handler` | 70% |
| `orchestrator/internal/pipeline` | 75% |
| `orchestrator/internal/statemachine` | 80% |
| `py/provider_gateway/app/core` | 80% |
| `py/provider_gateway/app/providers` | 70% |

### Coverage Exclusions

The following are typically excluded from coverage:
- Generated code (protobuf)
- Main entry points (`cmd/main.go`)
- Integration test helpers
- Debug utilities

## Load Testing

### WebSocket Load Test

```bash
# Install k6
brew install k6  # macOS

# Run load test
k6 run --vus 100 --duration 5m scripts/load-test.js
```

Example k6 script:

```javascript
// scripts/load-test.js
import ws from 'k6/ws';
import { check } from 'k6';

export default function () {
    const url = 'ws://localhost:8080/ws';
    const params = { tags: { my_tag: 'load_test' } };

    const res = ws.connect(url, params, function (socket) {
        socket.on('open', function () {
            // Send session start
            socket.send(JSON.stringify({
                type: 'session.start',
                audio_profile: {
                    sample_rate: 16000,
                    channels: 1,
                    encoding: 'pcm16'
                }
            }));
        });

        socket.on('message', function (msg) {
            const data = JSON.parse(msg);
            if (data.type === 'session.started') {
                // Send synthetic audio
                const audioData = generateSyntheticAudio();
                socket.send(JSON.stringify({
                    type: 'audio.chunk',
                    audio_data: audioData
                }));
            }
        });

        socket.setTimeout(function () {
            socket.send(JSON.stringify({
                type: 'session.stop'
            }));
            socket.close();
        }, 10000);
    });

    check(res, { 'status is 101': (r) => r && r.status === 101 });
}
```

## Performance Testing

### Latency Benchmarks

```bash
# Run latency benchmark
python scripts/benchmark-latency.py \
  --server ws://localhost:8080/ws \
  --iterations 100 \
  --output results.json
```

### Target Performance Metrics

| Metric | Target | Acceptable |
|--------|--------|------------|
| WebSocket connection | < 100ms | < 500ms |
| VAD speech detection | < 100ms | < 200ms |
| ASR first partial | < 500ms | < 1000ms |
| ASR final transcript | < 200ms after speech end | < 500ms |
| LLM time to first token | < 500ms | < 1000ms |
| TTS time to first chunk | < 200ms | < 500ms |
| End-to-end response | < 1500ms | < 3000ms |
| Interruption latency | < 300ms | < 500ms |

## Continuous Integration

### GitHub Actions Example

```yaml
# .github/workflows/test.yml
name: Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  go-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Run Go tests
        run: |
          cd go
          go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./go/coverage.out

  python-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          cd py/provider_gateway
          pip install -r requirements.txt
          pip install pytest pytest-asyncio pytest-cov

      - name: Run Python tests
        run: |
          cd py/provider_gateway
          python -m pytest --cov=app --cov-report=xml

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./py/provider_gateway/coverage.xml

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Start services
        run: |
          cd infra/compose
          docker-compose --env-file .env.mock up -d

      - name: Wait for services
        run: |
          sleep 30
          curl -f http://localhost:8080/health
          curl -f http://localhost:8081/health

      - name: Run integration tests
        run: |
          pip install websockets
          python scripts/ws-client.py --server ws://localhost:8080/ws --synthetic-duration 3

      - name: Stop services
        run: |
          cd infra/compose
          docker-compose down
```

## Debugging Tests

### Go Test Debugging

```bash
# Run specific test with debug output
go test -v -run TestSessionStateTransitions ./pkg/session/...

# Run with race detector
go test -race ./...

# Run with CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./pkg/session/...
go tool pprof cpu.prof

# Run with memory profiling
go test -memprofile=mem.prof -bench=. ./pkg/session/...
go tool pprof mem.prof
```

### Python Test Debugging

```bash
# Run with pdb
python -m pytest --pdb

# Run with detailed traceback
python -m pytest -v --tb=long

# Run specific test
python -m pytest app/tests/test_registry.py::TestRegistry::test_register_asr -v

# Run with asyncio debug
PYTHONASYNCIODEBUG=1 python -m pytest
```

## Test Data

### Sample Audio Files

```bash
# Generate test audio
ffmpeg -f lavfi -i "sine=frequency=1000:duration=5" -ar 16000 -ac 1 test-audio.wav

# Convert existing audio
ffmpeg -i input.mp3 -ar 16000 -ac 1 -acodec pcm_s16le output.wav
```

### Test Fixtures

Store test fixtures in:
- `go/pkg/*/testdata/` — Go test data
- `py/provider_gateway/app/tests/fixtures/` — Python test data

## Best Practices

1. **Mock External Dependencies**: Use mocks for external APIs (ASR, LLM, TTS)
2. **Table-Driven Tests**: Use table-driven tests for multiple test cases
3. **Parallel Tests**: Use `t.Parallel()` in Go for concurrent test execution
4. **Cleanup**: Always clean up resources in tests (defer in Go, fixtures in Python)
5. **Deterministic**: Tests should be deterministic and not depend on external state
6. **Fast**: Unit tests should complete in milliseconds
7. **Isolated**: Tests should not depend on each other
