package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"go.uber.org/zap"
)

type Scenario struct {
	Name        string
	Target      string
	MetricsURL  string
	Requests    int
	Concurrency int
	Timeout     time.Duration
	Model       string
	Prompt      string
	MaxTokens   uint32
}

type BenchMetrics struct {
	BatchesTotal          float64
	QueueRejectedTotal    float64
	AvgBatchSize          float64
	AvgQueueWaitSeconds   float64
	AvgExecutionSeconds   float64
	RequestCountObserved  float64
	BatchCountObserved    float64
	InflightRequestsFinal float64
	InflightBatchesFinal  float64
}

type Result struct {
	Scenario      Scenario
	Success       int
	Failed        int
	TotalDuration time.Duration
	ThroughputRPS float64
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P90Latency    time.Duration
	P99Latency    time.Duration
	Metrics       BenchMetrics
}

func ScenarioPreset(name string) (Scenario, error) {
	switch name {
	case "smoke":
		return Scenario{
			Name:        name,
			Requests:    100,
			Concurrency: 10,
			Timeout:     3 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	case "baseline_no_batching":
		return Scenario{
			Name:        name,
			Requests:    300,
			Concurrency: 10,
			Timeout:     30 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	case "dynamic_default", "dynamic_fastflush":
		return Scenario{
			Name:        name,
			Requests:    1000,
			Concurrency: 100,
			Timeout:     10 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	default:
		return Scenario{}, fmt.Errorf("unsupported mode: %s", name)
	}
}

func RunScenario(logger *zap.Logger, scenario Scenario) (Result, error) {
	sugar := logger.Sugar()
	inferenceClient := client.NewClientWithTimeout([]string{scenario.Target}, scenario.Timeout+2*time.Second)

	var (
		wg        sync.WaitGroup
		sem       = make(chan struct{}, scenario.Concurrency)
		latMu     sync.Mutex
		latencies = make([]time.Duration, 0, scenario.Requests)
		successMu sync.Mutex
		success   int
		failed    int
	)

	runStart := time.Now()
	for i := 0; i < scenario.Requests; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			resp, err := inferenceClient.Generate(context.Background(), &v1.GenerateRequest{
				RequestId: fmt.Sprintf("bench-%s-%06d", scenario.Name, i),
				Model:     scenario.Model,
				Prompt:    scenario.Prompt,
				MaxTokens: scenario.MaxTokens,
				TimeoutMs: durationToMilliseconds(scenario.Timeout),
				Labels: map[string]string{
					"scenario": scenario.Name,
				},
			})
			latency := time.Since(start)

			latMu.Lock()
			latencies = append(latencies, latency)
			latMu.Unlock()

			successMu.Lock()
			defer successMu.Unlock()

			if err != nil {
				failed++
				sugar.Errorw("request failed", "index", i, "err", err)
				return
			}
			if resp == nil {
				failed++
				sugar.Errorw("request returned nil response", "index", i)
				return
			}
			if resp.FinishReason != v1.FinishReasonStop {
				failed++
				sugar.Errorw("request returned unexpected finish reason", "index", i, "finish_reason", resp.FinishReason.String())
				return
			}
			success++
		}(i)
	}
	wg.Wait()

	totalDuration := time.Since(runStart)
	serverMetrics, err := fetchBenchMetrics(scenario.MetricsURL)
	if err != nil {
		return Result{}, err
	}

	slices.Sort(latencies)
	return Result{
		Scenario:      scenario,
		Success:       success,
		Failed:        failed,
		TotalDuration: totalDuration,
		ThroughputRPS: float64(success) / totalDuration.Seconds(),
		AvgLatency:    avgDuration(latencies),
		P50Latency:    percentileDuration(latencies, 50),
		P90Latency:    percentileDuration(latencies, 90),
		P99Latency:    percentileDuration(latencies, 99),
		Metrics:       serverMetrics,
	}, nil
}

func durationToMilliseconds(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	return uint32(d / time.Millisecond)
}

func fetchBenchMetrics(metricsURL string) (BenchMetrics, error) {
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get(metricsURL)
	if err != nil {
		return BenchMetrics{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BenchMetrics{}, err
	}
	return parseBenchMetrics(string(body)), nil
}

func parseBenchMetrics(body string) BenchMetrics {
	var (
		metrics        BenchMetrics
		batchSizeSum   float64
		batchSizeCount float64
		queueWaitSum   float64
		queueWaitCount float64
		executionSum   float64
		executionCount float64
		requestCount   float64
	)

	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, ok := parseMetricLine(line)
		if !ok {
			continue
		}

		switch {
		case strings.HasPrefix(name, "llm_batches_total"):
			metrics.BatchesTotal += value
		case name == "llm_queue_rejected_total":
			metrics.QueueRejectedTotal = value
		case name == "llm_batch_size_sum":
			batchSizeSum = value
		case name == "llm_batch_size_count":
			batchSizeCount = value
		case strings.HasPrefix(name, "llm_execution_seconds_sum"):
			executionSum += value
		case strings.HasPrefix(name, "llm_execution_seconds_count"):
			executionCount += value
		case name == "llm_queue_wait_seconds_sum":
			queueWaitSum = value
		case name == "llm_queue_wait_seconds_count":
			queueWaitCount = value
		case strings.HasPrefix(name, "llm_requests_total"):
			requestCount += value
		case name == "llm_inflight_requests":
			metrics.InflightRequestsFinal = value
		case name == "llm_inflight_batches":
			metrics.InflightBatchesFinal = value
		}
	}

	if batchSizeCount > 0 {
		metrics.AvgBatchSize = batchSizeSum / batchSizeCount
		metrics.BatchCountObserved = batchSizeCount
	}
	if queueWaitCount > 0 {
		metrics.AvgQueueWaitSeconds = queueWaitSum / queueWaitCount
	}
	if executionCount > 0 {
		metrics.AvgExecutionSeconds = executionSum / executionCount
	}
	metrics.RequestCountObserved = requestCount
	return metrics
}

func parseMetricLine(line string) (string, float64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", 0, false
	}
	value, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return "", 0, false
	}
	return fields[0], value, true
}

func avgDuration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return total / time.Duration(len(ds))
}

func percentileDuration(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	if p <= 0 {
		return ds[0]
	}
	if p >= 100 {
		return ds[len(ds)-1]
	}
	idx := int(math.Ceil((p/100)*float64(len(ds)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(ds) {
		idx = len(ds) - 1
	}
	return ds[idx]
}

func printResult(w io.Writer, result Result) {
	fmt.Fprintf(w, "scenario=%s requests=%d success=%d failed=%d concurrency=%d total=%s throughput_rps=%.2f\n",
		result.Scenario.Name,
		result.Scenario.Requests,
		result.Success,
		result.Failed,
		result.Scenario.Concurrency,
		result.TotalDuration.Round(time.Millisecond),
		result.ThroughputRPS,
	)
	fmt.Fprintf(w, "latency avg=%s p50=%s p90=%s p99=%s\n",
		result.AvgLatency.Round(time.Millisecond),
		result.P50Latency.Round(time.Millisecond),
		result.P90Latency.Round(time.Millisecond),
		result.P99Latency.Round(time.Millisecond),
	)
	fmt.Fprintf(w, "metrics batches_total=%.0f avg_batch_size=%.2f avg_queue_wait_s=%.4f avg_execution_s=%.4f queue_rejected=%.0f inflight_requests=%.0f inflight_batches=%.0f observed_requests=%.0f\n",
		result.Metrics.BatchesTotal,
		result.Metrics.AvgBatchSize,
		result.Metrics.AvgQueueWaitSeconds,
		result.Metrics.AvgExecutionSeconds,
		result.Metrics.QueueRejectedTotal,
		result.Metrics.InflightRequestsFinal,
		result.Metrics.InflightBatchesFinal,
		result.Metrics.RequestCountObserved,
	)
}
