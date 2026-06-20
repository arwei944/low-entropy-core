// Package core — 币安交易所核心类型定义
// 基于 Low-Entropy Core 四原语架构：Atom / Port / Adapter / Composer
package core

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// 交易核心类型
// ──────────────────────────────────────────────

// OrderSide 买卖方向
type OrderSide int8

const (
	OrderSideBuy  OrderSide = 1
	OrderSideSell OrderSide = -1
)

// OrderType 订单类型
type OrderType int8

const (
	OrderTypeLimit      OrderType = iota // 限价单
	OrderTypeMarket                      // 市价单
	OrderTypeStopLoss                    // 止损单
	OrderTypeTakeProfit                  // 止盈单
	OrderTypeStopLimit                   // 止损限价
)

// OrderStatus 订单状态
type OrderStatus int8

const (
	OrderStatusNew       OrderStatus = iota // 新建
	OrderStatusPartial                      // 部分成交
	OrderStatusFilled                       // 完全成交
	OrderStatusCancelled                    // 已撤销
	OrderStatusRejected                     // 已拒绝
)

// Order 订单实体 — 纯数据，无副作用 (Atom 的输入)
type Order struct {
	OrderID       string    `json:"order_id"`
	UserID        string    `json:"user_id"`
	Symbol        string    `json:"symbol"`        // 交易对，如 BTCUSDT
	Side          OrderSide `json:"side"`
	Type          OrderType `json:"type"`
	Price         float64   `json:"price"`
	Quantity      float64   `json:"quantity"`
	FilledQty     float64   `json:"filled_qty"`
	Status        OrderStatus `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ClientOrderID string    `json:"client_order_id"` // 用户自定义ID，用于幂等
}

// Trade 成交记录 — 撮合结果 (Atom 的输出)
type Trade struct {
	TradeID      string    `json:"trade_id"`
	BuyOrderID   string    `json:"buy_order_id"`
	SellOrderID  string    `json:"sell_order_id"`
	Symbol       string    `json:"symbol"`
	Price        float64   `json:"price"`
	Quantity     float64   `json:"quantity"`
	BuyUserID    string    `json:"buy_user_id"`
	SellUserID   string    `json:"sell_user_id"`
	Timestamp    time.Time `json:"timestamp"`
	IsMaker      bool      `json:"is_maker"`      // 是否为挂单方
}

// OrderBookLevel 订单簿深度层级
type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// OrderBookSnapshot 订单簿快照
type OrderBookSnapshot struct {
	Symbol      string           `json:"symbol"`
	Bids        []OrderBookLevel `json:"bids"`  // 买单深度
	Asks        []OrderBookLevel `json:"asks"`  // 卖单深度
	LastUpdateID int64           `json:"last_update_id"`
	Timestamp   time.Time        `json:"timestamp"`
}

// KLine K线数据
type KLine struct {
	Symbol    string    `json:"symbol"`
	Interval  string    `json:"interval"`  // 1m, 5m, 1h, 1d
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	QuoteVol  float64   `json:"quote_volume"`
	Trades    int64     `json:"trades"`
}

// ──────────────────────────────────────────────
// 账户资产类型
// ──────────────────────────────────────────────

// AssetBalance 资产余额
type AssetBalance struct {
	UserID     string  `json:"user_id"`
	Asset      string  `json:"asset"`       // BTC, ETH, USDT
	Free       float64 `json:"free"`        // 可用
	Locked     float64 `json:"locked"`      // 冻结（挂单中）
	Total      float64 `json:"total"`       // 总余额
	UpdateTime time.Time `json:"update_time"`
}

// WalletType 钱包类型
type WalletType int8

const (
	WalletTypeSpot    WalletType = iota // 现货
	WalletTypeMargin                    // 杠杆
	WalletTypeFutures                   // 合约
	WalletTypeEarn                      // 理财
)

// Wallet 钱包
type Wallet struct {
	UserID     string     `json:"user_id"`
	Type       WalletType `json:"type"`
	Balances   map[string]*AssetBalance `json:"balances"`
	UpdateTime time.Time  `json:"update_time"`
}

// DepositRecord 充值记录
type DepositRecord struct {
	RecordID    string    `json:"record_id"`
	UserID      string    `json:"user_id"`
	Asset       string    `json:"asset"`
	Amount      float64   `json:"amount"`
	TxHash      string    `json:"tx_hash"`
	FromAddress string    `json:"from_address"`
	Status      int8      `json:"status"`    // 0 pending, 1 confirmed, 2 failed
	Confirmations int64   `json:"confirmations"`
	CreatedAt   time.Time `json:"created_at"`
}

// WithdrawRecord 提现记录
type WithdrawRecord struct {
	RecordID  string    `json:"record_id"`
	UserID    string    `json:"user_id"`
	Asset     string    `json:"asset"`
	Amount    float64   `json:"amount"`
	ToAddress string    `json:"to_address"`
	TxHash    string    `json:"tx_hash"`
	Status    int8      `json:"status"`
	Fee       float64   `json:"fee"`
	CreatedAt time.Time `json:"created_at"`
}

// ──────────────────────────────────────────────
// 风控类型
// ──────────────────────────────────────────────

// RiskLevel 风险等级
type RiskLevel int8

const (
	RiskLevelLow    RiskLevel = iota // 低风险
	RiskLevelMedium                  // 中风险
	RiskLevelHigh                    // 高风险
	RiskLevelCritical                // 极高风险
)

// RiskEvent 风险事件
type RiskEvent struct {
	EventID     string    `json:"event_id"`
	UserID      string    `json:"user_id"`
	Type        string    `json:"type"`        // 异常登录、大额提现、异常交易等
	Level       RiskLevel `json:"level"`
	Description string    `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time `json:"timestamp"`
}

// KYCStatus KYC状态
type KYCStatus int8

const (
	KYCStatusNone       KYCStatus = iota // 未认证
	KYCStatusPending                     // 审核中
	KYCStatusVerified                    // 已认证
	KYCStatusRejected                    // 已拒绝
)

// KYCInfo KYC信息
type KYCInfo struct {
	UserID      string    `json:"user_id"`
	Status      KYCStatus `json:"status"`
	Level       int8      `json:"level"`       // 1-3 级
	Country     string    `json:"country"`
	IDNumber    string    `json:"id_number"`
	VerifiedAt  time.Time `json:"verified_at"`
}

// ──────────────────────────────────────────────
// 清算结算类型
// ──────────────────────────────────────────────

// Position 持仓
type Position struct {
	UserID       string    `json:"user_id"`
	Symbol       string    `json:"symbol"`
	Side         OrderSide `json:"side"`
	Quantity     float64   `json:"quantity"`
	EntryPrice   float64   `json:"entry_price"`
	MarkPrice    float64   `json:"mark_price"`
	Leverage     float64   `json:"leverage"`
	Margin       float64   `json:"margin"`
	UnrealizedPNL float64  `json:"unrealized_pnl"`
	RealizedPNL  float64   `json:"realized_pnl"`
	LiquidationPrice float64 `json:"liquidation_price"`
	UpdateTime   time.Time `json:"update_time"`
}

// MarginMode 保证金模式
type MarginMode int8

const (
	MarginModeIsolated MarginMode = iota // 逐仓
	MarginModeCross                      // 全仓
)

// SettlementRecord 结算记录
type SettlementRecord struct {
	RecordID    string    `json:"record_id"`
	UserID      string    `json:"user_id"`
	Symbol      string    `json:"symbol"`
	Type        string    `json:"type"`        // trade, funding, liquidation
	Amount      float64   `json:"amount"`
	Asset       string    `json:"asset"`
	RelatedID   string    `json:"related_id"`  // 关联订单/交易ID
	Timestamp   time.Time `json:"timestamp"`
}

// ──────────────────────────────────────────────
// 基础设施类型
// ──────────────────────────────────────────────

// SymbolInfo 交易对信息
type SymbolInfo struct {
	Symbol          string  `json:"symbol"`
	BaseAsset       string  `json:"base_asset"`
	QuoteAsset      string  `json:"quote_asset"`
	PricePrecision  int     `json:"price_precision"`
	QtyPrecision    int     `json:"qty_precision"`
	MinQty          float64 `json:"min_qty"`
	MaxQty          float64 `json:"max_qty"`
	MinNotional     float64 `json:"min_notional"`
	Status          string  `json:"status"`      // TRADING, BREAK, HALT
}

// Ticker 24小时行情
type Ticker struct {
	Symbol     string    `json:"symbol"`
	PriceChange    float64 `json:"price_change"`
	PriceChangePct float64 `json:"price_change_pct"`
	WeightedAvgPrice float64 `json:"weighted_avg_price"`
	LastPrice      float64 `json:"last_price"`
	HighPrice      float64 `json:"high_price"`
	LowPrice       float64 `json:"low_price"`
	Volume         float64 `json:"volume"`
	QuoteVolume    float64 `json:"quote_volume"`
	OpenTime       time.Time `json:"open_time"`
	CloseTime      time.Time `json:"close_time"`
	Count          int64     `json:"count"`
}

// ──────────────────────────────────────────────
// 四原语接口定义
// ──────────────────────────────────────────────

// Atom 纯函数 — 无副作用的计算单元
type Atom[In, Out any] interface {
	Execute(ctx context.Context, input In) (Out, error)
}

// Port 边界校验 — 输入验证与契约定义
type Port[In, Out any] interface {
	Validate(input In) error
	Transform(input In) (In, error)
}

// Adapter 副作用隔离 — IO/网络/DB/外部交互
type Adapter[In, Out any] interface {
	Execute(ctx context.Context, input In) (Out, error)
	HealthCheck(ctx context.Context) error
}

// Composer 编排引擎 — Pipeline/Branch/Parallel
type Composer[In, Out any] interface {
	Run(ctx context.Context, input In) (Out, []ExecutionStep, error)
}

// ExecutionStep 执行步骤记录
type ExecutionStep struct {
	Name      string        `json:"name"`
	Input     interface{}   `json:"input"`
	Output    interface{}   `json:"output"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}