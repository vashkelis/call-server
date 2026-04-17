-- Initial schema for CloudApp Voice Engine
-- Creates tables for sessions, transcripts, events, and provider configuration

-- Sessions table: Stores active and completed voice sessions
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255),
    transport_type VARCHAR(50) NOT NULL,
    asr_provider VARCHAR(100),
    llm_provider VARCHAR(100),
    tts_provider VARCHAR(100),
    system_prompt TEXT,
    audio_profile JSONB,
    voice_profile JSONB,
    model_options JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'idle',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);

-- Transcripts table: Stores conversation history
CREATE TABLE IF NOT EXISTS transcripts (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    turn_index INTEGER NOT NULL,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    generation_id VARCHAR(255),
    was_interrupted BOOLEAN DEFAULT FALSE,
    spoken_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Session events table: Stores lifecycle and debug events
CREATE TABLE IF NOT EXISTS session_events (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Provider configuration table: Stores tenant-specific provider settings
CREATE TABLE IF NOT EXISTS provider_config (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(255),
    provider_type VARCHAR(50) NOT NULL,
    provider_name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    priority INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, provider_type, provider_name)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_transcripts_session ON transcripts(session_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_session_turn ON transcripts(session_id, turn_index);
CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id);
CREATE INDEX IF NOT EXISTS idx_session_events_type ON session_events(event_type);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_provider_config_tenant ON provider_config(tenant_id, provider_type);
CREATE INDEX IF NOT EXISTS idx_provider_config_active ON provider_config(active);

-- Comments for documentation
COMMENT ON TABLE sessions IS 'Voice conversation sessions';
COMMENT ON TABLE transcripts IS 'Conversation transcripts per session';
COMMENT ON TABLE session_events IS 'Session lifecycle and debug events';
COMMENT ON TABLE provider_config IS 'Tenant-specific provider configurations';
