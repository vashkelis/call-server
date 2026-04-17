// Package main provides the entry point for the media-edge WebSocket service.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/parlona/cloudapp/media-edge/internal/handler"
	"github.com/parlona/cloudapp/pkg/config"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/session"
)

var (
	configPath = flag.String("config", "", "Path to configuration file (YAML)")
	version    = "dev"
	commit     = "unknown"
	date       = "unknown"
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := observability.NewLogger(observability.LoggerConfig{
		Level:  cfg.Observability.LogLevel,
		Format: cfg.Observability.LogFormat,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.WithFields(map[string]interface{}{
		"version": version,
		"commit":  commit,
		"date":    date,
	}).Info("Starting media-edge service")

	// Initialize tracer (optional)
	var tracer *observability.Tracer
	if cfg.Observability.EnableTracing {
		tracer, err = observability.NewTracer(observability.TracerConfig{
			ServiceName:    "media-edge",
			ServiceVersion: version,
			Endpoint:       cfg.Observability.OTelEndpoint,
			Enabled:        true,
		})
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize tracer")
		} else {
			defer tracer.Shutdown(context.Background())
		}
	}

	// Create session store (Redis)
	sessionStore, err := createSessionStore(cfg)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create session store")
	}
	defer sessionStore.Close()

	// Create orchestrator bridge (in-process for MVP)
	bridge := handler.NewChannelBridge()
	defer bridge.Close()

	// Create WebSocket handler
	wsHandler := handler.NewWebSocketHandler(handler.WebSocketHandlerConfig{
		SessionStore: sessionStore,
		Bridge:       bridge,
		Logger:       logger,
		Config:       cfg,
	})
	defer wsHandler.Close()

	// Create HTTP mux
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.Handle(cfg.Server.WSPath, wsHandler)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Readiness endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Check if service is ready
		ready := true

		// TODO: Add dependency checks (Redis, etc.)

		w.Header().Set("Content-Type", "application/json")
		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ready"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "not_ready"}`))
		}
	})

	// Metrics endpoint
	if cfg.Observability.EnableMetrics {
		mux.Handle("/metrics", promhttp.Handler())
	}

	// Create server with middleware chain
	chain := handler.Chain(
		handler.RecoveryMiddleware(logger),
		handler.LoggingMiddleware(logger),
		handler.MetricsMiddleware(),
		handler.SecurityHeadersMiddleware(),
		handler.CORSMiddleware(cfg.Security.AllowedOrigins),
		handler.AuthMiddleware(cfg.Security.AuthEnabled, cfg.Security.AuthToken, logger),
	)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      chain(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.WithField("addr", server.Addr).Info("HTTP server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.WithField("signal", sig.String()).Info("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Close WebSocket handler (drains connections)
	if err := wsHandler.Close(); err != nil {
		logger.WithError(err).Warn("Failed to close WebSocket handler")
	}

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Warn("HTTP server shutdown error")
	}

	// Close session store
	if err := sessionStore.Close(); err != nil {
		logger.WithError(err).Warn("Failed to close session store")
	}

	logger.Info("Service stopped gracefully")
}

// loadConfig loads configuration from file or uses defaults.
func loadConfig(path string) (*config.AppConfig, error) {
	if path == "" {
		// Check environment variable
		path = os.Getenv("CONFIG_PATH")
	}

	if path == "" {
		// Use default config
		cfg := config.DefaultConfig()
		return cfg, cfg.Validate()
	}

	// Load from file
	// TODO: Implement YAML config loading when config/loader.go is available
	cfg := config.DefaultConfig()
	return cfg, cfg.Validate()
}

// createSessionStore creates a session store based on configuration.
func createSessionStore(cfg *config.AppConfig) (session.SessionStore, error) {
	// For MVP, use a simple in-memory store or Redis if available
	// TODO: Implement Redis store when session/redis_store.go is available

	// For now, return a placeholder that implements the interface
	// This should be replaced with actual Redis store implementation
	return &placeholderSessionStore{}, nil
}

// placeholderSessionStore is a temporary in-memory session store for MVP.
type placeholderSessionStore struct {
	sessions map[string]*session.Session
	mu       sync.RWMutex
}

func (s *placeholderSessionStore) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sessions == nil {
		return nil, session.ErrSessionNotFound
	}

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}

	return sess.Clone(), nil
}

func (s *placeholderSessionStore) Save(ctx context.Context, sess *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions == nil {
		s.sessions = make(map[string]*session.Session)
	}

	s.sessions[sess.SessionID] = sess.Clone()
	return nil
}

func (s *placeholderSessionStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions == nil {
		return nil
	}

	delete(s.sessions, sessionID)
	return nil
}

func (s *placeholderSessionStore) UpdateTurn(ctx context.Context, sessionID string, turn *session.AssistantTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions == nil {
		return session.ErrSessionNotFound
	}

	sess, ok := s.sessions[sessionID]
	if !ok {
		return session.ErrSessionNotFound
	}

	sess.SetActiveTurn(turn)
	return nil
}

func (s *placeholderSessionStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sessions == nil {
		return []string{}, nil
	}

	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}

	return ids, nil
}

func (s *placeholderSessionStore) Close() error {
	return nil
}
