package core

import (
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// AgentPool — agent registry and lifecycle
// ──────────────────────────────────────────────

// AgentStatus represents the current state of an agent.
type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

// AgentHeartbeatTimeout is the maximum time before an agent is considered offline.
const AgentHeartbeatTimeout = 30 * time.Second

// AgentInfo describes an agent registered in the pool.
type AgentInfo struct {
	// ID uniquely identifies the agent.
	ID string `json:"id"`

	// Capabilities are the operations this agent can perform.
	Capabilities []string `json:"capabilities"`

	// Status is the current state of the agent.
	Status AgentStatus `json:"status"`

	// LastHeartbeat is the last time the agent reported its status.
	LastHeartbeat time.Time `json:"last_heartbeat"`

	// Phase is the development phase this agent specializes in.
	Phase string `json:"phase"`
}

// AgentPool manages a collection of agents.
// Thread-safe for concurrent use.
type AgentPool struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo
}

// NewAgentPool creates a new agent pool.
func NewAgentPool() *AgentPool {
	return &AgentPool{
		agents: make(map[string]*AgentInfo),
	}
}

// Add registers a new agent in the pool.
// Returns an error if the agent ID already exists.
func (p *AgentPool) Add(agent *AgentInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.agents[agent.ID]; exists {
		return NewStepError("AGENT_EXISTS", "agent already registered: "+agent.ID, false)
	}
	// Only set heartbeat if not already set (e.g., in tests)
	if agent.LastHeartbeat.IsZero() {
		agent.LastHeartbeat = time.Now()
	}
	if agent.Status == "" {
		agent.Status = AgentStatusIdle
	}
	p.agents[agent.ID] = agent
	return nil
}

// Remove deregisters an agent from the pool.
func (p *AgentPool) Remove(agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.agents, agentID)
}

// UpdateStatus updates an agent's status and heartbeat.
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

// Heartbeat updates the agent's last heartbeat time.
// Agents that fail to heartbeat within AgentHeartbeatTimeout are marked offline.
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

// Get retrieves an agent by ID.
func (p *AgentPool) Get(agentID string) (*AgentInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	agent, ok := p.agents[agentID]
	return agent, ok
}

// ListAvailable returns all agents with status "idle" whose heartbeat is recent.
// Agents with expired heartbeats are automatically marked offline.
func (p *AgentPool) ListAvailable() []*AgentInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	available := make([]*AgentInfo, 0)
	for _, agent := range p.agents {
		// Auto-mark offline if heartbeat expired
		if now.Sub(agent.LastHeartbeat) > AgentHeartbeatTimeout {
			agent.Status = AgentStatusOffline
		}
		if agent.Status == AgentStatusIdle {
			available = append(available, agent)
		}
	}
	return available
}

// ListByCapability returns all idle agents that have the specified capability.
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

// ListByPhase returns all idle agents that specialize in the given phase.
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

// Count returns the total number of registered agents.
func (p *AgentPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

// ──────────────────────────────────────────────
// AgentPoolAdapter — wraps AgentPool as Adapter
// ──────────────────────────────────────────────

// AgentPoolAdapter provides adapter access to the agent pool.
type AgentPoolAdapter struct {
	pool *AgentPool
}

// NewAgentPoolAdapter creates an agent pool adapter.
func NewAgentPoolAdapter(pool *AgentPool) *AgentPoolAdapter {
	return &AgentPoolAdapter{pool: pool}
}

// Execute implements Adapter[AgentPoolOp, AgentPoolOp].
func (a *AgentPoolAdapter) Execute(ctx context.Context, input AgentPoolOp) (AgentPoolOp, error) {
	switch input.Op {
	case "add":
		err := a.pool.Add(input.Agent)
		input.Error = err
	case "remove":
		a.pool.Remove(input.AgentID)
	case "heartbeat":
		err := a.pool.Heartbeat(input.AgentID)
		input.Error = err
	case "update_status":
		err := a.pool.UpdateStatus(input.AgentID, input.Status)
		input.Error = err
	}
	return input, nil
}

// AgentPoolOp represents an operation on the agent pool.
type AgentPoolOp struct {
	Op      string      `json:"op"`
	AgentID string      `json:"agent_id,omitempty"`
	Agent   *AgentInfo  `json:"agent,omitempty"`
	Status  AgentStatus `json:"status,omitempty"`
	Error   error       `json:"-"`
}