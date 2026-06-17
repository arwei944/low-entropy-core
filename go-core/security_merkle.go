package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Merkle 审计链 — 防篡改审计链
// ──────────────────────────────────────────────

// MerkleNode represents a node in the Merkle tree.
type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

// MerkleProof proves that a specific entry is in the chain.
type MerkleProof struct {
	EntryIndex  int
	EntryHash   string
	RootHash    string
	ProofHashes []string // sibling hashes for proof verification
	Directions  []bool   // true = right sibling, false = left sibling
}

// MerkleAuditChain wraps audit entries with Merkle tree integrity.
type MerkleAuditChain struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	rootHash string
}

// ──────────────────────────────────────────────
// 构造函数
// ──────────────────────────────────────────────

// NewMerkleAuditChain creates a new Merkle audit chain.
func NewMerkleAuditChain() *MerkleAuditChain {
	return &MerkleAuditChain{
		entries:  make([]AuditEntry, 0),
		rootHash: "",
	}
}

// ──────────────────────────────────────────────
// 方法
// ──────────────────────────────────────────────

// Append adds an audit entry to the chain and rebuilds the Merkle tree.
// It computes the entry hash, sets PrevHash, and updates the root hash.
func (m *MerkleAuditChain) Append(entry AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set PrevHash to the previous entry's Hash (or empty for first entry)
	if len(m.entries) > 0 {
		entry.PrevHash = m.entries[len(m.entries)-1].Hash
	} else {
		entry.PrevHash = ""
	}

	// Compute SHA-256 hash of the entry content
	entry.Hash = computeEntryHash(entry)

	// Append entry to the chain
	m.entries = append(m.entries, entry)

	// Rebuild Merkle tree and update rootHash
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

// RootHash returns the current Merkle root hash.
func (m *MerkleAuditChain) RootHash() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootHash
}

// GetEntries returns all audit entries in the chain.
func (m *MerkleAuditChain) GetEntries() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AuditEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

// Count returns the number of entries in the chain.
func (m *MerkleAuditChain) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// GenerateProof generates a Merkle proof for the entry at the given index.
// The proof contains sibling hashes and directions needed to verify
// that the entry is part of the Merkle tree rooted at the current root hash.
func (m *MerkleAuditChain) GenerateProof(index int) (*MerkleProof, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if index < 0 || index >= len(m.entries) {
		return nil, fmt.Errorf("index %d out of range [0, %d)", index, len(m.entries))
	}

	// Collect all entry hashes
	hashes := make([]string, len(m.entries))
	for i, e := range m.entries {
		hashes[i] = e.Hash
	}

	// Build the proof by walking up the tree levels
	var proofHashes []string
	var directions []bool
	currentLevel := hashes
	currentIndex := index

	for len(currentLevel) > 1 {
		var siblingIdx int
		if currentIndex%2 == 0 {
			// Current is left child, sibling is right
			siblingIdx = currentIndex + 1
			if siblingIdx < len(currentLevel) {
				proofHashes = append(proofHashes, currentLevel[siblingIdx])
				directions = append(directions, true) // right sibling
			}
		} else {
			// Current is right child, sibling is left
			siblingIdx = currentIndex - 1
			proofHashes = append(proofHashes, currentLevel[siblingIdx])
			directions = append(directions, false) // left sibling
		}

		// Build next level
		var nextLevel []string
		for i := 0; i < len(currentLevel); i += 2 {
			if i+1 < len(currentLevel) {
				nextLevel = append(nextLevel, hashPair(currentLevel[i], currentLevel[i+1]))
			} else {
				// Odd node — promote it directly
				nextLevel = append(nextLevel, currentLevel[i])
			}
		}

		currentIndex = currentIndex / 2
		currentLevel = nextLevel
	}

	return &MerkleProof{
		EntryIndex:  index,
		EntryHash:   hashes[index],
		RootHash:    currentLevel[0],
		ProofHashes: proofHashes,
		Directions:  directions,
	}, nil
}

// VerifyProof verifies a Merkle proof against a given root hash.
// It reconstructs the Merkle root from the proof and compares it
// with the expected root hash.
func VerifyProof(proof *MerkleProof, rootHash string) bool {
	if proof == nil {
		return false
	}

	hash := proof.EntryHash
	for i, siblingHash := range proof.ProofHashes {
		if proof.Directions[i] {
			// sibling is on the right: hash = H(hash || sibling)
			hash = hashPair(hash, siblingHash)
		} else {
			// sibling is on the left: hash = H(sibling || hash)
			hash = hashPair(siblingHash, hash)
		}
	}
	return hash == rootHash
}

// DetectTampering checks all entries for tampering.
// It recomputes each entry's hash and checks the PrevHash chain.
// Returns indices of tampered entries.
func (m *MerkleAuditChain) DetectTampering() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tampered []int
	seen := make(map[int]bool)

	for i, entry := range m.entries {
		// Recompute the entry's hash from its stored content
		expectedHash := computeEntryHash(entry)
		if entry.Hash != expectedHash {
			if !seen[i] {
				tampered = append(tampered, i)
				seen[i] = true
			}
		}

		// Check PrevHash matches previous entry's Hash
		if i > 0 {
			expectedPrevHash := m.entries[i-1].Hash
			if entry.PrevHash != expectedPrevHash {
				if !seen[i] {
					tampered = append(tampered, i)
					seen[i] = true
				}
			}
		} else {
			// First entry should have empty PrevHash
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

// DetectBreak checks for chain breaks (missing or deleted entries).
// It verifies the PrevHash chain is continuous.
// Returns indices where breaks occur (the index of the entry whose
// PrevHash does not match the previous entry's Hash).
func (m *MerkleAuditChain) DetectBreak() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var breaks []int

	if len(m.entries) == 0 {
		return breaks
	}

	// First entry: PrevHash should be empty
	if m.entries[0].PrevHash != "" {
		breaks = append(breaks, 0)
	}

	// For each subsequent entry, PrevHash should match previous entry's Hash
	for i := 1; i < len(m.entries); i++ {
		expectedPrevHash := m.entries[i-1].Hash
		if m.entries[i].PrevHash != expectedPrevHash {
			breaks = append(breaks, i)
		}
	}

	return breaks
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

// computeEntryHash computes SHA-256 hash of an audit entry's content.
// The hash includes: ID + AgentID + Action + Resource + ResourceID +
// Result + Timestamp + Details + PrevHash
func computeEntryHash(entry AuditEntry) string {
	h := sha256.New()

	// Write fields in deterministic order
	h.Write([]byte(entry.ID))
	h.Write([]byte(entry.AgentID))
	h.Write([]byte(entry.Action))
	h.Write([]byte(entry.Resource))
	h.Write([]byte(entry.ResourceID))
	h.Write([]byte(entry.Result))
	h.Write([]byte(entry.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(entry.Details))
	h.Write([]byte(entry.PrevHash))

	return hex.EncodeToString(h.Sum(nil))
}

// buildMerkleTree builds a Merkle tree from a list of hex-encoded
// SHA-256 hashes and returns the root node. Returns nil if the
// input list is empty.
func buildMerkleTree(hashes []string) *MerkleNode {
	if len(hashes) == 0 {
		return nil
	}

	// Create leaf nodes
	nodes := make([]*MerkleNode, len(hashes))
	for i, h := range hashes {
		data, _ := hex.DecodeString(h)
		nodes[i] = &MerkleNode{
			Hash: h,
			Data: data,
		}
	}

	// Build tree bottom-up
	for len(nodes) > 1 {
		var nextLevel []*MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				combinedHash := hashPair(nodes[i].Hash, nodes[i+1].Hash)
				nextLevel = append(nextLevel, &MerkleNode{
					Hash:  combinedHash,
					Left:  nodes[i],
					Right: nodes[i+1],
				})
			} else {
				// Odd node out — promote it directly
				nextLevel = append(nextLevel, nodes[i])
			}
		}
		nodes = nextLevel
	}

	return nodes[0]
}

// hashPair combines two hex-encoded SHA-256 hashes and returns
// their combined SHA-256 hash as a hex-encoded string.
func hashPair(left, right string) string {
	h := sha256.New()
	h.Write([]byte(left))
	h.Write([]byte(right))
	return hex.EncodeToString(h.Sum(nil))
}