package audio

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBufferFull is returned when the jitter buffer is full.
var ErrBufferFull = errors.New("jitter buffer is full")

// ErrBufferClosed is returned when operating on a closed buffer.
var ErrBufferClosed = errors.New("jitter buffer is closed")

// JitterBuffer provides thread-safe buffered audio queue with backpressure.
type JitterBuffer struct {
	mu          sync.RWMutex
	buffer      [][]byte
	maxSize     int
	closed      bool
	notifyCh    chan struct{}
	readTimeout time.Duration
}

// NewJitterBuffer creates a new jitter buffer with the specified max size.
func NewJitterBuffer(maxSize int) *JitterBuffer {
	if maxSize <= 0 {
		maxSize = 100 // default max frames
	}
	return &JitterBuffer{
		buffer:      make([][]byte, 0, maxSize),
		maxSize:     maxSize,
		notifyCh:    make(chan struct{}, 1),
		readTimeout: 100 * time.Millisecond,
	}
}

// Write adds a frame to the buffer.
// Returns ErrBufferFull if the buffer is at capacity.
func (jb *JitterBuffer) Write(frame []byte) error {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	if jb.closed {
		return ErrBufferClosed
	}

	if len(jb.buffer) >= jb.maxSize {
		return ErrBufferFull
	}

	// Copy the frame to avoid external modifications
	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)
	jb.buffer = append(jb.buffer, frameCopy)

	// Notify waiting readers
	select {
	case jb.notifyCh <- struct{}{}:
	default:
	}

	return nil
}

// Read removes and returns the oldest frame from the buffer.
// Blocks until a frame is available or the context is cancelled.
func (jb *JitterBuffer) Read(ctx context.Context) ([]byte, error) {
	for {
		jb.mu.Lock()

		if jb.closed && len(jb.buffer) == 0 {
			jb.mu.Unlock()
			return nil, ErrBufferClosed
		}

		if len(jb.buffer) > 0 {
			frame := jb.buffer[0]
			jb.buffer = jb.buffer[1:]
			jb.mu.Unlock()
			return frame, nil
		}

		jb.mu.Unlock()

		// Wait for data or context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-jb.notifyCh:
			// Data available, loop and try again
		}
	}
}

// TryRead attempts to read a frame without blocking.
// Returns nil, false if no frame is available.
func (jb *JitterBuffer) TryRead() ([]byte, bool) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	if len(jb.buffer) == 0 {
		return nil, false
	}

	frame := jb.buffer[0]
	jb.buffer = jb.buffer[1:]
	return frame, true
}

// Peek returns the oldest frame without removing it.
// Returns nil, false if the buffer is empty.
func (jb *JitterBuffer) Peek() ([]byte, bool) {
	jb.mu.RLock()
	defer jb.mu.RUnlock()

	if len(jb.buffer) == 0 {
		return nil, false
	}

	frame := make([]byte, len(jb.buffer[0]))
	copy(frame, jb.buffer[0])
	return frame, true
}

// Len returns the number of frames in the buffer.
func (jb *JitterBuffer) Len() int {
	jb.mu.RLock()
	defer jb.mu.RUnlock()
	return len(jb.buffer)
}

// IsFull returns true if the buffer is at capacity.
func (jb *JitterBuffer) IsFull() bool {
	jb.mu.RLock()
	defer jb.mu.RUnlock()
	return len(jb.buffer) >= jb.maxSize
}

// Available returns the number of additional frames that can be written.
func (jb *JitterBuffer) Available() int {
	jb.mu.RLock()
	defer jb.mu.RUnlock()
	return jb.maxSize - len(jb.buffer)
}

// Close closes the buffer, preventing further writes.
func (jb *JitterBuffer) Close() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	if !jb.closed {
		jb.closed = true
		close(jb.notifyCh)
	}
}

// IsClosed returns true if the buffer is closed.
func (jb *JitterBuffer) IsClosed() bool {
	jb.mu.RLock()
	defer jb.mu.RUnlock()
	return jb.closed
}

// Clear removes all frames from the buffer.
func (jb *JitterBuffer) Clear() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	jb.buffer = jb.buffer[:0]
}

// SetReadTimeout sets the timeout for read operations.
func (jb *JitterBuffer) SetReadTimeout(timeout time.Duration) {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	jb.readTimeout = timeout
}

// Stats returns buffer statistics.
func (jb *JitterBuffer) Stats() BufferStats {
	jb.mu.RLock()
	defer jb.mu.RUnlock()

	var totalBytes int
	for _, frame := range jb.buffer {
		totalBytes += len(frame)
	}

	return BufferStats{
		FrameCount: len(jb.buffer),
		TotalBytes: totalBytes,
		MaxSize:    jb.maxSize,
		IsFull:     len(jb.buffer) >= jb.maxSize,
		IsClosed:   jb.closed,
	}
}

// BufferStats contains statistics about the buffer state.
type BufferStats struct {
	FrameCount int
	TotalBytes int
	MaxSize    int
	IsFull     bool
	IsClosed   bool
}

// BufferedAudioWriter wraps a JitterBuffer with a Write method.
type BufferedAudioWriter struct {
	buffer *JitterBuffer
}

// NewBufferedAudioWriter creates a new buffered audio writer.
func NewBufferedAudioWriter(buffer *JitterBuffer) *BufferedAudioWriter {
	return &BufferedAudioWriter{buffer: buffer}
}

// Write implements io.Writer for audio frames.
func (w *BufferedAudioWriter) Write(p []byte) (n int, err error) {
	if err := w.buffer.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// BufferedAudioReader wraps a JitterBuffer with a Read method.
type BufferedAudioReader struct {
	buffer *JitterBuffer
	ctx    context.Context
}

// NewBufferedAudioReader creates a new buffered audio reader.
func NewBufferedAudioReader(buffer *JitterBuffer, ctx context.Context) *BufferedAudioReader {
	return &BufferedAudioReader{
		buffer: buffer,
		ctx:    ctx,
	}
}

// Read implements io.Reader for audio frames.
func (r *BufferedAudioReader) Read(p []byte) (n int, err error) {
	frame, err := r.buffer.Read(r.ctx)
	if err != nil {
		return 0, err
	}

	n = copy(p, frame)
	return n, nil
}

// CircularBuffer implements a fixed-size circular buffer for audio samples.
type CircularBuffer struct {
	mu       sync.RWMutex
	data     []byte
	size     int
	readPos  int
	writePos int
	count    int
}

// NewCircularBuffer creates a new circular buffer of the specified size.
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write writes data to the circular buffer.
// Returns the number of bytes written.
func (cb *CircularBuffer) Write(p []byte) int {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	written := 0
	for _, b := range p {
		if cb.count >= cb.size {
			// Buffer full - overwrite oldest data
			cb.readPos = (cb.readPos + 1) % cb.size
		} else {
			cb.count++
		}
		cb.data[cb.writePos] = b
		cb.writePos = (cb.writePos + 1) % cb.size
		written++
	}

	return written
}

// Read reads data from the circular buffer.
// Returns the number of bytes read.
func (cb *CircularBuffer) Read(p []byte) int {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	n := len(p)
	if n > cb.count {
		n = cb.count
	}

	for i := 0; i < n; i++ {
		p[i] = cb.data[cb.readPos]
		cb.readPos = (cb.readPos + 1) % cb.size
		cb.count--
	}

	return n
}

// Len returns the number of bytes available to read.
func (cb *CircularBuffer) Len() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count
}

// Available returns the number of bytes that can be written.
func (cb *CircularBuffer) Available() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size - cb.count
}

// Reset clears the buffer.
func (cb *CircularBuffer) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.readPos = 0
	cb.writePos = 0
	cb.count = 0
}
