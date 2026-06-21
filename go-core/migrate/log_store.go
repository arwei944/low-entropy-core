package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LogStore is the interface for persisting and querying migration logs.
type LogStore interface {
	Store(log *MigrationLog) error
	QueryByFile(path string) []MigrationLogEntry
	QueryByPhase(phase string) []MigrationLogEntry
	QueryByStep(stepID string) []MigrationLogEntry
	QueryByActionType(actionType string) []MigrationLogEntry
	Replay(fromSeq, toSeq int64) []MigrationLogEntry
	Export(format string) (string, error)
}

// FileLogStore persists migration logs as JSON Lines files on disk.
type FileLogStore struct {
	BaseDir string
}

// NewFileLogStore creates a new FileLogStore and ensures the base directory exists.
func NewFileLogStore(baseDir string) *FileLogStore {
	_ = os.MkdirAll(baseDir, 0o755)
	return &FileLogStore{BaseDir: baseDir}
}

// Store writes the full migration log to a JSON Lines file named {sessionID}.log.
// Each line is a single JSON-encoded MigrationLogEntry.
func (s *FileLogStore) Store(log *MigrationLog) error {
	if log == nil {
		return fmt.Errorf("log is nil")
	}

	filename := filepath.Join(s.BaseDir, log.SessionID+".log")
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range log.Entries {
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("encode entry: %w", err)
		}
	}
	return nil
}

// loadAll reads a single JSON Lines log file and returns all entries.
func (s *FileLogStore) loadAll(sessionID string) ([]MigrationLogEntry, error) {
	filename := filepath.Join(s.BaseDir, sessionID+".log")
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []MigrationLogEntry
	dec := json.NewDecoder(f)
	for dec.More() {
		var entry MigrationLogEntry
		if err := dec.Decode(&entry); err != nil {
			return nil, fmt.Errorf("decode entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// queryAll iterates over every .log file in BaseDir, loads entries, and
// returns those that satisfy the provided filter function.
func (s *FileLogStore) queryAll(filter func(MigrationLogEntry) bool) []MigrationLogEntry {
	var results []MigrationLogEntry

	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		return results
	}

	for _, fi := range entries {
		if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".log") {
			continue
		}
		sessionID := strings.TrimSuffix(fi.Name(), ".log")
		all, err := s.loadAll(sessionID)
		if err != nil {
			continue
		}
		for _, entry := range all {
			if filter(entry) {
				results = append(results, entry)
			}
		}
	}

	return results
}

// QueryByFile returns all entries whose FilePath matches the given path.
func (s *FileLogStore) QueryByFile(path string) []MigrationLogEntry {
	return s.queryAll(func(e MigrationLogEntry) bool {
		return e.FilePath == path
	})
}

// QueryByPhase returns all entries whose Phase matches the given phase.
func (s *FileLogStore) QueryByPhase(phase string) []MigrationLogEntry {
	return s.queryAll(func(e MigrationLogEntry) bool {
		return e.Phase == phase
	})
}

// QueryByStep returns all entries whose StepID matches the given step ID.
func (s *FileLogStore) QueryByStep(stepID string) []MigrationLogEntry {
	return s.queryAll(func(e MigrationLogEntry) bool {
		return e.StepID == stepID
	})
}

// QueryByActionType returns all entries whose ActionType matches the given type.
func (s *FileLogStore) QueryByActionType(actionType string) []MigrationLogEntry {
	return s.queryAll(func(e MigrationLogEntry) bool {
		return e.ActionType == actionType
	})
}

// Replay returns entries whose SeqNo falls within [fromSeq, toSeq], sorted by SeqNo.
func (s *FileLogStore) Replay(fromSeq, toSeq int64) []MigrationLogEntry {
	results := s.queryAll(func(e MigrationLogEntry) bool {
		return e.SeqNo >= fromSeq && e.SeqNo <= toSeq
	})
	sort.Slice(results, func(i, j int) bool {
		return results[i].SeqNo < results[j].SeqNo
	})
	return results
}

// Export serialises all entries into the requested format.
// Supported formats: "json" (indented JSON array) and "md" (Markdown table).
func (s *FileLogStore) Export(format string) (string, error) {
	all := s.queryAll(func(MigrationLogEntry) bool { return true })
	sort.Slice(all, func(i, j int) bool {
		return all[i].SeqNo < all[j].SeqNo
	})

	switch format {
	case "json":
		data, err := json.MarshalIndent(all, "", "  ")
		if err != nil {
			return "", fmt.Errorf("json marshal: %w", err)
		}
		return string(data), nil

	case "md":
		var sb strings.Builder
		sb.WriteString("| SeqNo | Phase | Action | File | Line |\n")
		sb.WriteString("|-------|-------|--------|------|------|\n")
		for _, e := range all {
			line := "-"
			if e.LineStart > 0 {
				line = fmt.Sprintf("%d-%d", e.LineStart, e.LineEnd)
			}
			fmt.Fprintf(&sb, "| %d | %s | %s | %s | %s |\n",
				e.SeqNo, e.Phase, e.ActionType, e.FilePath, line)
		}
		return sb.String(), nil

	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}
