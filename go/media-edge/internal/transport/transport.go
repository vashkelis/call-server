// Package transport provides transport abstractions for media-edge communication.
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/parlona/cloudapp/pkg/events"
)

// Transport defines the interface for media transport (WebSocket, SIP, WebRTC, etc.)
type Transport interface {
	// SendEvent sends a JSON event to the client.
	SendEvent(event events.Event) error

	// SendAudio sends binary audio data to the client.
	SendAudio(data []byte) error

	// ReceiveMessage receives a raw message from the client.
	// Returns the message bytes and any error.
	ReceiveMessage() ([]byte, error)

	// Close closes the transport connection.
	Close() error

	// IsClosed returns true if the transport is closed.
	IsClosed() bool

	// SetReadDeadline sets the read deadline.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the write deadline.
	SetWriteDeadline(t time.Time) error

	// RemoteAddr returns the remote address.
	RemoteAddr() string
}

// WebSocketTransport implements Transport for WebSocket connections.
type WebSocketTransport struct {
	conn       *websocket.Conn
	mu         sync.RWMutex
	closed     bool
	closeCh    chan struct{}
	writeCh    chan []byte
	remoteAddr string
}

// WebSocketConfig contains WebSocket transport configuration.
type WebSocketConfig struct {
	WriteBufferSize int
	ReadBufferSize  int
	WriteTimeout    time.Duration
	PingInterval    time.Duration
}

// DefaultWebSocketConfig returns default WebSocket configuration.
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		WriteBufferSize: 1024,
		ReadBufferSize:  1024,
		WriteTimeout:    10 * time.Second,
		PingInterval:    30 * time.Second,
	}
}

// NewWebSocketTransport creates a new WebSocket transport.
func NewWebSocketTransport(conn *websocket.Conn) *WebSocketTransport {
	return &WebSocketTransport{
		conn:       conn,
		closeCh:    make(chan struct{}),
		writeCh:    make(chan []byte, 100),
		remoteAddr: conn.RemoteAddr().String(),
	}
}

// SendEvent sends a JSON event to the client.
func (t *WebSocketTransport) SendEvent(event events.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return t.sendMessage(websocket.TextMessage, data)
}

// SendAudio sends binary audio data to the client.
func (t *WebSocketTransport) SendAudio(data []byte) error {
	return t.sendMessage(websocket.BinaryMessage, data)
}

// sendMessage sends a WebSocket message with the given type and data.
func (t *WebSocketTransport) sendMessage(messageType int, data []byte) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	t.mu.RUnlock()

	// Use a select to prevent blocking on write channel
	select {
	case t.writeCh <- append([]byte{byte(messageType)}, data...):
		return nil
	case <-t.closeCh:
		return fmt.Errorf("transport is closing")
	default:
		// Channel full, drop the message
		return fmt.Errorf("write channel full, message dropped")
	}
}

// StartWritePump starts the write pump goroutine.
// This should be called once after creating the transport.
func (t *WebSocketTransport) StartWritePump() {
	go t.writePump()
}

// writePump handles writing messages to the WebSocket.
func (t *WebSocketTransport) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-t.writeCh:
			if !ok {
				t.writeClose()
				return
			}

			if len(message) == 0 {
				continue
			}

			messageType := int(message[0])
			data := message[1:]

			t.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := t.conn.WriteMessage(messageType, data); err != nil {
				// Log error but continue
				return
			}

		case <-ticker.C:
			t.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := t.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-t.closeCh:
			t.writeClose()
			return
		}
	}
}

// writeClose sends a close message and closes the connection.
func (t *WebSocketTransport) writeClose() {
	t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	t.conn.Close()
}

// ReceiveMessage receives a raw message from the client.
func (t *WebSocketTransport) ReceiveMessage() ([]byte, error) {
	messageType, data, err := t.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
			return nil, fmt.Errorf("websocket error: %w", err)
		}
		return nil, err
	}

	// Prepend message type for the caller to distinguish
	result := make([]byte, len(data)+1)
	result[0] = byte(messageType)
	copy(result[1:], data)

	return result, nil
}

// Close closes the transport connection.
func (t *WebSocketTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()

	close(t.closeCh)

	// Drain write channel
	close(t.writeCh)

	return nil
}

// IsClosed returns true if the transport is closed.
func (t *WebSocketTransport) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// SetReadDeadline sets the read deadline.
func (t *WebSocketTransport) SetReadDeadline(deadline time.Time) error {
	return t.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline sets the write deadline.
func (t *WebSocketTransport) SetWriteDeadline(deadline time.Time) error {
	return t.conn.SetWriteDeadline(deadline)
}

// RemoteAddr returns the remote address.
func (t *WebSocketTransport) RemoteAddr() string {
	return t.remoteAddr
}

// Conn returns the underlying WebSocket connection.
func (t *WebSocketTransport) Conn() *websocket.Conn {
	return t.conn
}

// Upgrader is the WebSocket upgrader with default settings.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins by default - configure in production
		return true
	},
}

// ConfigureUpgrader configures the WebSocket upgrader with custom settings.
func ConfigureUpgrader(readBufferSize, writeBufferSize int, checkOrigin func(r *http.Request) bool) {
	Upgrader.ReadBufferSize = readBufferSize
	Upgrader.WriteBufferSize = writeBufferSize
	if checkOrigin != nil {
		Upgrader.CheckOrigin = checkOrigin
	}
}

// Upgrade upgrades an HTTP connection to WebSocket.
func Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*WebSocketTransport, error) {
	conn, err := Upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade connection: %w", err)
	}

	transport := NewWebSocketTransport(conn)
	transport.StartWritePump()

	return transport, nil
}

// MessageType represents WebSocket message types.
type MessageType int

const (
	// MessageText is a text message (JSON events).
	MessageText MessageType = websocket.TextMessage
	// MessageBinary is a binary message (audio data).
	MessageBinary MessageType = websocket.BinaryMessage
	// MessageClose is a close message.
	MessageClose MessageType = websocket.CloseMessage
	// MessagePing is a ping message.
	MessagePing MessageType = websocket.PingMessage
	// MessagePong is a pong message.
	MessagePong MessageType = websocket.PongMessage
)

// TransportFactory creates transports for different protocols.
type TransportFactory struct {
	upgrader websocket.Upgrader
}

// NewTransportFactory creates a new transport factory.
func NewTransportFactory() *TransportFactory {
	return &TransportFactory{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// CreateWebSocketTransport creates a WebSocket transport from an HTTP request.
func (f *TransportFactory) CreateWebSocketTransport(w http.ResponseWriter, r *http.Request, checkOrigin func(r *http.Request) bool) (Transport, error) {
	if checkOrigin != nil {
		f.upgrader.CheckOrigin = checkOrigin
	}

	conn, err := f.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade: %w", err)
	}

	transport := NewWebSocketTransport(conn)
	transport.StartWritePump()

	return transport, nil
}

// ContextKey is the type for context keys.
type ContextKey string

const (
	// TransportContextKey is the context key for the transport.
	TransportContextKey ContextKey = "transport"
)

// ContextWithTransport adds a transport to the context.
func ContextWithTransport(ctx context.Context, transport Transport) context.Context {
	return context.WithValue(ctx, TransportContextKey, transport)
}

// TransportFromContext retrieves the transport from context.
func TransportFromContext(ctx context.Context) (Transport, bool) {
	transport, ok := ctx.Value(TransportContextKey).(Transport)
	return transport, ok
}
