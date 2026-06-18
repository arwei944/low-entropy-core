// Tier L3 — Large Service: + persistent event store, event bus, schema registry,
// saga transactions, security (capability tokens), file storage.
// Build: go build -tags lecore_tier3
// Suitable for: 1000-10000 files, 10-50 developers, platform services.
//
// Demonstrates:
//   - PersistentEventStore with file storage backend
//   - EventBus with publish/subscribe
//   - SchemaRegistry with versioned schemas
//   - SagaComposer with compensation
//   - CapabilityToken security
//   - InMemoryObservationAdapter for tracing

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	core "low-entropy-core/go-core"
)

func main() {
	fmt.Println("=== Tier L3 — Large Service Demo ===")

	// Step 1: Create a temporary storage directory
	storageDir, err := os.MkdirTemp("", "lecore-l3-demo")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(storageDir)
	fmt.Printf("Storage dir: %s\n", storageDir)

	// Step 2: Create file storage backend
	storage, err := core.NewFileStorageBackend(storageDir)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Step 3: Create persistent event store
	eventStore, err := core.NewPersistentEventStore(storage)
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}
	fmt.Println("PersistentEventStore created")

	// Step 4: Create event bus and subscribe
	eventBus := core.NewEventBus()
	received := make(chan string, 10)

	sub := eventBus.Subscribe("order.created", func(event core.EventEnvelope) error {
		fmt.Printf("  [Subscriber] Received: %s (aggregate: %s)\n", event.EventID, event.AggregateID)
		received <- event.EventID
		return nil
	})
	defer eventBus.Unsubscribe(sub.ID)

	// Step 5: Publish events via EventBus
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		event := core.EventEnvelope{
			EventID:       fmt.Sprintf("evt-%d", i),
			AggregateID:   fmt.Sprintf("order-%d", i),
			AggregateType: "Order",
			EventType:     "order.created",
			EventData:     []byte(fmt.Sprintf(`{"amount":%d,"item":"item-%d"}`, i*100, i)),
			Version:       1,
			Timestamp:     time.Now(),
		}
		_, err := eventBus.Execute(ctx, event)
		if err != nil {
			log.Printf("Publish error: %v", err)
		}

		// Also persist to event store
		_, err = eventStore.Execute(ctx, event)
		if err != nil {
			log.Printf("Append error: %v", err)
		}
	}

	// Wait for async subscribers
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Events received: %d\n", len(received))

	// Step 6: Schema registry
	schemaReg := core.NewSchemaRegistry()
	type OrderV1 struct {
		Amount int    `json:"amount"`
		Item   string `json:"item"`
	}
	schemaReg.Register("Order", "v1", OrderV1{})

	_, err = schemaReg.Get("Order", "v1")
	if err != nil {
		fmt.Printf("Schema lookup failed: %v\n", err)
	} else {
		fmt.Println("Schema v1 registered and retrieved")
	}

	// Step 7: Saga transaction with compensation
	obs := &core.InMemoryObservationAdapter{}
	saga := core.NewSagaComposer(obs)

	executed := []string{}
	compensated := []string{}

	// Step 1: Create order
	createOrder := core.SagaStep{
		Name: "create-order",
		Execute: core.AtomAsStep(core.Atom[any, any](func(input any) any {
			executed = append(executed, "create-order")
			fmt.Println("  [Saga] Creating order...")
			return input
		})),
		Compensate: core.AtomAsStep(core.Atom[any, any](func(input any) any {
			compensated = append(compensated, "cancel-order")
			fmt.Println("  [Saga] Compensating: cancel order")
			return input
		})),
	}
	saga.AddStep(createOrder)

	// Step 2: Reserve inventory
	reserveInventory := core.SagaStep{
		Name: "reserve-inventory",
		Execute: core.AtomAsStep(core.Atom[any, any](func(input any) any {
			executed = append(executed, "reserve-inventory")
			fmt.Println("  [Saga] Reserving inventory...")
			return input
		})),
		Compensate: core.AtomAsStep(core.Atom[any, any](func(input any) any {
			compensated = append(compensated, "release-inventory")
			fmt.Println("  [Saga] Compensating: release inventory")
			return input
		})),
	}
	saga.AddStep(reserveInventory)

	_, err = saga.Run(ctx, "order-request")
	if err != nil {
		fmt.Printf("Saga failed: %v\n", err)
	} else {
		fmt.Printf("Saga completed: executed=%v\n", executed)
	}

	// Step 8: Security — capability tokens
	secretKey := []byte("my-secret-key-32-bytes-long!")
	token := core.NewCapabilityToken("agent-1", []string{"read", "write"})
	token.Sign(secretKey)

	if token.Verify(secretKey) && !token.IsExpired() {
		fmt.Printf("Token verified: agent=%s, caps=%v\n", token.AgentID, token.Capabilities)

		// Check specific capability
		if token.HasCapability("write") {
			fmt.Println("  Agent has 'write' capability")
		}
	}

	// Step 9: Query persisted events
	results := eventStore.StreamAll("order-1")
	fmt.Printf("\nPersisted events for order-1: %d\n", len(results))
	for _, evt := range results {
		payload, _ := json.Marshal(string(evt.EventData))
		fmt.Printf("  [%s] %s: %s (v%d)\n", evt.EventType, evt.EventID, payload, evt.Version)
	}

	// Step 10: Tier info
	tier := core.AutoDetect(".")
	fmt.Printf("\nProject tier: %s (L%d), framework files: %d\n",
		tier, tier, tier.FrameworkFileCount())

	fmt.Println("\n=== Demo Complete ===")
}