//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type Order struct {
	OrderID    string
	CustomerID string
	Items      []OrderItem
	Total      float64
	Status     string
}

type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

func TestScenario_OrderProcessingPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	validateOrder := NewPort[Order, Order](func(ctx context.Context, in Order) (Order, error) {
		if in.OrderID == "" {
			return in, fmt.Errorf("order ID is required")
		}
		if len(in.Items) == 0 {
			return in, fmt.Errorf("order must have at least one item")
		}
		if in.Total <= 0 {
			return in, fmt.Errorf("order total must be positive")
		}
		in.Status = "validated"
		return in, nil
	})

	checkInventory := func(ctx context.Context, in Order) (Order, error) {
		for _, item := range in.Items {
			if item.Quantity > 100 {
				return in, fmt.Errorf("insufficient stock for %s: requested %d, available 100", item.ProductID, item.Quantity)
			}
		}
		in.Status = "inventory_checked"
		return in, nil
	}

	processPayment := NewAdapter[Order, Order](func(ctx context.Context, in Order) (Order, error) {
		time.Sleep(time.Microsecond * 100)
		if in.Total > 100000 {
			return in, fmt.Errorf("payment declined: amount exceeds limit")
		}
		in.Status = "payment_processed"
		return in, nil
	})

	sendNotification := Compose[Order](obs, NewStepFunc[Order, Order]("Adapter", func(ctx context.Context, in Order) (Order, error) {
		in.Status = "shipped"
		return in, nil
	}))

	pipeline := NewPipeline[Order](obs,
		PortAsStep[Order, Order](validateOrder),
		NewStepFunc[Order, Order]("Atom", checkInventory),
		AdapterAsStep[Order, Order](processPayment),
	)

	order := Order{
		OrderID:    "ORD-001",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{ProductID: "PROD-001", Quantity: 2, Price: 49.99},
		},
		Total: 99.98,
	}

	result, steps, err := pipeline.Run(ctx, order)
	if err != nil {
		t.Fatalf("order processing failed: %v", err)
	}
	if result.Status != "payment_processed" {
		t.Errorf("expected status 'payment_processed', got '%s'", result.Status)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	notifyResult, _, err := sendNotification.Run(ctx, result)
	if err != nil {
		t.Fatalf("notification failed: %v", err)
	}
	if notifyResult.Status != "shipped" {
		t.Errorf("expected status 'shipped', got '%s'", notifyResult.Status)
	}

	badOrder := Order{OrderID: "", CustomerID: "CUST-002"}
	_, _, err = pipeline.Run(ctx, badOrder)
	if err == nil {
		t.Error("expected error for invalid order")
	}

	bigOrder := Order{
		OrderID:    "ORD-003",
		CustomerID: "CUST-003",
		Items:      []OrderItem{{ProductID: "PROD-001", Quantity: 1, Price: 200000}},
		Total:      200000,
	}
	_, _, err = pipeline.Run(ctx, bigOrder)
	if err == nil {
		t.Error("expected error for payment exceeding limit")
	}

	t.Logf("OrderPipeline: 1 valid, 1 invalid, 1 exceeded-limit, 1 notification sent")
}
