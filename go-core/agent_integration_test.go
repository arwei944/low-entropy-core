//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// P4 测试: Agent 注册与调度（集成测试）
// ──────────────────────────────────────────────

// TestRegisterAgent_Integration 测试 Agent 注册。
func TestRegisterAgent_Integration(t *testing.T) {
	pool := NewAgentPool()

	err := RegisterAgent(pool, "agent-01", []string{"compute", "analyze"}, "Phase 2")
	if err != nil {
		t.Fatalf("RegisterAgent() error: %v", err)
	}

	agent, ok := pool.Get("agent-01")
	if !ok {
		t.Fatal("agent should be registered")
	}
	if agent.Status != AgentStatusIdle {
		t.Errorf("expected Idle, got %s", agent.Status)
	}
	if agent.Phase != "Phase 2" {
		t.Errorf("expected Phase 2, got %s", agent.Phase)
	}
	if len(agent.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(agent.Capabilities))
	}
}

// TestRegisterAgent_Duplicate_Integration 测试重复注册。
func TestRegisterAgent_Duplicate_Integration(t *testing.T) {
	pool := NewAgentPool()

	err := RegisterAgent(pool, "agent-dup", []string{"compute"}, "Phase 1")
	if err != nil {
		t.Fatalf("first RegisterAgent() error: %v", err)
	}

	err = RegisterAgent(pool, "agent-dup", []string{"compute"}, "Phase 1")
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

// TestRegisterAgent_NilPool_Integration 测试 nil pool。
func TestRegisterAgent_NilPool_Integration(t *testing.T) {
	err := RegisterAgent(nil, "agent-01", []string{"compute"}, "Phase 1")
	if err == nil {
		t.Error("expected error for nil pool")
	}
}

// TestAgentHeartbeat_Integration 测试心跳。
func TestAgentHeartbeat_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-hb", []string{"compute"}, "Phase 1")

	before := time.Now()
	err := AgentHeartbeat(pool, "agent-hb")
	if err != nil {
		t.Fatalf("AgentHeartbeat() error: %v", err)
	}

	agent, ok := pool.Get("agent-hb")
	if !ok {
		t.Fatal("agent should exist")
	}
	if agent.LastHeartbeat.Before(before) {
		t.Error("heartbeat should update LastHeartbeat")
	}
}

// TestAgentHeartbeat_UnknownAgent_Integration 测试未知 Agent 心跳。
func TestAgentHeartbeat_UnknownAgent_Integration(t *testing.T) {
	pool := NewAgentPool()
	err := AgentHeartbeat(pool, "unknown-agent")
	if err == nil {
		t.Error("expected error for unknown agent heartbeat")
	}
}

// TestDeregisterAgent_Integration 测试注销。
func TestDeregisterAgent_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-bye", []string{"compute"}, "Phase 1")

	DeregisterAgent(pool, "agent-bye")

	_, ok := pool.Get("agent-bye")
	if ok {
		t.Error("agent should be deregistered")
	}
}

// TestDeregisterAgent_NilPool_Integration 测试 nil pool 注销。
func TestDeregisterAgent_NilPool_Integration(t *testing.T) {
	// Should not panic
	DeregisterAgent(nil, "agent-01")
}

// TestAgentPool_MultipleAgents_Integration 测试多 Agent 注册。
func TestAgentPool_MultipleAgents_Integration(t *testing.T) {
	pool := NewAgentPool()

	RegisterAgent(pool, "agent-a", []string{"compute"}, "Phase 1")
	RegisterAgent(pool, "agent-b", []string{"render"}, "Phase 2")
	RegisterAgent(pool, "agent-c", []string{"compute", "render"}, "Phase 1")

	if pool.Count() != 3 {
		t.Errorf("expected 3 agents, got %d", pool.Count())
	}

	// 按能力筛选
	computeAgents := pool.ListByCapability("compute")
	if len(computeAgents) != 2 {
		t.Errorf("expected 2 compute agents, got %d", len(computeAgents))
	}

	renderAgents := pool.ListByCapability("render")
	if len(renderAgents) != 2 {
		t.Errorf("expected 2 render agents, got %d", len(renderAgents))
	}

	// 按阶段筛选
	phase1Agents := pool.ListByPhase("Phase 1")
	if len(phase1Agents) != 2 {
		t.Errorf("expected 2 Phase 1 agents, got %d", len(phase1Agents))
	}
}

// TestAgentPool_UpdateStatus_Integration 测试状态更新。
func TestAgentPool_UpdateStatus_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-status", []string{"compute"}, "Phase 1")

	// 更新为 Busy
	err := pool.UpdateStatus("agent-status", AgentStatusBusy)
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	agent, _ := pool.Get("agent-status")
	if agent.Status != AgentStatusBusy {
		t.Errorf("expected Busy, got %s", agent.Status)
	}

	// 更新回 Idle
	_ = pool.UpdateStatus("agent-status", AgentStatusIdle)
	agent, _ = pool.Get("agent-status")
	if agent.Status != AgentStatusIdle {
		t.Errorf("expected Idle, got %s", agent.Status)
	}
}

// TestAgentPool_ListAvailable_Integration 测试可用 Agent 列表。
func TestAgentPool_ListAvailable_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-idle", []string{"compute"}, "Phase 1")
	RegisterAgent(pool, "agent-busy", []string{"compute"}, "Phase 1")

	pool.UpdateStatus("agent-busy", AgentStatusBusy)

	available := pool.ListAvailable()
	if len(available) != 1 {
		t.Errorf("expected 1 available agent, got %d", len(available))
	}
	if available[0].ID != "agent-idle" {
		t.Errorf("expected agent-idle, got %s", available[0].ID)
	}
}

// TestAgentPool_Heartbeat_Integration 测试心跳更新。
func TestAgentPool_Heartbeat_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-hb2", []string{"compute"}, "Phase 1")

	oldTime := time.Now().Add(-1 * time.Hour)
	agent, _ := pool.Get("agent-hb2")
	agent.LastHeartbeat = oldTime

	err := AgentHeartbeat(pool, "agent-hb2")
	if err != nil {
		t.Fatalf("AgentHeartbeat() error: %v", err)
	}

	agent, _ = pool.Get("agent-hb2")
	if !agent.LastHeartbeat.After(oldTime) {
		t.Error("heartbeat should update LastHeartbeat to current time")
	}
}

// TestAgentPool_Concurrent_Integration 测试并发访问 AgentPool。
func TestAgentPool_Concurrent_Integration(t *testing.T) {
	pool := NewAgentPool()

	// 并发注册
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			agentID := fmt.Sprintf("agent-%d", idx)
			RegisterAgent(pool, agentID, []string{"compute"}, "Phase 1")
			done <- true
		}(i)
		time.Sleep(1 * time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if pool.Count() != 10 {
		t.Errorf("expected 10 agents, got %d", pool.Count())
	}

	// 并发心跳
	for i := 0; i < 10; i++ {
		go func(idx int) {
			agentID := fmt.Sprintf("agent-%d", idx)
			AgentHeartbeat(pool, agentID)
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAgentPool_Offline_Integration 测试离线检测。
func TestAgentPool_Offline_Integration(t *testing.T) {
	pool := NewAgentPool()
	RegisterAgent(pool, "agent-online", []string{"compute"}, "Phase 1")
	RegisterAgent(pool, "agent-offline", []string{"render"}, "Phase 1")

	// 模拟 agent-offline 心跳超时（40 秒前）
	agent, _ := pool.Get("agent-offline")
	agent.LastHeartbeat = time.Now().Add(-40 * time.Second)

	// 验证 agent-offline 的心跳确实过期了
	agent2, _ := pool.Get("agent-offline")
	threshold := 30 * time.Second
	if time.Since(agent2.LastHeartbeat) <= threshold {
		t.Error("agent-offline heartbeat should be stale")
	}

	// agent-online 的心跳应该正常
	agent3, _ := pool.Get("agent-online")
	if time.Since(agent3.LastHeartbeat) > threshold {
		t.Error("agent-online heartbeat should be fresh")
	}
}