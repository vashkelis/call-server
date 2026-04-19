package handler

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/parlona/cloudapp/pkg/observability"
)

// Middleware is a function that wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middleware together.
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(logger *observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// Log the request
			logger.WithFields(map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      wrapped.statusCode,
				"duration":    duration.Milliseconds(),
				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),
			}).Info("HTTP request")
		})
	}
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware(logger *observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic
					logger.WithFields(map[string]interface{}{
						"error":       fmt.Sprintf("%v", err),
						"stack":       string(debug.Stack()),
						"path":        r.URL.Path,
						"remote_addr": r.RemoteAddr,
					}).Error("Panic recovered")

					// Return 500
					http.Error(w, `{"error": "Internal Server Error"}`, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// MetricsMiddleware records request metrics.
func MetricsMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// Record metrics
			observability.RecordProviderRequest("media_edge", "http")
			observability.RecordProviderRequestDuration("media_edge", "http", duration)
		})
	}
}

// AuthMiddleware checks for API key authentication.
func AuthMiddleware(authEnabled bool, authToken string, logger *observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for health endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}

			// Check API key
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			if apiKey != authToken {
				logger.WithFields(map[string]interface{}{
					"remote_addr": r.RemoteAddr,
					"path":        r.URL.Path,
				}).Warn("Unauthorized request")

				http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware handles CORS headers for browser WebSocket clients.
func CORSMiddleware(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			if len(allowedOrigins) == 0 || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*") {
				allowed = true
			} else {
				for _, o := range allowedOrigins {
					if o == origin || o == "*" {
						allowed = true
						break
					}
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware adds a request ID to the context.
func RequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Add to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TimeoutMiddleware adds a timeout to the request context.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
// It preserves the http.Hijacker interface (required for WebSocket upgrades)
// by delegating Hijack to the underlying ResponseWriter when available.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the status code on first write.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements the http.Hijacker interface.
// This is required for WebSocket upgrades to work through middleware.
// If the underlying ResponseWriter does not implement http.Hijacker,
// this returns an error.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

// contextKey is the type for context keys.
type contextKey string

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
	// SessionIDKey is the context key for session ID.
	SessionIDKey contextKey = "session_id"
)

// generateRequestID generates a simple request ID.
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// RequestIDFromContext retrieves the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// SecurityHeadersMiddleware adds security headers to responses.
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// IPFilterMiddleware filters requests by IP address.
func IPFilterMiddleware(allowedIPs []string, logger *observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)

			allowed := false
			for _, ip := range allowedIPs {
				if ip == clientIP {
					allowed = true
					break
				}
			}

			if !allowed {
				logger.WithFields(map[string]interface{}{
					"client_ip": clientIP,
					"path":      r.URL.Path,
				}).Warn("IP not allowed")

				http.Error(w, `{"error": "Forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-Ip header
	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// RateLimitMiddleware is a placeholder for rate limiting.
// In production, use a proper rate limiter like golang.org/x/time/rate.
func RateLimitMiddleware(requestsPerSecond int, burstSize int) Middleware {
	return func(next http.Handler) http.Handler {
		// For MVP, just pass through
		// TODO: Implement proper rate limiting
		return next
	}
}
