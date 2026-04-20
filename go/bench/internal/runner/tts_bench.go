package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/parlona/cloudapp/bench/internal/report"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/providers"
)

// TTSBench benchmarks a TTS provider.
type TTSBench struct {
	provider providers.TTSProvider
	opts     TTSBenchOpts
}

// TTSBenchOpts configures the TTS benchmark.
type TTSBenchOpts struct {
	Iterations   int
	Warmup       int
	VoiceID      string
	Speed        float32
	Pitch        float32
	AudioFmt     contracts.AudioFormat
	DelayBetween time.Duration
}

// DefaultTTSBenchOpts returns default TTS benchmark options.
func DefaultTTSBenchOpts() TTSBenchOpts {
	return TTSBenchOpts{
		Iterations: 10,
		Warmup:     2,
		Speed:      1.0,
		Pitch:      1.0,
		AudioFmt: contracts.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   contracts.PCM16,
		},
		DelayBetween: 500 * time.Millisecond,
	}
}

// NewTTSBench creates a new TTS benchmark.
func NewTTSBench(provider providers.TTSProvider, opts TTSBenchOpts) *TTSBench {
	return &TTSBench{provider: provider, opts: opts}
}

// Run executes the TTS benchmark with the given text and returns results.
func (b *TTSBench) Run(ctx context.Context, text string) ([]report.BenchResult, error) {
	var results []report.BenchResult
	totalRuns := b.opts.Warmup + b.opts.Iterations

	for i := 0; i < totalRuns; i++ {
		isWarmup := i < b.opts.Warmup
		result := b.runOnce(ctx, text, i)
		if isWarmup {
			continue
		}
		result.Iter = i - b.opts.Warmup + 1
		results = append(results, result)

		if i < totalRuns-1 && b.opts.DelayBetween > 0 {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			case <-time.After(b.opts.DelayBetween):
			}
		}
	}
	return results, nil
}

func (b *TTSBench) runOnce(ctx context.Context, text string, iter int) report.BenchResult {
	start := time.Now()

	ttsOpts := providers.TTSOptions{
		SessionID:   fmt.Sprintf("bench-tts-%d", iter),
		VoiceID:     b.opts.VoiceID,
		Speed:       b.opts.Speed,
		Pitch:       b.opts.Pitch,
		AudioFormat: b.opts.AudioFmt,
	}

	audioCh, err := b.provider.StreamSynthesize(ctx, text, ttsOpts)
	if err != nil {
		return report.BenchResult{
			Stage:     "tts",
			Provider:  b.provider.Name(),
			Iter:      iter,
			StartTime: start,
			EndTime:   time.Now(),
			TotalMs:   time.Since(start).Milliseconds(),
			InputDesc: truncate(text, 50),
			Error:     fmt.Sprintf("StreamSynthesize failed: %v", err),
		}
	}

	var firstChunk time.Time
	var totalBytes int
	for chunk := range audioCh {
		if firstChunk.IsZero() {
			firstChunk = time.Now()
		}
		totalBytes += len(chunk)
	}

	end := time.Now()
	ttftMs := int64(0)
	if !firstChunk.IsZero() {
		ttftMs = firstChunk.Sub(start).Milliseconds()
	}

	var rtf float64
	synthDur := end.Sub(start).Seconds()
	if synthDur > 0 && totalBytes > 0 {
		bytesPerSecond := float64(b.opts.AudioFmt.SampleRate) * float64(b.opts.AudioFmt.Channels) * 2.0
		audioDur := float64(totalBytes) / bytesPerSecond
		rtf = audioDur / synthDur
	}

	return report.BenchResult{
		Stage:           "tts",
		Provider:        b.provider.Name(),
		InputDesc:       truncate(text, 50),
		StartTime:       start,
		FirstOutputTime: firstChunk,
		EndTime:         end,
		TTFTMs:          ttftMs,
		TotalMs:         end.Sub(start).Milliseconds(),
		OutputSize:      totalBytes,
		RealTimeFactor:  rtf,
	}
}
