package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/parlona/cloudapp/bench/internal/dataset"
	"github.com/parlona/cloudapp/bench/internal/report"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/providers"
)

// ChainBench benchmarks the full ASR->LLM->TTS chain end-to-end.
type ChainBench struct {
	asrProvider providers.ASRProvider
	llmProvider providers.LLMProvider
	ttsProvider providers.TTSProvider
	opts        ChainBenchOpts
}

// ChainBenchOpts configures the chain benchmark.
type ChainBenchOpts struct {
	Iterations   int
	Warmup       int
	Pacing       bool
	ChunkMs      int
	Language     string
	SystemPrompt string
	ModelName    string
	MaxTokens    int32
	Temperature  float32
	TopP         float32
	VoiceID      string
	TTSSpeed     float32
	AudioFmt     contracts.AudioFormat
	DelayBetween time.Duration
}

// DefaultChainBenchOpts returns default chain benchmark options.
func DefaultChainBenchOpts() ChainBenchOpts {
	return ChainBenchOpts{
		Iterations:  5,
		Warmup:      1,
		Pacing:      false,
		ChunkMs:     20,
		MaxTokens:   1024,
		Temperature: 0.7,
		TopP:        1.0,
		TTSSpeed:    1.0,
		AudioFmt: contracts.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   contracts.PCM16,
		},
		DelayBetween: 1 * time.Second,
	}
}

// NewChainBench creates a new chain benchmark.
func NewChainBench(
	asr providers.ASRProvider,
	llm providers.LLMProvider,
	tts providers.TTSProvider,
	opts ChainBenchOpts,
) *ChainBench {
	return &ChainBench{
		asrProvider: asr,
		llmProvider: llm,
		ttsProvider: tts,
		opts:        opts,
	}
}

// ChainResult holds the result of a single chain benchmark iteration.
type ChainResult struct {
	ASR report.BenchResult
	LLM report.BenchResult
	TTS report.BenchResult

	ChainStart   time.Time
	ChainEnd     time.Time
	ChainTotalMs int64
	ChainTTFAms  int64 // audio-in -> first-audio-out
}

// Run executes the chain benchmark and returns per-stage + e2e results.
func (b *ChainBench) Run(ctx context.Context, audio *dataset.AudioSample) ([]ChainResult, error) {
	var results []ChainResult
	totalRuns := b.opts.Warmup + b.opts.Iterations

	for i := 0; i < totalRuns; i++ {
		isWarmup := i < b.opts.Warmup
		result := b.runOnce(ctx, audio, i)
		if isWarmup {
			continue
		}
		result.ASR.Iter = i - b.opts.Warmup + 1
		result.LLM.Iter = i - b.opts.Warmup + 1
		result.TTS.Iter = i - b.opts.Warmup + 1
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

func (b *ChainBench) runOnce(ctx context.Context, audio *dataset.AudioSample, iter int) ChainResult {
	chainStart := time.Now()
	sessionID := fmt.Sprintf("bench-chain-%d", iter)

	// Stage 1: ASR
	asrStart := time.Now()
	audioStream := make(chan []byte, 100)

	asrOpts := providers.ASROptions{
		SessionID:    sessionID,
		LanguageHint: b.opts.Language,
		AudioFormat:  b.opts.AudioFmt,
	}

	asrResultCh, err := b.asrProvider.StreamRecognize(ctx, audioStream, asrOpts)
	asrResult := report.BenchResult{
		Stage:     "asr",
		Provider:  b.asrProvider.Name(),
		Iter:      iter,
		StartTime: asrStart,
		InputDesc: audio.Name,
	}

	if err != nil {
		asrResult.Error = fmt.Sprintf("ASR StreamRecognize failed: %v", err)
		asrResult.EndTime = time.Now()
		asrResult.TotalMs = time.Since(asrStart).Milliseconds()
		return ChainResult{ASR: asrResult, ChainStart: chainStart, ChainEnd: time.Now(), ChainTotalMs: time.Since(chainStart).Milliseconds()}
	}

	go func() {
		defer close(audioStream)
		chunkBytes := audio.SampleRate * audio.Channels * 2 * b.opts.ChunkMs / 1000
		if chunkBytes == 0 {
			chunkBytes = 640
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

	var asrFirstOutput time.Time
	var transcript string
	for res := range asrResultCh {
		if res.Error != nil {
			asrResult.Error = res.Error.Error()
			continue
		}
		if asrFirstOutput.IsZero() && (res.Transcript != "" || res.IsPartial) {
			asrFirstOutput = time.Now()
		}
		if res.IsFinal {
			transcript = res.Transcript
		}
	}
	asrResult.EndTime = time.Now()
	asrResult.TotalMs = asrResult.EndTime.Sub(asrStart).Milliseconds()
	if !asrFirstOutput.IsZero() {
		asrResult.FirstOutputTime = asrFirstOutput
		asrResult.TTFTMs = asrFirstOutput.Sub(asrStart).Milliseconds()
	}
	asrResult.OutputSize = len(transcript)
	if transcript == "" && asrResult.Error == "" {
		asrResult.Error = "ASR produced empty transcript"
	}

	// Stage 2: LLM
	llmStart := time.Now()
	var messages []contracts.ChatMessage
	if b.opts.SystemPrompt != "" {
		messages = append(messages, contracts.ChatMessage{Role: "system", Content: b.opts.SystemPrompt})
	}
	messages = append(messages, contracts.ChatMessage{Role: "user", Content: transcript})

	llmOpts := providers.LLMOptions{
		SessionID:   sessionID,
		ModelName:   b.opts.ModelName,
		MaxTokens:   b.opts.MaxTokens,
		Temperature: b.opts.Temperature,
		TopP:        b.opts.TopP,
	}

	llmResult := report.BenchResult{
		Stage:     "llm",
		Provider:  b.llmProvider.Name(),
		Iter:      iter,
		StartTime: llmStart,
		InputDesc: truncate(transcript, 50),
	}

	tokenCh, err := b.llmProvider.StreamGenerate(ctx, messages, llmOpts)
	if err != nil {
		llmResult.Error = fmt.Sprintf("LLM StreamGenerate failed: %v", err)
		llmResult.EndTime = time.Now()
		llmResult.TotalMs = time.Since(llmStart).Milliseconds()
		chainEnd := time.Now()
		return ChainResult{ASR: asrResult, LLM: llmResult, ChainStart: chainStart, ChainEnd: chainEnd, ChainTotalMs: chainEnd.Sub(chainStart).Milliseconds()}
	}

	var llmFirstToken time.Time
	var tokenCount int
	var fullText strings.Builder
	for token := range tokenCh {
		if token.Error != nil {
			continue
		}
		if llmFirstToken.IsZero() {
			llmFirstToken = time.Now()
		}
		tokenCount++
		fullText.WriteString(token.Token)
		if token.IsFinal {
			break
		}
	}
	llmResult.EndTime = time.Now()
	llmResult.TotalMs = llmResult.EndTime.Sub(llmStart).Milliseconds()
	if !llmFirstToken.IsZero() {
		llmResult.FirstOutputTime = llmFirstToken
		llmResult.TTFTMs = llmFirstToken.Sub(llmStart).Milliseconds()
	}
	llmResult.OutputSize = tokenCount
	llmDur := llmResult.EndTime.Sub(llmStart).Seconds()
	if llmDur > 0 {
		llmResult.TokensPerSec = float64(tokenCount) / llmDur
	}
	generatedText := fullText.String()
	if generatedText == "" && llmResult.Error == "" {
		llmResult.Error = "LLM produced empty response"
	}

	// Stage 3: TTS
	ttsStart := time.Now()
	ttsOpts := providers.TTSOptions{
		SessionID:   sessionID,
		VoiceID:     b.opts.VoiceID,
		Speed:       b.opts.TTSSpeed,
		AudioFormat: b.opts.AudioFmt,
	}

	ttsResult := report.BenchResult{
		Stage:     "tts",
		Provider:  b.ttsProvider.Name(),
		Iter:      iter,
		StartTime: ttsStart,
		InputDesc: truncate(generatedText, 50),
	}

	audioCh, err := b.ttsProvider.StreamSynthesize(ctx, generatedText, ttsOpts)
	if err != nil {
		ttsResult.Error = fmt.Sprintf("TTS StreamSynthesize failed: %v", err)
		ttsResult.EndTime = time.Now()
		ttsResult.TotalMs = time.Since(ttsStart).Milliseconds()
		chainEnd := time.Now()
		return ChainResult{ASR: asrResult, LLM: llmResult, TTS: ttsResult, ChainStart: chainStart, ChainEnd: chainEnd, ChainTotalMs: chainEnd.Sub(chainStart).Milliseconds()}
	}

	var ttsFirstChunk time.Time
	var totalBytes int
	for chunk := range audioCh {
		if ttsFirstChunk.IsZero() {
			ttsFirstChunk = time.Now()
		}
		totalBytes += len(chunk)
	}
	ttsResult.EndTime = time.Now()
	ttsResult.TotalMs = ttsResult.EndTime.Sub(ttsStart).Milliseconds()
	if !ttsFirstChunk.IsZero() {
		ttsResult.FirstOutputTime = ttsFirstChunk
		ttsResult.TTFTMs = ttsFirstChunk.Sub(ttsStart).Milliseconds()
	}
	ttsResult.OutputSize = totalBytes
	synthDur := ttsResult.EndTime.Sub(ttsStart).Seconds()
	if synthDur > 0 && totalBytes > 0 {
		bytesPerSecond := float64(b.opts.AudioFmt.SampleRate) * float64(b.opts.AudioFmt.Channels) * 2.0
		audioDur := float64(totalBytes) / bytesPerSecond
		ttsResult.RealTimeFactor = audioDur / synthDur
	}

	chainEnd := time.Now()
	ttfaMs := int64(0)
	if !ttsFirstChunk.IsZero() {
		ttfaMs = ttsFirstChunk.Sub(chainStart).Milliseconds()
	}

	return ChainResult{
		ASR:          asrResult,
		LLM:          llmResult,
		TTS:          ttsResult,
		ChainStart:   chainStart,
		ChainEnd:     chainEnd,
		ChainTotalMs: chainEnd.Sub(chainStart).Milliseconds(),
		ChainTTFAms:  ttfaMs,
	}
}

// ChainResultsToBenchResults flattens ChainResult slice into individual BenchResult entries.
func ChainResultsToBenchResults(chainResults []ChainResult) []report.BenchResult {
	var results []report.BenchResult
	for _, cr := range chainResults {
		results = append(results, cr.ASR, cr.LLM, cr.TTS)
	}
	return results
}
