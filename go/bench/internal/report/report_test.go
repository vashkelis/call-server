package report

import (
	"testing"
)

func TestSummarize_EmptyResults(t *testing.T) {
	summaries := Summarize(nil)
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestSummarize_SingleStage(t *testing.T) {
	results := []BenchResult{
		{Stage: "asr", Provider: "deepgram", Iter: 1, TTFTMs: 100, TotalMs: 200, OutputSize: 10},
		{Stage: "asr", Provider: "deepgram", Iter: 2, TTFTMs: 150, TotalMs: 250, OutputSize: 12},
		{Stage: "asr", Provider: "deepgram", Iter: 3, TTFTMs: 200, TotalMs: 300, OutputSize: 8},
	}

	summaries := Summarize(results)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	s := summaries[0]
	if s.Stage != "asr" {
		t.Errorf("expected stage=asr, got %s", s.Stage)
	}
	if s.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", s.Iterations)
	}
	if s.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", s.Errors)
	}
	if s.MeanTTFTMs != 150 {
		t.Errorf("expected mean TTFT=150, got %d", s.MeanTTFTMs)
	}
	if s.MeanTotalMs != 250 {
		t.Errorf("expected mean Total=250, got %d", s.MeanTotalMs)
	}
	if s.P50TTFTMs != 150 {
		t.Errorf("expected p50 TTFT=150, got %d", s.P50TTFTMs)
	}
	if s.MinTTFTMs != 100 {
		t.Errorf("expected min TTFT=100, got %d", s.MinTTFTMs)
	}
	if s.MaxTTFTMs != 200 {
		t.Errorf("expected max TTFT=200, got %d", s.MaxTTFTMs)
	}
}

func TestSummarize_MultipleStages(t *testing.T) {
	results := []BenchResult{
		{Stage: "asr", Provider: "deepgram", Iter: 1, TTFTMs: 100, TotalMs: 200},
		{Stage: "llm", Provider: "openai", Iter: 1, TTFTMs: 300, TotalMs: 500, TokensPerSec: 42.5},
		{Stage: "tts", Provider: "elevenlabs", Iter: 1, TTFTMs: 50, TotalMs: 150, RealTimeFactor: 2.5},
	}

	summaries := Summarize(results)
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	if summaries[0].Stage != "asr" {
		t.Errorf("expected first summary to be asr, got %s", summaries[0].Stage)
	}
	if summaries[1].Stage != "llm" {
		t.Errorf("expected second summary to be llm, got %s", summaries[1].Stage)
	}
	if summaries[2].Stage != "tts" {
		t.Errorf("expected third summary to be tts, got %s", summaries[2].Stage)
	}

	if summaries[1].MeanTokensPerSec != 42.5 {
		t.Errorf("expected mean TPS=42.5, got %f", summaries[1].MeanTokensPerSec)
	}
	if summaries[2].MeanRTF != 2.5 {
		t.Errorf("expected mean RTF=2.5, got %f", summaries[2].MeanRTF)
	}
}

func TestSummarize_WithErrors(t *testing.T) {
	results := []BenchResult{
		{Stage: "llm", Provider: "openai", Iter: 1, TTFTMs: 100, TotalMs: 200},
		{Stage: "llm", Provider: "openai", Iter: 2, Error: "timeout"},
		{Stage: "llm", Provider: "openai", Iter: 3, TTFTMs: 150, TotalMs: 250},
	}

	summaries := Summarize(results)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	s := summaries[0]
	if s.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", s.Iterations)
	}
	if s.Errors != 1 {
		t.Errorf("expected 1 error, got %d", s.Errors)
	}
	if s.MeanTTFTMs != 125 {
		t.Errorf("expected mean TTFT=125, got %d", s.MeanTTFTMs)
	}
}

func TestPercentile(t *testing.T) {
	vals := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	p50 := percentile(vals, 50)
	if p50 != 55 {
		t.Errorf("expected p50=55, got %d", p50)
	}

	p0 := percentile(vals, 0)
	if p0 != 10 {
		t.Errorf("expected p0=10, got %d", p0)
	}

	p100 := percentile(vals, 100)
	if p100 != 100 {
		t.Errorf("expected p100=100, got %d", p100)
	}
}

func TestFormatCSV(t *testing.T) {
	results := []BenchResult{
		{Stage: "asr", Provider: "test", Iter: 1, TTFTMs: 100, TotalMs: 200, OutputSize: 10},
	}
	rpt := NewBenchReport(results)
	csv := rpt.FormatCSV()

	if csv == "" {
		t.Error("expected non-empty CSV output")
	}
}

func TestFormatMarkdown(t *testing.T) {
	results := []BenchResult{
		{Stage: "llm", Provider: "openai", Iter: 1, TTFTMs: 100, TotalMs: 500, OutputSize: 50, TokensPerSec: 30.0},
	}
	rpt := NewBenchReport(results)
	md := rpt.FormatMarkdown()

	if md == "" {
		t.Error("expected non-empty Markdown output")
	}
	if len(md) < 50 {
		t.Errorf("expected substantial markdown output, got %d chars", len(md))
	}
}
