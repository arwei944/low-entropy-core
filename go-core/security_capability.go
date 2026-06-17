package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// CapabilityToken — agent capability certificate
// ──────────────────────────────────────────────

// CapabilityToken is a signed certificate that grants an agent the right
// to perform specific operations. It carries the agent's identity, its
// capabilities, issuance time, and expiration time, all protected by
// an HMAC-SHA256 signature.
type CapabilityToken struct {
	// AgentID identifies the agent this token was issued to.
	AgentID string `json:"agent_id"`

	// Capabilities are the operations this agent is authorized to perform.
	Capabilities []string `json:"capabilities"`

	// IssuedAt is when this token was created.
	IssuedAt time.Time `json:"issued_at"`

	// ExpiresAt is when this token becomes invalid.
	ExpiresAt time.Time `json:"expires_at"`

	// Signature is the HMAC-SHA256 of the token's payload.
	Signature string `json:"signature"`
}

// TokenLifetime is the default validity period for capability tokens.
const TokenLifetime = 1 * time.Hour

// NewCapabilityToken creates a new CapabilityToken with the given parameters.
// The token is not yet signed — call Sign() to sign it.
func NewCapabilityToken(agentID string, capabilities []string) *CapabilityToken {
	now := time.Now()
	return &CapabilityToken{
		AgentID:      agentID,
		Capabilities: capabilities,
		IssuedAt:     now,
		ExpiresAt:    now.Add(TokenLifetime),
	}
}

// payload returns the concatenated string that gets signed.
// Format: "agentID|cap1,cap2,...|issuedAt.Unix|expiresAt.Unix"
func (t *CapabilityToken) payload() string {
	caps := strings.Join(t.Capabilities, ",")
	return fmt.Sprintf("%s|%s|%d|%d",
		t.AgentID,
		caps,
		t.IssuedAt.Unix(),
		t.ExpiresAt.Unix(),
	)
}

// Sign computes the HMAC-SHA256 signature of the token's payload
// and sets the Signature field.
func (t *CapabilityToken) Sign(secretKey []byte) {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(t.payload()))
	t.Signature = hex.EncodeToString(mac.Sum(nil))
}

// Verify checks if the token's signature is valid for the given secret key.
func (t *CapabilityToken) Verify(secretKey []byte) bool {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(t.payload()))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(t.Signature), []byte(expected))
}

// IsExpired checks if the token has passed its expiration time.
func (t *CapabilityToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// HasCapability checks if the token includes the given capability.
func (t *CapabilityToken) HasCapability(cap string) bool {
	for _, c := range t.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────
// CapabilityPort — Port for token validation
// ──────────────────────────────────────────────

// CapabilityPort is a Port that validates CapabilityTokens.
// It verifies the token's signature, expiration, and required capabilities.
type CapabilityPort struct {
	secretKey          []byte
	requiredCapability string
}

// NewCapabilityPort creates a new CapabilityPort.
// secretKey is the HMAC key used to verify signatures.
// requiredCapability is the capability the agent must have (empty means any).
func NewCapabilityPort(secretKey []byte, requiredCapability string) *CapabilityPort {
	return &CapabilityPort{
		secretKey:          secretKey,
		requiredCapability: requiredCapability,
	}
}

// Validate implements the Port[CapabilityToken, CapabilityToken] interface.
// It checks:
//   1. Signature is valid
//   2. Token is not expired
//   3. Agent has the required capability (if specified)
func (p *CapabilityPort) Validate(ctx context.Context, input CapabilityToken) (CapabilityToken, error) {
	// 1. Signature verification
	if !input.Verify(p.secretKey) {
		return input, NewStepError("INVALID_SIGNATURE",
			"capability token signature is invalid", false)
	}

	// 2. Expiration check
	if input.IsExpired() {
		return input, NewStepError("TOKEN_EXPIRED",
			"capability token has expired", false)
	}

	// 3. Capability check
	if p.requiredCapability != "" && !input.HasCapability(p.requiredCapability) {
		return input, NewStepError("INSUFFICIENT_CAPABILITY",
			fmt.Sprintf("agent lacks required capability: %s", p.requiredCapability), false)
	}

	return input, nil
}

// CapabilityPortAsStep wraps the CapabilityPort as a Step.
func CapabilityPortAsStep(p *CapabilityPort) Step[CapabilityToken, CapabilityToken] {
	return PortAsStep[CapabilityToken, CapabilityToken](p)
}