//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync"
	"time"
)

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

const AgentHeartbeatTimeout = 30 * time.Second

type AgentInfo struct {
	ID            string      `json:"id"`
	Capabilities  []string    `json:"capabilities"`
	Status        AgentStatus `json:"status"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	Phase         string      `json:"phase"`
}

type AgentPool struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo
}

func NewAgentPool() *AgentPool { return &AgentPool{agents: make(map[string]*AgentInfo)} }

func (p *AgentPool) Add(agent *AgentInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.agents[agent.ID]; exists {
		return NewStepError("AGENT_EXISTS", "agent already registered: "+agent.ID, false)
	}
	if agent.LastHeartbeat.IsZero() {
		agent.LastHeartbeat = time.Now()
	}
	if agent.Status == "" {
		agent.Status = AgentStatusIdle
	}
	p.agents[agent.ID] = agent
	return nil
}

func (p *AgentPool) Remove(agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.agents, agentID)
}

func (p *AgentPool) UpdateStatus(agentID string, status AgentStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	agent, ok := p.agents[agentID]
	if !ok {
		return NewStepError("AGENT_NOT_FOUND", "agent not found: "+agentID, false)
	}
	agent.Status = status
	agent.LastHeartbeat = time.Now()
	return nil
}

func (p *AgentPool) Heartbeat(agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	agent, ok := p.agents[agentID]
	if !ok {
		return NewStepError("AGENT_NOT_FOUND", "agent not found: "+agentID, false)
	}
	agent.LastHeartbeat = time.Now()
	if agent.Status == AgentStatusOffline {
		agent.Status = AgentStatusIdle
	}
	return nil
}

func (p *AgentPool) Get(agentID string) (*AgentInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	agent, ok := p.agents[agentID]
	return agent, ok
}

func (p *AgentPool) ListAvailable() []*AgentInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	available := make([]*AgentInfo, 0)
	for _, agent := range p.agents {
		if now.Sub(agent.LastHeartbeat) > AgentHeartbeatTimeout {
			agent.Status = AgentStatusOffline
		}
		if agent.Status == AgentStatusIdle {
			available = append(available, agent)
		}
	}
	return available
}

func (p *AgentPool) ListByCapability(capability string) []*AgentInfo {
	available := p.ListAvailable()
	result := make([]*AgentInfo, 0)
	for _, agent := range available {
		for _, cap := range agent.Capabilities {
			if cap == capability {
				result = append(result, agent)
				break
			}
		}
	}
	return result
}

func (p *AgentPool) ListByPhase(phase string) []*AgentInfo {
	available := p.ListAvailable()
	result := make([]*AgentInfo, 0)
	for _, agent := range available {
		if agent.Phase == phase {
			result = append(result, agent)
		}
	}
	return result
}

func (p *AgentPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

type AgentPoolAdapter struct{ pool *AgentPool }

func NewAgentPoolAdapter(pool *AgentPool) *AgentPoolAdapter { return &AgentPoolAdapter{pool: pool} }

type AgentPoolOp struct {
	Op      string      `json:"op"`
	AgentID string      `json:"agent_id,omitempty"`
	Agent   *AgentInfo  `json:"agent,omitempty"`
	Status  AgentStatus `json:"status,omitempty"`
	Error   error       `json:"-"`
}

func (a *AgentPoolAdapter) Execute(ctx context.Context, input AgentPoolOp) (AgentPoolOp, error) {
	switch input.Op {
	case "add":
		input.Error = a.pool.Add(input.Agent)
	case "remove":
		a.pool.Remove(input.AgentID)
	case "heartbeat":
		input.Error = a.pool.Heartbeat(input.AgentID)
	case "update_status":
		input.Error = a.pool.UpdateStatus(input.AgentID, input.Status)
	}
	return input, nil
}
