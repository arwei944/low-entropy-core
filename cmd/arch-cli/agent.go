package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// AgentPool — Agent 生命周期管理 (Phase 2 P6)
// ============================================================================

// AgentPool 管理所有已注册的 Agent，提供线程安全的注册、状态更新、
// 提交记录和 SSE 事件广播。
type AgentPool struct {
	mu          sync.RWMutex
	agents      map[string]*Agent
	submissions map[string][]SubmissionResult // agentID -> submissions
	eventCh     chan AgentEvent
	subscribers map[chan AgentEvent]struct{}
	subMu       sync.Mutex
}

var agentPool = &AgentPool{
	agents:      make(map[string]*Agent),
	submissions: make(map[string][]SubmissionResult),
	eventCh:     make(chan AgentEvent, 100),
	subscribers: make(map[chan AgentEvent]struct{}),
}

// init 启动事件广播协程
func (p *AgentPool) init() {
	go p.broadcast()
}

// broadcast 将事件分发给所有订阅者
func (p *AgentPool) broadcast() {
	for evt := range p.eventCh {
		p.subMu.Lock()
		for ch := range p.subscribers {
			select {
			case ch <- evt:
			default:
				// 订阅者消费过慢，跳过
			}
		}
		p.subMu.Unlock()
	}
}

// Register 注册一个新 Agent
func (p *AgentPool) Register(agent *Agent) {
	p.mu.Lock()
	agent.LastHeartbeat = time.Now()
	p.agents[agent.ID] = agent
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "register",
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data:      agent,
	}
}

// Unregister 注销一个 Agent
func (p *AgentPool) Unregister(agentID string) {
	p.mu.Lock()
	delete(p.agents, agentID)
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "unregister",
		AgentID:   agentID,
		Timestamp: time.Now(),
	}
}

// UpdateStatus 更新 Agent 状态和当前任务
func (p *AgentPool) UpdateStatus(agentID string, status AgentStatus, currentTask string) {
	p.mu.Lock()
	agent, ok := p.agents[agentID]
	if ok {
		agent.Status = status
		agent.CurrentTask = currentTask
		agent.LastHeartbeat = time.Now()
	}
	p.mu.Unlock()
	if ok {
		p.eventCh <- AgentEvent{
			Type:      "status_change",
			AgentID:   agentID,
			Timestamp: time.Now(),
			Data:      agent,
		}
	}
}

// GetAgents 返回所有 Agent 的快照
func (p *AgentPool) GetAgents() []Agent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]Agent, 0, len(p.agents))
	for _, a := range p.agents {
		result = append(result, *a)
	}
	return result
}

// GetAgent 返回指定 Agent 的副本
func (p *AgentPool) GetAgent(agentID string) (*Agent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	a, ok := p.agents[agentID]
	if !ok {
		return nil, false
	}
	copy := *a
	return &copy, true
}

// AddSubmission 记录一次任务提交
func (p *AgentPool) AddSubmission(result SubmissionResult) {
	p.mu.Lock()
	p.submissions[result.AgentID] = append(p.submissions[result.AgentID], result)
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "submission",
		AgentID:   result.AgentID,
		Timestamp: time.Now(),
		Data:      result,
	}
}

// GetSubmissions 返回指定 Agent 的提交历史副本
func (p *AgentPool) GetSubmissions(agentID string) []SubmissionResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	subs := p.submissions[agentID]
	if subs == nil {
		return []SubmissionResult{}
	}
	result := make([]SubmissionResult, len(subs))
	copy(result, subs)
	return result
}

// Subscribe 创建一个事件订阅通道
func (p *AgentPool) Subscribe() chan AgentEvent {
	ch := make(chan AgentEvent, 50)
	p.subMu.Lock()
	p.subscribers[ch] = struct{}{}
	p.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅并关闭通道
func (p *AgentPool) Unsubscribe(ch chan AgentEvent) {
	p.subMu.Lock()
	delete(p.subscribers, ch)
	p.subMu.Unlock()
	close(ch)
}

// ============================================================================
// Agent API 端点 (Phase 2 P6 - Agent Workbench)
// ============================================================================

// handleAgents 返回 AgentPool 中所有 Agent 信息
// GET /api/agents
func handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agents := agentPool.GetAgents()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	})
}

// handleAgentSubmissions 返回指定 Agent 的提交历史
// GET /api/agents/{id}/submissions
func handleAgentSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析路径: /api/agents/{id}/submissions
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/agents/") || !strings.HasSuffix(path, "/submissions") {
		http.Error(w, `{"error":"invalid path"}`, http.StatusNotFound)
		return
	}
	agentID := strings.TrimPrefix(path, "/api/agents/")
	agentID = strings.TrimSuffix(agentID, "/submissions")
	if agentID == "" {
		http.Error(w, `{"error":"missing agent id"}`, http.StatusBadRequest)
		return
	}

	// 验证 agent 是否存在
	if _, ok := agentPool.GetAgent(agentID); !ok {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
		return
	}

	submissions := agentPool.GetSubmissions(agentID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":    agentID,
		"submissions": submissions,
		"total":       len(submissions),
	})
}

// handleAgentEvents SSE 端点，推送 Agent 状态变化事件
// GET /api/agents/events
func handleAgentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := agentPool.Subscribe()
	defer agentPool.Unsubscribe(ch)

	// 发送初始快照：所有当前 Agent
	agents := agentPool.GetAgents()
	initData, _ := json.Marshal(map[string]interface{}{
		"type":   "initial",
		"agents": agents,
	})
	fmt.Fprintf(w, "data: %s\n\n", initData)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
