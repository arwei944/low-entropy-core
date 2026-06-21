// Package core — 撮合引擎：Atom 层纯计算 (MatchBox)
// L1 层：撮合算法 — 纯函数，无 I/O，无副作用，无时间读取。
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// MatchEngine — Atom 层撮合引擎
// ──────────────────────────────────────────────

// MatchInput 撮合输入。
type MatchInput struct {
	Order *Order
}

// MatchOutput 撮合输出。
type MatchOutput struct {
	Trades       []*Trade
	UpdatedOrder *Order
	Matched     bool
}

// MatchEngine 撮合引擎 — 实现 Atom[MatchInput, MatchOutput]。
// 内部持有 OrderBook 状态，属于带状态的 Atom（Adapter 边界在交易所通信层）。
type MatchEngine struct {
	orderBooks map[string]*OrderBook
	mu        sync.RWMutex
	tradeSeq  int64
}

// NewMatchEngine 创建撮合引擎。
func NewMatchEngine() *MatchEngine {
	return &MatchEngine{
		orderBooks: make(map[string]*OrderBook),
	}
}

// getOrCreateOrderBook 获取或创建订单簿。
func (me *MatchEngine) getOrCreateOrderBook(symbol string) *OrderBook {
	me.mu.Lock()
	defer me.mu.Unlock()
	ob, ok := me.orderBooks[symbol]
	if !ok {
		ob = NewOrderBook(symbol)
		me.orderBooks[symbol] = ob
	}
	return ob
}

// Execute 执行撮合 — Atom 纯函数接口。
func (me *MatchEngine) Execute(ctx context.Context, input MatchInput) (MatchOutput, error) {
	order := input.Order
	if order == nil {
		return MatchOutput{}, fmt.Errorf("order is nil")
	}

	ob := me.getOrCreateOrderBook(order.Symbol)
	trades := make([]*Trade, 0)

	if order.Type == OrderTypeMarket {
		trades = me.matchMarketOrderAtom(ob, order)
	} else {
		trades = me.matchLimitOrderAtom(ob, order)
	}

	return MatchOutput{
		Trades:       trades,
		UpdatedOrder:  order,
		Matched:      len(trades) > 0,
	}, nil
}

// ──────────────────────────────────────────────
// Atom: 纯撮合算法
// ──────────────────────────────────────────────

// matchLimitOrderAtom 限价单撮合 — 纯函数。
func (me *MatchEngine) matchLimitOrderAtom(ob *OrderBook, order *Order) []*Trade {
	trades := make([]*Trade, 0)
	remainingQty := order.Quantity - order.FilledQty

	if order.Side == OrderSideBuy {
		for remainingQty > 0 {
			bestAsk, ok := ob.BestAsk()
			if !ok || order.Price < bestAsk {
				break
			}
			trade := me.matchWithBestAtom(ob, order, remainingQty, OrderSideSell)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	} else {
		for remainingQty > 0 {
			bestBid, ok := ob.BestBid()
			if !ok || order.Price > bestBid {
				break
			}
			trade := me.matchWithBestAtom(ob, order, remainingQty, OrderSideBuy)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	}

	if order.FilledQty >= order.Quantity {
		order.Status = OrderStatusFilled
		ob.RemoveOrder(order.OrderID)
	} else if order.FilledQty > 0 {
		order.Status = OrderStatusPartial
		ob.UpdateQuantity(order.OrderID, order.FilledQty)
		if order.Type == OrderTypeLimit {
			ob.AddOrder(order)
		}
	} else {
		order.Status = OrderStatusNew
		if order.Type == OrderTypeLimit {
			ob.AddOrder(order)
		}
	}
	order.UpdatedAt = time.Now()

	return trades
}

// matchMarketOrderAtom 市价单撮合 — 纯函数。
func (me *MatchEngine) matchMarketOrderAtom(ob *OrderBook, order *Order) []*Trade {
	trades := make([]*Trade, 0)
	remainingQty := order.Quantity - order.FilledQty

	if order.Side == OrderSideBuy {
		for remainingQty > 0 {
			if _, ok := ob.BestAsk(); !ok {
				break
			}
			trade := me.matchWithBestAtom(ob, order, remainingQty, OrderSideSell)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	} else {
		for remainingQty > 0 {
			if _, ok := ob.BestBid(); !ok {
				break
			}
			trade := me.matchWithBestAtom(ob, order, remainingQty, OrderSideBuy)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	}

	if order.FilledQty >= order.Quantity {
		order.Status = OrderStatusFilled
	} else if order.FilledQty > 0 {
		order.Status = OrderStatusPartial
	} else {
		order.Status = OrderStatusRejected
	}
	order.UpdatedAt = time.Now()

	return trades
}

// matchWithBestAtom 与最优价格订单撮合 — 纯函数。
func (me *MatchEngine) matchWithBestAtom(ob *OrderBook, order *Order, remainingQty float64, counterSide OrderSide) *Trade {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	var counterEntries []*OrderBookEntry
	if counterSide == OrderSideSell {
		counterEntries = ob.asks
	} else {
		counterEntries = ob.bids
	}

	if len(counterEntries) == 0 {
		return nil
	}
	counter := counterEntries[0]
	if counter.Quantity <= 0 || counter.UserID == order.UserID {
		return nil
	}

	tradeQty := counter.Quantity
	if remainingQty < tradeQty {
		tradeQty = remainingQty
	}

	me.tradeSeq++
	var buyOrderID, sellOrderID, buyUserID, sellUserID string
	if order.Side == OrderSideBuy {
		buyOrderID = order.OrderID
		buyUserID = order.UserID
		sellOrderID = counter.OrderID
		sellUserID = counter.UserID
	} else {
		buyOrderID = counter.OrderID
		buyUserID = counter.UserID
		sellOrderID = order.OrderID
		sellUserID = order.UserID
	}

	trade := &Trade{
		TradeID:     fmt.Sprintf("trade-%d", me.tradeSeq),
		BuyOrderID:  buyOrderID,
		SellOrderID: sellOrderID,
		Symbol:      order.Symbol,
		Price:       counter.Price,
		Quantity:    tradeQty,
		BuyUserID:   buyUserID,
		SellUserID:  sellUserID,
		Timestamp:   time.Now(),
		IsMaker:     true,
	}

	counter.Quantity -= tradeQty
	if counter.Quantity <= 0 {
		delete(ob.orders, counter.OrderID)
		if counterSide == OrderSideSell {
			ob.asks = ob.asks[1:]
		} else {
			ob.bids = ob.bids[1:]
		}
	}

	return trade
}

// ──────────────────────────────────────────────
// Adapter: 交易所通信层（外部 I/O）
// ──────────────────────────────────────────────

// GetOrderBook 获取指定交易对的订单簿。
func (me *MatchEngine) GetOrderBook(symbol string) *OrderBook {
	me.mu.RLock()
	defer me.mu.RUnlock()
	return me.orderBooks[symbol]
}

// GetSymbols 获取所有交易对。
func (me *MatchEngine) GetSymbols() []string {
	me.mu.RLock()
	defer me.mu.RUnlock()
	symbols := make([]string, 0, len(me.orderBooks))
	for s := range me.orderBooks {
		symbols = append(symbols, s)
	}
	return symbols
}

// Ensure MatchEngine implements Atom.
var _ Atom[MatchInput, MatchOutput] = (*MatchEngine)(nil)
