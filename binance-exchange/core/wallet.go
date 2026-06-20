// Package core — 钱包与资产管理
// Adapter: 副作用隔离（余额更新、数据库写入）
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// WalletAdapter 钱包适配器 — 实现 Adapter 接口
// 管理用户资产余额，支持多币种、多钱包类型
// ──────────────────────────────────────────────

type BalanceUpdateInput struct {
	UserID   string
	Asset    string
	WalletType WalletType
	Delta    float64 // 正数增加，负数减少
	Reason   string  // 变更原因
}

type BalanceUpdateOutput struct {
	Success   bool
	Balance   *AssetBalance
	Error     string
}

type WalletAdapter struct {
	mu       sync.RWMutex
	wallets  map[string]*Wallet // userID -> Wallet
	deposits map[string]*DepositRecord
	withdraws map[string]*WithdrawRecord
}

// NewWalletAdapter 创建钱包适配器
func NewWalletAdapter() *WalletAdapter {
	return &WalletAdapter{
		wallets:   make(map[string]*Wallet),
		deposits:  make(map[string]*DepositRecord),
		withdraws: make(map[string]*WithdrawRecord),
	}
}

// Execute 执行余额更新 — Adapter 接口
func (wa *WalletAdapter) Execute(ctx context.Context, input BalanceUpdateInput) (BalanceUpdateOutput, error) {
	wa.mu.Lock()
	defer wa.mu.Unlock()

	wallet, ok := wa.wallets[input.UserID]
	if !ok {
		// 创建新钱包
		wallet = &Wallet{
			UserID:     input.UserID,
			Type:       input.WalletType,
			Balances:   make(map[string]*AssetBalance),
			UpdateTime: time.Now(),
		}
		wa.wallets[input.UserID] = wallet
	}

	balance, ok := wallet.Balances[input.Asset]
	if !ok {
		balance = &AssetBalance{
			UserID: input.UserID,
			Asset:  input.Asset,
			Free:   0,
			Locked: 0,
			Total:  0,
		}
		wallet.Balances[input.Asset] = balance
	}

	// 检查余额充足性（减少时）
	if input.Delta < 0 && balance.Free < -input.Delta {
		return BalanceUpdateOutput{
			Success: false,
			Error:   fmt.Sprintf("insufficient balance: need %.8f, have %.8f", -input.Delta, balance.Free),
		}, nil
	}

	// 更新余额
	balance.Free += input.Delta
	balance.Total = balance.Free + balance.Locked
	balance.UpdateTime = time.Now()
	wallet.UpdateTime = time.Now()

	return BalanceUpdateOutput{
		Success: true,
		Balance: balance,
	}, nil
}

// HealthCheck 健康检查
func (wa *WalletAdapter) HealthCheck(ctx context.Context) error {
	return nil
}

// GetBalance 查询余额
func (wa *WalletAdapter) GetBalance(userID, asset string) (*AssetBalance, bool) {
	wa.mu.RLock()
	defer wa.mu.RUnlock()

	wallet, ok := wa.wallets[userID]
	if !ok {
		return nil, false
	}
	balance, ok := wallet.Balances[asset]
	return balance, ok
}

// GetWallet 获取用户钱包
func (wa *WalletAdapter) GetWallet(userID string) (*Wallet, bool) {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	wallet, ok := wa.wallets[userID]
	return wallet, ok
}

// LockBalance 冻结余额（挂单时）
func (wa *WalletAdapter) LockBalance(userID, asset string, amount float64) error {
	wa.mu.Lock()
	defer wa.mu.Unlock()

	wallet, ok := wa.wallets[userID]
	if !ok {
		return fmt.Errorf("wallet not found for user %s", userID)
	}
	balance, ok := wallet.Balances[asset]
	if !ok {
		return fmt.Errorf("balance not found for asset %s", asset)
	}
	if balance.Free < amount {
		return fmt.Errorf("insufficient free balance")
	}

	balance.Free -= amount
	balance.Locked += amount
	balance.Total = balance.Free + balance.Locked
	balance.UpdateTime = time.Now()
	return nil
}

// UnlockBalance 解冻余额（撤单时）
func (wa *WalletAdapter) UnlockBalance(userID, asset string, amount float64) error {
	wa.mu.Lock()
	defer wa.mu.Unlock()

	wallet, ok := wa.wallets[userID]
	if !ok {
		return fmt.Errorf("wallet not found")
	}
	balance, ok := wallet.Balances[asset]
	if !ok {
		return fmt.Errorf("balance not found")
	}
	if balance.Locked < amount {
		return fmt.Errorf("insufficient locked balance")
	}

	balance.Locked -= amount
	balance.Free += amount
	balance.Total = balance.Free + balance.Locked
	balance.UpdateTime = time.Now()
	return nil
}

// ProcessDeposit 处理充值
func (wa *WalletAdapter) ProcessDeposit(record *DepositRecord) error {
	wa.mu.Lock()
	defer wa.mu.Unlock()

	wa.deposits[record.RecordID] = record

	// 更新余额
	wallet, ok := wa.wallets[record.UserID]
	if !ok {
		wallet = &Wallet{
			UserID:     record.UserID,
			Type:       WalletTypeSpot,
			Balances:   make(map[string]*AssetBalance),
			UpdateTime: time.Now(),
		}
		wa.wallets[record.UserID] = wallet
	}

	balance, ok := wallet.Balances[record.Asset]
	if !ok {
		balance = &AssetBalance{
			UserID: record.UserID,
			Asset:  record.Asset,
		}
		wallet.Balances[record.Asset] = balance
	}

	balance.Free += record.Amount
	balance.Total = balance.Free + balance.Locked
	balance.UpdateTime = time.Now()
	record.Status = 1 // confirmed

	return nil
}

// ProcessWithdraw 处理提现
func (wa *WalletAdapter) ProcessWithdraw(record *WithdrawRecord) error {
	wa.mu.Lock()
	defer wa.mu.Unlock()

	wallet, ok := wa.wallets[record.UserID]
	if !ok {
		return fmt.Errorf("wallet not found")
	}
	balance, ok := wallet.Balances[record.Asset]
	if !ok {
		return fmt.Errorf("balance not found")
	}
	if balance.Free < record.Amount+record.Fee {
		return fmt.Errorf("insufficient balance for withdraw")
	}

	balance.Free -= record.Amount + record.Fee
	balance.Total = balance.Free + balance.Locked
	balance.UpdateTime = time.Now()

	wa.withdraws[record.RecordID] = record
	record.Status = 1 // processing

	return nil
}

// Ensure WalletAdapter implements Adapter
var _ Adapter[BalanceUpdateInput, BalanceUpdateOutput] = (*WalletAdapter)(nil)