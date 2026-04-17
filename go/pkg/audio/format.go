// Package audio provides audio processing utilities including format conversion,
// resampling, chunking, buffering, and playout tracking.
package audio

import (
	"fmt"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// AudioProfile defines the characteristics of an audio stream.
type AudioProfile struct {
	SampleRate int                     `json:"sample_rate"`
	Channels   int                     `json:"channels"`
	Encoding   contracts.AudioEncoding `json:"encoding"`
	FrameSize  int                     `json:"frame_size"` // samples per frame
}

// BytesPerSample returns the number of bytes per sample based on encoding.
func (p *AudioProfile) BytesPerSample() int {
	switch p.Encoding {
	case contracts.PCM16:
		return 2
	case contracts.G711ULAW, contracts.G711ALAW:
		return 1
	default:
		return 2 // default to PCM16
	}
}

// BytesPerFrame returns the number of bytes per frame.
func (p *AudioProfile) BytesPerFrame() int {
	return p.FrameSize * p.Channels * p.BytesPerSample()
}

// DurationFromBytes calculates the duration represented by a byte count.
func (p *AudioProfile) DurationFromBytes(bytes int) float64 {
	if p.SampleRate == 0 || p.Channels == 0 {
		return 0
	}
	samples := bytes / (p.BytesPerSample() * p.Channels)
	return float64(samples) / float64(p.SampleRate)
}

// BytesFromDuration calculates the byte count for a given duration in seconds.
func (p *AudioProfile) BytesFromDuration(seconds float64) int {
	samples := int(seconds * float64(p.SampleRate))
	return samples * p.Channels * p.BytesPerSample()
}

// Validate checks if the audio profile is valid.
func (p *AudioProfile) Validate() error {
	if p.SampleRate <= 0 {
		return fmt.Errorf("invalid sample rate: %d", p.SampleRate)
	}
	if p.Channels <= 0 {
		return fmt.Errorf("invalid channel count: %d", p.Channels)
	}
	if p.FrameSize <= 0 {
		p.FrameSize = p.SampleRate / 100 // 10ms default
	}
	return nil
}

// Canonical profile: 16kHz mono PCM16 - used internally for ASR/VAD
var InternalProfile = AudioProfile{
	SampleRate: 16000,
	Channels:   1,
	Encoding:   contracts.PCM16,
	FrameSize:  160, // 10ms at 16kHz
}

// TelephonyProfile: 16kHz mono PCM16 - standard telephony quality
var TelephonyProfile = AudioProfile{
	SampleRate: 16000,
	Channels:   1,
	Encoding:   contracts.PCM16,
	FrameSize:  160, // 10ms at 16kHz
}

// Telephony8kProfile: 8kHz mono PCM16 - legacy telephony
var Telephony8kProfile = AudioProfile{
	SampleRate: 8000,
	Channels:   1,
	Encoding:   contracts.PCM16,
	FrameSize:  80, // 10ms at 8kHz
}

// WebRTCProfile: 48kHz mono PCM16 - high quality for WebRTC/game
var WebRTCProfile = AudioProfile{
	SampleRate: 48000,
	Channels:   1,
	Encoding:   contracts.PCM16,
	FrameSize:  480, // 10ms at 48kHz
}

// IsCanonical returns true if the profile matches the internal canonical format.
func (p *AudioProfile) IsCanonical() bool {
	return p.SampleRate == InternalProfile.SampleRate &&
		p.Channels == InternalProfile.Channels &&
		p.Encoding == InternalProfile.Encoding
}

// ToContract converts the audio profile to a contract AudioFormat.
func (p *AudioProfile) ToContract() contracts.AudioFormat {
	return contracts.AudioFormat{
		SampleRate: int32(p.SampleRate),
		Channels:   int32(p.Channels),
		Encoding:   p.Encoding,
	}
}

// ProfileFromContract creates an AudioProfile from a contract AudioFormat.
func ProfileFromContract(format contracts.AudioFormat) AudioProfile {
	return AudioProfile{
		SampleRate: int(format.SampleRate),
		Channels:   int(format.Channels),
		Encoding:   format.Encoding,
		FrameSize:  int(format.SampleRate) / 100, // 10ms default
	}
}

// SampleRateFromString parses common sample rate strings.
func SampleRateFromString(s string) int {
	switch s {
	case "8k", "8000":
		return 8000
	case "16k", "16000":
		return 16000
	case "22k", "22050":
		return 22050
	case "44k", "44100":
		return 44100
	case "48k", "48000":
		return 48000
	default:
		return 16000 // default
	}
}
