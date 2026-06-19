//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Phase 4: 商业场景实战测试
// ============================================================================

// ============================================================================
// 场景A: 订单处理流水线 (Order Processing Pipeline)
// 模拟: 订单验证 -> 库存检查 -> 支付处理 -> 发货通知
// ============================================================================

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

type OrderResult struct {
	OrderID     string
	Success     bool
	Message     string
	ProcessedAt time.Time
}

func TestScenario_OrderProcessingPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	// Step 1: 订单验证 (Port)
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

	// Step 2: 库存检查 (Atom)
	checkInventory := func(ctx context.Context, in Order) (Order, error) {
		for _, item := range in.Items {
			if item.Quantity > 100 {
				return in, fmt.Errorf("insufficient stock for %s: requested %d, available 100", item.ProductID, item.Quantity)
			}
		}
		in.Status = "inventory_checked"
		return in, nil
	}

	// Step 3: 支付处理 (Adapter - 模拟外部调用)
	processPayment := NewAdapter[Order, Order](func(ctx context.Context, in Order) (Order, error) {
		time.Sleep(time.Microsecond * 100) // 模拟支付网关延迟
		if in.Total > 100000 {
			return in, fmt.Errorf("payment declined: amount exceeds limit")
		}
		in.Status = "payment_processed"
		return in, nil
	})

	// Step 4: 发货通知 - 使用Compose包装
	sendNotification := Compose[Order](obs, NewStepFunc[Order, Order]("Adapter", func(ctx context.Context, in Order) (Order, error) {
		// 返回带状态标记的Order
		in.Status = "shipped"
		return in, nil
	}))

	// 构建Pipeline（所有步骤保持Order -> Order）
	pipeline := NewPipeline[Order](obs,
		PortAsStep[Order, Order](validateOrder),
		NewStepFunc[Order, Order]("Atom", checkInventory),
		AdapterAsStep[Order, Order](processPayment),
	)

	// 正常订单
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

	// 发货通知（单独Composer）
	notifyResult, _, err := sendNotification.Run(ctx, result)
	if err != nil {
		t.Fatalf("notification failed: %v", err)
	}
	if notifyResult.Status != "shipped" {
		t.Errorf("expected status 'shipped', got '%s'", notifyResult.Status)
	}

	// 无效订单
	badOrder := Order{OrderID: "", CustomerID: "CUST-002"}
	_, _, err = pipeline.Run(ctx, badOrder)
	if err == nil {
		t.Error("expected error for invalid order")
	}

	// 超额支付
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

// ============================================================================
// 场景B: 多Agent协同开发 (Multi-Agent Dev Pipeline)
// 模拟: 需求分析 -> 架构设计 -> 编码 -> 代码审查 -> 测试 -> 部署
// ============================================================================

func TestScenario_MultiAgentPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	handoffComposer := NewHandoffComposer(obs, persistence, transport)
	ctx := context.Background()

	// 模拟6个阶段的Agent
	phases := []string{"analysis", "design", "coding", "review", "testing", "deployment"}
	agents := []string{"analyst-1", "architect-1", "dev-1", "reviewer-1", "tester-1", "devops-1"}

	// 初始快照
	snapshot := NewDevSnapshot("task-dev-001", agents[0], phases[0], "start")
	snapshot.Artifacts = []Artifact{
		{Path: "requirements.md", Type: "doc", Description: "User requirements for feature X", Hash: "req-hash-001"},
	}

	completedPhases := 0
	currentSnapshot := snapshot

	for i := 0; i < len(phases)-1; i++ {
		currentPhase := phases[i]
		nextPhase := phases[i+1]
		sourceAgent := agents[i]
		targetAgent := agents[i+1]

		// 模拟当前阶段工作
		currentSnapshot.Phase = currentPhase
		currentSnapshot.AgentID = sourceAgent
		currentSnapshot.Checkpoint = fmt.Sprintf("%s-complete", currentPhase)
		currentSnapshot.Artifacts = append(currentSnapshot.Artifacts, Artifact{
			Path:        fmt.Sprintf("%s-output.md", currentPhase),
			Type:        "doc",
			Description: fmt.Sprintf("Output of %s phase", currentPhase),
			Hash:        fmt.Sprintf("%s-hash", currentPhase),
		})

		// Handoff到下一阶段
		input := HandoffInput{
			SourceAgent:   currentSnapshot,
			TargetAgentID: targetAgent,
			TaskID:        "task-dev-001",
			Phase:         nextPhase,
		}

		output, _, err := handoffComposer.Execute(ctx, input)
		if err != nil {
			t.Fatalf("handoff from %s to %s failed: %v", currentPhase, nextPhase, err)
		}
		if !output.Success {
			t.Fatalf("handoff from %s to %s not successful: %s", currentPhase, nextPhase, output.Error)
		}

		// 接收方验证
		received, _, err := handoffComposer.ReceiveSnapshot(ctx, output.SnapshotChecksum)
		if err != nil {
			t.Fatalf("receive failed for %s: %v", nextPhase, err)
		}
		if !received.VerifyChecksum() {
			t.Fatalf("checksum verification failed for %s", nextPhase)
		}

		currentSnapshot = received
		completedPhases++
	}

	if completedPhases != 5 {
		t.Errorf("expected 5 handoffs, got %d", completedPhases)
	}

	t.Logf("MultiAgent: %d phases, %d handoffs, all checksums verified", len(phases), completedPhases)
}

// ============================================================================
// 场景C: 流式数据处理 (Stream Processing Pipeline)
// 模拟: 数据接收 -> 验证 -> 转换 -> 聚合 -> 存储
// ============================================================================

type DataPoint struct {
	SensorID  string
	Timestamp time.Time
	Value     float64
	Unit      string
}

func TestScenario_StreamProcessingPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	// 使用TDigest做高精度聚合
	td := NewTDigestDefault()

	// Step 1: 数据验证 (Port)
	validateData := NewPort[DataPoint, DataPoint](func(ctx context.Context, in DataPoint) (DataPoint, error) {
		if in.SensorID == "" {
			return in, fmt.Errorf("sensor ID required")
		}
		if in.Value < 0 {
			return in, fmt.Errorf("negative value not allowed")
		}
		return in, nil
	})

	// Step 2: 数据转换 (Atom)
	transformData := func(ctx context.Context, in DataPoint) (DataPoint, error) {
		// 模拟单位转换: 将所有值标准化
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

	// 模拟1000个传感器数据点
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

// ============================================================================
// 场景D: 多租户限流与隔离 (Multi-Tenant Rate Limiting)
// 模拟: 多个租户共享系统，每个租户有独立限流
// ============================================================================

type TenantReq struct {
	TenantID string
	APIKey   string
	Endpoint string
	Payload  string
}

func TestScenario_MultiTenantRateLimiting(t *testing.T) {
	// 每个租户独立的限流器
	limiter := NewShardedRateLimiter[string](100, 100) // 100 tokens/s per tenant

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
					// 模拟处理
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

// ============================================================================
// 场景E: 高频交易引擎 (High-Frequency Trading Engine)
// 模拟: 订单验证 -> 风险检查 -> 订单匹配
// ============================================================================

type TradeOrder struct {
	OrderID  string
	Symbol   string
	Side     string // "buy" or "sell"
	Quantity int
	Price    float64
}

type TradeResult struct {
	OrderID    string
	Matched    bool
	MatchPrice float64
	FilledQty  int
	Status     string
}

func TestScenario_TradingEngine(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	// Step 1: 订单验证 (Port)
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

	// Step 2: 风险检查 (Port)
	riskCheck := NewPort[TradeOrder, TradeOrder](func(ctx context.Context, in TradeOrder) (TradeOrder, error) {
		if in.Quantity > 10000 {
			return in, fmt.Errorf("order quantity exceeds risk limit")
		}
		if in.Price > 10000 {
			return in, fmt.Errorf("order price exceeds risk limit")
		}
		return in, nil
	})

	// Step 3: 订单匹配 (Atom) - 使用Compose保持类型一致
	matchOrder := Compose[TradeOrder](obs, NewStepFunc[TradeOrder, TradeOrder]("Atom", func(ctx context.Context, in TradeOrder) (TradeOrder, error) {
		// 模拟撮合引擎 - 将匹配信息写入Order的Status字段
		if in.Quantity <= 5000 {
			if in.Side == "buy" {
				in.Price *= 1.0001 // 模拟滑点
			} else {
				in.Price *= 0.9999
			}
			in.Symbol = "matched" // 标记为已匹配
		}
		return in, nil
	}))

	pipeline := NewPipeline[TradeOrder](obs,
		PortAsStep[TradeOrder, TradeOrder](validateTrade),
		PortAsStep[TradeOrder, TradeOrder](riskCheck),
	)

	// 批量订单测试
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

		// 匹配
		matched, _, err := matchOrder.Run(ctx, result)
		if err != nil {
			errorCount++
			continue
		}
		if matched.Symbol == "matched" {
			successCount++
		}
	}

	// 测试风险拒绝
	riskyOrder := TradeOrder{OrderID: "RISK-001", Symbol: "ETH-USD", Side: "sell", Quantity: 20000, Price: 5000}
	_, _, err := pipeline.Run(ctx, riskyOrder)
	if err == nil {
		t.Error("expected risk check to reject large order")
	}

	t.Logf("TradingEngine: %d orders, %d matched, %d errors, 1 risk-rejected", orders, successCount, errorCount)
}

// ============================================================================
// 场景F: 长时间浸泡测试 (Soak Test)
// 模拟: 持续运行30秒，观察内存、GC、错误率
// ============================================================================

func TestScenario_SoakTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	// 构建混合负载Pipeline
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