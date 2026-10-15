package main

import (
	"testing"
	"time"
)

func TestRecorderSummaryUsesNearestRankPercentiles(t *testing.T) {
	var result recorder
	for value := 1; value <= 100; value++ {
		result.success(time.Duration(value) * time.Millisecond)
	}
	result.failure(true, false, true)
	snapshot := result.summarize(10 * time.Second)
	if snapshot.Attempts != 101 || snapshot.Successes != 100 || snapshot.Failures != 1 || snapshot.Timeouts != 1 || snapshot.BusinessErrors != 1 {
		t.Fatalf("unexpected counters: %+v", snapshot)
	}
	if snapshot.P50MS != 50 || snapshot.P95MS != 95 || snapshot.P99MS != 99 || snapshot.Throughput != 10 {
		t.Fatalf("unexpected distribution: %+v", snapshot)
	}
}
