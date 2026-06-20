// Package core — 撮合引擎 (MatchBox)
// 基于 Low-Entropy Core 四原语架构
// Atom: 纯内存撮合，无副作用
package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// OrderBookEntry 订单簿条目 — 内存化存储
// ──────────────────────────────────────────────

type OrderBookEntry struct {
	OrderID   string
	UserID    string
	Price     float64
	Quantity  float64
	Side      OrderSide
	Timestamp time.Time
}

// OrderBook 内存订单簿 — 纯内存数据结构
// 使用跳表/红黑树优化可达到 O(log n)，此处用有序切片演示
type OrderBook struct {
	mu       sync.RWMutex
	symbol   string
	bids     []*OrderBookEntry // 买单：价格降序
	asks     []*OrderBookEntry // 卖单：价格升序
	orders   map[string]*OrderBookEntry // orderID -> entry (快速查找)
	updateID int64
}

// NewOrderBook 创建订单簿
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make([]*OrderBookEntry, 0),
		asks:   make([]*OrderBookEntry, 0),
		orders: make(map[string]*OrderBookEntry),
	}
}

// AddOrder 添加订单到订单簿
func (ob *OrderBook) AddOrder(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	entry := &OrderBookEntry{
		OrderID:   order.OrderID,
		UserID:    order.UserID,
		Price:     order.Price,
		Quantity:  order.Quantity - order.FilledQty,
		Side:      order.Side,
		Timestamp: order.CreatedAt,
	}
	ob.orders[order.OrderID] = entry

	if order.Side == OrderSideBuy {
		ob.bids = append(ob.bids, entry)
		// 价格降序排序
		sort.Slice(ob.bids, func(i, j int) bool {
			return ob.bids[i].Price > ob.bids[j].Price
		})
	} else {
		ob.asks = append(ob.asks, entry)
		// 价格升序排序
		sort.Slice(ob.asks, func(i, j int) bool {
			return ob.asks[i].Price < ob.asks[j].Price
		})
	}
	ob.updateID++
}

// RemoveOrder 从订单簿移除订单
func (ob *OrderBook) RemoveOrder(orderID string) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	entry, ok := ob.orders[orderID]
	if !ok {
		return
	}
	delete(ob.orders, orderID)

	if entry.Side == OrderSideBuy {
		for i, e := range ob.bids {
			if e.OrderID == orderID {
				ob.bids = append(ob.bids[:i], ob.bids[i+1:]...)
				break
			}
		}
	} else {
		for i, e := range ob.asks {
			if e.OrderID == orderID {
				ob.asks = append(ob.asks[:i], ob.asks[i+1:]...)
				break
			}
		}
	}
	ob.updateID++
}

// UpdateQuantity 更新订单数量（部分成交后）
func (ob *OrderBook) UpdateQuantity(orderID string, filledQty float64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	entry, ok := ob.orders[orderID]
	if !ok {
		return
	}
	entry.Quantity -= filledQty
	if entry.Quantity <= 0 {
		delete(ob.orders, orderID)
		if entry.Side == OrderSideBuy {
			for i, e := range ob.bids {
				if e.OrderID == orderID {
					ob.bids = append(ob.bids[:i], ob.bids[i+1:]...)
					break
				}
			}
		} else {
			for i, e := range ob.asks {
				if e.OrderID == orderID {
					ob.asks = append(ob.asks[:i], ob.asks[i+1:]...)
					break
				}
			}
		}
	}
	ob.updateID++
}

// Snapshot 获取订单簿快照
func (ob *OrderBook) Snapshot() OrderBookSnapshot {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]OrderBookLevel, 0, len(ob.bids))
	for _, e := range ob.bids {
		if e.Quantity > 0 {
			bids = append(bids, OrderBookLevel{Price: e.Price, Quantity: e.Quantity})
		}
	}

	asks := make([]OrderBookLevel, 0, len(ob.asks))
	for _, e := range ob.asks {
		if e.Quantity > 0 {
			asks = append(asks, OrderBookLevel{Price: e.Price, Quantity: e.Quantity})
		}
	}

	return OrderBookSnapshot{
		Symbol:       ob.symbol,
		Bids:         bids,
		Asks:         asks,
		LastUpdateID: ob.updateID,
		Timestamp:    time.Now(),
	}
}

// BestBid 最优买价
func (ob *OrderBook) BestBid() (float64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.bids) == 0 {
		return 0, false
	}
	return ob.bids[0].Price, true
}

// BestAsk 最优卖价
func (ob *OrderBook) BestAsk() (float64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.asks) == 0 {
		return 0, false
	}
	return ob.asks[0].Price, true
}

// ──────────────────────────────────────────────
// MatchEngine 撮合引擎 — Atom 纯函数实现
// ──────────────────────────────────────────────

// MatchInput 撮合输入
type MatchInput struct {
	Order *Order
}

// MatchOutput 撮合输出
type MatchOutput struct {
	Trades      []*Trade
	UpdatedOrder *Order
	Matched     bool
}

// MatchEngine 撮合引擎 — 实现 Atom[MatchInput, MatchOutput]
type MatchEngine struct {
	orderBooks map[string]*OrderBook // symbol -> orderBook
	mu         sync.RWMutex
	tradeSeq   int64
}

// NewMatchEngine 创建撮合引擎
func NewMatchEngine() *MatchEngine {
	return &MatchEngine{
		orderBooks: make(map[string]*OrderBook),
	}
}

// getOrCreateOrderBook 获取或创建订单簿
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

// Execute 执行撮合 — Atom 纯函数接口
// 输入：新订单
// 输出：成交记录列表 + 更新后的订单
func (me *MatchEngine) Execute(ctx context.Context, input MatchInput) (MatchOutput, error) {
	order := input.Order
	if order == nil {
		return MatchOutput{}, fmt.Errorf("order is nil")
	}

	ob := me.getOrCreateOrderBook(order.Symbol)
	trades := make([]*Trade, 0)

	// 市价单直接以最优价格成交
	if order.Type == OrderTypeMarket {
		trades = me.matchMarketOrder(ob, order)
	} else {
		// 限价单先尝试撮合，未成交部分入簿
		trades = me.matchLimitOrder(ob, order)
	}

	matched := len(trades) > 0
	return MatchOutput{
		Trades:       trades,
		UpdatedOrder: order,
		Matched:      matched,
	}, nil
}

// matchLimitOrder 限价单撮合
func (me *MatchEngine) matchLimitOrder(ob *OrderBook, order *Order) []*Trade {
	trades := make([]*Trade, 0)
	remainingQty := order.Quantity - order.FilledQty

	if order.Side == OrderSideBuy {
		// 买单：与卖单撮合，买价 >= 卖价即可成交
		for remainingQty > 0 {
			bestAsk, ok := ob.BestAsk()
			if !ok || order.Price < bestAsk {
				break // 无法继续撮合
			}
			// 找到最优卖单并撮合
			trade := me.matchWithBest(ob, order, remainingQty, OrderSideSell)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	} else {
		// 卖单：与买单撮合，卖价 <= 买价即可成交
		for remainingQty > 0 {
			bestBid, ok := ob.BestBid()
			if !ok || order.Price > bestBid {
				break
			}
			trade := me.matchWithBest(ob, order, remainingQty, OrderSideBuy)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	}

	// 更新订单状态
	if order.FilledQty >= order.Quantity {
		order.Status = OrderStatusFilled
		ob.RemoveOrder(order.OrderID)
	} else if order.FilledQty > 0 {
		order.Status = OrderStatusPartial
		ob.UpdateQuantity(order.OrderID, order.FilledQty)
		if order.Type == OrderTypeLimit {
			ob.AddOrder(order) // 未成交部分入簿
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

// matchMarketOrder 市价单撮合
func (me *MatchEngine) matchMarketOrder(ob *OrderBook, order *Order) []*Trade {
	trades := make([]*Trade, 0)
	remainingQty := order.Quantity - order.FilledQty

	if order.Side == OrderSideBuy {
		for remainingQty > 0 {
			_, ok := ob.BestAsk()
			if !ok {
				break
			}
			trade := me.matchWithBest(ob, order, remainingQty, OrderSideSell)
			if trade == nil {
				break
			}
			trades = append(trades, trade)
			remainingQty -= trade.Quantity
			order.FilledQty += trade.Quantity
		}
	} else {
		for remainingQty > 0 {
			_, ok := ob.BestBid()
			if !ok {
				break
			}
			trade := me.matchWithBest(ob, order, remainingQty, OrderSideBuy)
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
		order.Status = OrderStatusRejected // 市价单无对手方则拒绝
	}
	order.UpdatedAt = time.Now()

	return trades
}

// matchWithBest 与最优价格订单撮合
func (me *MatchEngine) matchWithBest(ob *OrderBook, order *Order, remainingQty float64, counterSide OrderSide) *Trade {
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
	if counter.Quantity <= 0 {
		return nil
	}

	// 检查自成交（同一用户）
	if counter.UserID == order.UserID {
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
		IsMaker:     true, // 挂单方是 maker
	}

	// 更新对手方数量
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

// GetOrderBook 获取指定交易对的订单簿
func (me *MatchEngine) GetOrderBook(symbol string) *OrderBook {
	me.mu.RLock()
	defer me.mu.RUnlock()
	return me.orderBooks[symbol]
}

// GetSymbols 获取所有交易对
func (me *MatchEngine) GetSymbols() []string {
	me.mu.RLock()
	defer me.mu.RUnlock()
	symbols := make([]string, 0, len(me.orderBooks))
	for s := range me.orderBooks {
		symbols = append(symbols, s)
	}
	return symbols
}

// Ensure MatchEngine implements Atom
var _ Atom[MatchInput, MatchOutput] = (*MatchEngine)(nil)