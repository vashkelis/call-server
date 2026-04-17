package audio

import (
	"context"
	"sync"
	"time"
)

// PlayoutTracker tracks audio playout progress for a session.
type PlayoutTracker struct {
	mu sync.RWMutex

	totalBytes     int
	bytesSent      int
	startTime      *time.Time
	paused         bool
	sampleRate     int
	bytesPerSample int
	channels       int

	// Callbacks
	onProgress func(position time.Duration)
	onComplete func()
}

// NewPlayoutTracker creates a new playout tracker.
func NewPlayoutTracker(sampleRate, channels int) *PlayoutTracker {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}

	return &PlayoutTracker{
		sampleRate:     sampleRate,
		bytesPerSample: 2, // PCM16
		channels:       channels,
	}
}

// SetTotalBytes sets the total expected bytes for this playout.
func (pt *PlayoutTracker) SetTotalBytes(bytes int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.totalBytes = bytes
}

// Advance records that bytes have been sent to the client.
func (pt *PlayoutTracker) Advance(bytes int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.paused {
		return
	}

	if pt.startTime == nil {
		now := time.Now().UTC()
		pt.startTime = &now
	}

	pt.bytesSent += bytes

	if pt.onProgress != nil {
		position := pt.calculateDuration(pt.bytesSent)
		pt.onProgress(position)
	}

	if pt.totalBytes > 0 && pt.bytesSent >= pt.totalBytes && pt.onComplete != nil {
		pt.onComplete()
	}
}

// CurrentPosition returns the current playout position as a duration.
func (pt *PlayoutTracker) CurrentPosition() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.calculateDuration(pt.bytesSent)
}

// CurrentBytes returns the number of bytes sent.
func (pt *PlayoutTracker) CurrentBytes() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.bytesSent
}

// RemainingBytes returns the number of bytes remaining.
func (pt *PlayoutTracker) RemainingBytes() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	if pt.totalBytes <= pt.bytesSent {
		return 0
	}
	return pt.totalBytes - pt.bytesSent
}

// RemainingDuration returns the estimated remaining duration.
func (pt *PlayoutTracker) RemainingDuration() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	remaining := pt.totalBytes - pt.bytesSent
	if remaining <= 0 {
		return 0
	}
	return pt.calculateDuration(remaining)
}

// Progress returns the playout progress as a percentage (0.0 to 1.0).
func (pt *PlayoutTracker) Progress() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.totalBytes == 0 {
		return 0
	}

	progress := float64(pt.bytesSent) / float64(pt.totalBytes)
	if progress > 1.0 {
		progress = 1.0
	}
	return progress
}

// IsComplete returns true if playout is complete.
func (pt *PlayoutTracker) IsComplete() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.totalBytes > 0 && pt.bytesSent >= pt.totalBytes
}

// IsPaused returns true if playout is paused.
func (pt *PlayoutTracker) IsPaused() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.paused
}

// Pause pauses playout tracking.
func (pt *PlayoutTracker) Pause() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.paused = true
}

// Resume resumes playout tracking.
func (pt *PlayoutTracker) Resume() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.paused = false
}

// Reset resets the playout tracker.
func (pt *PlayoutTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.bytesSent = 0
	pt.startTime = nil
	pt.paused = false
}

// SetOnProgress sets the progress callback.
func (pt *PlayoutTracker) SetOnProgress(fn func(position time.Duration)) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.onProgress = fn
}

// SetOnComplete sets the completion callback.
func (pt *PlayoutTracker) SetOnComplete(fn func()) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.onComplete = fn
}

// ElapsedTime returns the time since playout started.
func (pt *PlayoutTracker) ElapsedTime() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.startTime == nil {
		return 0
	}
	return time.Since(*pt.startTime)
}

// calculateDuration converts bytes to duration.
func (pt *PlayoutTracker) calculateDuration(bytes int) time.Duration {
	if pt.sampleRate == 0 || pt.bytesPerSample == 0 || pt.channels == 0 {
		return 0
	}

	samples := bytes / (pt.bytesPerSample * pt.channels)
	seconds := float64(samples) / float64(pt.sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

// PlayoutStats contains playout statistics.
type PlayoutStats struct {
	BytesSent       int
	TotalBytes      int
	CurrentPosition time.Duration
	RemainingTime   time.Duration
	Progress        float64
	IsComplete      bool
	IsPaused        bool
	ElapsedTime     time.Duration
}

// Stats returns current playout statistics.
func (pt *PlayoutTracker) Stats() PlayoutStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return PlayoutStats{
		BytesSent:       pt.bytesSent,
		TotalBytes:      pt.totalBytes,
		CurrentPosition: pt.calculateDuration(pt.bytesSent),
		RemainingTime:   pt.calculateDuration(pt.totalBytes - pt.bytesSent),
		Progress:        pt.Progress(),
		IsComplete:      pt.totalBytes > 0 && pt.bytesSent >= pt.totalBytes,
		IsPaused:        pt.paused,
		ElapsedTime:     pt.ElapsedTime(),
	}
}

// MultiTrackPlayout manages playout for multiple concurrent audio tracks.
type MultiTrackPlayout struct {
	mu     sync.RWMutex
	tracks map[string]*PlayoutTracker
}

// NewMultiTrackPlayout creates a new multi-track playout manager.
func NewMultiTrackPlayout() *MultiTrackPlayout {
	return &MultiTrackPlayout{
		tracks: make(map[string]*PlayoutTracker),
	}
}

// AddTrack adds a new playout track.
func (mtp *MultiTrackPlayout) AddTrack(trackID string, sampleRate, channels int) *PlayoutTracker {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	tracker := NewPlayoutTracker(sampleRate, channels)
	mtp.tracks[trackID] = tracker
	return tracker
}

// GetTrack returns a playout track by ID.
func (mtp *MultiTrackPlayout) GetTrack(trackID string) (*PlayoutTracker, bool) {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()
	tracker, ok := mtp.tracks[trackID]
	return tracker, ok
}

// RemoveTrack removes a playout track.
func (mtp *MultiTrackPlayout) RemoveTrack(trackID string) {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()
	delete(mtp.tracks, trackID)
}

// Advance advances the specified track by the given bytes.
func (mtp *MultiTrackPlayout) Advance(trackID string, bytes int) bool {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	tracker, ok := mtp.tracks[trackID]
	if !ok {
		return false
	}

	tracker.Advance(bytes)
	return true
}

// TotalPosition returns the combined position of all tracks.
func (mtp *MultiTrackPlayout) TotalPosition() time.Duration {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	var total time.Duration
	for _, tracker := range mtp.tracks {
		total += tracker.CurrentPosition()
	}
	return total
}

// Clear removes all tracks.
func (mtp *MultiTrackPlayout) Clear() {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()
	mtp.tracks = make(map[string]*PlayoutTracker)
}

// PlayoutController provides high-level control over audio playout.
type PlayoutController struct {
	tracker    *PlayoutTracker
	buffer     *JitterBuffer
	profile    AudioProfile
	onUnderrun func()
}

// NewPlayoutController creates a new playout controller.
func NewPlayoutController(profile AudioProfile, bufferSize int) (*PlayoutController, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	return &PlayoutController{
		tracker: NewPlayoutTracker(profile.SampleRate, profile.Channels),
		buffer:  NewJitterBuffer(bufferSize),
		profile: profile,
	}, nil
}

// Write adds audio data to the playout buffer.
func (pc *PlayoutController) Write(data []byte) error {
	return pc.buffer.Write(data)
}

// Read reads the next frame from the playout buffer.
func (pc *PlayoutController) Read(ctx context.Context) ([]byte, error) {
	frame, err := pc.buffer.Read(ctx)
	if err != nil {
		if err == ErrBufferClosed && pc.onUnderrun != nil {
			pc.onUnderrun()
		}
		return nil, err
	}

	pc.tracker.Advance(len(frame))
	return frame, nil
}

// TryRead attempts to read without blocking.
func (pc *PlayoutController) TryRead() ([]byte, bool) {
	frame, ok := pc.buffer.TryRead()
	if !ok {
		if pc.onUnderrun != nil {
			pc.onUnderrun()
		}
		return nil, false
	}

	pc.tracker.Advance(len(frame))
	return frame, true
}

// Position returns the current playout position.
func (pc *PlayoutController) Position() time.Duration {
	return pc.tracker.CurrentPosition()
}

// SetOnUnderrun sets the underrun callback.
func (pc *PlayoutController) SetOnUnderrun(fn func()) {
	pc.onUnderrun = fn
}

// Pause pauses playout.
func (pc *PlayoutController) Pause() {
	pc.tracker.Pause()
}

// Resume resumes playout.
func (pc *PlayoutController) Resume() {
	pc.tracker.Resume()
}

// Stop stops playout and clears the buffer.
func (pc *PlayoutController) Stop() {
	pc.tracker.Reset()
	pc.buffer.Clear()
}

// Close closes the playout controller.
func (pc *PlayoutController) Close() {
	pc.buffer.Close()
}
