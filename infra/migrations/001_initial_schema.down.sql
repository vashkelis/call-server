-- Rollback script for initial schema
-- Drops all tables created by 001_initial_schema.up.sql

DROP TABLE IF EXISTS session_events;
DROP TABLE IF EXISTS transcripts;
DROP TABLE IF EXISTS provider_config;
DROP TABLE IF EXISTS sessions;
