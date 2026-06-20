// Package core — 币安交易所核心测试
// 演示完整交易流程：下单 -> 撮合 -> 清算 -> 余额更新
package core

import (
	"context"
	"testing"
	"time"
)

// TestFullTradeFlow 完整交易流程测试
func TestFullTradeFlow(t *testing.T) {
	ctx := context.Background()

	// 初始化核心组件
	walletAdapter := NewWalletAdapter()
	matchEngine := NewMatchEngine()
	orderPort := NewOrderPort(walletAdapter)
	riskEngine := NewRiskEngine()
	pipeline := NewTradePipeline(orderPort, riskEngine, walletAdapter, matchEngine)

	// 注册交易对 BTCUSDT
	btcusdt := &SymbolInfo{
		Symbol:         "BTCUSDT",
		BaseAsset:      "BTC",
		QuoteAsset:     "USDT",
		PricePrecision: 2,
		QtyPrecision:   6,
		MinQty:         0.0001,
		MaxQty:         1000,
		MinNotional:    10,
		Status:         "TRADING",
	}
	pipeline.RegisterSymbol(btcusdt)

	// 设置用户 KYC
	orderPort.SetKYC(&KYCInfo{
		UserID:   "user-1",
		Status:   KYCStatusVerified,
		Level:    2,
		Country:  "CN",
	})
	orderPort.SetKYC(&KYCInfo{
		UserID:   "user-2",
		Status:   KYCStatusVerified,
		Level:    2,
		Country:  "US",
	})

	// 给用户充值
	walletAdapter.ProcessDeposit(&DepositRecord{
		RecordID: "dep-1",
		UserID:   "user-1",
		Asset:    "USDT",
		Amount:   100000,
		Status:   1,
		CreatedAt: time.Now(),
	})
	walletAdapter.ProcessDeposit(&DepositRecord{
		RecordID: "dep-2",
		UserID:   "user-2",
		Asset:    "BTC",
		Amount:   10,
		Status:   1,
		CreatedAt: time.Now(),
	})

	// 验证初始余额
	bal1, _ := walletAdapter.GetBalance("user-1", "USDT")
	if bal1.Free != 100000 {
		t.Fatalf("user-1 USDT balance expected 100000, got %.2f", bal1.Free)
	}
	bal2, _ := walletAdapter.GetBalance("user-2", "BTC")
	if bal2.Free != 10 {
		t.Fatalf("user-2 BTC balance expected 10, got %.8f", bal2.Free)
	}

	// 用户1 挂买单：买 1 BTC @ 50000 USDT
	buyOrder := &Order{
		UserID:    "user-1",
		Symbol:    "BTCUSDT",
		Side:      OrderSideBuy,
		Type:      OrderTypeLimit,
		Price:     50000,
		Quantity:  1,
		CreatedAt: time.Now(),
	}

	result1, steps1, err := pipeline.Run(ctx, TradePipelineInput{
		Order:  buyOrder,
		UserID: "user-1",
		IP:     "192.168.1.1",
	})
	if err != nil {
		t.Fatalf("pipeline error: %v", err)
	}
	if !result1.Success {
		t.Fatalf("buy order failed: %s", result1.Error)
	}
	if len(steps1) != 6 {
		t.Errorf("expected 6 steps, got %d", len(steps1))
	}

	// 买单未成交（无对手方），应入簿
	if result1.Order.Status != OrderStatusNew {
		t.Errorf("buy order status expected NEW, got %d", result1.Order.Status)
	}

	// 验证余额冻结
	bal1After, _ := walletAdapter.GetBalance("user-1", "USDT")
	if bal1After.Free != 50000 {
		t.Errorf("user-1 free USDT expected 50000, got %.2f", bal1After.Free)
	}
	if bal1After.Locked != 50000 {
		t.Errorf("user-1 locked USDT expected 50000, got %.2f", bal1After.Locked)
	}

	// 用户2 挂卖单：卖 0.5 BTC @ 49000 USDT（低于买价，应成交）
	sellOrder := &Order{
		UserID:    "user-2",
		Symbol:    "BTCUSDT",
		Side:      OrderSideSell,
		Type:      OrderTypeLimit,
		Price:     49000,
		Quantity:  0.5,
		CreatedAt: time.Now(),
	}

	result2, steps2, err := pipeline.Run(ctx, TradePipelineInput{
		Order:  sellOrder,
		UserID: "user-2",
		IP:     "192.168.1.2",
	})
	if err != nil {
		t.Fatalf("pipeline error: %v", err)
	}
	if !result2.Success {
		t.Fatalf("sell order failed: %s", result2.Error)
	}

	// 卖单应完全成交
	if result2.Order.Status != OrderStatusFilled {
		t.Errorf("sell order status expected FILLED, got %d", result2.Order.Status)
	}
	if len(result2.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(result2.Trades))
	}

	trade := result2.Trades[0]
	if trade.Price != 50000 {
		t.Errorf("trade price expected 50000, got %.2f", trade.Price)
	}
	if trade.Quantity != 0.5 {
		t.Errorf("trade quantity expected 0.5, got %.8f", trade.Quantity)
	}

	// 验证用户1余额（买方）
	bal1Final, _ := walletAdapter.GetBalance("user-1", "USDT")
	if bal1Final.Free != 75000 {
		t.Errorf("user-1 free USDT expected 75000, got %.2f", bal1Final.Free)
	}
	bal1BTC, _ := walletAdapter.GetBalance("user-1", "BTC")
	if bal1BTC == nil || bal1BTC.Free != 0.5 {
		t.Errorf("user-1 BTC expected 0.5, got %.8f", bal1BTC.Free)
	}

	// 验证用户2余额（卖方）
	bal2Final, _ := walletAdapter.GetBalance("user-2", "BTC")
	if bal2Final.Free != 9.5 {
		t.Errorf("user-2 free BTC expected 9.5, got %.8f", bal2Final.Free)
	}
	bal2USDT, _ := walletAdapter.GetBalance("user-2", "USDT")
	if bal2USDT == nil || bal2USDT.Free != 25000 {
		t.Errorf("user-2 USDT expected 25000, got %.2f", bal2USDT.Free)
	}

	t.Logf("Trade completed: %s, Price: %.2f, Qty: %.8f", trade.TradeID, trade.Price, trade.Quantity)
	t.Logf("Total pipeline steps: %d", len(steps2))
	for _, step := range steps2 {
		t.Logf("  - %s: %v", step.Name, step.Duration)
	}
}

// TestMarketOrder 市价单测试
func TestMarketOrder(t *testing.T) {
	ctx := context.Background()

	walletAdapter := NewWalletAdapter()
	matchEngine := NewMatchEngine()
	orderPort := NewOrderPort(walletAdapter)
	riskEngine := NewRiskEngine()
	pipeline := NewTradePipeline(orderPort, riskEngine, walletAdapter, matchEngine)

	// 注册交易对
	pipeline.RegisterSymbol(&SymbolInfo{
		Symbol:         "ETHUSDT",
		BaseAsset:      "ETH",
		QuoteAsset:     "USDT",
		PricePrecision: 2,
		QtyPrecision:   4,
		MinQty:         0.001,
		Status:         "TRADING",
	})

	orderPort.SetKYC(&KYCInfo{UserID: "user-3", Status: KYCStatusVerified, Level: 2})
	orderPort.SetKYC(&KYCInfo{UserID: "user-4", Status: KYCStatusVerified, Level: 2})

	// 充值
	walletAdapter.ProcessDeposit(&DepositRecord{RecordID: "dep-3", UserID: "user-3", Asset: "USDT", Amount: 50000, Status: 1, CreatedAt: time.Now()})
	walletAdapter.ProcessDeposit(&DepositRecord{RecordID: "dep-4", UserID: "user-4", Asset: "ETH", Amount: 100, Status: 1, CreatedAt: time.Now()})

	// 用户3 挂限价买单
	buyOrder := &Order{
		UserID:    "user-3",
		Symbol:    "ETHUSDT",
		Side:      OrderSideBuy,
		Type:      OrderTypeLimit,
		Price:     3000,
		Quantity:  2,
		CreatedAt: time.Now(),
	}
	pipeline.Run(ctx, TradePipelineInput{Order: buyOrder, UserID: "user-3"})

	// 用户4 下市价卖单（应立即成交）
	sellOrder := &Order{
		UserID:    "user-4",
		Symbol:    "ETHUSDT",
		Side:      OrderSideSell,
		Type:      OrderTypeMarket,
		Quantity:  1,
		CreatedAt: time.Now(),
	}
	result, _, err := pipeline.Run(ctx, TradePipelineInput{Order: sellOrder, UserID: "user-4"})
	if err != nil {
		t.Fatalf("market order error: %v", err)
	}
	if !result.Success {
		t.Fatalf("market order failed: %s", result.Error)
	}
	if result.Order.Status != OrderStatusFilled {
		t.Errorf("market sell expected FILLED, got %d", result.Order.Status)
	}
	if len(result.Trades) == 0 {
		t.Fatal("market order should produce trades")
	}

	t.Logf("Market order executed at price %.2f", result.Trades[0].Price)
}

// TestRiskEngine 风控引擎测试
func TestRiskEngine(t *testing.T) {
	re := NewRiskEngine()

	// 正常用户
	result := re.Check(RiskCheckInput{
		UserID: "user-normal",
		Order:  &Order{Price: 1000, Quantity: 1},
		Action: "place_order",
		IP:     "192.168.1.1",
	})
	if !result.Allowed {
		t.Error("normal user should be allowed")
	}
	if result.RiskLevel != RiskLevelLow {
		t.Errorf("expected LOW risk, got %d", result.RiskLevel)
	}

	// 黑名单IP
	re.AddBlacklistIP("10.0.0.1")
	result = re.Check(RiskCheckInput{
		UserID: "user-normal",
		Action: "place_order",
		IP:     "10.0.0.1",
	})
	if result.Allowed {
		t.Error("blacklisted IP should be blocked")
	}

	// 高风险用户
	re.UpdateRiskScore("user-risky", 85)
	result = re.Check(RiskCheckInput{
		UserID: "user-risky",
		Action: "place_order",
		IP:     "192.168.1.2",
	})
	if result.Allowed {
		t.Error("high risk user should be blocked")
	}

	// 大额交易
	result = re.Check(RiskCheckInput{
		UserID: "user-normal",
		Order:  &Order{Price: 200000, Quantity: 1},
		Action: "place_order",
		IP:     "192.168.1.1",
	})
	if !result.Allowed {
		t.Error("large order should be allowed with 2FA")
	}
	if !result.Require2FA {
		t.Error("large order should require 2FA")
	}
}

// TestOrderBook 订单簿测试
func TestOrderBook(t *testing.T) {
	ob := NewOrderBook("BTCUSDT")

	// 添加买单
	ob.AddOrder(&Order{OrderID: "b1", UserID: "u1", Price: 50000, Quantity: 1, Side: OrderSideBuy, CreatedAt: time.Now()})
	ob.AddOrder(&Order{OrderID: "b2", UserID: "u2", Price: 51000, Quantity: 0.5, Side: OrderSideBuy, CreatedAt: time.Now()})
	ob.AddOrder(&Order{OrderID: "b3", UserID: "u3", Price: 49000, Quantity: 2, Side: OrderSideBuy, CreatedAt: time.Now()})

	// 添加卖单
	ob.AddOrder(&Order{OrderID: "s1", UserID: "u4", Price: 52000, Quantity: 1, Side: OrderSideSell, CreatedAt: time.Now()})
	ob.AddOrder(&Order{OrderID: "s2", UserID: "u5", Price: 51500, Quantity: 0.5, Side: OrderSideSell, CreatedAt: time.Now()})

	// 检查最优价格
	bestBid, ok := ob.BestBid()
	if !ok || bestBid != 51000 {
		t.Errorf("best bid expected 51000, got %.2f", bestBid)
	}
	bestAsk, ok := ob.BestAsk()
	if !ok || bestAsk != 51500 {
		t.Errorf("best ask expected 51500, got %.2f", bestAsk)
	}

	// 快照
	snapshot := ob.Snapshot()
	if len(snapshot.Bids) != 3 {
		t.Errorf("expected 3 bids, got %d", len(snapshot.Bids))
	}
	if len(snapshot.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(snapshot.Asks))
	}

	// 移除订单
	ob.RemoveOrder("b2")
	bestBid, _ = ob.BestBid()
	if bestBid != 50000 {
		t.Errorf("after removal best bid expected 50000, got %.2f", bestBid)
	}
}