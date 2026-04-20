// Package report provides benchmark result types, aggregation, and output formatters.
package report

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// BenchResult holds the result of a single benchmark iteration.
type BenchResult struct {
	Stage     string `json:"stage"`
	Provider  string `json:"provider"`
	Iter      int    `json:"iteration"`
	InputDesc string `json:"input_desc"`

	// Timing
	StartTime       time.Time `json:"start_time"`
	FirstOutputTime time.Time `json:"first_output_time"`
	EndTime         time.Time `json:"end_time"`

	// Derived latencies (milliseconds)
	TTFTMs  int64 `json:"ttft_ms"`  // Time To First Token/Output
	TotalMs int64 `json:"total_ms"` // Total wall-clock duration

	// Stage-specific metrics
	OutputSize     int     `json:"output_size"`              // transcript chars / token count / audio bytes
	TokensPerSec   float64 `json:"tokens_per_sec,omitempty"` // LLM only
	RealTimeFactor float64 `json:"rtf,omitempty"`            // TTS only

	// Error handling
	Error string `json:"error,omitempty"`
}

// StageSummary holds aggregated statistics for a stage across iterations.
type StageSummary struct {
	Stage      string `json:"stage"`
	Provider   string `json:"provider"`
	Iterations int    `json:"iterations"`
	Errors     int    `json:"errors"`

	MeanTTFTMs int64 `json:"mean_ttft_ms"`
	P50TTFTMs  int64 `json:"p50_ttft_ms"`
	P95TTFTMs  int64 `json:"p95_ttft_ms"`
	P99TTFTMs  int64 `json:"p99_ttft_ms"`

	MeanTotalMs int64 `json:"mean_total_ms"`
	P50TotalMs  int64 `json:"p50_total_ms"`
	P95TotalMs  int64 `json:"p95_total_ms"`
	P99TotalMs  int64 `json:"p99_total_ms"`

	MinTTFTMs  int64 `json:"min_ttft_ms"`
	MaxTTFTMs  int64 `json:"max_ttft_ms"`
	MinTotalMs int64 `json:"min_total_ms"`
	MaxTotalMs int64 `json:"max_total_ms"`

	MeanOutputSize   int     `json:"mean_output_size"`
	MeanTokensPerSec float64 `json:"mean_tokens_per_sec,omitempty"`
	MeanRTF          float64 `json:"mean_rtf,omitempty"`
}

// BenchReport is the top-level report for a benchmark run.
type BenchReport struct {
	Timestamp time.Time      `json:"timestamp"`
	Results   []BenchResult  `json:"results"`
	Summaries []StageSummary `json:"summaries"`
}

// NewBenchReport creates a new report from results.
func NewBenchReport(results []BenchResult) *BenchReport {
	return &BenchReport{
		Timestamp: time.Now().UTC(),
		Results:   results,
		Summaries: Summarize(results),
	}
}

// Summarize produces StageSummary values grouped by (stage, provider).
func Summarize(results []BenchResult) []StageSummary {
	type key struct{ stage, provider string }
	groups := map[key][]BenchResult{}

	for _, r := range results {
		k := key{r.Stage, r.Provider}
		groups[k] = append(groups[k], r)
	}

	var summaries []StageSummary
	for k, rs := range groups {
		s := StageSummary{
			Stage:    k.stage,
			Provider: k.provider,
		}
		s.Iterations = len(rs)

		var ttfts, totals []int64
		var totalOutputSize int
		var totalTPS, totalRTF float64
		var tpsCount, rtfCount int

		for _, r := range rs {
			if r.Error != "" {
				s.Errors++
				continue
			}
			ttfts = append(ttfts, r.TTFTMs)
			totals = append(totals, r.TotalMs)
			totalOutputSize += r.OutputSize
			if r.TokensPerSec > 0 {
				totalTPS += r.TokensPerSec
				tpsCount++
			}
			if r.RealTimeFactor > 0 {
				totalRTF += r.RealTimeFactor
				rtfCount++
			}
		}

		if len(ttfts) == 0 {
			summaries = append(summaries, s)
			continue
		}

		s.MeanTTFTMs = mean(ttfts)
		s.P50TTFTMs = percentile(ttfts, 50)
		s.P95TTFTMs = percentile(ttfts, 95)
		s.P99TTFTMs = percentile(ttfts, 99)
		s.MinTTFTMs = min(ttfts)
		s.MaxTTFTMs = max(ttfts)

		s.MeanTotalMs = mean(totals)
		s.P50TotalMs = percentile(totals, 50)
		s.P95TotalMs = percentile(totals, 95)
		s.P99TotalMs = percentile(totals, 99)
		s.MinTotalMs = min(totals)
		s.MaxTotalMs = max(totals)

		s.MeanOutputSize = totalOutputSize / len(ttfts)
		if tpsCount > 0 {
			s.MeanTokensPerSec = roundTo(totalTPS/float64(tpsCount), 2)
		}
		if rtfCount > 0 {
			s.MeanRTF = roundTo(totalRTF/float64(rtfCount), 4)
		}

		summaries = append(summaries, s)
	}

	// Sort by stage then provider
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Stage != summaries[j].Stage {
			return summaries[i].Stage < summaries[j].Stage
		}
		return summaries[i].Provider < summaries[j].Provider
	})

	return summaries
}

// --- statistical helpers ---

func mean(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return sum / int64(len(vals))
}

func percentile(vals []int64, p int) int64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lower)
	return int64(float64(sorted[lower]) + frac*float64(sorted[upper]-sorted[lower]))
}

func min(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func max(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func roundTo(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}

// FormatMarkdown renders the report as a markdown table.
func (r *BenchReport) FormatMarkdown() string {
	out := fmt.Sprintf("# Benchmark Report — %s\n\n", r.Timestamp.Format(time.RFC3339))

	for _, s := range r.Summaries {
		out += fmt.Sprintf("## %s (%s)\n\n", s.Stage, s.Provider)
		out += "| Metric | Value |\n|---|---|\n"
		out += fmt.Sprintf("| Iterations | %d (errors: %d) |\n", s.Iterations, s.Errors)
		out += fmt.Sprintf("| TTFT (mean) | %d ms |\n", s.MeanTTFTMs)
		out += fmt.Sprintf("| TTFT (p50) | %d ms |\n", s.P50TTFTMs)
		out += fmt.Sprintf("| TTFT (p95) | %d ms |\n", s.P95TTFTMs)
		out += fmt.Sprintf("| TTFT (p99) | %d ms |\n", s.P99TTFTMs)
		out += fmt.Sprintf("| TTFT (min/max) | %d / %d ms |\n", s.MinTTFTMs, s.MaxTTFTMs)
		out += fmt.Sprintf("| Total (mean) | %d ms |\n", s.MeanTotalMs)
		out += fmt.Sprintf("| Total (p50) | %d ms |\n", s.P50TotalMs)
		out += fmt.Sprintf("| Total (p95) | %d ms |\n", s.P95TotalMs)
		out += fmt.Sprintf("| Total (p99) | %d ms |\n", s.P99TotalMs)
		out += fmt.Sprintf("| Total (min/max) | %d / %d ms |\n", s.MinTotalMs, s.MaxTotalMs)
		out += fmt.Sprintf("| Mean output size | %d |\n", s.MeanOutputSize)
		if s.MeanTokensPerSec > 0 {
			out += fmt.Sprintf("| Mean tokens/sec | %.2f |\n", s.MeanTokensPerSec)
		}
		if s.MeanRTF > 0 {
			out += fmt.Sprintf("| Mean RTF | %.4f |\n", s.MeanRTF)
		}
		out += "\n"
	}

	// Detailed iteration results
	if len(r.Results) > 0 {
		out += "## Raw Results\n\n"
		out += "| # | Stage | Provider | TTFT (ms) | Total (ms) | Output Size | Error |\n"
		out += "|---|---|---|---|---|---|---|\n"
		for _, res := range r.Results {
			errCell := ""
			if res.Error != "" {
				errCell = res.Error
			}
			out += fmt.Sprintf("| %d | %s | %s | %d | %d | %d | %s |\n",
				res.Iter, res.Stage, res.Provider, res.TTFTMs, res.TotalMs, res.OutputSize, errCell)
		}
		out += "\n"
	}

	return out
}

// FormatCSV renders the report as CSV.
func (r *BenchReport) FormatCSV() string {
	out := "iteration,stage,provider,ttft_ms,total_ms,output_size,tokens_per_sec,rtf,error\n"
	for _, res := range r.Results {
		out += fmt.Sprintf("%d,%s,%s,%d,%d,%d,%.2f,%.4f,%s\n",
			res.Iter, res.Stage, res.Provider, res.TTFTMs, res.TotalMs,
			res.OutputSize, res.TokensPerSec, res.RealTimeFactor, res.Error)
	}
	return out
}
