// Package main provides the bench CLI tool for measuring ASR/LLM/TTS provider latencies.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/parlona/cloudapp/bench/internal/dataset"
	"github.com/parlona/cloudapp/bench/internal/report"
	"github.com/parlona/cloudapp/bench/internal/runner"
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/providers"
)

const usage = `CloudApp Bench — Latency benchmarking tool for ASR, LLM, and TTS providers

Usage:
  bench <command> [flags]

Commands:
  asr    Benchmark ASR provider (audio -> text)
  llm    Benchmark LLM provider (text -> text)
  tts    Benchmark TTS provider (text -> audio)
  chain  Benchmark full ASR->LLM->TTS chain end-to-end

Global Flags:
  --gateway <addr>       Provider gateway gRPC address (default: localhost:50051)
  --iterations <n>       Number of benchmark iterations (default: 10)
  --warmup <n>           Number of warmup iterations to discard (default: 2)
  --delay <ms>           Delay between iterations in ms (default: 500)
  --json <file>          Write JSON report to file
  --csv <file>           Write CSV report to file
  --markdown <file>      Write Markdown report to file
  --quiet                Only print summary, not per-iteration results

ASR Flags:
  --audio <path>         Path to WAV/PCM audio file to transcribe
  --pacing               Stream audio at real-time pace (default: send as fast as possible)
  --chunk-ms <n>         Audio chunk size in milliseconds (default: 20)
  --language <lang>      Language hint (e.g. "en-US")

LLM Flags:
  --prompt <text>        Prompt text to send (or use --prompts-file)
  --prompts-file <path>  File with one prompt per line
  --system-prompt <text> System prompt prefix
  --model <name>         Model name to use
  --max-tokens <n>       Max tokens to generate (default: 1024)
  --temperature <f>      Sampling temperature (default: 0.7)

TTS Flags:
  --text <text>          Text to synthesize (or use --texts-file)
  --texts-file <path>    File with one text sample per line
  --voice <id>           Voice ID to use
  --speed <f>            Speech speed factor (default: 1.0)

Chain Flags:
  --audio <path>         Path to WAV/PCM audio file (required)
  --system-prompt <text> System prompt for LLM stage
  --model <name>         Model name for LLM stage
  --voice <id>           Voice ID for TTS stage
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "asr":
		runASR(args)
	case "llm":
		runLLM(args)
	case "tts":
		runTTS(args)
	case "chain":
		runChain(args)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Print(usage)
		os.Exit(1)
	}
}

// flags holds all parsed CLI flags.
type flags struct {
	Gateway      string
	Iterations   int
	Warmup       int
	DelayMs      int
	JSONFile     string
	CSVFile      string
	MarkdownFile string
	Quiet        bool

	// ASR
	AudioPath string
	Pacing    bool
	ChunkMs   int
	Language  string

	// LLM
	Prompt       string
	PromptsFile  string
	SystemPrompt string
	ModelName    string
	MaxTokens    int
	Temperature  float64

	// TTS
	Text      string
	TextsFile string
	VoiceID   string
	Speed     float64
}

func parseFlags(args []string) flags {
	f := flags{
		Gateway:     "localhost:50051",
		Iterations:  10,
		Warmup:      2,
		DelayMs:     500,
		ChunkMs:     20,
		MaxTokens:   1024,
		Temperature: 0.7,
		Speed:       1.0,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--gateway":
			f.Gateway = nextArg(args, &i)
		case "--iterations":
			f.Iterations = parseInt(nextArg(args, &i), 10)
		case "--warmup":
			f.Warmup = parseInt(nextArg(args, &i), 2)
		case "--delay":
			f.DelayMs = parseInt(nextArg(args, &i), 500)
		case "--json":
			f.JSONFile = nextArg(args, &i)
		case "--csv":
			f.CSVFile = nextArg(args, &i)
		case "--markdown":
			f.MarkdownFile = nextArg(args, &i)
		case "--quiet":
			f.Quiet = true
		case "--audio":
			f.AudioPath = nextArg(args, &i)
		case "--pacing":
			f.Pacing = true
		case "--chunk-ms":
			f.ChunkMs = parseInt(nextArg(args, &i), 20)
		case "--language":
			f.Language = nextArg(args, &i)
		case "--prompt":
			f.Prompt = nextArg(args, &i)
		case "--prompts-file":
			f.PromptsFile = nextArg(args, &i)
		case "--system-prompt":
			f.SystemPrompt = nextArg(args, &i)
		case "--model":
			f.ModelName = nextArg(args, &i)
		case "--max-tokens":
			f.MaxTokens = parseInt(nextArg(args, &i), 1024)
		case "--temperature":
			f.Temperature = parseFloat(nextArg(args, &i), 0.7)
		case "--text":
			f.Text = nextArg(args, &i)
		case "--texts-file":
			f.TextsFile = nextArg(args, &i)
		case "--voice":
			f.VoiceID = nextArg(args, &i)
		case "--speed":
			f.Speed = parseFloat(nextArg(args, &i), 1.0)
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}
	return f
}

func nextArg(args []string, i *int) string {
	*i++
	if *i >= len(args) {
		fmt.Fprintf(os.Stderr, "Flag %s requires a value\n", args[*i-1])
		os.Exit(1)
	}
	return args[*i]
}

func parseInt(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseFloat(s string, def float64) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	return ctx
}

func createASRProvider(gateway string) providers.ASRProvider {
	p, err := providers.NewGRPCASRProvider("bench-asr", providers.GRPCClientConfig{
		Address: gateway,
		Timeout: 120,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create ASR provider: %v\n", err)
		os.Exit(1)
	}
	return p
}

func createLLMProvider(gateway string) providers.LLMProvider {
	p, err := providers.NewGRPCLLMProvider("bench-llm", providers.GRPCClientConfig{
		Address: gateway,
		Timeout: 120,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create LLM provider: %v\n", err)
		os.Exit(1)
	}
	return p
}

func createTTSProvider(gateway string) providers.TTSProvider {
	p, err := providers.NewGRPCTTSProvider("bench-tts", providers.GRPCClientConfig{
		Address: gateway,
		Timeout: 120,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create TTS provider: %v\n", err)
		os.Exit(1)
	}
	return p
}

func writeReport(r *report.BenchReport, f flags) {
	if !f.Quiet {
		fmt.Println(r.FormatMarkdown())
	} else {
		for _, s := range r.Summaries {
			fmt.Printf("[%s] %s — TTFT: mean=%dms p50=%dms p95=%dms | Total: mean=%dms p50=%dms p95=%dms | errors=%d/%d\n",
				s.Stage, s.Provider,
				s.MeanTTFTMs, s.P50TTFTMs, s.P95TTFTMs,
				s.MeanTotalMs, s.P50TotalMs, s.P95TotalMs,
				s.Errors, s.Iterations)
		}
	}

	if f.JSONFile != "" {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		} else if err := os.WriteFile(f.JSONFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write JSON: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "JSON report written to %s\n", f.JSONFile)
		}
	}

	if f.CSVFile != "" {
		if err := os.WriteFile(f.CSVFile, []byte(r.FormatCSV()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write CSV: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "CSV report written to %s\n", f.CSVFile)
		}
	}

	if f.MarkdownFile != "" {
		if err := os.WriteFile(f.MarkdownFile, []byte(r.FormatMarkdown()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write Markdown: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Markdown report written to %s\n", f.MarkdownFile)
		}
	}
}

func runASR(args []string) {
	f := parseFlags(args)
	ctx := signalContext()

	if f.AudioPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --audio flag is required for ASR benchmark")
		os.Exit(1)
	}

	audio, err := dataset.LoadAudioFile(f.AudioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading audio: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Audio: %s (%dms, %dHz, %dch, %d bytes)\n",
		audio.Name, audio.DurationMs, audio.SampleRate, audio.Channels, len(audio.Data))

	provider := createASRProvider(f.Gateway)
	fmt.Fprintf(os.Stderr, "Gateway: %s\n\n", f.Gateway)

	opts := runner.ASRBenchOpts{
		Iterations:   f.Iterations,
		Warmup:       f.Warmup,
		Pacing:       f.Pacing,
		ChunkMs:      f.ChunkMs,
		Language:     f.Language,
		AudioFmt:     contracts.AudioFormat{SampleRate: 16000, Channels: 1, Encoding: contracts.PCM16},
		DelayBetween: time.Duration(f.DelayMs) * time.Millisecond,
	}

	bench := runner.NewASRBench(provider, opts)
	fmt.Fprintf(os.Stderr, "Running ASR benchmark (%d iterations, %d warmup)...\n", f.Iterations, f.Warmup)

	results, err := bench.Run(ctx, audio)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
	}

	for i := range results {
		r := &results[i]
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "  iter %d: ERROR %s\n", r.Iter, r.Error)
		} else {
			fmt.Fprintf(os.Stderr, "  iter %d: TTFT=%dms Total=%dms transcript_len=%d\n",
				r.Iter, r.TTFTMs, r.TotalMs, r.OutputSize)
		}
	}

	rpt := report.NewBenchReport(results)
	writeReport(rpt, f)
}

func runLLM(args []string) {
	f := parseFlags(args)
	ctx := signalContext()

	if f.Prompt == "" && f.PromptsFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --prompt or --prompts-file is required for LLM benchmark")
		os.Exit(1)
	}

	var prompts []string
	if f.PromptsFile != "" {
		samples, err := dataset.LoadPromptsFile(f.PromptsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading prompts: %v\n", err)
			os.Exit(1)
		}
		for _, s := range samples {
			prompts = append(prompts, s.Text)
		}
	} else {
		prompts = []string{f.Prompt}
	}

	provider := createLLMProvider(f.Gateway)
	fmt.Fprintf(os.Stderr, "Gateway: %s\n\n", f.Gateway)

	var allResults []report.BenchResult

	for pIdx, prompt := range prompts {
		if len(prompts) > 1 {
			fmt.Fprintf(os.Stderr, "Prompt %d/%d: %q\n", pIdx+1, len(prompts), truncateStr(prompt, 60))
		}

		opts := runner.LLMBenchOpts{
			Iterations:   f.Iterations,
			Warmup:       f.Warmup,
			MaxTokens:    int32(f.MaxTokens),
			Temperature:  float32(f.Temperature),
			TopP:         1.0,
			ModelName:    f.ModelName,
			SystemPrompt: f.SystemPrompt,
			DelayBetween: time.Duration(f.DelayMs) * time.Millisecond,
		}

		bench := runner.NewLLMBench(provider, opts)
		fmt.Fprintf(os.Stderr, "Running LLM benchmark (%d iterations, %d warmup)...\n", f.Iterations, f.Warmup)

		results, err := bench.Run(ctx, prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
		}

		for i := range results {
			r := &results[i]
			if r.Error != "" {
				fmt.Fprintf(os.Stderr, "  iter %d: ERROR %s\n", r.Iter, r.Error)
			} else {
				fmt.Fprintf(os.Stderr, "  iter %d: TTFT=%dms Total=%dms tokens=%d tps=%.1f\n",
					r.Iter, r.TTFTMs, r.TotalMs, r.OutputSize, r.TokensPerSec)
			}
		}

		allResults = append(allResults, results...)
	}

	rpt := report.NewBenchReport(allResults)
	writeReport(rpt, f)
}

func runTTS(args []string) {
	f := parseFlags(args)
	ctx := signalContext()

	if f.Text == "" && f.TextsFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --text or --texts-file is required for TTS benchmark")
		os.Exit(1)
	}

	var texts []string
	if f.TextsFile != "" {
		samples, err := dataset.LoadTextsFile(f.TextsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading texts: %v\n", err)
			os.Exit(1)
		}
		for _, s := range samples {
			texts = append(texts, s.Text)
		}
	} else {
		texts = []string{f.Text}
	}

	provider := createTTSProvider(f.Gateway)
	fmt.Fprintf(os.Stderr, "Gateway: %s\n\n", f.Gateway)

	var allResults []report.BenchResult

	for tIdx, text := range texts {
		if len(texts) > 1 {
			fmt.Fprintf(os.Stderr, "Text %d/%d: %q\n", tIdx+1, len(texts), truncateStr(text, 60))
		}

		opts := runner.TTSBenchOpts{
			Iterations:   f.Iterations,
			Warmup:       f.Warmup,
			VoiceID:      f.VoiceID,
			Speed:        float32(f.Speed),
			AudioFmt:     contracts.AudioFormat{SampleRate: 16000, Channels: 1, Encoding: contracts.PCM16},
			DelayBetween: time.Duration(f.DelayMs) * time.Millisecond,
		}

		bench := runner.NewTTSBench(provider, opts)
		fmt.Fprintf(os.Stderr, "Running TTS benchmark (%d iterations, %d warmup)...\n", f.Iterations, f.Warmup)

		results, err := bench.Run(ctx, text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
		}

		for i := range results {
			r := &results[i]
			if r.Error != "" {
				fmt.Fprintf(os.Stderr, "  iter %d: ERROR %s\n", r.Iter, r.Error)
			} else {
				fmt.Fprintf(os.Stderr, "  iter %d: TTFT=%dms Total=%dms bytes=%d rtf=%.4f\n",
					r.Iter, r.TTFTMs, r.TotalMs, r.OutputSize, r.RealTimeFactor)
			}
		}

		allResults = append(allResults, results...)
	}

	rpt := report.NewBenchReport(allResults)
	writeReport(rpt, f)
}

func runChain(args []string) {
	f := parseFlags(args)
	ctx := signalContext()

	if f.AudioPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --audio flag is required for chain benchmark")
		os.Exit(1)
	}

	audio, err := dataset.LoadAudioFile(f.AudioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading audio: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Audio: %s (%dms, %dHz, %dch, %d bytes)\n",
		audio.Name, audio.DurationMs, audio.SampleRate, audio.Channels, len(audio.Data))

	asrProvider := createASRProvider(f.Gateway)
	llmProvider := createLLMProvider(f.Gateway)
	ttsProvider := createTTSProvider(f.Gateway)
	fmt.Fprintf(os.Stderr, "Gateway: %s\n\n", f.Gateway)

	opts := runner.ChainBenchOpts{
		Iterations:   f.Iterations,
		Warmup:       f.Warmup,
		Pacing:       f.Pacing,
		ChunkMs:      f.ChunkMs,
		Language:     f.Language,
		SystemPrompt: f.SystemPrompt,
		ModelName:    f.ModelName,
		MaxTokens:    int32(f.MaxTokens),
		Temperature:  float32(f.Temperature),
		VoiceID:      f.VoiceID,
		TTSSpeed:     float32(f.Speed),
		AudioFmt:     contracts.AudioFormat{SampleRate: 16000, Channels: 1, Encoding: contracts.PCM16},
		DelayBetween: time.Duration(f.DelayMs) * time.Millisecond,
	}

	bench := runner.NewChainBench(asrProvider, llmProvider, ttsProvider, opts)
	fmt.Fprintf(os.Stderr, "Running chain benchmark (%d iterations, %d warmup)...\n", f.Iterations, f.Warmup)

	chainResults, err := bench.Run(ctx, audio)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
	}

	for i := range chainResults {
		cr := &chainResults[i]
		fmt.Fprintf(os.Stderr, "  iter %d: ASR=%dms | LLM=%dms (TTFT=%dms, tps=%.1f) | TTS=%dms (RTF=%.4f) | Chain=%dms TTFA=%dms\n",
			cr.ASR.Iter,
			cr.ASR.TotalMs,
			cr.LLM.TotalMs, cr.LLM.TTFTMs, cr.LLM.TokensPerSec,
			cr.TTS.TotalMs, cr.TTS.RealTimeFactor,
			cr.ChainTotalMs, cr.ChainTTFAms)
		if cr.ASR.Error != "" {
			fmt.Fprintf(os.Stderr, "    ASR error: %s\n", cr.ASR.Error)
		}
		if cr.LLM.Error != "" {
			fmt.Fprintf(os.Stderr, "    LLM error: %s\n", cr.LLM.Error)
		}
		if cr.TTS.Error != "" {
			fmt.Fprintf(os.Stderr, "    TTS error: %s\n", cr.TTS.Error)
		}
	}

	allResults := runner.ChainResultsToBenchResults(chainResults)
	rpt := report.NewBenchReport(allResults)

	chainSummary := formatChainSummary(chainResults)
	if !f.Quiet {
		fmt.Println(rpt.FormatMarkdown())
		fmt.Println(chainSummary)
	} else {
		for _, s := range rpt.Summaries {
			fmt.Printf("[%s] %s — TTFT: mean=%dms p50=%dms | Total: mean=%dms | errors=%d/%d\n",
				s.Stage, s.Provider, s.MeanTTFTMs, s.P50TTFTMs, s.MeanTotalMs, s.Errors, s.Iterations)
		}
		fmt.Print(chainSummary)
	}

	if f.JSONFile != "" {
		type chainReport struct {
			report.BenchReport
			ChainResults []runner.ChainResult `json:"chain_results"`
		}
		cr := chainReport{
			BenchReport:  *rpt,
			ChainResults: chainResults,
		}
		data, err := json.MarshalIndent(cr, "", "  ")
		if err == nil {
			os.WriteFile(f.JSONFile, data, 0644)
			fmt.Fprintf(os.Stderr, "JSON report written to %s\n", f.JSONFile)
		}
	}
	if f.CSVFile != "" {
		os.WriteFile(f.CSVFile, []byte(rpt.FormatCSV()), 0644)
		fmt.Fprintf(os.Stderr, "CSV report written to %s\n", f.CSVFile)
	}
	if f.MarkdownFile != "" {
		os.WriteFile(f.MarkdownFile, []byte(rpt.FormatMarkdown()+"\n"+chainSummary+"\n"), 0644)
		fmt.Fprintf(os.Stderr, "Markdown report written to %s\n", f.MarkdownFile)
	}
}

func formatChainSummary(results []runner.ChainResult) string {
	if len(results) == 0 {
		return ""
	}

	var totalMs, ttfaMs []int64
	for _, r := range results {
		if r.ASR.Error == "" && r.LLM.Error == "" && r.TTS.Error == "" {
			totalMs = append(totalMs, r.ChainTotalMs)
			ttfaMs = append(ttfaMs, r.ChainTTFAms)
		}
	}

	out := "## Chain Summary\n\n"
	out += "| Metric | Value |\n|---|---|\n"

	if len(totalMs) > 0 {
		out += fmt.Sprintf("| E2E Total (mean) | %d ms |\n", meanInt64(totalMs))
		out += fmt.Sprintf("| E2E Total (min/max) | %d / %d ms |\n", minInt64(totalMs), maxInt64(totalMs))
	}
	if len(ttfaMs) > 0 {
		out += fmt.Sprintf("| TTFA (mean) | %d ms |\n", meanInt64(ttfaMs))
		out += fmt.Sprintf("| TTFA (min/max) | %d / %d ms |\n", minInt64(ttfaMs), maxInt64(ttfaMs))
	}

	return out
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func meanInt64(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return sum / int64(len(vals))
}

func minInt64(vals []int64) int64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxInt64(vals []int64) int64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// Ensure strings import is used
var _ = strings.TrimSpace
