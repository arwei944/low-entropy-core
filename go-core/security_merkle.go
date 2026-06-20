//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

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
