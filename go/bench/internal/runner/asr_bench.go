package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/parlona/cloudapp/bench/internal/dataset"
	"github.com/parlona/cloudapp/bench/internal/report"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/providers"
)

// ASRBench benchmarks an ASR provider.
type ASRBench struct {
	provider providers.ASRProvider
	opts     ASRBenchOpts
}

// ASRBenchOpts configures the ASR benchmark.
type ASRBenchOpts struct {
	Iterations   int
	Warmup       int
	Pacing       bool // if true, stream audio at real-time pace
	ChunkMs      int  // chunk duration in milliseconds (default 20)
	Language     string
	AudioFmt     contracts.AudioFormat
	DelayBetween time.Duration
}

// DefaultASRBenchOpts returns default ASR benchmark options.
func DefaultASRBenchOpts() ASRBenchOpts {
	return ASRBenchOpts{
		Iterations: 10,
		Warmup:     2,
		Pacing:     false,
		ChunkMs:    20,
		AudioFmt: contracts.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   contracts.PCM16,
		},
		DelayBetween: 500 * time.Millisecond,
	}
}

// NewASRBench creates a new ASR benchmark.
func NewASRBench(provider providers.ASRProvider, opts ASRBenchOpts) *ASRBench {
	return &ASRBench{provider: provider, opts: opts}
}

// Run executes the ASR benchmark and returns results.
func (b *ASRBench) Run(ctx context.Context, audio *dataset.AudioSample) ([]report.BenchResult, error) {
	var results []report.BenchResult
	totalRuns := b.opts.Warmup + b.opts.Iterations

	for i := 0; i < totalRuns; i++ {
		isWarmup := i < b.opts.Warmup
		result := b.runOnce(ctx, audio, i)
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

func (b *ASRBench) runOnce(ctx context.Context, audio *dataset.AudioSample, iter int) report.BenchResult {
	start := time.Now()
	audioStream := make(chan []byte, 100)

	asrOpts := providers.ASROptions{
		SessionID:    fmt.Sprintf("bench-asr-%d", iter),
		LanguageHint: b.opts.Language,
		AudioFormat:  b.opts.AudioFmt,
	}

	resultCh, err := b.provider.StreamRecognize(ctx, audioStream, asrOpts)
	if err != nil {
		return report.BenchResult{
			Stage:     "asr",
			Provider:  b.provider.Name(),
			Iter:      iter,
			StartTime: start,
			EndTime:   time.Now(),
			TotalMs:   time.Since(start).Milliseconds(),
			InputDesc: audio.Name,
			Error:     fmt.Sprintf("StreamRecognize failed: %v", err),
		}
	}

	// Stream audio chunks in a goroutine
	go func() {
		defer close(audioStream)
		chunkBytes := audio.SampleRate * audio.Channels * 2 * b.opts.ChunkMs / 1000
		if chunkBytes == 0 {
			chunkBytes = 640 // 20ms @ 16kHz mono PCM16
		}
		for offset := 0; offset < len(audio.Data); offset += chunkBytes {
			end := offset + chunkBytes
			if end > len(audio.Data) {
				end = len(audio.Data)
			}
			select {
			case <-ctx.Done():
				return
			case audioStream <- audio.Data[offset:end]:
			}
			if b.opts.Pacing {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(b.opts.ChunkMs) * time.Millisecond):
				}
			}
		}
	}()

	// Consume results
	var firstOutput time.Time
	var transcript string
	var resultErr string
	for res := range resultCh {
		if res.Error != nil {
			resultErr = res.Error.Error()
			continue
		}
		if firstOutput.IsZero() && (res.Transcript != "" || res.IsPartial) {
			firstOutput = time.Now()
		}
		if res.IsFinal {
			transcript = res.Transcript
		}
	}

	end := time.Now()
	ttftMs := int64(0)
	if !firstOutput.IsZero() {
		ttftMs = firstOutput.Sub(start).Milliseconds()
	}

	return report.BenchResult{
		Stage:           "asr",
		Provider:        b.provider.Name(),
		InputDesc:       audio.Name,
		StartTime:       start,
		FirstOutputTime: firstOutput,
		EndTime:         end,
		TTFTMs:          ttftMs,
		TotalMs:         end.Sub(start).Milliseconds(),
		OutputSize:      len(transcript),
		Error:           resultErr,
	}
}
