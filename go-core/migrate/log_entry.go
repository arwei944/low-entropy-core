package migrate

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// MigrationLogEntry is an atomic-level migration log entry.
// Every AST node transformation, import rewrite, and signature change
// must be recorded as an immutable, ordered entry.
type MigrationLogEntry struct {
	ID             string            `json:"id"`
	SeqNo          int64             `json:"seq_no"`
	Timestamp      time.Time         `json:"timestamp"`
	Phase          string            `json:"phase"`           // "parse"|"pattern"|"transform"|"shim"|"validate"
	StepID         string            `json:"step_id"`
	ActionType     string            `json:"action_type"`     // "ast_node_add"|"import_rewrite"|"signature_change"|...
	FilePath       string            `json:"file_path"`
	LineStart      int               `json:"line_start"`
	LineEnd        int               `json:"line_end"`
	Original       string            `json:"original"`
	Transformed    string            `json:"transformed"`
	Diff           string            `json:"diff"`
	ChecksumBefore string            `json:"checksum_before"`
	ChecksumAfter  string            `json:"checksum_after"`
	ConstraintRef  string            `json:"constraint_ref"`
	Metadata       map[string]string `json:"metadata"`
}

// MigrationLog is an append-only, sealable log of migration entries.
type MigrationLog struct {
	mu        sync.Mutex
	Entries   []MigrationLogEntry `json:"entries"`
	Sealed    bool                `json:"sealed"`
	SessionID string              `json:"session_id"`
	seqCounter int64
}

// NewMigrationLog creates a new MigrationLog for the given session.
func NewMigrationLog(sessionID string) *MigrationLog {
	return &MigrationLog{
		SessionID: sessionID,
		Entries:   make([]MigrationLogEntry, 0),
	}
}

// Append adds an entry to the log. Returns error if the log is sealed.
func (l *MigrationLog) Append(entry MigrationLogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.Sealed {
		return fmt.Errorf("log is sealed, cannot append")
	}

	// Auto-generate ID and SeqNo if not set.
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("log-%d-%d", time.Now().UnixNano(), l.seqCounter)
	}
	l.seqCounter++
	entry.SeqNo = l.seqCounter
	entry.Timestamp = time.Now()

	l.Entries = append(l.Entries, entry)
	return nil
}

// Seal marks the log as read-only. No further appends are allowed.
func (l *MigrationLog) Seal() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Sealed = true
	return nil
}

// VerifyIntegrity checks SeqNo continuity and Checksum consistency.
func (l *MigrationLog) VerifyIntegrity() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, entry := range l.Entries {
		expected := int64(i + 1)
		if entry.SeqNo != expected {
			return fmt.Errorf("seqno gap at index %d: expected %d, got %d", i, expected, entry.SeqNo)
		}
	}

	for _, entry := range l.Entries {
		if entry.ChecksumBefore != "" && entry.Original != "" {
			expected := sha256Sum(entry.Original)
			if entry.ChecksumBefore != expected {
				return fmt.Errorf("checksum mismatch at seqno %d: expected %s, got %s",
					entry.SeqNo, expected, entry.ChecksumBefore)
			}
		}
	}

	return nil
}

// Count returns the number of entries in the log.
func (l *MigrationLog) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.Entries)
}

func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
