// Package vad provides Voice Activity Detection implementations.
package vad

import (
	"math"
	"sync"
	"time"
)

// VADState represents the current state of voice activity detection.
type VADState int

const (
	StateSilence VADState = iota
	StateSpeechStart
	StateSpeech
	StateSpeechEnd
)

// String returns the string representation of the VAD state.
func (s VADState) String() string {
	switch s {
	case StateSilence:
		return "silence"
	case StateSpeechStart:
		return "speech_start"
	case StateSpeech:
		return "speech"
	case StateSpeechEnd:
		return "speech_end"
	default:
		return "unknown"
	}
}

// VADResult contains the result of VAD processing.
type VADResult struct {
	IsSpeech    bool
	Energy      float64
	Timestamp   time.Time
	State       VADState
	SpeechStart bool
	SpeechEnd   bool
}

// VADConfig contains configuration for VAD.
type VADConfig struct {
	Threshold      float64 // Energy threshold (0-32768 for PCM16)
	MinSpeechMs    int     // Minimum speech duration in milliseconds
	MinSilenceMs   int     // Minimum silence duration in milliseconds
	HangoverFrames int     // Number of frames to continue after energy drops
	FrameSizeMs    int     // Frame size in milliseconds
	SampleRate     int     // Sample rate in Hz
}

// DefaultVADConfig returns a default VAD configuration.
func DefaultVADConfig() VADConfig {
	return VADConfig{
		Threshold:      500,   // ~-36 dBFS
		MinSpeechMs:    200,   // 200ms minimum speech
		MinSilenceMs:   300,   // 300ms minimum silence
		HangoverFrames: 5,     // 50ms at 10ms frames
		FrameSizeMs:    10,    // 10ms frames
		SampleRate:     16000, // 16kHz
	}
}

// VADDetector is the interface for voice activity detection.
type VADDetector interface {
	// Process analyzes an audio frame and returns VAD result.
	Process(audioFrame []byte) VADResult

	// Reset resets the detector state.
	Reset()

	// State returns the current VAD state.
	State() VADState
}

// EnergyVAD implements a simple energy-based VAD detector.
type EnergyVAD struct {
	mu            sync.RWMutex
	config        VADConfig
	state         VADState
	energyBuffer  []float64
	speechFrames  int
	silenceFrames int
	hangoverCount int
	lastTimestamp time.Time
	frameSamples  int
}

// NewEnergyVAD creates a new energy-based VAD detector.
func NewEnergyVAD(config VADConfig) *EnergyVAD {
	frameSamples := (config.SampleRate * config.FrameSizeMs) / 1000
	return &EnergyVAD{
		config:        config,
		state:         StateSilence,
		energyBuffer:  make([]float64, 0, 10),
		lastTimestamp: time.Now(),
		frameSamples:  frameSamples * 2, // 2 bytes per sample (PCM16)
	}
}

// Process analyzes an audio frame and returns VAD result.
func (v *EnergyVAD) Process(audioFrame []byte) VADResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	v.lastTimestamp = now

	// Calculate energy of the frame
	energy := v.calculateEnergy(audioFrame)

	// Determine if this frame contains speech
	isSpeechFrame := energy > v.config.Threshold

	result := VADResult{
		Energy:    energy,
		Timestamp: now,
		State:     v.state,
	}

	// State machine for VAD
	switch v.state {
	case StateSilence:
		if isSpeechFrame {
			v.speechFrames++
			// Check if we have enough consecutive speech frames
			minFrames := v.msToFrames(v.config.MinSpeechMs)
			if v.speechFrames >= minFrames/2 { // Start earlier, confirm later
				v.state = StateSpeechStart
				result.SpeechStart = true
				result.State = StateSpeechStart
				v.silenceFrames = 0
				v.hangoverCount = 0
			}
		} else {
			v.speechFrames = 0
		}

	case StateSpeechStart:
		if isSpeechFrame {
			v.speechFrames++
			v.state = StateSpeech
			result.State = StateSpeech
			v.silenceFrames = 0
		} else {
			// False start, go back to silence
			v.state = StateSilence
			v.speechFrames = 0
			result.State = StateSilence
		}

	case StateSpeech:
		result.IsSpeech = true
		if isSpeechFrame {
			v.speechFrames++
			v.silenceFrames = 0
			v.hangoverCount = 0
		} else {
			v.silenceFrames++
			v.hangoverCount++

			// Check if hangover expired
			if v.hangoverCount > v.config.HangoverFrames {
				// Check if we have enough silence to end speech
				minSilenceFrames := v.msToFrames(v.config.MinSilenceMs)
				if v.silenceFrames >= minSilenceFrames {
					v.state = StateSpeechEnd
					result.SpeechEnd = true
					result.State = StateSpeechEnd
					result.IsSpeech = false
				}
			}
		}

	case StateSpeechEnd:
		// Transition back to silence
		v.state = StateSilence
		v.speechFrames = 0
		v.silenceFrames = 0
		v.hangoverCount = 0
		result.State = StateSilence

		if isSpeechFrame {
			// Immediate restart
			v.speechFrames = 1
			v.state = StateSpeechStart
			result.SpeechStart = true
			result.State = StateSpeechStart
		}
	}

	return result
}

// Reset resets the detector state.
func (v *EnergyVAD) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.state = StateSilence
	v.speechFrames = 0
	v.silenceFrames = 0
	v.hangoverCount = 0
	v.energyBuffer = v.energyBuffer[:0]
}

// State returns the current VAD state.
func (v *EnergyVAD) State() VADState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

// calculateEnergy calculates the RMS energy of a PCM16 audio frame.
func (v *EnergyVAD) calculateEnergy(audioFrame []byte) float64 {
	if len(audioFrame) < 2 {
		return 0
	}

	var sum float64
	n := len(audioFrame) / 2

	for i := 0; i < len(audioFrame); i += 2 {
		// Convert bytes to int16 sample
		sample := int16(audioFrame[i]) | int16(audioFrame[i+1])<<8
		sum += float64(sample) * float64(sample)
	}

	if n == 0 {
		return 0
	}

	return math.Sqrt(sum / float64(n))
}

// msToFrames converts milliseconds to number of frames.
func (v *EnergyVAD) msToFrames(ms int) int {
	if v.config.FrameSizeMs <= 0 {
		return ms / 10 // Default 10ms frames
	}
	return ms / v.config.FrameSizeMs
}

// Config returns the VAD configuration.
func (v *EnergyVAD) Config() VADConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.config
}

// UpdateConfig updates the VAD configuration.
func (v *EnergyVAD) UpdateConfig(config VADConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.config = config
	v.frameSamples = (config.SampleRate * config.FrameSizeMs) / 1000 * 2
}

// AdaptiveEnergyVAD implements an adaptive threshold energy-based VAD.
type AdaptiveEnergyVAD struct {
	*EnergyVAD
	noiseEstimate  float64
	adaptationRate float64
}

// NewAdaptiveEnergyVAD creates a new adaptive energy-based VAD detector.
func NewAdaptiveEnergyVAD(config VADConfig) *AdaptiveEnergyVAD {
	return &AdaptiveEnergyVAD{
		EnergyVAD:      NewEnergyVAD(config),
		noiseEstimate:  config.Threshold / 2,
		adaptationRate: 0.05,
	}
}

// Process analyzes an audio frame with adaptive threshold.
func (v *AdaptiveEnergyVAD) Process(audioFrame []byte) VADResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Calculate energy
	energy := v.calculateEnergy(audioFrame)

	// Update noise estimate during silence
	if v.state == StateSilence && energy < v.config.Threshold {
		v.noiseEstimate = (1-v.adaptationRate)*v.noiseEstimate + v.adaptationRate*energy
		// Adapt threshold based on noise estimate
		v.config.Threshold = v.noiseEstimate * 3 // 3x noise floor
		if v.config.Threshold < 100 {
			v.config.Threshold = 100 // Minimum threshold
		}
	}

	// Use parent implementation with potentially updated threshold
	v.mu.Unlock()
	result := v.EnergyVAD.Process(audioFrame)
	v.mu.Lock()

	return result
}

// VADProcessor wraps a VAD detector with additional processing capabilities.
type VADProcessor struct {
	detector      VADDetector
	onSpeechStart func()
	onSpeechEnd   func(duration time.Duration)
	speechStart   *time.Time
	mu            sync.RWMutex
}

// NewVADProcessor creates a new VAD processor with callbacks.
func NewVADProcessor(detector VADDetector) *VADProcessor {
	return &VADProcessor{
		detector: detector,
	}
}

// Process analyzes an audio frame.
func (p *VADProcessor) Process(audioFrame []byte) VADResult {
	result := p.detector.Process(audioFrame)

	p.mu.Lock()
	defer p.mu.Unlock()

	if result.SpeechStart {
		now := time.Now()
		p.speechStart = &now
		if p.onSpeechStart != nil {
			p.onSpeechStart()
		}
	}

	if result.SpeechEnd && p.speechStart != nil {
		duration := time.Since(*p.speechStart)
		p.speechStart = nil
		if p.onSpeechEnd != nil {
			p.onSpeechEnd(duration)
		}
	}

	return result
}

// SetOnSpeechStart sets the speech start callback.
func (p *VADProcessor) SetOnSpeechStart(fn func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onSpeechStart = fn
}

// SetOnSpeechEnd sets the speech end callback.
func (p *VADProcessor) SetOnSpeechEnd(fn func(duration time.Duration)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onSpeechEnd = fn
}

// Reset resets the processor.
func (p *VADProcessor) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.detector.Reset()
	p.speechStart = nil
}

// Detector returns the underlying detector.
func (p *VADProcessor) Detector() VADDetector {
	return p.detector
}
