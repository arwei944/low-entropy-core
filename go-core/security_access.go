//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// SECTION 2: Access Control — capability-based access control
// ============================================================================

type AccessRequest struct {
	AgentID    string           `json:"agent_id"`
	Action     string           `json:"action"`
	Resource   string           `json:"resource"`
	ResourceID string           `json:"resource_id"`
	Token      *CapabilityToken `json:"token,omitempty"`
}

type AccessDecision struct {
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
	AgentID  string `json:"agent_id"`
	Action   string `json:"action"`
}

type AccessControlPort struct {
	secretKey []byte
}

func NewAccessControlPort(secretKey []byte) *AccessControlPort {
	return &AccessControlPort{secretKey: secretKey}
}

func (p *AccessControlPort) Validate(ctx context.Context, input AccessRequest) (AccessDecision, error) {
	if input.Token == nil {
		return AccessDecision{Allowed: false, Reason: "no capability token provided", AgentID: input.AgentID, Action: input.Action},
			NewStepError("ACCESS_DENIED", "no capability token provided", false)
	}
	if !input.Token.Verify(p.secretKey) {
		return AccessDecision{Allowed: false, Reason: "invalid capability token signature", AgentID: input.AgentID, Action: input.Action},
			NewStepError("ACCESS_DENIED", "invalid token signature", false)
	}
	if input.Token.IsExpired() {
		return AccessDecision{Allowed: false, Reason: "capability token has expired", AgentID: input.AgentID, Action: input.Action},
			NewStepError("ACCESS_DENIED", "token expired", false)
	}
	if input.Token.AgentID != input.AgentID {
		return AccessDecision{Allowed: false, Reason: fmt.Sprintf("agent ID mismatch: %s != %s", input.Token.AgentID, input.AgentID), AgentID: input.AgentID, Action: input.Action},
			NewStepError("ACCESS_DENIED", "agent ID mismatch", false)
	}
	requiredCap := input.Action
	if input.Resource != "" {
		requiredCap = input.Resource + ":" + input.Action
	}
	if !input.Token.HasCapability(requiredCap) {
		return AccessDecision{Allowed: false, Reason: fmt.Sprintf("agent lacks capability: %s", requiredCap), AgentID: input.AgentID, Action: input.Action},
			NewStepError("ACCESS_DENIED", "insufficient capabilities", false)
	}
	return AccessDecision{Allowed: true, Reason: "access granted", AgentID: input.AgentID, Action: input.Action}, nil
}

func AccessControlPortAsStep(p *AccessControlPort) Step[AccessRequest, AccessDecision] {
	return PortAsStep[AccessRequest, AccessDecision](p)
}

type AccessPolicy struct {
	Resource string            `json:"resource"`
	Rules    map[string]string `json:"rules"`
}

func DefaultAccessPolicy() *AccessPolicy {
	return &AccessPolicy{
		Resource: "pipeline",
		Rules: map[string]string{
			"read":   "pipeline:read",
			"write":  "pipeline:write",
			"deploy": "pipeline:deploy",
			"delete": "pipeline:admin",
		},
	}
}

func (p *AccessPolicy) CheckAccess(token *CapabilityToken, action string) bool {
	required, ok := p.Rules[action]
	if !ok {
		return false
	}
	if token.HasCapability(p.Resource+":*") || token.HasCapability("*") {
		return true
	}
	return token.HasCapability(required)
}

func (p *AccessPolicy) DescribeAccessPolicy() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("AccessPolicy for %s:\n", p.Resource))
	for action, cap := range p.Rules {
		sb.WriteString(fmt.Sprintf("  %s → %s\n", action, cap))
	}
	return sb.String()
}
