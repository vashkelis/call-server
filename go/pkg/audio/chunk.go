package audio

import (
	"fmt"
)

// Chunker splits and reassembles audio data into fixed-size frames.
type Chunker struct {
	frameSize int
	buffer    []byte
	onFrame   func([]byte)
}

// NewChunker creates a new audio chunker with the specified frame size.
func NewChunker(frameSize int, onFrame func([]byte)) *Chunker {
	return &Chunker{
		frameSize: frameSize,
		buffer:    make([]byte, 0, frameSize*2),
		onFrame:   onFrame,
	}
}

// Write adds audio data to the chunker, emitting complete frames.
func (c *Chunker) Write(data []byte) error {
	c.buffer = append(c.buffer, data...)

	// Emit complete frames
	for len(c.buffer) >= c.frameSize {
		frame := make([]byte, c.frameSize)
		copy(frame, c.buffer[:c.frameSize])

		if c.onFrame != nil {
			c.onFrame(frame)
		}

		c.buffer = c.buffer[c.frameSize:]
	}

	return nil
}

// Flush emits any remaining buffered data as a partial frame.
func (c *Chunker) Flush() ([]byte, error) {
	if len(c.buffer) == 0 {
		return nil, nil
	}

	result := make([]byte, len(c.buffer))
	copy(result, c.buffer)
	c.buffer = c.buffer[:0]

	return result, nil
}

// Reset clears the internal buffer.
func (c *Chunker) Reset() {
	c.buffer = c.buffer[:0]
}

// Buffered returns the number of bytes currently buffered.
func (c *Chunker) Buffered() int {
	return len(c.buffer)
}

// FrameSize returns the configured frame size.
func (c *Chunker) FrameSize() int {
	return c.frameSize
}

// ChunkResult represents the result of chunking audio data.
type ChunkResult struct {
	Frames       [][]byte
	PartialFrame []byte
}

// ChunkStatic chunks audio data into frames without maintaining state.
func ChunkStatic(data []byte, frameSize int) ChunkResult {
	if frameSize <= 0 {
		return ChunkResult{PartialFrame: data}
	}

	numCompleteFrames := len(data) / frameSize
	remaining := len(data) % frameSize

	result := ChunkResult{
		Frames: make([][]byte, numCompleteFrames),
	}

	for i := 0; i < numCompleteFrames; i++ {
		frame := make([]byte, frameSize)
		copy(frame, data[i*frameSize:(i+1)*frameSize])
		result.Frames[i] = frame
	}

	if remaining > 0 {
		result.PartialFrame = make([]byte, remaining)
		copy(result.PartialFrame, data[numCompleteFrames*frameSize:])
	}

	return result
}

// Reassembler collects audio chunks and reassembles them into a stream.
type Reassembler struct {
	expectedSeq   uint32
	buffer        map[uint32][]byte
	maxBufferSize int
	onData        func([]byte)
}

// NewReassembler creates a new audio reassembler.
func NewReassembler(maxBufferSize int, onData func([]byte)) *Reassembler {
	if maxBufferSize <= 0 {
		maxBufferSize = 100 // default max buffered chunks
	}
	return &Reassembler{
		expectedSeq:   0,
		buffer:        make(map[uint32][]byte),
		maxBufferSize: maxBufferSize,
		onData:        onData,
	}
}

// AddChunk adds a chunk with a sequence number.
func (r *Reassembler) AddChunk(seq uint32, data []byte) error {
	// If this is the expected chunk, emit it and any buffered sequential chunks
	if seq == r.expectedSeq {
		r.emit(data)
		r.expectedSeq++
		r.emitBuffered()
		return nil
	}

	// If this is an old chunk, ignore it
	if seq < r.expectedSeq {
		return nil
	}

	// Buffer for later
	if len(r.buffer) >= r.maxBufferSize {
		// Buffer full - advance to clear space
		r.advanceExpected()
	}

	r.buffer[seq] = data
	return nil
}

// emit sends data to the callback.
func (r *Reassembler) emit(data []byte) {
	if r.onData != nil {
		r.onData(data)
	}
}

// emitBuffered emits any buffered chunks that are now in sequence.
func (r *Reassembler) emitBuffered() {
	for {
		data, ok := r.buffer[r.expectedSeq]
		if !ok {
			break
		}
		r.emit(data)
		delete(r.buffer, r.expectedSeq)
		r.expectedSeq++
	}
}

// advanceExpected advances the expected sequence number and clears old buffers.
func (r *Reassembler) advanceExpected() {
	// Emit placeholder silence or skip
	delete(r.buffer, r.expectedSeq)
	r.expectedSeq++
}

// Reset resets the reassembler state.
func (r *Reassembler) Reset() {
	r.expectedSeq = 0
	r.buffer = make(map[uint32][]byte)
}

// ExpectedSeq returns the next expected sequence number.
func (r *Reassembler) ExpectedSeq() uint32 {
	return r.expectedSeq
}

// BufferedCount returns the number of buffered chunks.
func (r *Reassembler) BufferedCount() int {
	return len(r.buffer)
}

// FrameChunker is a convenience wrapper for chunking with audio profiles.
type FrameChunker struct {
	profile AudioProfile
	chunker *Chunker
}

// NewFrameChunker creates a frame chunker for the given audio profile.
func NewFrameChunker(profile AudioProfile, onFrame func([]byte)) (*FrameChunker, error) {
	if err := profile.Validate(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	frameSize := profile.BytesPerFrame()
	return &FrameChunker{
		profile: profile,
		chunker: NewChunker(frameSize, onFrame),
	}, nil
}

// Write adds audio data to the chunker.
func (fc *FrameChunker) Write(data []byte) error {
	return fc.chunker.Write(data)
}

// Flush emits any remaining buffered data.
func (fc *FrameChunker) Flush() ([]byte, error) {
	return fc.chunker.Flush()
}

// Reset clears the internal buffer.
func (fc *FrameChunker) Reset() {
	fc.chunker.Reset()
}

// Profile returns the audio profile.
func (fc *FrameChunker) Profile() AudioProfile {
	return fc.profile
}
