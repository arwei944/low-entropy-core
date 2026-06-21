//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"time"
)

type MatchInput struct {
	Task *QueuedTask
	Pool *AgentPool
}

type MatchOutput struct {
	Matched    *AgentInfo
	Candidates []*AgentInfo
	Reason     string
}

func MatchEngine(input MatchInput) MatchOutput {
	if input.Pool == nil || input.Task == nil {
		return MatchOutput{Reason: "invalid input: pool or task is nil"}
	}
	phaseCandidates := input.Pool.ListByPhase(input.Task.Phase)
	if len(phaseCandidates) == 0 {
		return MatchOutput{Reason: "no agents available for phase: " + input.Task.Phase}
	}
	capabilityCandidates := make([]*AgentInfo, 0)
	for _, agent := range phaseCandidates {
		if hasAllCapabilities(agent.Capabilities, input.Task.RequiredCapabilities) {
			capabilityCandidates = append(capabilityCandidates, agent)
		}
	}
	if len(capabilityCandidates) == 0 {
		return MatchOutput{Candidates: phaseCandidates, Reason: "no agents with required capabilities"}
	}
	best := findLongestIdle(capabilityCandidates)
	return MatchOutput{
		Matched: best, Candidates: capabilityCandidates,
		Reason: "matched agent " + best.ID + " (idle since " + best.LastHeartbeat.Format(time.RFC3339) + ")",
	}
}

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

func MatchEngineAsStep() Step[MatchInput, MatchOutput] {
	return AtomAsStep(Atom[MatchInput, MatchOutput](MatchEngine))
}
