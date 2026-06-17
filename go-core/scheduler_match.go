package core

import (
	"time"
)

// ──────────────────────────────────────────────
// MatchEngine — pure function agent matching
// ──────────────────────────────────────────────

// MatchInput is the input to the matching algorithm.
type MatchInput struct {
	// Task is the task to be assigned.
	Task *QueuedTask

	// Pool is the agent pool to search in.
	Pool *AgentPool
}

// MatchOutput is the result of the matching algorithm.
type MatchOutput struct {
	// Matched is the best matching agent, or nil if no match.
	Matched *AgentInfo

	// Candidates are all agents that could potentially handle the task.
	Candidates []*AgentInfo

	// Reason explains why a particular agent was chosen or why no match was found.
	Reason string
}

// MatchEngine is a pure function (Atom) that matches tasks to agents.
// It implements the Atom[MatchInput, MatchOutput] type.
//
// Matching algorithm:
//   1. Filter agents by Phase (must match)
//   2. Filter agents by Capabilities (all required capabilities must be present)
//   3. Among remaining candidates, select the agent that has been idle the longest
//
// This is a pure function — no side effects, no I/O.
func MatchEngine(input MatchInput) MatchOutput {
	if input.Pool == nil || input.Task == nil {
		return MatchOutput{Reason: "invalid input: pool or task is nil"}
	}

	// Step 1: Get available agents for the task's phase
	phaseCandidates := input.Pool.ListByPhase(input.Task.Phase)

	if len(phaseCandidates) == 0 {
		return MatchOutput{
			Reason: "no agents available for phase: " + input.Task.Phase,
		}
	}

	// Step 2: Filter by required capabilities
	capabilityCandidates := make([]*AgentInfo, 0)
	for _, agent := range phaseCandidates {
		if hasAllCapabilities(agent.Capabilities, input.Task.RequiredCapabilities) {
			capabilityCandidates = append(capabilityCandidates, agent)
		}
	}

	if len(capabilityCandidates) == 0 {
		return MatchOutput{
			Candidates: phaseCandidates,
			Reason:     "no agents with required capabilities: " + input.Task.Phase,
		}
	}

	// Step 3: Select the agent that has been idle the longest
	best := findLongestIdle(capabilityCandidates)

	return MatchOutput{
		Matched:    best,
		Candidates: capabilityCandidates,
		Reason:     "matched agent " + best.ID + " (idle since " + best.LastHeartbeat.Format(time.RFC3339) + ")",
	}
}

// hasAllCapabilities checks if agentCaps contains all requiredCaps.
func hasAllCapabilities(agentCaps, requiredCaps []string) bool {
	if len(requiredCaps) == 0 {
		return true
	}
	capSet := make(map[string]bool, len(agentCaps))
	for _, c := range agentCaps {
		capSet[c] = true
	}
	for _, required := range requiredCaps {
		if !capSet[required] {
			return false
		}
	}
	return true
}

// findLongestIdle returns the agent that has been idle the longest.
func findLongestIdle(agents []*AgentInfo) *AgentInfo {
	if len(agents) == 0 {
		return nil
	}
	best := agents[0]
	for _, a := range agents[1:] {
		if a.LastHeartbeat.Before(best.LastHeartbeat) {
			best = a
		}
	}
	return best
}

// MatchEngineAsStep wraps MatchEngine as a Step.
func MatchEngineAsStep() Step[MatchInput, MatchOutput] {
	return AtomAsStep(Atom[MatchInput, MatchOutput](MatchEngine))
}