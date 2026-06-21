//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type DataPoint struct {
	SensorID  string
	Timestamp time.Time
	Value     float64
	Unit      string
}

func TestScenario_StreamProcessingPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	td := NewTDigestDefault()

	validateData := NewPort[DataPoint, DataPoint](func(ctx context.Context, in DataPoint) (DataPoint, error) {
		if in.SensorID == "" {
			return in, fmt.Errorf("sensor ID required")
		}
		if in.Value < 0 {
			return in, fmt.Errorf("negative value not allowed")
		}
		return in, nil
	})

	transformData := func(ctx context.Context, in DataPoint) (DataPoint, error) {
		if in.Unit == "F" {
			in.Value = (in.Value - 32) * 5 / 9
			in.Unit = "C"
		}
		return in, nil
	}

	pipeline := NewPipeline[DataPoint](obs,
		PortAsStep[DataPoint, DataPoint](validateData),
		NewStepFunc[DataPoint, DataPoint]("Atom", transformData),
	)

	const dataPoints = 1000
	rng := rand.New(rand.NewSource(42))

	successCount := 0
	errorCount := 0

	for i := 0; i < dataPoints; i++ {
		dp := DataPoint{
			SensorID:  fmt.Sprintf("sensor-%d", i%10),
			Timestamp: time.Now(),
			Value:     rng.Float64() * 100,
			Unit:      "C",
		}

		result, _, err := pipeline.Run(ctx, dp)
		if err != nil {
			errorCount++
			continue
		}
		td.Add(result.Value)
		successCount++
	}

	t.Logf("StreamPipeline: %d data points, %d success, %d errors, count=%d, p50=%.2f, p95=%.2f, p99=%.2f",
		dataPoints, successCount, errorCount, td.Count(),
		td.Quantile(0.5), td.Quantile(0.95), td.Quantile(0.99))
}

type TenantReq struct {
	TenantID string
	APIKey   string
	Endpoint string
	Payload  string
}

func TestScenario_MultiTenantRateLimiting(t *testing.T) {
	limiter := NewShardedRateLimiter[string](100, 100)

	tenants := []string{"tenant-a", "tenant-b", "tenant-c", "tenant-d", "tenant-e"}
	const requestsPerTenant = 200

	results := make(map[string]int)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, tenant := range tenants {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			allowed := 0
			for i := 0; i < requestsPerTenant; i++ {
				if limiter.Allow(tid) {
					allowed++
					time.Sleep(time.Microsecond)
				}
			}
			mu.Lock()
			results[tid] = allowed
			mu.Unlock()
		}(tenant)
	}
	wg.Wait()

	totalAllowed := 0
	for _, tenant := range tenants {
		allowed := results[tenant]
		totalAllowed += allowed
		t.Logf("  Tenant %s: %d/%d allowed", tenant, allowed, requestsPerTenant)
	}

	t.Logf("MultiTenant: %d tenants, %d total allowed / %d total requests",
		len(tenants), totalAllowed, len(tenants)*requestsPerTenant)
}
