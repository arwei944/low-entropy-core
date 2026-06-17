package core

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────
// AccessControlPort — capability-based access control
// ──────────────────────────────────────────────

// AccessRequest is the input to the access control port.
type AccessRequest struct {
	// AgentID is the agent requesting access.
	AgentID string `json:"agent_id"`

	// Action is the operation being requested (read, write, deploy, etc.).
	Action string `json:"action"`

	// Resource is the resource being accessed (pipeline, task, snapshot, etc.).
	Resource string `json:"resource"`

	// ResourceID is the specific resource identifier.
	ResourceID string `json:"resource_id"`

	// Token is the capability token for authorization.
	Token *CapabilityToken `json:"token,omitempty"`
}

// AccessDecision is the result of an access control check.
type AccessDecision struct {
	// Allowed indicates whether access is granted.
	Allowed bool `json:"allowed"`

	// Reason explains why access was granted or denied.
	Reason string `json:"reason"`

	// AgentID is the agent that was evaluated.
	AgentID string `json:"agent_id"`

	// Action is the requested action.
	Action string `json:"action"`
}

// ──────────────────────────────────────────────
// AccessControlPort — Port for access validation
// ──────────────────────────────────────────────

// AccessControlPort is a Port that validates access requests.
// It checks the agent's capability token and ensures the requested
// action is within the agent's authorized capabilities.
type AccessControlPort struct {
	secretKey []byte
}

// NewAccessControlPort creates a new access control port.
func NewAccessControlPort(secretKey []byte) *AccessControlPort {
	return &AccessControlPort{secretKey: secretKey}
}

// Validate implements Port[AccessRequest, AccessDecision].
func (p *AccessControlPort) Validate(ctx context.Context, input AccessRequest) (AccessDecision, error) {
	// 1. Token must be present
	if input.Token == nil {
		decision := AccessDecision{
			Allowed:  false,
			Reason:   "no capability token provided",
			AgentID:  input.AgentID,
			Action:   input.Action,
		}
		return decision, NewStepError("ACCESS_DENIED", "no capability token provided", false)
	}

	// 2. Verify token signature
	if !input.Token.Verify(p.secretKey) {
		decision := AccessDecision{
			Allowed:  false,
			Reason:   "invalid capability token signature",
			AgentID:  input.AgentID,
			Action:   input.Action,
		}
		return decision, NewStepError("ACCESS_DENIED", "invalid token signature", false)
	}

	// 3. Check token expiration
	if input.Token.IsExpired() {
		decision := AccessDecision{
			Allowed:  false,
			Reason:   "capability token has expired",
			AgentID:  input.AgentID,
			Action:   input.Action,
		}
		return decision, NewStepError("ACCESS_DENIED", "token expired", false)
	}

	// 4. Check agent ID matches token
	if input.Token.AgentID != input.AgentID {
		decision := AccessDecision{
			Allowed:  false,
			Reason:   fmt.Sprintf("agent ID mismatch: %s != %s", input.AgentID, input.Token.AgentID),
			AgentID:  input.AgentID,
			Action:   input.Action,
		}
		return decision, NewStepError("ACCESS_DENIED", "agent ID mismatch", false)
	}

	// 5. Check the agent has the required capability
	requiredCap := input.Action
	if input.Resource != "" {
		requiredCap = input.Resource + ":" + input.Action
	}

	if !input.Token.HasCapability(requiredCap) {
		decision := AccessDecision{
			Allowed:  false,
			Reason:   fmt.Sprintf("agent lacks capability: %s", requiredCap),
			AgentID:  input.AgentID,
			Action:   input.Action,
		}
		return decision, NewStepError("ACCESS_DENIED", "insufficient capabilities", false)
	}

	decision := AccessDecision{
		Allowed:  true,
		Reason:   "access granted",
		AgentID:  input.AgentID,
		Action:   input.Action,
	}
	return decision, nil
}

// AccessControlPortAsStep wraps the access control port as a Step.
func AccessControlPortAsStep(p *AccessControlPort) Step[AccessRequest, AccessDecision] {
	return PortAsStep[AccessRequest, AccessDecision](p)
}

// ──────────────────────────────────────────────
// AccessPolicy — declarative access rules
// ──────────────────────────────────────────────

// AccessPolicy defines a set of access rules for a resource.
type AccessPolicy struct {
	// Resource is the resource this policy applies to.
	Resource string `json:"resource"`

	// Rules maps actions to the capabilities required to perform them.
	Rules map[string]string `json:"rules"`
}

// DefaultAccessPolicy returns a sensible default policy.
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

// CheckAccess validates an access request against a policy using a capability token.
// Returns true if the agent has the required capability for the action.
func (p *AccessPolicy) CheckAccess(token *CapabilityToken, action string) bool {
	required, ok := p.Rules[action]
	if !ok {
		return false
	}

	// Also check wildcard capabilities
	if token.HasCapability(p.Resource + ":*") || token.HasCapability("*") {
		return true
	}

	return token.HasCapability(required)
}

// DescribeAccessPolicy returns a human-readable representation of the policy.
func (p *AccessPolicy) DescribeAccessPolicy() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("AccessPolicy for %s:\n", p.Resource))
	for action, cap := range p.Rules {
		sb.WriteString(fmt.Sprintf("  %s → %s\n", action, cap))
	}
	return sb.String()
}