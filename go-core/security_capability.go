//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

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

// ============================================================================
// SECTION 1: Capability Tokens — signed capability certificates
// ============================================================================

const TokenLifetime = 1 * time.Hour

type CapabilityToken struct {
	AgentID      string    `json:"agent_id"`
	Capabilities []string `json:"capabilities"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Signature    string    `json:"signature"`
}

func NewCapabilityToken(agentID string, capabilities []string) *CapabilityToken {
	now := time.Now()
	return &CapabilityToken{
		AgentID: agentID, Capabilities: capabilities,
		IssuedAt: now, ExpiresAt: now.Add(TokenLifetime),
	}
}

func (t *CapabilityToken) payload() string {
	caps := strings.Join(t.Capabilities, ",")
	return fmt.Sprintf("%s|%s|%d|%d", t.AgentID, caps, t.IssuedAt.Unix(), t.ExpiresAt.Unix())
}

func (t *CapabilityToken) Sign(secretKey []byte) {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(t.payload()))
	t.Signature = hex.EncodeToString(mac.Sum(nil))
}

func (t *CapabilityToken) Verify(secretKey []byte) bool {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(t.payload()))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(t.Signature), []byte(expected))
}

func (t *CapabilityToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

func (t *CapabilityToken) HasCapability(cap string) bool {
	for _, c := range t.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

type CapabilityPort struct {
	secretKey          []byte
	requiredCapability string
}

func NewCapabilityPort(secretKey []byte, requiredCapability string) *CapabilityPort {
	return &CapabilityPort{secretKey: secretKey, requiredCapability: requiredCapability}
}

func (p *CapabilityPort) Validate(ctx context.Context, input CapabilityToken) (CapabilityToken, error) {
	if !input.Verify(p.secretKey) {
		return input, NewStepError("INVALID_SIGNATURE", "capability token signature is invalid", false)
	}
	if input.IsExpired() {
		return input, NewStepError("TOKEN_EXPIRED", "capability token has expired", false)
	}
	if p.requiredCapability != "" && !input.HasCapability(p.requiredCapability) {
		return input, NewStepError("INSUFFICIENT_CAPABILITY", fmt.Sprintf("agent lacks required capability: %s", p.requiredCapability), false)
	}
	return input, nil
}

func CapabilityPortAsStep(p *CapabilityPort) Step[CapabilityToken, CapabilityToken] {
	return PortAsStep[CapabilityToken, CapabilityToken](p)
}
