package audio

import (
	"fmt"
	"math"
)

// Resampler converts audio between different sample rates.
type Resampler interface {
	// Resample converts audio from one sample rate to another.
	Resample(data []byte, fromRate, toRate int) ([]byte, error)

	// SupportedRates returns the sample rates this resampler can handle.
	SupportedRates() []int
}

// LinearResampler implements simple linear interpolation resampling.
// This is suitable for MVP but production should use higher quality resampling.
type LinearResampler struct{}

// NewLinearResampler creates a new linear interpolation resampler.
func NewLinearResampler() *LinearResampler {
	return &LinearResampler{}
}

// Resample converts audio using linear interpolation.
func (r *LinearResampler) Resample(data []byte, fromRate, toRate int) ([]byte, error) {
	if fromRate == toRate {
		return data, nil
	}

	if fromRate <= 0 || toRate <= 0 {
		return nil, fmt.Errorf("invalid sample rates: from=%d, to=%d", fromRate, toRate)
	}

	if len(data)%2 != 0 {
		return nil, fmt.Errorf("data length must be even for PCM16")
	}

	// Convert bytes to samples
	samples := bytesToSamples(data)
	if len(samples) == 0 {
		return []byte{}, nil
	}

	// Calculate output length
	ratio := float64(toRate) / float64(fromRate)
	outLen := int(float64(len(samples)) * ratio)
	if outLen == 0 {
		return []byte{}, nil
	}

	// Resample using linear interpolation
	result := make([]int16, outLen)
	for i := 0; i < outLen; i++ {
		srcPos := float64(i) / ratio
		result[i] = r.interpolate(samples, srcPos)
	}

	return samplesToBytes(result), nil
}

// SupportedRates returns commonly supported rates for linear resampling.
func (r *LinearResampler) SupportedRates() []int {
	return []int{8000, 16000, 22050, 44100, 48000}
}

// interpolate performs linear interpolation at the given position.
func (r *LinearResampler) interpolate(samples []int16, pos float64) int16 {
	if pos <= 0 {
		return samples[0]
	}
	if pos >= float64(len(samples)-1) {
		return samples[len(samples)-1]
	}

	i := int(pos)
	t := pos - float64(i)

	s1 := float64(samples[i])
	s2 := float64(samples[i+1])

	return int16(s1 + t*(s2-s1))
}

// bytesToSamples converts byte slice to int16 samples.
func bytesToSamples(data []byte) []int16 {
	samples := make([]int16, len(data)/2)
	for i := range samples {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}
	return samples
}

// samplesToBytes converts int16 samples to byte slice.
func samplesToBytes(samples []int16) []byte {
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		data[i*2] = byte(s)
		data[i*2+1] = byte(s >> 8)
	}
	return data
}

// Resample8kTo16k resamples 8kHz audio to 16kHz.
func Resample8kTo16k(data []byte) ([]byte, error) {
	resampler := NewLinearResampler()
	return resampler.Resample(data, 8000, 16000)
}

// Resample48kTo16k resamples 48kHz audio to 16kHz.
func Resample48kTo16k(data []byte) ([]byte, error) {
	resampler := NewLinearResampler()
	return resampler.Resample(data, 48000, 16000)
}

// Resample16kTo8k resamples 16kHz audio to 8kHz.
func Resample16kTo8k(data []byte) ([]byte, error) {
	resampler := NewLinearResampler()
	return resampler.Resample(data, 16000, 8000)
}

// Resample16kTo48k resamples 16kHz audio to 48kHz.
func Resample16kTo48k(data []byte) ([]byte, error) {
	resampler := NewLinearResampler()
	return resampler.Resample(data, 16000, 48000)
}

// SincResampler implements higher quality sinc interpolation resampling.
// This is a placeholder for future improvement.
type SincResampler struct {
	windowSize int
}

// NewSincResampler creates a new sinc resampler.
func NewSincResampler(windowSize int) *SincResampler {
	if windowSize <= 0 {
		windowSize = 32
	}
	return &SincResampler{windowSize: windowSize}
}

// Resample converts audio using sinc interpolation.
func (r *SincResampler) Resample(data []byte, fromRate, toRate int) ([]byte, error) {
	// TODO: Implement proper sinc resampling
	// For now, fall back to linear resampling
	linear := NewLinearResampler()
	return linear.Resample(data, fromRate, toRate)
}

// SupportedRates returns commonly supported rates for sinc resampling.
func (r *SincResampler) SupportedRates() []int {
	return []int{8000, 16000, 22050, 32000, 44100, 48000, 96000}
}

// sinc function: sin(pi*x) / (pi*x)
func sinc(x float64) float64 {
	if math.Abs(x) < 1e-10 {
		return 1.0
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

// blackmanWindow applies a Blackman window function.
func blackmanWindow(n, size int) float64 {
	alpha := 0.16
	a0 := (1 - alpha) / 2
	a1 := 0.5
	a2 := alpha / 2
	x := float64(n) / float64(size-1)
	return a0 - a1*math.Cos(2*math.Pi*x) + a2*math.Cos(4*math.Pi*x)
}
