package audio

import (
	"fmt"
	"math"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// Normalizer converts audio from any format to the internal canonical format.
type Normalizer interface {
	// Normalize converts audio data from source profile to canonical format.
	Normalize(data []byte, from AudioProfile) ([]byte, error)

	// CanNormalize returns true if the normalizer can handle the given profile.
	CanNormalize(profile AudioProfile) bool
}

// PCM16Normalizer normalizes audio to PCM16 16kHz mono.
type PCM16Normalizer struct {
	resampler Resampler
}

// NewPCM16Normalizer creates a new PCM16 normalizer.
func NewPCM16Normalizer() *PCM16Normalizer {
	return &PCM16Normalizer{
		resampler: NewLinearResampler(),
	}
}

// Normalize converts audio data to the canonical format.
func (n *PCM16Normalizer) Normalize(data []byte, from AudioProfile) ([]byte, error) {
	if err := from.Validate(); err != nil {
		return nil, fmt.Errorf("invalid source profile: %w", err)
	}

	// If already canonical, just return
	if from.IsCanonical() {
		return data, nil
	}

	result := data

	// Step 1: Convert encoding to PCM16 if needed
	if from.Encoding != contracts.PCM16 {
		converted, err := n.convertEncoding(result, from.Encoding, contracts.PCM16)
		if err != nil {
			return nil, fmt.Errorf("encoding conversion failed: %w", err)
		}
		result = converted
		from.Encoding = contracts.PCM16
	}

	// Step 2: Convert channels to mono if needed
	if from.Channels > 1 {
		converted, err := n.convertToMono(result, from.Channels)
		if err != nil {
			return nil, fmt.Errorf("channel conversion failed: %w", err)
		}
		result = converted
		from.Channels = 1
	}

	// Step 3: Resample to 16kHz if needed
	if from.SampleRate != InternalProfile.SampleRate {
		converted, err := n.resampler.Resample(result, from.SampleRate, InternalProfile.SampleRate)
		if err != nil {
			return nil, fmt.Errorf("resampling failed: %w", err)
		}
		result = converted
	}

	return result, nil
}

// CanNormalize returns true for profiles we can handle.
func (n *PCM16Normalizer) CanNormalize(profile AudioProfile) bool {
	// We can handle PCM16, G.711, and basic conversions
	switch profile.Encoding {
	case contracts.PCM16, contracts.G711ULAW, contracts.G711ALAW:
		return profile.SampleRate >= 8000 && profile.SampleRate <= 48000
	default:
		return false
	}
}

// convertEncoding converts between audio encodings.
func (n *PCM16Normalizer) convertEncoding(data []byte, from, to contracts.AudioEncoding) ([]byte, error) {
	if from == to {
		return data, nil
	}

	// G.711 to PCM16
	if from == contracts.G711ULAW && to == contracts.PCM16 {
		return n.ulawToPCM16(data), nil
	}
	if from == contracts.G711ALAW && to == contracts.PCM16 {
		return n.alawToPCM16(data), nil
	}

	// PCM16 to G.711
	if from == contracts.PCM16 && to == contracts.G711ULAW {
		return n.pcm16ToUlaw(data), nil
	}
	if from == contracts.PCM16 && to == contracts.G711ALAW {
		return n.pcm16ToAlaw(data), nil
	}

	return nil, fmt.Errorf("unsupported encoding conversion: %v to %v", from, to)
}

// convertToMono converts multi-channel audio to mono.
func (n *PCM16Normalizer) convertToMono(data []byte, channels int) ([]byte, error) {
	if channels == 1 {
		return data, nil
	}

	// For PCM16, average samples across channels
	samples := len(data) / 2 // 2 bytes per sample
	monoSamples := samples / channels
	result := make([]byte, monoSamples*2)

	for i := 0; i < monoSamples; i++ {
		var sum int32
		for ch := 0; ch < channels; ch++ {
			idx := (i*channels + ch) * 2
			sample := int16(data[idx]) | int16(data[idx+1])<<8
			sum += int32(sample)
		}
		avg := int16(sum / int32(channels))
		result[i*2] = byte(avg)
		result[i*2+1] = byte(avg >> 8)
	}

	return result, nil
}

// ulawToPCM16 converts G.711 u-law to PCM16.
func (n *PCM16Normalizer) ulawToPCM16(data []byte) []byte {
	result := make([]byte, len(data)*2)
	for i, b := range data {
		sample := ulawDecodeTable[b]
		result[i*2] = byte(sample)
		result[i*2+1] = byte(sample >> 8)
	}
	return result
}

// alawToPCM16 converts G.711 A-law to PCM16.
func (n *PCM16Normalizer) alawToPCM16(data []byte) []byte {
	result := make([]byte, len(data)*2)
	for i, b := range data {
		sample := alawDecodeTable[b]
		result[i*2] = byte(sample)
		result[i*2+1] = byte(sample >> 8)
	}
	return result
}

// pcm16ToUlaw converts PCM16 to G.711 u-law.
func (n *PCM16Normalizer) pcm16ToUlaw(data []byte) []byte {
	result := make([]byte, len(data)/2)
	for i := range result {
		sample := int16(data[i*2]) | int16(data[i*2+1])<<8
		result[i] = pcm16ToUlaw(sample)
	}
	return result
}

// pcm16ToAlaw converts PCM16 to G.711 A-law.
func (n *PCM16Normalizer) pcm16ToAlaw(data []byte) []byte {
	result := make([]byte, len(data)/2)
	for i := range result {
		sample := int16(data[i*2]) | int16(data[i*2+1])<<8
		result[i] = pcm16ToAlaw(sample)
	}
	return result
}

// pcm16ToUlaw converts a single PCM16 sample to u-law.
func pcm16ToUlaw(sample int16) byte {
	const (
		bias = 0x84
		clip = 32635
	)

	s := int(sample)
	if s < 0 {
		s = clip - s
	} else {
		s = bias + s
	}

	if s > clip {
		s = clip
	}

	// Get the 8-bit output
	var u byte
	if s >= 0x4000 {
		u = 0x80 | byte((s>>8)&0x7F)
	} else if s >= 0x2000 {
		u = 0x70 | byte((s>>7)&0x7F)
	} else if s >= 0x1000 {
		u = 0x60 | byte((s>>6)&0x7F)
	} else if s >= 0x0800 {
		u = 0x50 | byte((s>>5)&0x7F)
	} else if s >= 0x0400 {
		u = 0x40 | byte((s>>4)&0x7F)
	} else if s >= 0x0200 {
		u = 0x30 | byte((s>>3)&0x7F)
	} else if s >= 0x0100 {
		u = 0x20 | byte((s>>2)&0x7F)
	} else {
		u = 0x10 | byte((s>>1)&0x7F)
	}

	return ^u
}

// pcm16ToAlaw converts a single PCM16 sample to A-law.
func pcm16ToAlaw(sample int16) byte {
	const (
		bias = 0x55
	)

	s := int(sample)
	sign := 0
	if s < 0 {
		sign = 0x80
		s = -s - 1
	}

	if s > 32767 {
		s = 32767
	}

	// Convert to A-law
	var a byte
	if s >= 256 {
		// Find the segment
		seg := 7
		for seg > 0 && s < (256<<uint(seg-1)) {
			seg--
		}
		a = byte(sign | (seg << 4) | ((s >> uint(seg+3)) & 0x0F))
	} else {
		a = byte(sign | (s >> 4))
	}

	return a ^ 0x55
}

// Decode tables for G.711
var (
	ulawDecodeTable [256]int16
	alawDecodeTable [256]int16
)

func init() {
	// Initialize u-law decode table
	for i := 0; i < 256; i++ {
		ulawDecodeTable[i] = ulawDecode(byte(i))
	}
	// Initialize A-law decode table
	for i := 0; i < 256; i++ {
		alawDecodeTable[i] = alawDecode(byte(i))
	}
}

func ulawDecode(u byte) int16 {
	u = ^u
	sign := int16(u & 0x80)
	segment := (u >> 4) & 0x07
	quant := u & 0x0F

	var sample int16
	if segment != 0 {
		sample = int16((0x21 + int(quant)) << (segment + 2))
	} else {
		sample = int16((int(quant) << 1) + 1)
	}

	if sign != 0 {
		sample = -sample
	}
	return sample
}

func alawDecode(a byte) int16 {
	a ^= 0x55
	sign := int16(a & 0x80)
	segment := (a >> 4) & 0x07
	quant := a & 0x0F

	var sample int16
	if segment != 0 {
		sample = int16((0x10 + int(quant)) << (segment + 3))
	} else {
		sample = int16((int(quant) << 4) + 8)
	}

	if sign != 0 {
		sample = -sample - 8
	} else {
		sample += 8
	}
	return sample
}

// NormalizeAudio is a convenience function to normalize audio to canonical format.
func NormalizeAudio(data []byte, from AudioProfile) ([]byte, error) {
	normalizer := NewPCM16Normalizer()
	return normalizer.Normalize(data, from)
}

// Clamp clamps a float64 value to int16 range.
func Clamp(val float64) int16 {
	if val > 32767 {
		return 32767
	}
	if val < -32768 {
		return -32768
	}
	return int16(val)
}

// RMS calculates the root mean square of audio samples.
func RMS(data []byte) float64 {
	if len(data) < 2 {
		return 0
	}

	var sum float64
	n := len(data) / 2

	for i := 0; i < len(data); i += 2 {
		sample := int16(data[i]) | int16(data[i+1])<<8
		sum += float64(sample) * float64(sample)
	}

	return math.Sqrt(sum / float64(n))
}

// dBFS converts a sample value to dBFS (decibels relative to full scale).
func dBFS(sample int16) float64 {
	if sample == 0 {
		return -96.0 // effectively silence
	}
	return 20 * math.Log10(math.Abs(float64(sample))/32768.0)
}
