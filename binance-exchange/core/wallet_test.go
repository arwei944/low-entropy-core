package core

import (
	"context"
	"testing"
	"time"
)

func TestWalletAdapter_LockBalance(t *testing.T) {
	wa := NewWalletAdapter()

	// 充值
	wa.ProcessDeposit(&DepositRecord{
		RecordID:  "dep-1",
		UserID:    "user-1",
		Asset:     "BTC",
		Amount:    10,
		Status:    1,
		CreatedAt: time.Now(),
	})

	// 检查初始余额
	bal, ok := wa.GetBalance("user-1", "BTC")
	if !ok {
		t.Fatal("balance not found")
	}
	if bal.Free != 10 {
		t.Fatalf("expected free=10, got %.8f", bal.Free)
	}
	if bal.Locked != 0 {
		t.Fatalf("expected locked=0, got %.8f", bal.Locked)
	}

	// 冻结 0.5
	err := wa.LockBalance("user-1", "BTC", 0.5)
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	// 检查冻结后余额
	bal, _ = wa.GetBalance("user-1", "BTC")
	if bal.Free != 9.5 {
		t.Errorf("expected free=9.5, got %.8f", bal.Free)
	}
	if bal.Locked != 0.5 {
		t.Errorf("expected locked=0.5, got %.8f", bal.Locked)
	}

	// 解冻 0.5
	err = wa.UnlockBalance("user-1", "BTC", 0.5)
	if err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	// 检查解冻后余额
	bal, _ = wa.GetBalance("user-1", "BTC")
	if bal.Free != 10 {
		t.Errorf("expected free=10, got %.8f", bal.Free)
	}
	if bal.Locked != 0 {
		t.Errorf("expected locked=0, got %.8f", bal.Locked)
	}
}

func TestWalletAdapter_Execute(t *testing.T) {
	wa := NewWalletAdapter()

	// 充值
	wa.ProcessDeposit(&DepositRecord{
		RecordID:  "dep-1",
		UserID:    "user-1",
		Asset:     "USDT",
		Amount:    100000,
		Status:    1,
		CreatedAt: time.Now(),
	})

	// 减少余额
	result, err := wa.Execute(context.Background(), BalanceUpdateInput{
		UserID: "user-1",
		Asset:  "USDT",
		Delta:  -50000,
		Reason: "test",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("execute not successful: %s", result.Error)
	}

	bal, _ := wa.GetBalance("user-1", "USDT")
	if bal.Free != 50000 {
		t.Errorf("expected free=50000, got %.2f", bal.Free)
	}
}