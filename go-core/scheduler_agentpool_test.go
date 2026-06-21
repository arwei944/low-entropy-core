//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAgentPool_Add(t *testing.T) {
	pool := NewAgentPool()
	agent := &AgentInfo{ID: "agent-1", Capabilities: []string{"read"}, Phase: "coding"}

	err := pool.Add(agent)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if pool.Count() != 1 {
		t.Errorf("expected count=1, got %d", pool.Count())
	}
}

func TestAgentPool_AddDuplicate(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})
	err := pool.Add(&AgentInfo{ID: "agent-1", Phase: "testing"})
	if err == nil {
		t.Fatal("expected error for duplicate agent")
	}
}

func TestAgentPool_Remove(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})
	pool.Remove("agent-1")
	if pool.Count() != 0 {
		t.Errorf("expected count=0, got %d", pool.Count())
	}
}

func TestAgentPool_UpdateStatus(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})

	err := pool.UpdateStatus("agent-1", AgentStatusBusy)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	agent, _ := pool.Get("agent-1")
	if agent.Status != AgentStatusBusy {
		t.Errorf("expected status=busy, got %s", agent.Status)
	}
}

func TestAgentPool_Heartbeat(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})

	pool.UpdateStatus("agent-1", AgentStatusOffline)
	pool.Heartbeat("agent-1")

	agent, _ := pool.Get("agent-1")
	if agent.Status != AgentStatusIdle {
		t.Errorf("expected status=idle after heartbeat, got %s", agent.Status)
	}
}

func TestAgentPool_ListAvailable(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "agent-2", Phase: "coding", Status: AgentStatusBusy})
	pool.Add(&AgentInfo{ID: "agent-3", Phase: "testing", Status: AgentStatusIdle})

	available := pool.ListAvailable()
	if len(available) != 2 {
		t.Errorf("expected 2 available agents, got %d", len(available))
	}
}

func TestAgentPool_ListByCapability(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "a-1", Capabilities: []string{"read", "write"}, Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-2", Capabilities: []string{"read"}, Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-3", Capabilities: []string{"deploy"}, Phase: "testing", Status: AgentStatusIdle})

	writeAgents := pool.ListByCapability("write")
	if len(writeAgents) != 1 {
		t.Errorf("expected 1 agent with write capability, got %d", len(writeAgents))
	}

	readAgents := pool.ListByCapability("read")
	if len(readAgents) != 2 {
		t.Errorf("expected 2 agents with read capability, got %d", len(readAgents))
	}
}

func TestAgentPool_ListByPhase(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "a-1", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-2", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-3", Phase: "testing", Status: AgentStatusIdle})

	codingAgents := pool.ListByPhase("coding")
	if len(codingAgents) != 2 {
		t.Errorf("expected 2 coding agents, got %d", len(codingAgents))
	}
}

func TestAgentPool_AutoMarkOffline(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Status: AgentStatusIdle})

	agent, _ := pool.Get("agent-1")
	agent.LastHeartbeat = time.Now().Add(-AgentHeartbeatTimeout - time.Second)

	available := pool.ListAvailable()
	if len(available) != 0 {
		t.Errorf("expected 0 available agents (timed out), got %d", len(available))
	}

	agent, _ = pool.Get("agent-1")
	if agent.Status != AgentStatusOffline {
		t.Errorf("expected status=offline, got %s", agent.Status)
	}
}

func TestAgentPool_Concurrency(t *testing.T) {
	pool := NewAgentPool()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pool.Add(&AgentInfo{ID: fmt.Sprintf("agent-%d", id), Phase: "coding"})
		}(i)
	}
	wg.Wait()

	if pool.Count() != 50 {
		t.Errorf("expected 50 agents, got %d", pool.Count())
	}
}
