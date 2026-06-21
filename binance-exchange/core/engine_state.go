// Package core — 撮合引擎：状态管理层 (MatchBox)
// L1 层：OrderBook — 纯内存数据结构，无 I/O，无副作用。
package core

import (
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// OrderBookEntry 订单簿条目
// ──────────────────────────────────────────────

// OrderBookEntry 订单簿条目 — 内存化存储。
type OrderBookEntry struct {
	OrderID   string
	UserID    string
	Price     float64
	Quantity  float64
	Side      OrderSide
	Timestamp time.Time
}

// ──────────────────────────────────────────────
// OrderBook 内存订单簿
// ──────────────────────────────────────────────

// OrderBook 内存订单簿 — 纯内存数据结构。
// 使用有序切片（O(n) 插入），生产环境可替换为跳表/红黑树。
type OrderBook struct {
	mu       sync.RWMutex
	symbol   string
	bids     []*OrderBookEntry // 买单：价格降序
	asks     []*OrderBookEntry // 卖单：价格升序
	orders   map[string]*OrderBookEntry // orderID -> entry
	updateID int64
}

// NewOrderBook 创建订单簿。
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make([]*OrderBookEntry, 0),
		asks:   make([]*OrderBookEntry, 0),
		orders: make(map[string]*OrderBookEntry),
	}
}

// AddOrder 添加订单到订单簿。
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
		sort.Slice(ob.bids, func(i, j int) bool {
			return ob.bids[i].Price > ob.bids[j].Price
		})
	} else {
		ob.asks = append(ob.asks, entry)
		sort.Slice(ob.asks, func(i, j int) bool {
			return ob.asks[i].Price < ob.asks[j].Price
		})
	}
	ob.updateID++
}

// RemoveOrder 从订单簿移除订单。
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

// UpdateQuantity 更新订单数量（部分成交后）。
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

// Snapshot 获取订单簿快照。
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

// BestBid 返回最优买价。
func (ob *OrderBook) BestBid() (float64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.bids) == 0 {
		return 0, false
	}
	return ob.bids[0].Price, true
}

// BestAsk 返回最优卖价。
func (ob *OrderBook) BestAsk() (float64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.asks) == 0 {
		return 0, false
	}
	return ob.asks[0].Price, true
}
