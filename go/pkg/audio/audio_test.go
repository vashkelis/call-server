package audio

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
)

func TestResample8kTo16k(t *testing.T) {
	// Create 1 second of 8kHz PCM16 audio (mono)
	// 8000 samples * 2 bytes = 16000 bytes
	input := make([]byte, 16000)
	for i := range input {
		input[i] = byte(i % 256)
	}

	output, err := Resample8kTo16k(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected output: 16000 samples * 2 bytes = 32000 bytes
	expectedLen := 32000
	if len(output) != expectedLen {
		t.Errorf("expected output length %d, got %d", expectedLen, len(output))
	}

	// Verify it's roughly double the input size
	if len(output)/len(input) != 2 {
		t.Errorf("expected output to be roughly 2x input size, got %d/%d", len(output), len(input))
	}
}

func TestResample48kTo16k(t *testing.T) {
	// Create 1 second of 48kHz PCM16 audio (mono)
	// 48000 samples * 2 bytes = 96000 bytes
	input := make([]byte, 96000)
	for i := range input {
		input[i] = byte(i % 256)
	}

	output, err := Resample48kTo16k(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected output: 16000 samples * 2 bytes = 32000 bytes
	expectedLen := 32000
	if len(output) != expectedLen {
		t.Errorf("expected output length %d, got %d", expectedLen, len(output))
	}

	// Verify it's roughly 1/3 the input size
	if len(input)/len(output) != 3 {
		t.Errorf("expected output to be roughly 1/3 of input size, got %d/%d", len(output), len(input))
	}
}

func TestResampleSameRate(t *testing.T) {
	// Resampling same rate should return input unchanged
	input := make([]byte, 32000)
	for i := range input {
		input[i] = byte(i % 256)
	}

	resampler := NewLinearResampler()
	output, err := resampler.Resample(input, 16000, 16000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(input, output) {
		t.Error("expected output to equal input when resampling to same rate")
	}
}

func TestResampleInvalidRates(t *testing.T) {
	resampler := NewLinearResampler()

	input := make([]byte, 100)

	_, err := resampler.Resample(input, 0, 16000)
	if err == nil {
		t.Error("expected error for invalid fromRate")
	}

	_, err = resampler.Resample(input, 16000, 0)
	if err == nil {
		t.Error("expected error for invalid toRate")
	}
}

func TestResampleOddLength(t *testing.T) {
	resampler := NewLinearResampler()

	// Odd length should fail for PCM16
	input := make([]byte, 101)
	_, err := resampler.Resample(input, 8000, 16000)
	if err == nil {
		t.Error("expected error for odd-length input")
	}
}

func TestChunker(t *testing.T) {
	frameSize := 320 // 20ms at 16kHz mono PCM16
	var frames [][]byte

	chunker := NewChunker(frameSize, func(frame []byte) {
		frameCopy := make([]byte, len(frame))
		copy(frameCopy, frame)
		frames = append(frames, frameCopy)
	})

	// Write 5 frames worth of data
	data := make([]byte, frameSize*5)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := chunker.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 5 complete frames
	if len(frames) != 5 {
		t.Errorf("expected 5 frames, got %d", len(frames))
	}

	// Verify frame sizes
	for i, frame := range frames {
		if len(frame) != frameSize {
			t.Errorf("frame %d: expected size %d, got %d", i, frameSize, len(frame))
		}
	}

	// Verify frame content
	for i, frame := range frames {
		expectedStart := byte((i * frameSize) % 256)
		if frame[0] != expectedStart {
			t.Errorf("frame %d: expected first byte %d, got %d", i, expectedStart, frame[0])
		}
	}
}

func TestChunkerPartialFrame(t *testing.T) {
	frameSize := 320
	var frames [][]byte

	chunker := NewChunker(frameSize, func(frame []byte) {
		frameCopy := make([]byte, len(frame))
		copy(frameCopy, frame)
		frames = append(frames, frameCopy)
	})

	// Write 2.5 frames worth of data
	data := make([]byte, int(float64(frameSize)*2.5))
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := chunker.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 complete frames
	if len(frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(frames))
	}

	// Flush should return remaining partial frame
	partial, err := chunker.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPartialSize := frameSize / 2
	if len(partial) != expectedPartialSize {
		t.Errorf("expected partial frame size %d, got %d", expectedPartialSize, len(partial))
	}
}

func TestChunkerReset(t *testing.T) {
	frameSize := 320
	var frames [][]byte

	chunker := NewChunker(frameSize, func(frame []byte) {
		frameCopy := make([]byte, len(frame))
		copy(frameCopy, frame)
		frames = append(frames, frameCopy)
	})

	// Write partial frame
	data := make([]byte, frameSize/2)
	chunker.Write(data)

	if chunker.Buffered() != frameSize/2 {
		t.Errorf("expected %d bytes buffered, got %d", frameSize/2, chunker.Buffered())
	}

	// Reset
	chunker.Reset()

	if chunker.Buffered() != 0 {
		t.Errorf("expected 0 bytes buffered after reset, got %d", chunker.Buffered())
	}

	// Write full frame
	chunker.Write(make([]byte, frameSize))

	if len(frames) != 1 {
		t.Errorf("expected 1 frame after reset and write, got %d", len(frames))
	}
}

func TestJitterBuffer(t *testing.T) {
	jb := NewJitterBuffer(5)

	// Test initial state
	if jb.Len() != 0 {
		t.Errorf("expected empty buffer, got %d frames", jb.Len())
	}
	if jb.IsFull() {
		t.Error("expected buffer to not be full initially")
	}
	if jb.Available() != 5 {
		t.Errorf("expected 5 available slots, got %d", jb.Available())
	}

	// Write frames
	for i := 0; i < 3; i++ {
		frame := []byte{byte(i), byte(i + 1), byte(i + 2)}
		err := jb.Write(frame)
		if err != nil {
			t.Fatalf("unexpected error writing frame %d: %v", i, err)
		}
	}

	if jb.Len() != 3 {
		t.Errorf("expected 3 frames, got %d", jb.Len())
	}

	// Read frames
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		frame, err := jb.Read(ctx)
		if err != nil {
			t.Fatalf("unexpected error reading frame %d: %v", i, err)
		}
		if frame[0] != byte(i) {
			t.Errorf("frame %d: expected first byte %d, got %d", i, i, frame[0])
		}
	}

	if jb.Len() != 0 {
		t.Errorf("expected empty buffer after reading all, got %d", jb.Len())
	}
}

func TestJitterBufferBackpressure(t *testing.T) {
	jb := NewJitterBuffer(3)

	// Fill buffer to capacity
	for i := 0; i < 3; i++ {
		err := jb.Write([]byte{byte(i)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if !jb.IsFull() {
		t.Error("expected buffer to be full")
	}

	// Try to write to full buffer
	err := jb.Write([]byte{99})
	if err != ErrBufferFull {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}

	// Read one frame
	ctx := context.Background()
	_, err = jb.Read(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now we should be able to write again
	err = jb.Write([]byte{99})
	if err != nil {
		t.Errorf("unexpected error after making space: %v", err)
	}
}

func TestJitterBufferClose(t *testing.T) {
	jb := NewJitterBuffer(5)

	// Write some frames
	jb.Write([]byte{1})
	jb.Write([]byte{2})

	// Close buffer
	jb.Close()

	if !jb.IsClosed() {
		t.Error("expected buffer to be closed")
	}

	// Try to write to closed buffer
	err := jb.Write([]byte{3})
	if err != ErrBufferClosed {
		t.Errorf("expected ErrBufferClosed, got %v", err)
	}

	// Try to read from closed buffer with remaining data
	ctx := context.Background()
	frame, err := jb.Read(ctx)
	if err != nil {
		t.Errorf("expected to read remaining data before error, got: %v", err)
	}
	if frame[0] != 1 {
		t.Errorf("expected first frame, got %v", frame)
	}

	// Read remaining frame
	frame, err = jb.Read(ctx)
	if err != nil {
		t.Errorf("expected to read remaining data, got: %v", err)
	}
	if frame[0] != 2 {
		t.Errorf("expected second frame, got %v", frame)
	}

	// Now read should return error
	_, err = jb.Read(ctx)
	if err != ErrBufferClosed {
		t.Errorf("expected ErrBufferClosed after reading all, got %v", err)
	}
}

func TestJitterBufferTryRead(t *testing.T) {
	jb := NewJitterBuffer(5)

	// TryRead on empty buffer should return false
	_, ok := jb.TryRead()
	if ok {
		t.Error("expected TryRead to return false on empty buffer")
	}

	// Write and try read
	jb.Write([]byte{1, 2, 3})

	frame, ok := jb.TryRead()
	if !ok {
		t.Error("expected TryRead to return true")
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Errorf("expected frame [1 2 3], got %v", frame)
	}
}

func TestJitterBufferPeek(t *testing.T) {
	jb := NewJitterBuffer(5)

	// Peek on empty buffer should return false
	_, ok := jb.Peek()
	if ok {
		t.Error("expected Peek to return false on empty buffer")
	}

	// Write and peek
	jb.Write([]byte{1, 2, 3})

	frame, ok := jb.Peek()
	if !ok {
		t.Error("expected Peek to return true")
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Errorf("expected frame [1 2 3], got %v", frame)
	}

	// Buffer should still have the frame
	if jb.Len() != 1 {
		t.Errorf("expected buffer to still have frame, got %d", jb.Len())
	}
}

func TestPlayoutTracker(t *testing.T) {
	pt := NewPlayoutTracker(16000, 1)

	// Initial state
	if pt.CurrentBytes() != 0 {
		t.Errorf("expected 0 bytes initially, got %d", pt.CurrentBytes())
	}
	if pt.CurrentPosition() != 0 {
		t.Errorf("expected 0 position initially, got %v", pt.CurrentPosition())
	}
	if pt.IsComplete() {
		t.Error("expected not complete initially")
	}

	// Set total bytes
	pt.SetTotalBytes(64000) // 2 seconds at 16kHz mono PCM16

	// Advance
	pt.Advance(16000) // 0.5 seconds

	if pt.CurrentBytes() != 16000 {
		t.Errorf("expected 16000 bytes, got %d", pt.CurrentBytes())
	}

	expectedPos := time.Millisecond * 500
	pos := pt.CurrentPosition()
	if pos < expectedPos-10*time.Millisecond || pos > expectedPos+10*time.Millisecond {
		t.Errorf("expected position ~%v, got %v", expectedPos, pos)
	}

	// Progress
	progress := pt.Progress()
	expectedProgress := 0.25 // 16000/64000
	if progress < expectedProgress-0.01 || progress > expectedProgress+0.01 {
		t.Errorf("expected progress ~%.2f, got %.2f", expectedProgress, progress)
	}

	// Remaining
	remaining := pt.RemainingBytes()
	if remaining != 48000 {
		t.Errorf("expected 48000 remaining bytes, got %d", remaining)
	}
}

func TestPlayoutTrackerComplete(t *testing.T) {
	pt := NewPlayoutTracker(16000, 1)
	pt.SetTotalBytes(32000)

	if pt.IsComplete() {
		t.Error("expected not complete before advancing")
	}

	pt.Advance(32000)

	if !pt.IsComplete() {
		t.Error("expected complete after advancing to total")
	}

	if pt.Progress() != 1.0 {
		t.Errorf("expected progress 1.0, got %f", pt.Progress())
	}
}

func TestPlayoutTrackerReset(t *testing.T) {
	pt := NewPlayoutTracker(16000, 1)
	pt.SetTotalBytes(32000)
	pt.Advance(16000)

	if pt.CurrentBytes() != 16000 {
		t.Error("expected bytes to be advanced")
	}

	pt.Reset()

	if pt.CurrentBytes() != 0 {
		t.Errorf("expected 0 bytes after reset, got %d", pt.CurrentBytes())
	}
	if pt.IsComplete() {
		t.Error("expected not complete after reset")
	}
}

func TestPlayoutTrackerPauseResume(t *testing.T) {
	pt := NewPlayoutTracker(16000, 1)

	if pt.IsPaused() {
		t.Error("expected not paused initially")
	}

	pt.Pause()
	if !pt.IsPaused() {
		t.Error("expected paused after Pause()")
	}

	// Advance while paused should not count
	pt.Advance(16000)
	if pt.CurrentBytes() != 0 {
		t.Errorf("expected 0 bytes after paused advance, got %d", pt.CurrentBytes())
	}

	pt.Resume()
	if pt.IsPaused() {
		t.Error("expected not paused after Resume()")
	}

	// Now advance should count
	pt.Advance(16000)
	if pt.CurrentBytes() != 16000 {
		t.Errorf("expected 16000 bytes after resume and advance, got %d", pt.CurrentBytes())
	}
}

func TestAudioProfiles(t *testing.T) {
	tests := []struct {
		name          string
		profile       AudioProfile
		wantRate      int
		wantChannels  int
		wantEncoding  contracts.AudioEncoding
		wantFrameSize int
	}{
		{
			name:          "InternalProfile",
			profile:       InternalProfile,
			wantRate:      16000,
			wantChannels:  1,
			wantEncoding:  contracts.PCM16,
			wantFrameSize: 160,
		},
		{
			name:          "TelephonyProfile",
			profile:       TelephonyProfile,
			wantRate:      16000,
			wantChannels:  1,
			wantEncoding:  contracts.PCM16,
			wantFrameSize: 160,
		},
		{
			name:          "Telephony8kProfile",
			profile:       Telephony8kProfile,
			wantRate:      8000,
			wantChannels:  1,
			wantEncoding:  contracts.PCM16,
			wantFrameSize: 80,
		},
		{
			name:          "WebRTCProfile",
			profile:       WebRTCProfile,
			wantRate:      48000,
			wantChannels:  1,
			wantEncoding:  contracts.PCM16,
			wantFrameSize: 480,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.profile.SampleRate != tt.wantRate {
				t.Errorf("expected sample rate %d, got %d", tt.wantRate, tt.profile.SampleRate)
			}
			if tt.profile.Channels != tt.wantChannels {
				t.Errorf("expected channels %d, got %d", tt.wantChannels, tt.profile.Channels)
			}
			if tt.profile.Encoding != tt.wantEncoding {
				t.Errorf("expected encoding %v, got %v", tt.wantEncoding, tt.profile.Encoding)
			}
			if tt.profile.FrameSize != tt.wantFrameSize {
				t.Errorf("expected frame size %d, got %d", tt.wantFrameSize, tt.profile.FrameSize)
			}
		})
	}
}

func TestAudioProfileBytesPerSample(t *testing.T) {
	tests := []struct {
		encoding contracts.AudioEncoding
		expected int
	}{
		{contracts.PCM16, 2},
		{contracts.G711ULAW, 1},
		{contracts.G711ALAW, 1},
		{contracts.AudioEncodingUnspecified, 2}, // default
	}

	for _, tt := range tests {
		profile := AudioProfile{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   tt.encoding,
			FrameSize:  160,
		}

		if got := profile.BytesPerSample(); got != tt.expected {
			t.Errorf("encoding %v: expected %d bytes per sample, got %d", tt.encoding, tt.expected, got)
		}
	}
}

func TestAudioProfileBytesPerFrame(t *testing.T) {
	profile := AudioProfile{
		SampleRate: 16000,
		Channels:   2,
		Encoding:   contracts.PCM16,
		FrameSize:  160,
	}

	// 160 samples * 2 channels * 2 bytes = 640 bytes per frame
	expected := 640
	if got := profile.BytesPerFrame(); got != expected {
		t.Errorf("expected %d bytes per frame, got %d", expected, got)
	}
}

func TestAudioProfileDurationCalculations(t *testing.T) {
	profile := AudioProfile{
		SampleRate: 16000,
		Channels:   1,
		Encoding:   contracts.PCM16,
		FrameSize:  160,
	}

	// Test DurationFromBytes
	// 32000 bytes = 16000 samples = 1 second at 16kHz
	bytes := 32000
	duration := profile.DurationFromBytes(bytes)
	expectedDuration := 1.0
	if duration < expectedDuration-0.01 || duration > expectedDuration+0.01 {
		t.Errorf("expected duration ~%.2f seconds, got %.2f", expectedDuration, duration)
	}

	// Test BytesFromDuration
	// 1 second at 16kHz mono PCM16 = 32000 bytes
	calculatedBytes := profile.BytesFromDuration(1.0)
	if calculatedBytes != 32000 {
		t.Errorf("expected 32000 bytes, got %d", calculatedBytes)
	}
}

func TestAudioProfileValidate(t *testing.T) {
	// Valid profile
	validProfile := AudioProfile{
		SampleRate: 16000,
		Channels:   1,
		FrameSize:  160,
	}
	if err := validProfile.Validate(); err != nil {
		t.Errorf("expected valid profile, got error: %v", err)
	}

	// Invalid sample rate
	invalidRate := AudioProfile{
		SampleRate: 0,
		Channels:   1,
	}
	if err := invalidRate.Validate(); err == nil {
		t.Error("expected error for invalid sample rate")
	}

	// Invalid channels
	invalidChannels := AudioProfile{
		SampleRate: 16000,
		Channels:   0,
	}
	if err := invalidChannels.Validate(); err == nil {
		t.Error("expected error for invalid channels")
	}

	// Auto-set frame size
	noFrameSize := AudioProfile{
		SampleRate: 16000,
		Channels:   1,
		FrameSize:  0,
	}
	if err := noFrameSize.Validate(); err != nil {
		t.Errorf("expected validation to set default frame size, got error: %v", err)
	}
	if noFrameSize.FrameSize != 160 {
		t.Errorf("expected frame size 160, got %d", noFrameSize.FrameSize)
	}
}

func TestAudioProfileIsCanonical(t *testing.T) {
	if !InternalProfile.IsCanonical() {
		t.Error("expected InternalProfile to be canonical")
	}

	nonCanonical := AudioProfile{
		SampleRate: 8000,
		Channels:   1,
		Encoding:   contracts.PCM16,
		FrameSize:  80,
	}
	if nonCanonical.IsCanonical() {
		t.Error("expected 8kHz profile to not be canonical")
	}
}

func TestSampleRateFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"8k", 8000},
		{"8000", 8000},
		{"16k", 16000},
		{"16000", 16000},
		{"22k", 22050},
		{"22050", 22050},
		{"44k", 44100},
		{"44100", 44100},
		{"48k", 48000},
		{"48000", 48000},
		{"unknown", 16000}, // default
	}

	for _, tt := range tests {
		got := SampleRateFromString(tt.input)
		if got != tt.expected {
			t.Errorf("SampleRateFromString(%q): expected %d, got %d", tt.input, tt.expected, got)
		}
	}
}

func TestChunkStatic(t *testing.T) {
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	result := ChunkStatic(data, 320)

	// 1000 / 320 = 3 complete frames, 40 bytes remaining
	if len(result.Frames) != 3 {
		t.Errorf("expected 3 frames, got %d", len(result.Frames))
	}

	if len(result.PartialFrame) != 40 {
		t.Errorf("expected 40 bytes partial frame, got %d", len(result.PartialFrame))
	}

	// Verify frame content
	for i, frame := range result.Frames {
		if len(frame) != 320 {
			t.Errorf("frame %d: expected 320 bytes, got %d", i, len(frame))
		}
		expectedFirstByte := byte((i * 320) % 256)
		if frame[0] != expectedFirstByte {
			t.Errorf("frame %d: expected first byte %d, got %d", i, expectedFirstByte, frame[0])
		}
	}
}

func TestChunkStaticInvalidFrameSize(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}

	result := ChunkStatic(data, 0)

	if len(result.Frames) != 0 {
		t.Errorf("expected 0 frames for invalid frame size, got %d", len(result.Frames))
	}

	if !bytes.Equal(result.PartialFrame, data) {
		t.Errorf("expected partial frame to be all data, got %v", result.PartialFrame)
	}
}
