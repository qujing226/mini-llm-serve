package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPercentileDuration(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	require.Equal(t, 30*time.Millisecond, percentileDuration(latencies, 50))
	require.Equal(t, 50*time.Millisecond, percentileDuration(latencies, 90))
	require.Equal(t, 50*time.Millisecond, percentileDuration(latencies, 99))
}

func TestParseBenchMetrics(t *testing.T) {
	raw := `
# HELP llm_batch_size Number of requests in a batch
llm_batch_size_sum 101
llm_batch_size_count 11
llm_batches_total{executor="mock-python"} 11
llm_execution_seconds_sum{executor="mock-python"} 13.938
llm_execution_seconds_count{executor="mock-python"} 101
llm_queue_wait_seconds_sum 9.996
llm_queue_wait_seconds_count 101
llm_queue_rejected_total 0
llm_active_requests 0
llm_inflight_batches 0
llm_requests_total{executor="mock-python",status="ok"} 101
`

	metrics := parseBenchMetrics(raw)
	require.Equal(t, 11.0, metrics.BatchesTotal)
	require.InDelta(t, 9.1818, metrics.AvgBatchSize, 0.001)
	require.InDelta(t, 0.09897, metrics.AvgQueueWaitSeconds, 0.001)
	require.InDelta(t, 0.138, metrics.AvgExecutionSeconds, 0.001)
	require.Equal(t, 101.0, metrics.RequestCountObserved)
	require.Equal(t, 0.0, metrics.ActiveRequestsFinal)
	require.Equal(t, 0.0, metrics.InflightBatchesFinal)
}

func TestScenarioPreset(t *testing.T) {
	baseline, err := ScenarioPreset("baseline_no_batching")
	require.NoError(t, err)
	require.Equal(t, 300, baseline.Requests)
	require.Equal(t, 10, baseline.Concurrency)
	require.Equal(t, 30*time.Second, baseline.Timeout)

	dynamicDefault, err := ScenarioPreset("dynamic_default")
	require.NoError(t, err)
	require.Equal(t, 1000, dynamicDefault.Requests)
	require.Equal(t, 100, dynamicDefault.Concurrency)
	require.Equal(t, 10*time.Second, dynamicDefault.Timeout)

	smoke, err := ScenarioPreset("smoke")
	require.NoError(t, err)
	require.Equal(t, 100, smoke.Requests)
	require.Equal(t, 10, smoke.Concurrency)
	require.Equal(t, 3*time.Second, smoke.Timeout)
}
