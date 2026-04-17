// Package main provides the orchestrator service entry point.
// The orchestrator owns the session state machine and coordinates the ASR -> LLM -> TTS pipeline.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/parlona/cloudapp/orchestrator/internal/persistence"
	"github.com/parlona/cloudapp/orchestrator/internal/pipeline"
	"github.com/parlona/cloudapp/pkg/config"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
	"github.com/parlona/cloudapp/pkg/session"
)

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file (YAML)")
	flag.Parse()

	// If not provided via flag, check environment
	if configPath == "" {
		configPath = os.Getenv("CLOUDAPP_CONFIG")
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
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
		"version":     "0.1.0",
		"config_path": configPath,
	}).Info("Starting orchestrator service")

	// Initialize tracer (optional)
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:    "orchestrator",
		ServiceVersion: "0.1.0",
		Endpoint:       cfg.Observability.OTelEndpoint,
		Enabled:        cfg.Observability.EnableTracing,
	})
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize tracer, continuing without tracing")
	} else {
		defer tracer.Shutdown(context.Background())
	}

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.WithError(err).Fatal("Failed to connect to Redis")
	}
	logger.WithField("address", cfg.Redis.Address).Info("Connected to Redis")

	// Initialize Redis persistence
	redisPersistence := persistence.NewRedisPersistence(redisClient, cfg.Redis.KeyPrefix)

	// Initialize session store
	sessionStore := session.NewRedisSessionStore(redisClient,
		session.WithKeyPrefix(cfg.Redis.KeyPrefix+"session:"),
		session.WithTTL(3600),
	)

	// Initialize PostgreSQL persistence (stub for MVP)
	postgresPersistence := persistence.NewPostgresPersistence(logger)

	// Initialize provider registry
	providerRegistry := providers.NewProviderRegistry()

	// Register gRPC providers (connecting to provider-gateway)
	if err := registerProviders(providerRegistry, cfg, logger); err != nil {
		logger.WithError(err).Fatal("Failed to register providers")
	}

	// Initialize orchestrator engine
	engineConfig := pipeline.DefaultConfig()
	engineConfig.MaxSessionDuration = cfg.Security.MaxSessionDuration
	engineConfig.MaxContextMessages = 20

	_ = pipeline.NewEngine(
		providerRegistry,
		sessionStore,
		redisPersistence,
		postgresPersistence,
		engineConfig,
		logger,
	)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Readiness endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Check Redis connection
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","reason":"redis_unavailable"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.WithField("addr", server.Addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down orchestrator service...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("HTTP server shutdown failed")
	}

	// Close Redis connection
	if err := redisPersistence.Close(); err != nil {
		logger.WithError(err).Error("Failed to close Redis connection")
	}

	// Close PostgreSQL connection
	if err := postgresPersistence.Close(); err != nil {
		logger.WithError(err).Error("Failed to close PostgreSQL connection")
	}

	logger.Info("Orchestrator service stopped")
}

// registerProviders registers gRPC provider clients with the provider registry.
func registerProviders(
	registry *providers.ProviderRegistry,
	cfg *config.AppConfig,
	logger *observability.Logger,
) error {
	// Get provider-gateway address from config
	// For MVP, we assume a single provider-gateway at a configurable address
	gatewayAddress := os.Getenv("PROVIDER_GATEWAY_ADDRESS")
	if gatewayAddress == "" {
		gatewayAddress = "localhost:50051"
	}

	logger.WithField("address", gatewayAddress).Info("Connecting to provider gateway")

	// Register ASR provider
	asrConfig := providers.GRPCClientConfig{
		Address:    gatewayAddress,
		Timeout:    30,
		MaxRetries: 3,
	}
	asrProvider, err := providers.NewGRPCASRProvider("default", asrConfig)
	if err != nil {
		return fmt.Errorf("failed to create ASR provider: %w", err)
	}
	registry.RegisterASR("default", asrProvider)
	logger.WithField("provider", "asr").Info("Registered provider")

	// Register LLM provider
	llmConfig := providers.GRPCClientConfig{
		Address:    gatewayAddress,
		Timeout:    60,
		MaxRetries: 3,
	}
	llmProvider, err := providers.NewGRPCLLMProvider("default", llmConfig)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}
	registry.RegisterLLM("default", llmProvider)
	logger.WithField("provider", "llm").Info("Registered provider")

	// Register TTS provider
	ttsConfig := providers.GRPCClientConfig{
		Address:    gatewayAddress,
		Timeout:    30,
		MaxRetries: 3,
	}
	ttsProvider, err := providers.NewGRPCTTSProvider("default", ttsConfig)
	if err != nil {
		return fmt.Errorf("failed to create TTS provider: %w", err)
	}
	registry.RegisterTTS("default", ttsProvider)
	logger.WithField("provider", "tts").Info("Registered provider")

	// Set provider defaults
	registry.SetConfig(session.SelectedProviders{
		ASR: "default",
		LLM: "default",
		TTS: "default",
	}, nil)

	return nil
}
