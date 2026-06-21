// Package core — 交易所风控与订单簿测试
package core

import (
	"testing"
	"time"
)

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
