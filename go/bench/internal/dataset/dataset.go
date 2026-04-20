// Package dataset loads benchmark test inputs (audio files, prompt lists, text samples).
package dataset

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AudioSample holds raw PCM16 audio data and its metadata.
type AudioSample struct {
	Name       string
	Data       []byte
	SampleRate int
	Channels   int
	DurationMs int64
}

// LoadPCM16File reads a raw PCM16 file from disk.
func LoadPCM16File(path string, sampleRate int, channels int) (*AudioSample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file %s: %w", path, err)
	}

	name := filepath.Base(path)
	samples := len(data) / (channels * 2) // 2 bytes per PCM16 sample
	durationMs := int64(float64(samples) / float64(sampleRate) * 1000)

	return &AudioSample{
		Name:       name,
		Data:       data,
		SampleRate: sampleRate,
		Channels:   channels,
		DurationMs: durationMs,
	}, nil
}

// LoadWAVFile reads a WAV file and extracts the PCM16 payload.
func LoadWAVFile(path string) (*AudioSample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAV file %s: %w", path, err)
	}

	if len(data) < 44 {
		return nil, fmt.Errorf("WAV file too small: %s", path)
	}

	// Check RIFF header
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid WAV header in %s", path)
	}

	// Parse format chunk
	// Find "fmt " subchunk
	fmtOffset := int(binary.LittleEndian.Uint32(data[16:20])) + 20
	if string(data[fmtOffset-4:fmtOffset]) != "fmt " {
		// Try standard offset
		if string(data[12:16]) != "fmt " {
			return nil, fmt.Errorf("fmt chunk not found in %s", path)
		}
		fmtOffset = 12
	}

	audioFormat := binary.LittleEndian.Uint16(data[fmtOffset+8 : fmtOffset+10])
	if audioFormat != 1 { // PCM
		return nil, fmt.Errorf("unsupported WAV format %d (only PCM supported) in %s", audioFormat, path)
	}

	channels := int(binary.LittleEndian.Uint16(data[fmtOffset+10 : fmtOffset+12]))
	sampleRate := int(binary.LittleEndian.Uint32(data[fmtOffset+12 : fmtOffset+16]))
	bitsPerSample := int(binary.LittleEndian.Uint16(data[fmtOffset+22 : fmtOffset+24]))

	if bitsPerSample != 16 {
		return nil, fmt.Errorf("unsupported bits per sample %d (only 16-bit supported) in %s", bitsPerSample, path)
	}

	// Find "data" subchunk
	dataOffset := -1
	for i := 36; i < len(data)-4; i++ {
		if string(data[i:i+4]) == "data" {
			dataOffset = i + 8 // skip "data" + 4-byte length
			break
		}
	}
	if dataOffset == -1 {
		return nil, fmt.Errorf("data chunk not found in %s", path)
	}

	audioData := data[dataOffset:]
	samples := len(audioData) / (channels * 2)
	durationMs := int64(float64(samples) / float64(sampleRate) * 1000)

	name := filepath.Base(path)
	return &AudioSample{
		Name:       name,
		Data:       audioData,
		SampleRate: sampleRate,
		Channels:   channels,
		DurationMs: durationMs,
	}, nil
}

// LoadAudioFile auto-detects the format (WAV or raw PCM16) based on extension.
func LoadAudioFile(path string) (*AudioSample, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return LoadWAVFile(path)
	case ".pcm", ".raw", ".bin":
		return LoadPCM16File(path, 16000, 1) // assume 16kHz mono for raw
	default:
		return LoadWAVFile(path) // try WAV first
	}
}

// PromptSample holds a text prompt for LLM benchmarking.
type PromptSample struct {
	Name string
	Text string
	Role string // "system", "user", "assistant"
}

// LoadPromptsFile reads a simple text file with one prompt per line.
// Lines starting with # are comments.
func LoadPromptsFile(path string) ([]PromptSample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompts file %s: %w", path, err)
	}

	var prompts []PromptSample
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prompts = append(prompts, PromptSample{
			Name: fmt.Sprintf("line_%d", i+1),
			Text: line,
			Role: "user",
		})
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf("no prompts found in %s", path)
	}
	return prompts, nil
}

// TextSample holds text for TTS benchmarking.
type TextSample struct {
	Name string
	Text string
}

// LoadTextsFile reads a simple text file with one text sample per line.
func LoadTextsFile(path string) ([]TextSample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read texts file %s: %w", path, err)
	}

	var texts []TextSample
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		texts = append(texts, TextSample{
			Name: fmt.Sprintf("line_%d", i+1),
			Text: line,
		})
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no text samples found in %s", path)
	}
	return texts, nil
}

// GenerateSilence generates PCM16 silence of the given duration.
func GenerateSilence(durationMs int, sampleRate int) []byte {
	samples := int(float64(sampleRate) * float64(durationMs) / 1000.0)
	return make([]byte, samples*2) // 2 bytes per sample, all zeros
}
