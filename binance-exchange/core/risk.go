// Package core — 风控引擎
// Port: 边界校验 — 订单验证、风控检查、KYC验证
package core

import (
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// OrderPort 订单校验端口 — 实现 Port 接口
// 验证订单参数合法性、用户权限、余额充足性
// ──────────────────────────────────────────────

type OrderValidationInput struct {
	Order  *Order
	UserID string
	Symbol *SymbolInfo
}

type OrderValidationOutput struct {
	Valid   bool
	Order   *Order
	Error   string
}

type OrderPort struct {
	walletAdapter *WalletAdapter
	kycStore      map[string]*KYCInfo
	minOrderValue float64
	maxOrderValue float64
}

// NewOrderPort 创建订单校验端口
func NewOrderPort(wallet *WalletAdapter) *OrderPort {
	return &OrderPort{
		walletAdapter: wallet,
		kycStore:      make(map[string]*KYCInfo),
		minOrderValue: 10.0,     // 最小订单价值 10 USDT
		maxOrderValue: 1000000.0, // 最大订单价值 1M USDT
	}
}

// Validate 验证订单 — Port 接口
func (op *OrderPort) Validate(input OrderValidationInput) error {
	order := input.Order
	if order == nil {
		return fmt.Errorf("order is nil")
	}
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if order.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if order.Type == OrderTypeLimit && order.Price <= 0 {
		return fmt.Errorf("limit order price must be positive")
	}
	if order.Side != OrderSideBuy && order.Side != OrderSideSell {
		return fmt.Errorf("invalid order side")
	}

	// 检查交易对状态
	if input.Symbol != nil && input.Symbol.Status != "TRADING" {
		return fmt.Errorf("symbol %s is not trading", order.Symbol)
	}

	// 检查订单价值
	orderValue := order.Price * order.Quantity
	if order.Type == OrderTypeLimit {
		if orderValue < op.minOrderValue {
			return fmt.Errorf("order value %.2f below minimum %.2f", orderValue, op.minOrderValue)
		}
		if orderValue > op.maxOrderValue {
			return fmt.Errorf("order value %.2f exceeds maximum %.2f", orderValue, op.maxOrderValue)
		}
	}

	// 检查精度
	if input.Symbol != nil {
		// 价格精度检查
		priceStr := fmt.Sprintf("%.*f", input.Symbol.PricePrecision, order.Price)
		if priceStr != fmt.Sprintf("%.*f", input.Symbol.PricePrecision, order.Price) {
			return fmt.Errorf("price precision exceeds %d", input.Symbol.PricePrecision)
		}
		// 数量精度检查
		qtyStr := fmt.Sprintf("%.*f", input.Symbol.QtyPrecision, order.Quantity)
		if qtyStr != fmt.Sprintf("%.*f", input.Symbol.QtyPrecision, order.Quantity) {
			return fmt.Errorf("quantity precision exceeds %d", input.Symbol.QtyPrecision)
		}
		// 最小数量检查
		if order.Quantity < input.Symbol.MinQty {
			return fmt.Errorf("quantity %.8f below minimum %.8f", order.Quantity, input.Symbol.MinQty)
		}
	}

	// KYC 检查
	kyc, ok := op.kycStore[input.UserID]
	if !ok || kyc.Status != KYCStatusVerified {
		return fmt.Errorf("KYC not verified")
	}

	return nil
}

// Transform 转换订单 — Port 接口
// 填充默认值、生成订单ID等
func (op *OrderPort) Transform(input OrderValidationInput) (OrderValidationInput, error) {
	order := input.Order
	if order.OrderID == "" {
		order.OrderID = fmt.Sprintf("order-%d", time.Now().UnixNano())
	}
	if order.ClientOrderID == "" {
		order.ClientOrderID = order.OrderID
	}
	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}
	if order.Status == 0 {
		order.Status = OrderStatusNew
	}
	return input, nil
}

// CheckBalance 检查余额充足性
func (op *OrderPort) CheckBalance(userID string, order *Order, symbol *SymbolInfo) error {
	var asset string
	var required float64

	if order.Side == OrderSideBuy {
		asset = symbol.QuoteAsset // 买入需要报价资产（如 USDT）
		required = order.Price * order.Quantity
	} else {
		asset = symbol.BaseAsset // 卖出需要基础资产（如 BTC）
		required = order.Quantity
	}

	balance, ok := op.walletAdapter.GetBalance(userID, asset)
	if !ok {
		return fmt.Errorf("no balance for asset %s", asset)
	}
	if balance.Free < required {
		return fmt.Errorf("insufficient balance: need %.8f %s, have %.8f", required, asset, balance.Free)
	}

	return nil
}

// SetKYC 设置用户KYC信息
func (op *OrderPort) SetKYC(kyc *KYCInfo) {
	op.kycStore[kyc.UserID] = kyc
}

// Ensure OrderPort implements Port
var _ Port[OrderValidationInput, OrderValidationOutput] = (*OrderPort)(nil)

// ──────────────────────────────────────────────
// RiskEngine 风控引擎 — 实时风险监控
// ──────────────────────────────────────────────

type RiskCheckInput struct {
	UserID    string
	Order     *Order
	Action    string // place_order, withdraw, login
	IP        string
	DeviceID  string
}

type RiskCheckOutput struct {
	Allowed     bool
	RiskLevel   RiskLevel
	Reason      string
	Require2FA  bool
}

type RiskEngine struct {
	userRiskScores map[string]float64 // 用户风险评分
	blacklistIPs   map[string]bool
	blacklistAddrs map[string]bool // 区块链地址黑名单
	dailyLimits    map[string]map[string]float64 // userID -> asset -> daily limit
	dailyUsed      map[string]map[string]float64 // userID -> asset -> daily used
}

// NewRiskEngine 创建风控引擎
func NewRiskEngine() *RiskEngine {
	return &RiskEngine{
		userRiskScores: make(map[string]float64),
		blacklistIPs:   make(map[string]bool),
		blacklistAddrs: make(map[string]bool),
		dailyLimits:    make(map[string]map[string]float64),
		dailyUsed:      make(map[string]map[string]float64),
	}
}

// Check 执行风控检查
func (re *RiskEngine) Check(input RiskCheckInput) RiskCheckOutput {
	// 1. IP 黑名单检查
	if re.blacklistIPs[input.IP] {
		return RiskCheckOutput{
			Allowed:   false,
			RiskLevel: RiskLevelCritical,
			Reason:    "IP blacklisted",
		}
	}

	// 2. 用户风险评分
	score := re.userRiskScores[input.UserID]
	if score > 80 {
		return RiskCheckOutput{
			Allowed:    false,
			RiskLevel:  RiskLevelCritical,
			Reason:     "user risk score too high",
			Require2FA: true,
		}
	}
	if score > 50 {
		return RiskCheckOutput{
			Allowed:    true,
			RiskLevel:  RiskLevelHigh,
			Reason:     "elevated risk score",
			Require2FA: true,
		}
	}

	// 3. 大额交易检查
	if input.Order != nil {
		orderValue := input.Order.Price * input.Order.Quantity
		if orderValue > 100000 { // 10万 USDT 以上
			return RiskCheckOutput{
				Allowed:    true,
				RiskLevel:  RiskLevelMedium,
				Reason:     "large order requires 2FA",
				Require2FA: true,
			}
		}
	}

	// 4. 提现地址黑名单检查
	if input.Action == "withdraw" {
		// 检查提现地址是否在黑名单
	}

	return RiskCheckOutput{
		Allowed:   true,
		RiskLevel: RiskLevelLow,
	}
}

// AddBlacklistIP 添加IP到黑名单
func (re *RiskEngine) AddBlacklistIP(ip string) {
	re.blacklistIPs[ip] = true
}

// AddBlacklistAddr 添加地址到黑名单
func (re *RiskEngine) AddBlacklistAddr(addr string) {
	re.blacklistAddrs[addr] = true
}

// UpdateRiskScore 更新用户风险评分
func (re *RiskEngine) UpdateRiskScore(userID string, delta float64) {
	re.userRiskScores[userID] += delta
	if re.userRiskScores[userID] < 0 {
		re.userRiskScores[userID] = 0
	}
}

// SetDailyLimit 设置日限额
func (re *RiskEngine) SetDailyLimit(userID, asset string, limit float64) {
	if re.dailyLimits[userID] == nil {
		re.dailyLimits[userID] = make(map[string]float64)
		re.dailyUsed[userID] = make(map[string]float64)
	}
	re.dailyLimits[userID][asset] = limit
}

// CheckDailyLimit 检查日限额
func (re *RiskEngine) CheckDailyLimit(userID, asset string, amount float64) bool {
	limit, ok := re.dailyLimits[userID][asset]
	if !ok {
		return true // 无限额
	}
	used := re.dailyUsed[userID][asset]
	return used+amount <= limit
}