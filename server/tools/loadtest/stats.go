package main

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

type recorder struct {
	mu             sync.Mutex
	attempts       uint64
	successes      uint64
	failures       uint64
	timeouts       uint64
	protocolErrors uint64
	businessErrors uint64
	latencies      []time.Duration
}

func (r *recorder) success(latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts++
	r.successes++
	r.latencies = append(r.latencies, latency)
}

func (r *recorder) failure(timeout, protocol, business bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts++
	r.failures++
	if timeout {
		r.timeouts++
	}
	if protocol {
		r.protocolErrors++
	}
	if business {
		r.businessErrors++
	}
}

type summary struct {
	Attempts       uint64  `json:"attempts"`
	Successes      uint64  `json:"successes"`
	Failures       uint64  `json:"failures"`
	Timeouts       uint64  `json:"timeouts"`
	ProtocolErrors uint64  `json:"protocolErrors"`
	BusinessErrors uint64  `json:"businessErrors"`
	Throughput     float64 `json:"throughputPerSecond"`
	P50MS          float64 `json:"p50Ms"`
	P95MS          float64 `json:"p95Ms"`
	P99MS          float64 `json:"p99Ms"`
	DurationMS     int64   `json:"durationMs"`
}

func (r *recorder) summarize(elapsed time.Duration) summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := append([]time.Duration(nil), r.latencies...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	seconds := elapsed.Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	return summary{
		Attempts: r.attempts, Successes: r.successes, Failures: r.failures, Timeouts: r.timeouts,
		ProtocolErrors: r.protocolErrors, BusinessErrors: r.businessErrors,
		Throughput: float64(r.successes) / seconds, P50MS: percentileMS(values, 50),
		P95MS: percentileMS(values, 95), P99MS: percentileMS(values, 99), DurationMS: elapsed.Milliseconds(),
	}
}

func percentileMS(values []time.Duration, percentile int) float64 {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(values) {
		index = len(values)
	}
	return float64(values[index-1].Microseconds()) / 1000
}

func printJSON(value any) error {
	encoder := json.NewEncoder(outputWriter)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
