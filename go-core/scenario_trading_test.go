//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type TradeOrder struct {
	OrderID  string
	Symbol   string
	Side     string
	Quantity int
	Price    float64
}

func TestScenario_TradingEngine(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	validateTrade := NewPort[TradeOrder, TradeOrder](func(ctx context.Context, in TradeOrder) (TradeOrder, error) {
		if in.Quantity <= 0 {
			return in, fmt.Errorf("quantity must be positive")
		}
		if in.Price <= 0 {
			return in, fmt.Errorf("price must be positive")
		}
		if in.Side != "buy" && in.Side != "sell" {
			return in, fmt.Errorf("side must be 'buy' or 'sell'")
		}
		return in, nil
	})

	riskCheck := NewPort[TradeOrder, TradeOrder](func(ctx context.Context, in TradeOrder) (TradeOrder, error) {
		if in.Quantity > 10000 {
			return in, fmt.Errorf("order quantity exceeds risk limit")
		}
		if in.Price > 10000 {
			return in, fmt.Errorf("order price exceeds risk limit")
		}
		return in, nil
	})

	matchOrder := Compose[TradeOrder](obs, NewStepFunc[TradeOrder, TradeOrder]("Atom", func(ctx context.Context, in TradeOrder) (TradeOrder, error) {
		if in.Quantity <= 5000 {
			if in.Side == "buy" {
				in.Price *= 1.0001
			} else {
				in.Price *= 0.9999
			}
			in.Symbol = "matched"
		}
		return in, nil
	}))

	pipeline := NewPipeline[TradeOrder](obs,
		PortAsStep[TradeOrder, TradeOrder](validateTrade),
		PortAsStep[TradeOrder, TradeOrder](riskCheck),
	)

	const orders = 500
	successCount := 0
	errorCount := 0

	for i := 0; i < orders; i++ {
		order := TradeOrder{
			OrderID:  fmt.Sprintf("TRD-%06d", i),
			Symbol:   "BTC-USD",
			Side:     "buy",
			Quantity: 100,
			Price:    5000.0,
		}

		result, _, err := pipeline.Run(ctx, order)
		if err != nil {
			errorCount++
			continue
		}

		matched, _, err := matchOrder.Run(ctx, result)
		if err != nil {
			errorCount++
			continue
		}
		if matched.Symbol == "matched" {
			successCount++
		}
	}

	riskyOrder := TradeOrder{OrderID: "RISK-001", Symbol: "ETH-USD", Side: "sell", Quantity: 20000, Price: 5000}
	_, _, err := pipeline.Run(ctx, riskyOrder)
	if err == nil {
		t.Error("expected risk check to reject large order")
	}

	t.Logf("TradingEngine: %d orders, %d matched, %d errors, 1 risk-rejected", orders, successCount, errorCount)
}

func TestScenario_SoakTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	steps := []Step[string, string]{
		NewStepFunc[string, string]("Port", func(ctx context.Context, in string) (string, error) {
			if len(in) == 0 {
				return "", fmt.Errorf("empty input")
			}
			return in, nil
		}),
		NewStepFunc[string, string]("Atom", func(ctx context.Context, in string) (string, error) {
			return in + "_processed", nil
		}),
		NewStepFunc[string, string]("Adapter", func(ctx context.Context, in string) (string, error) {
			time.Sleep(time.Microsecond)
			return in, nil
		}),
	}

	pipeline := NewPipeline[string](obs, steps...)

	duration := 30 * time.Second
	deadline := time.Now().Add(duration)

	var ops atomic.Int64
	var errors atomic.Int64
	var wg sync.WaitGroup

	const workers = 50
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				input := fmt.Sprintf("worker-%d-data-%d", workerID, ops.Load())
				_, _, err := pipeline.Run(ctx, input)
				ops.Add(1)
				if err != nil {
					errors.Add(1)
				}
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(deadline.Add(-duration))

	totalOps := ops.Load()
	totalErrors := errors.Load()
	errorRate := float64(totalErrors) / float64(totalOps) * 100
	throughput := float64(totalOps) / elapsed.Seconds()

	t.Logf("SoakTest: %v, %d workers, %d ops, %d errors (%.4f%%), throughput=%.0f ops/s",
		elapsed.Round(time.Second), workers, totalOps, totalErrors, errorRate, throughput)

	if errorRate > 1.0 {
		t.Errorf("error rate too high: %.4f%%", errorRate)
	}
}
