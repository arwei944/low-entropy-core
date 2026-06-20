// Package core — 交易流程编排器
// Composer: 编排引擎 — Pipeline/Branch/Parallel
// 将订单验证 -> 风控检查 -> 余额冻结 -> 撮合 -> 清算 -> 余额更新 编排为完整流程
package core

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// TradePipeline 交易流程编排器 — 实现 Composer 接口
// ──────────────────────────────────────────────

type TradePipelineInput struct {
	Order      *Order
	UserID     string
	IP         string
	DeviceID   string
}

type TradePipelineOutput struct {
	Success    bool
	Order      *Order
	Trades     []*Trade
	Error      string
}

type TradePipeline struct {
	orderPort     *OrderPort
	riskEngine    *RiskEngine
	walletAdapter *WalletAdapter
	matchEngine   *MatchEngine
	symbols       map[string]*SymbolInfo
}

// NewTradePipeline 创建交易流程编排器
func NewTradePipeline(
	orderPort *OrderPort,
	riskEngine *RiskEngine,
	walletAdapter *WalletAdapter,
	matchEngine *MatchEngine,
) *TradePipeline {
	return &TradePipeline{
		orderPort:     orderPort,
		riskEngine:    riskEngine,
		walletAdapter: walletAdapter,
		matchEngine:   matchEngine,
		symbols:       make(map[string]*SymbolInfo),
	}
}

// RegisterSymbol 注册交易对
func (tp *TradePipeline) RegisterSymbol(symbol *SymbolInfo) {
	tp.symbols[symbol.Symbol] = symbol
}

// Run 执行完整交易流程 — Composer 接口
// Pipeline: 验证 -> 风控 -> 冻结 -> 撮合 -> 清算 -> 更新
func (tp *TradePipeline) Run(ctx context.Context, input TradePipelineInput) (TradePipelineOutput, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0)
	startTime := time.Now()

	// Step 1: 订单验证 (Port)
	step1Start := time.Now()
	validationInput := OrderValidationInput{
		Order:  input.Order,
		UserID: input.UserID,
		Symbol: tp.symbols[input.Order.Symbol],
	}

	if err := tp.orderPort.Validate(validationInput); err != nil {
		steps = append(steps, ExecutionStep{
			Name:      "OrderValidation",
			Input:     validationInput,
			Error:     err.Error(),
			Duration:  time.Since(step1Start),
			Timestamp: time.Now(),
		})
		return TradePipelineOutput{
			Success: false,
			Error:   fmt.Sprintf("validation failed: %v", err),
		}, steps, nil
	}

	validationInput, _ = tp.orderPort.Transform(validationInput)
	steps = append(steps, ExecutionStep{
		Name:      "OrderValidation",
		Input:     validationInput,
		Output:    validationInput,
		Duration:  time.Since(step1Start),
		Timestamp: time.Now(),
	})

	// Step 2: 风控检查
	step2Start := time.Now()
	riskInput := RiskCheckInput{
		UserID:   input.UserID,
		Order:    input.Order,
		Action:   "place_order",
		IP:       input.IP,
		DeviceID: input.DeviceID,
	}
	riskResult := tp.riskEngine.Check(riskInput)
	steps = append(steps, ExecutionStep{
		Name:      "RiskCheck",
		Input:     riskInput,
		Output:    riskResult,
		Duration:  time.Since(step2Start),
		Timestamp: time.Now(),
	})

	if !riskResult.Allowed {
		return TradePipelineOutput{
			Success: false,
			Error:   fmt.Sprintf("risk check failed: %s", riskResult.Reason),
		}, steps, nil
	}

	// Step 3: 余额检查与冻结 (Adapter)
	step3Start := time.Now()
	symbol := tp.symbols[input.Order.Symbol]
	if err := tp.orderPort.CheckBalance(input.UserID, input.Order, symbol); err != nil {
		steps = append(steps, ExecutionStep{
			Name:      "BalanceCheck",
			Input:     input.Order,
			Error:     err.Error(),
			Duration:  time.Since(step3Start),
			Timestamp: time.Now(),
		})
		return TradePipelineOutput{
			Success: false,
			Error:   fmt.Sprintf("balance check failed: %v", err),
		}, steps, nil
	}

	// 冻结余额
	var freezeAsset string
	var freezeAmount float64
	if input.Order.Side == OrderSideBuy {
		freezeAsset = symbol.QuoteAsset
		if input.Order.Type == OrderTypeMarket {
			// 市价单按最优卖价估算冻结金额
			if ob := tp.matchEngine.GetOrderBook(input.Order.Symbol); ob != nil {
				if price, has := ob.BestAsk(); has {
					freezeAmount = price * input.Order.Quantity * 1.05 // 5% 滑点保护
				} else {
					freezeAmount = input.Order.Quantity * 1000000 // 无对手方时大额冻结
				}
			} else {
				freezeAmount = input.Order.Quantity * 1000000
			}
		} else {
			freezeAmount = input.Order.Price * input.Order.Quantity
		}
	} else {
		freezeAsset = symbol.BaseAsset
		freezeAmount = input.Order.Quantity
	}

	if err := tp.walletAdapter.LockBalance(input.UserID, freezeAsset, freezeAmount); err != nil {
		steps = append(steps, ExecutionStep{
			Name:      "BalanceFreeze",
			Input:     map[string]interface{}{"asset": freezeAsset, "amount": freezeAmount},
			Error:     err.Error(),
			Duration:  time.Since(step3Start),
			Timestamp: time.Now(),
		})
		return TradePipelineOutput{
			Success: false,
			Error:   fmt.Sprintf("balance freeze failed: %v", err),
		}, steps, nil
	}

	steps = append(steps, ExecutionStep{
		Name:      "BalanceFreeze",
		Input:     map[string]interface{}{"asset": freezeAsset, "amount": freezeAmount},
		Output:    "success",
		Duration:  time.Since(step3Start),
		Timestamp: time.Now(),
	})

	// Step 4: 撮合 (Atom)
	step4Start := time.Now()
	matchInput := MatchInput{Order: input.Order}
	matchOutput, err := tp.matchEngine.Execute(ctx, matchInput)
	if err != nil {
		// 撮合失败，解冻余额
		tp.walletAdapter.UnlockBalance(input.UserID, freezeAsset, freezeAmount)
		steps = append(steps, ExecutionStep{
			Name:      "MatchEngine",
			Input:     matchInput,
			Error:     err.Error(),
			Duration:  time.Since(step4Start),
			Timestamp: time.Now(),
		})
		return TradePipelineOutput{
			Success: false,
			Error:   fmt.Sprintf("match failed: %v", err),
		}, steps, nil
	}

	steps = append(steps, ExecutionStep{
		Name:      "MatchEngine",
		Input:     matchInput,
		Output:    matchOutput,
		Duration:  time.Since(step4Start),
		Timestamp: time.Now(),
	})

	// Step 5: 清算结算 (Adapter)
	step5Start := time.Now()
	if len(matchOutput.Trades) > 0 {
		for _, trade := range matchOutput.Trades {
			actualCost := trade.Price * trade.Quantity

			// 处理对手方余额（对方用户的订单已在订单簿中，需要解冻并更新）
			if input.Order.Side == OrderSideBuy {
				// 当前是买方下单，对手方是卖方（已在订单簿中的卖单）
				// 卖方解冻基础资产并获得报价资产
				tp.walletAdapter.UnlockBalance(trade.SellUserID, symbol.BaseAsset, trade.Quantity)
				tp.walletAdapter.Execute(ctx, BalanceUpdateInput{
					UserID: trade.SellUserID,
					Asset:  symbol.QuoteAsset,
					Delta:  actualCost,
					Reason: "trade",
				})

				// 买方：解冻差额并获得基础资产
				if freezeAmount > actualCost {
					tp.walletAdapter.UnlockBalance(input.UserID, freezeAsset, freezeAmount-actualCost)
				}
				tp.walletAdapter.Execute(ctx, BalanceUpdateInput{
					UserID: input.UserID,
					Asset:  symbol.BaseAsset,
					Delta:  trade.Quantity,
					Reason: "trade",
				})
			} else {
				// 当前是卖方下单，对手方是买方（已在订单簿中的买单）
				// 买方解冻报价资产并获得基础资产
				// 注意：买方的冻结金额是按其订单价格计算的，可能与实际成交价不同
				buyerFreezeAmount := trade.Price * trade.Quantity // 简化处理，实际应按买方原订单价格
				tp.walletAdapter.UnlockBalance(trade.BuyUserID, symbol.QuoteAsset, buyerFreezeAmount)
				tp.walletAdapter.Execute(ctx, BalanceUpdateInput{
					UserID: trade.BuyUserID,
					Asset:  symbol.BaseAsset,
					Delta:  trade.Quantity,
					Reason: "trade",
				})

				// 卖方：解冻并扣除卖出的基础资产，获得报价资产
			tp.walletAdapter.UnlockBalance(input.UserID, freezeAsset, freezeAmount)
			// 扣除已卖出的基础资产
			tp.walletAdapter.Execute(ctx, BalanceUpdateInput{
				UserID: input.UserID,
				Asset:  symbol.BaseAsset,
				Delta:  -trade.Quantity,
				Reason: "trade",
			})
			// 获得报价资产
			tp.walletAdapter.Execute(ctx, BalanceUpdateInput{
				UserID: input.UserID,
				Asset:  symbol.QuoteAsset,
				Delta:  actualCost,
				Reason: "trade",
			})
			}
		}
	} else {
		// 未成交
		if input.Order.Type == OrderTypeMarket {
			// 市价单未成交则拒绝并解冻
			tp.walletAdapter.UnlockBalance(input.UserID, freezeAsset, freezeAmount)
			matchOutput.UpdatedOrder.Status = OrderStatusRejected
		}
		// 限价单保持冻结（已入簿，等待成交或撤单）
	}

	steps = append(steps, ExecutionStep{
		Name:      "Settlement",
		Input:     matchOutput.Trades,
		Output:    "completed",
		Duration:  time.Since(step5Start),
		Timestamp: time.Now(),
	})

	// 总耗时
	totalDuration := time.Since(startTime)
	steps = append(steps, ExecutionStep{
		Name:      "Total",
		Duration:  totalDuration,
		Timestamp: time.Now(),
	})

	return TradePipelineOutput{
		Success: true,
		Order:   matchOutput.UpdatedOrder,
		Trades:  matchOutput.Trades,
	}, steps, nil
}

// Ensure TradePipeline implements Composer
var _ Composer[TradePipelineInput, TradePipelineOutput] = (*TradePipeline)(nil)