package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/parlona/cloudapp/media-edge/internal/transport"
	"github.com/parlona/cloudapp/media-edge/internal/vad"
	"github.com/parlona/cloudapp/pkg/audio"
	"github.com/parlona/cloudapp/pkg/config"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/events"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/session"
)

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	upgrader     websocket.Upgrader
	sessionStore session.SessionStore
	bridge       OrchestratorBridge
	logger       *observability.Logger
	config       *config.AppConfig

	// Active connections
	connections map[string]*Connection
	mu          sync.RWMutex

	// Metrics
	metrics *observability.MetricsCollector
}

// Connection represents a WebSocket connection.
type Connection struct {
	id        string
	sessionID string
	conn      *websocket.Conn
	transport transport.Transport
	handler   *SessionHandler
	ctx       context.Context
	cancel    context.CancelFunc
	writeCh   chan []byte
	logger    *observability.Logger

	// Connection state
	mu           sync.RWMutex
	closed       bool
	lastActivity time.Time
}

// WebSocketHandlerConfig contains configuration for the WebSocket handler.
type WebSocketHandlerConfig struct {
	SessionStore session.SessionStore
	Bridge       OrchestratorBridge
	Logger       *observability.Logger
	Config       *config.AppConfig
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(cfg WebSocketHandlerConfig) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.Config.Security.MaxChunkSize,
			WriteBufferSize: cfg.Config.Security.MaxChunkSize,
			CheckOrigin: func(r *http.Request) bool {
				// Check allowed origins
				origin := r.Header.Get("Origin")
				if len(cfg.Config.Security.AllowedOrigins) == 0 ||
					(len(cfg.Config.Security.AllowedOrigins) == 1 && cfg.Config.Security.AllowedOrigins[0] == "*") {
					return true
				}
				for _, o := range cfg.Config.Security.AllowedOrigins {
					if o == origin || o == "*" {
						return true
					}
				}
				return false
			},
		},
		sessionStore: cfg.SessionStore,
		bridge:       cfg.Bridge,
		logger:       cfg.Logger,
		config:       cfg.Config,
		connections:  make(map[string]*Connection),
		metrics:      observability.NewMetricsCollector("media_edge"),
	}
}

// ServeHTTP handles WebSocket upgrade requests.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to upgrade WebSocket")
		return
	}

	// Create connection
	connectionID := generateConnectionID()
	ctx, cancel := context.WithCancel(r.Context())

	c := &Connection{
		id:           connectionID,
		conn:         conn,
		ctx:          ctx,
		cancel:       cancel,
		writeCh:      make(chan []byte, 100),
		lastActivity: time.Now(),
		logger:       h.logger.WithField("connection_id", connectionID),
	}

	// Register connection
	h.mu.Lock()
	h.connections[connectionID] = c
	h.mu.Unlock()

	// Update metrics
	observability.RecordWebSocketConnectionActive()

	h.logger.WithField("connection_id", connectionID).Info("WebSocket connection established")

	// Handle connection
	go h.handleConnection(c)
}

// handleConnection handles a WebSocket connection lifecycle.
func (h *WebSocketHandler) handleConnection(c *Connection) {
	defer h.cleanupConnection(c)

	// Set read deadline
	c.conn.SetReadDeadline(time.Now().Add(h.config.Server.ReadTimeout))

	// Set pong handler
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(h.config.Server.ReadTimeout))
		c.mu.Lock()
		c.lastActivity = time.Now()
		c.mu.Unlock()
		return nil
	})

	// Start write pump
	go h.writePump(c)

	// Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Message loop
	for {
		select {
		case <-c.ctx.Done():
			return

		case <-pingTicker.C:
			// Send ping
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				c.logger.WithError(err).Debug("Failed to send ping")
				return
			}

		default:
			// Read message
			c.conn.SetReadDeadline(time.Now().Add(h.config.Server.ReadTimeout))
			messageType, data, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.WithError(err).Debug("WebSocket error")
				}
				return
			}

			c.mu.Lock()
			c.lastActivity = time.Now()
			c.mu.Unlock()

			// Handle message
			if err := h.handleMessage(c, messageType, data); err != nil {
				c.logger.WithError(err).Warn("Failed to handle message")

				// Send error to client
				errorEvent := events.NewErrorEvent(c.sessionID, "MESSAGE_ERROR", err.Error())
				h.sendEvent(c, errorEvent)
			}
		}
	}
}

// writePump handles writing messages to the WebSocket.
func (h *WebSocketHandler) writePump(c *Connection) {
	defer func() {
		c.cancel()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return

		case data, ok := <-c.writeCh:
			if !ok {
				c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(5*time.Second))
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(h.config.Server.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.logger.WithError(err).Debug("Failed to write message")
				return
			}
		}
	}
}

// handleMessage handles an incoming WebSocket message.
func (h *WebSocketHandler) handleMessage(c *Connection, messageType int, data []byte) error {
	// Only accept text messages for JSON events
	if messageType != websocket.TextMessage {
		return fmt.Errorf("unsupported message type: %d", messageType)
	}

	// Check max message size
	if len(data) > h.config.Security.MaxChunkSize {
		return fmt.Errorf("message too large: %d bytes", len(data))
	}

	// Parse event
	event, err := events.ParseEvent(data)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// Handle event based on type
	switch e := event.(type) {
	case *events.SessionStartEvent:
		return h.handleSessionStart(c, e)

	case *events.AudioChunkEvent:
		return h.handleAudioChunk(c, e)

	case *events.SessionUpdateEvent:
		return h.handleSessionUpdate(c, e)

	case *events.SessionInterruptEvent:
		return h.handleSessionInterrupt(c, e)

	case *events.SessionStopEvent:
		return h.handleSessionStop(c, e)

	default:
		return fmt.Errorf("unknown event type: %s", event.GetType())
	}
}

// handleSessionStart handles a session.start event.
func (h *WebSocketHandler) handleSessionStart(c *Connection, e *events.SessionStartEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionID != "" {
		return fmt.Errorf("session already started")
	}

	// Generate session ID
	sessionID := generateSessionID()
	c.sessionID = sessionID

	// Create session
	traceID := generateTraceID()
	sess := session.NewSession(sessionID, traceID, session.TransportTypeWebSocket)

	if e.TenantID != "" {
		sess.SetTenantID(e.TenantID)
	}

	// Set audio profile
	audioProfile := contracts.AudioFormat{
		SampleRate: int32(e.AudioProfile.SampleRate),
		Channels:   int32(e.AudioProfile.Channels),
		Encoding:   contracts.AudioEncoding(audioProfileEncoding(e.AudioProfile.Encoding)),
	}
	sess.SetAudioProfile(audioProfile)

	// Set voice profile
	sess.SetVoiceProfile(session.VoiceProfile{
		VoiceID: e.VoiceProfile.VoiceID,
		Speed:   e.VoiceProfile.Speed,
		Pitch:   e.VoiceProfile.Pitch,
	})

	// Set model options
	sess.SetModelOptions(session.ModelOptions{
		ModelName:     e.ModelOptions.ModelName,
		MaxTokens:     e.ModelOptions.MaxTokens,
		Temperature:   e.ModelOptions.Temperature,
		TopP:          e.ModelOptions.TopP,
		StopSequences: e.ModelOptions.StopSequences,
		SystemPrompt:  e.SystemPrompt,
	})

	// Set providers
	sess.SetProviders(session.SelectedProviders{
		ASR: e.Providers.ASR,
		LLM: e.Providers.LLM,
		TTS: e.Providers.TTS,
		VAD: e.Providers.VAD,
	})

	// Save session
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.sessionStore.Save(ctx, sess); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Create session handler
	vadConfig := vad.DefaultVADConfig()
	vadConfig.SampleRate = int(audioProfile.SampleRate)

	profile := audio.AudioProfile{
		SampleRate: int(audioProfile.SampleRate),
		Channels:   int(audioProfile.Channels),
		Encoding:   audioProfile.Encoding,
	}

	sessionHandler, err := NewSessionHandler(SessionHandlerConfig{
		SessionID:    sessionID,
		Session:      sess,
		Transport:    transport.NewWebSocketTransport(c.conn),
		Bridge:       h.bridge,
		Logger:       h.logger,
		AudioProfile: profile,
		VADConfig:    vadConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create session handler: %w", err)
	}

	c.handler = sessionHandler

	// Start session handler
	if err := sessionHandler.Start(); err != nil {
		return fmt.Errorf("failed to start session handler: %w", err)
	}

	// Start bridge session
	bridgeConfig := SessionConfig{
		SessionID:    sessionID,
		TenantID:     e.TenantID,
		SystemPrompt: e.SystemPrompt,
		VoiceProfile: e.VoiceProfile,
		ModelOptions: e.ModelOptions,
		Providers:    e.Providers,
		AudioProfile: e.AudioProfile,
	}

	if err := h.bridge.StartSession(c.ctx, sessionID, bridgeConfig); err != nil {
		return fmt.Errorf("failed to start bridge session: %w", err)
	}

	// Send session.started event
	startedEvent := events.NewSessionStartedEvent(sessionID, e.AudioProfile)
	h.sendEvent(c, startedEvent)

	c.logger.WithField("session_id", sessionID).Info("Session started")

	return nil
}

// handleAudioChunk handles an audio.chunk event.
func (h *WebSocketHandler) handleAudioChunk(c *Connection, e *events.AudioChunkEvent) error {
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no active session")
	}

	// Decode audio data
	audioData, err := e.GetAudioData()
	if err != nil {
		return fmt.Errorf("failed to decode audio: %w", err)
	}

	// Get session audio profile
	c.mu.RLock()
	sess := c.handler.session
	c.mu.RUnlock()

	profile := audio.ProfileFromContract(sess.AudioProfile)

	// Process audio chunk
	if err := handler.ProcessAudioChunk(audioData, profile); err != nil {
		return fmt.Errorf("failed to process audio: %w", err)
	}

	return nil
}

// handleSessionUpdate handles a session.update event.
func (h *WebSocketHandler) handleSessionUpdate(c *Connection, e *events.SessionUpdateEvent) error {
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no active session")
	}

	// Update session configuration
	if err := handler.UpdateConfig(e.SystemPrompt, e.VoiceProfile, e.ModelOptions, e.Providers); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	c.logger.Info("Session updated")

	return nil
}

// handleSessionInterrupt handles a session.interrupt event.
func (h *WebSocketHandler) handleSessionInterrupt(c *Connection, e *events.SessionInterruptEvent) error {
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no active session")
	}

	// Handle interrupt
	if err := handler.HandleInterrupt(); err != nil {
		return fmt.Errorf("failed to handle interrupt: %w", err)
	}

	c.logger.Info("Session interrupted")

	return nil
}

// handleSessionStop handles a session.stop event.
func (h *WebSocketHandler) handleSessionStop(c *Connection, e *events.SessionStopEvent) error {
	c.mu.RLock()
	handler := c.handler
	sessionID := c.sessionID
	c.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no active session")
	}

	// Stop session handler
	if err := handler.Stop(); err != nil {
		c.logger.WithError(err).Warn("Failed to stop session handler")
	}

	// Delete session from store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.sessionStore.Delete(ctx, sessionID); err != nil {
		c.logger.WithError(err).Warn("Failed to delete session")
	}

	// Send session.ended event
	endedEvent := events.NewSessionEndedEvent(sessionID, e.Reason)
	h.sendEvent(c, endedEvent)

	c.logger.WithField("session_id", sessionID).Info("Session stopped")

	// Close connection
	c.cancel()

	return nil
}

// sendEvent sends an event to the client.
func (h *WebSocketHandler) sendEvent(c *Connection, event events.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	select {
	case c.writeCh <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("write channel full")
	}
}

// cleanupConnection cleans up a connection.
func (h *WebSocketHandler) cleanupConnection(c *Connection) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	handler := c.handler
	sessionID := c.sessionID
	c.mu.Unlock()

	// Stop handler
	if handler != nil {
		handler.Stop()
	}

	// Close connection
	c.conn.Close()

	// Unregister
	h.mu.Lock()
	delete(h.connections, c.id)
	h.mu.Unlock()

	// Update metrics
	observability.RecordWebSocketConnectionInactive()

	// Delete session if exists
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.sessionStore.Delete(ctx, sessionID)
	}

	c.logger.Info("Connection cleaned up")
}

// Close closes all connections.
func (h *WebSocketHandler) Close() error {
	h.mu.Lock()
	connections := make([]*Connection, 0, len(h.connections))
	for _, c := range h.connections {
		connections = append(connections, c)
	}
	h.mu.Unlock()

	// Close all connections
	for _, c := range connections {
		c.cancel()
	}

	return nil
}

// ActiveConnections returns the number of active connections.
func (h *WebSocketHandler) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// generateConnectionID generates a unique connection ID.
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d", time.Now().UnixNano())
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	return fmt.Sprintf("trace_%d", time.Now().UnixNano())
}

// audioProfileEncoding converts string encoding to AudioEncoding.
func audioProfileEncoding(encoding string) int32 {
	switch encoding {
	case "pcm16":
		return 1
	case "opus":
		return 2
	case "g711_ulaw":
		return 3
	case "g711_alaw":
		return 4
	default:
		return 1 // default to pcm16
	}
}
