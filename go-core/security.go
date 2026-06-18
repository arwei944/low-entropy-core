package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Security — security primitives: capability-based access control + audit logging
// 合并自: security_merkle.go + security_audit.go + security_access.go + security_capability.go
// ──────────────────────────────────────────────

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

// ============================================================================
// SECTION 3: Audit Trail — immutable audit logging
// ============================================================================

type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	AgentID    string    `json:"agent_id"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Result     string    `json:"result"`
	Details    string    `json:"details"`
	TraceID    string    `json:"trace_id,omitempty"`
	PrevHash   string    `json:"prev_hash,omitempty"`
	Hash       string    `json:"hash,omitempty"`
}

type AuditTrailAdapter struct {
	mu      sync.RWMutex
	entries []AuditEntry
}

func NewAuditTrailAdapter() *AuditTrailAdapter {
	return &AuditTrailAdapter{entries: make([]AuditEntry, 0)}
}

func (a *AuditTrailAdapter) Execute(ctx context.Context, input AuditEntry) (AuditEntry, error) {
	if input.ID == "" {
		input.ID = string(NewSpanID())
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, input)
	return input, nil
}

func (a *AuditTrailAdapter) GetEntries() []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AuditEntry, len(a.entries))
	copy(result, a.entries)
	return result
}

func (a *AuditTrailAdapter) QueryEntries(agentID, action, resource, result string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	filtered := make([]AuditEntry, 0)
	for _, entry := range a.entries {
		if agentID != "" && entry.AgentID != agentID { continue }
		if action != "" && entry.Action != action { continue }
		if resource != "" && entry.Resource != resource { continue }
		if result != "" && entry.Result != result { continue }
		filtered = append(filtered, entry)
	}
	resultEntries := make([]AuditEntry, len(filtered))
	copy(resultEntries, filtered)
	return resultEntries
}

func (a *AuditTrailAdapter) QueryByAgent(agentID string) []AuditEntry {
	return a.QueryEntries(agentID, "", "", "")
}

func (a *AuditTrailAdapter) QueryByResult(result string) []AuditEntry {
	return a.QueryEntries("", "", "", result)
}

func (a *AuditTrailAdapter) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

func (a *AuditTrailAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}

func NewAuditEntry(agentID, action, resource, resourceID, result, details string) AuditEntry {
	return AuditEntry{
		ID: string(NewSpanID()), Timestamp: time.Now(),
		AgentID: agentID, Action: action, Resource: resource,
		ResourceID: resourceID, Result: result, Details: details,
	}
}

func AuditSuccess(agentID, action, resource, resourceID, details string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "success", details)
}

func AuditFailure(agentID, action, resource, resourceID, details string, err error) AuditEntry {
	detail := details
	if err != nil {
		detail = details + ": " + err.Error()
	}
	return NewAuditEntry(agentID, action, resource, resourceID, "failure", detail)
}

func AuditDenied(agentID, action, resource, resourceID, reason string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "denied", reason)
}

func AuditTrailAsStep(a *AuditTrailAdapter) Step[AuditEntry, AuditEntry] {
	return AdapterAsStep[AuditEntry, AuditEntry](a)
}

// ============================================================================
// SECTION 4: Merkle Audit Chain — tamper-proof audit chain
// ============================================================================

type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

type MerkleProof struct {
	EntryIndex  int
	EntryHash   string
	RootHash    string
	ProofHashes []string
	Directions  []bool
}

type MerkleAuditChain struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	rootHash string
}

func NewMerkleAuditChain() *MerkleAuditChain {
	return &MerkleAuditChain{entries: make([]AuditEntry, 0), rootHash: ""}
}

func (m *MerkleAuditChain) Append(entry AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) > 0 {
		entry.PrevHash = m.entries[len(m.entries)-1].Hash
	} else {
		entry.PrevHash = ""
	}
	entry.Hash = computeEntryHash(entry)
	m.entries = append(m.entries, entry)
	hashes := make([]string, len(m.entries))
	for i, e := range m.entries {
		hashes[i] = e.Hash
	}
	root := buildMerkleTree(hashes)
	if root != nil {
		m.rootHash = root.Hash
	} else {
		m.rootHash = ""
	}
	return nil
}

func (m *MerkleAuditChain) RootHash() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootHash
}

func (m *MerkleAuditChain) GetEntries() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AuditEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

func (m *MerkleAuditChain) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

func (m *MerkleAuditChain) GenerateProof(index int) (*MerkleProof, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if index < 0 || index >= len(m.entries) {
		return nil, fmt.Errorf("index %d out of range [0, %d)", index, len(m.entries))
	}
	hashes := make([]string, len(m.entries))
	for i, e := range m.entries {
		hashes[i] = e.Hash
	}
	var proofHashes []string
	var directions []bool
	currentLevel := hashes
	currentIndex := index
	for len(currentLevel) > 1 {
		var siblingIdx int
		if currentIndex%2 == 0 {
			siblingIdx = currentIndex + 1
			if siblingIdx < len(currentLevel) {
				proofHashes = append(proofHashes, currentLevel[siblingIdx])
				directions = append(directions, true)
			}
		} else {
			siblingIdx = currentIndex - 1
			proofHashes = append(proofHashes, currentLevel[siblingIdx])
			directions = append(directions, false)
		}
		var nextLevel []string
		for i := 0; i < len(currentLevel); i += 2 {
			if i+1 < len(currentLevel) {
				combinedHash := hashPair(currentLevel[i], currentLevel[i+1])
				nextLevel = append(nextLevel, combinedHash)
			} else {
				nextLevel = append(nextLevel, currentLevel[i])
			}
		}
		currentIndex = currentIndex / 2
		currentLevel = nextLevel
	}
	return &MerkleProof{
		EntryIndex: index,
		EntryHash:   hashes[index],
		RootHash:    currentLevel[0],
		ProofHashes: proofHashes,
		Directions:  directions,
	}, nil
}

func VerifyProof(proof *MerkleProof, rootHash string) bool {
	if proof == nil {
		return false
	}
	hash := proof.EntryHash
	for i, siblingHash := range proof.ProofHashes {
		if proof.Directions[i] {
			hash = hashPair(hash, siblingHash)
		} else {
			hash = hashPair(siblingHash, hash)
		}
	}
	return hash == rootHash
}

func (m *MerkleAuditChain) DetectTampering() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tampered []int
	seen := make(map[int]bool)
	for i, entry := range m.entries {
		expectedHash := computeEntryHash(entry)
		if entry.Hash != expectedHash {
			if !seen[i] {
				tampered = append(tampered, i)
				seen[i] = true
			}
		}
		if i > 0 {
			expectedPrevHash := m.entries[i-1].Hash
			if entry.PrevHash != expectedPrevHash {
				if !seen[i] {
					tampered = append(tampered, i)
					seen[i] = true
				}
			}
		} else {
			if entry.PrevHash != "" {
				if !seen[i] {
					tampered = append(tampered, i)
					seen[i] = true
				}
			}
		}
	}
	return tampered
}

func (m *MerkleAuditChain) DetectBreak() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var breaks []int
	if len(m.entries) == 0 {
		return breaks
	}
	if m.entries[0].PrevHash != "" {
		breaks = append(breaks, 0)
	}
	for i := 1; i < len(m.entries); i++ {
		expectedPrevHash := m.entries[i-1].Hash
		if m.entries[i].PrevHash != expectedPrevHash {
			breaks = append(breaks, i)
		}
	}
	return breaks
}

// Internal helpers
func computeEntryHash(entry AuditEntry) string {
	h := sha256.New()
	h.Write([]byte(entry.ID))
	h.Write([]byte(entry.AgentID))
	h.Write([]byte(entry.Action))
	h.Write([]byte(entry.Resource))
	h.Write([]byte(entry.ResourceID))
	h.Write([]byte(entry.Result))
	h.Write([]byte(entry.Details))
	h.Write([]byte(entry.PrevHash))
	return hex.EncodeToString(h.Sum(nil))
}

func buildMerkleTree(hashes []string) *MerkleNode {
	if len(hashes) == 0 {
		return nil
	}
	nodes := make([]*MerkleNode, len(hashes))
	for i, h := range hashes {
		data, _ := hex.DecodeString(h)
		nodes[i] = &MerkleNode{Hash: h, Data: data}
	}
	for len(nodes) > 1 {
		var nextLevel []*MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				combinedHash := hashPair(nodes[i].Hash, nodes[i+1].Hash)
				nextLevel = append(nextLevel, &MerkleNode{Hash: combinedHash, Left: nodes[i], Right: nodes[i+1]})
			} else {
				nextLevel = append(nextLevel, nodes[i])
			}
		}
		nodes = nextLevel
	}
	return nodes[0]
}

func hashPair(left, right string) string {
	h := sha256.New()
	h.Write([]byte(left))
	h.Write([]byte(right))
	return hex.EncodeToString(h.Sum(nil))
}