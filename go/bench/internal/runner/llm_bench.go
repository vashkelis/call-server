package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/parlona/cloudapp/bench/internal/report"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/providers"
)

// LLMBench benchmarks an LLM provider.
type LLMBench struct {
	provider providers.LLMProvider
	opts     LLMBenchOpts
}

// LLMBenchOpts configures the LLM benchmark.
type LLMBenchOpts struct {
	Iterations   int
	Warmup       int
	MaxTokens    int32
	Temperature  float32
	TopP         float32
	ModelName    string
	SystemPrompt string
	DelayBetween time.Duration
}

// DefaultLLMBenchOpts returns default LLM benchmark options.
func DefaultLLMBenchOpts() LLMBenchOpts {
	return LLMBenchOpts{
		Iterations:   10,
		Warmup:       2,
		MaxTokens:    1024,
		Temperature:  0.7,
		TopP:         1.0,
		DelayBetween: 500 * time.Millisecond,
	}
}

// NewLLMBench creates a new LLM benchmark.
func NewLLMBench(provider providers.LLMProvider, opts LLMBenchOpts) *LLMBench {
	return &LLMBench{provider: provider, opts: opts}
}

// Run executes the LLM benchmark with the given prompt and returns results.
func (b *LLMBench) Run(ctx context.Context, prompt string) ([]report.BenchResult, error) {
	var results []report.BenchResult

	var messages []contracts.ChatMessage
	if b.opts.SystemPrompt != "" {
		messages = append(messages, contracts.ChatMessage{
			Role:    "system",
			Content: b.opts.SystemPrompt,
		})
	}
	messages = append(messages, contracts.ChatMessage{
		Role:    "user",
		Content: prompt,
	})

	totalRuns := b.opts.Warmup + b.opts.Iterations
	for i := 0; i < totalRuns; i++ {
		isWarmup := i < b.opts.Warmup
		result := b.runOnce(ctx, messages, i)
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

func (b *LLMBench) runOnce(ctx context.Context, messages []contracts.ChatMessage, iter int) report.BenchResult {
	start := time.Now()

	llmOpts := providers.LLMOptions{
		SessionID:   fmt.Sprintf("bench-llm-%d", iter),
		ModelName:   b.opts.ModelName,
		MaxTokens:   b.opts.MaxTokens,
		Temperature: b.opts.Temperature,
		TopP:        b.opts.TopP,
	}

	tokenCh, err := b.provider.StreamGenerate(ctx, messages, llmOpts)
	if err != nil {
		return report.BenchResult{
			Stage:     "llm",
			Provider:  b.provider.Name(),
			Iter:      iter,
			StartTime: start,
			EndTime:   time.Now(),
			TotalMs:   time.Since(start).Milliseconds(),
			Error:     fmt.Sprintf("StreamGenerate failed: %v", err),
		}
	}

	var firstToken time.Time
	var tokenCount int
	var fullText strings.Builder

	for token := range tokenCh {
		if token.Error != nil {
			continue
		}
		if firstToken.IsZero() {
			firstToken = time.Now()
		}
		tokenCount++
		fullText.WriteString(token.Token)
		if token.IsFinal {
			break
		}
	}

	end := time.Now()
	duration := end.Sub(start).Seconds()

	ttftMs := int64(0)
	if !firstToken.IsZero() {
		ttftMs = firstToken.Sub(start).Milliseconds()
	}

	var tps float64
	if duration > 0 {
		tps = float64(tokenCount) / duration
	}

	return report.BenchResult{
		Stage:           "llm",
		Provider:        b.provider.Name(),
		InputDesc:       truncate(promptFromMessages(messages), 50),
		StartTime:       start,
		FirstOutputTime: firstToken,
		EndTime:         end,
		TTFTMs:          ttftMs,
		TotalMs:         end.Sub(start).Milliseconds(),
		OutputSize:      tokenCount,
		TokensPerSec:    tps,
	}
}

func promptFromMessages(msgs []contracts.ChatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
