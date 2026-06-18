// auto_detect.go — Automatic project complexity detection
// This file is part of the kernel (always compiled, no build tags).
// It scans a project directory and determines the appropriate ComplexityTier.

package core

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectStats contains metrics gathered from scanning a project directory.
type ProjectStats struct {
	// TotalFiles is the total number of source files found.
	TotalFiles int

	// Languages maps file extensions to file counts.
	Languages map[string]int

	// ModuleCount is the number of go.mod files found (proxy for module count).
	ModuleCount int

	// TeamCount is the estimated number of teams (from CODEOWNERS entries).
	TeamCount int

	// MaxDepth is the deepest directory nesting level found.
	MaxDepth int
}

// AutoDetect scans the given root directory and returns the recommended
// ComplexityTier based on project size metrics.
//
// Detection heuristics:
//   - <10 files        → TierL0 (Prototype)
//   - 10-100 files     → TierL1 (Microservice)
//   - 100-1000 files   → TierL2 (Mid-size)
//   - 1000-10000 files → TierL3 (Large)
//   - 10K-100K files   → TierL4 (Platform)
//   - 100K-1M files    → TierL5 (Enterprise)
//   - 1M-10M files     → TierL6 (System-level)
//   - 10M+ files       → TierL7 (Windows-scale)
//
// Multi-language projects and multi-module projects receive a +1 tier boost.
func AutoDetect(root string) ComplexityTier {
	stats := scanProject(root)

	switch {
	case stats.TotalFiles < 10:
		return TierL0
	case stats.TotalFiles < 100:
		return TierL1
	case stats.TotalFiles < 1000:
		return TierL2
	case stats.TotalFiles < 10000:
		return TierL3
	case stats.TotalFiles < 100000:
		return TierL4
	case stats.TotalFiles < 1000000:
		return TierL5
	case stats.TotalFiles < 10000000:
		return TierL6
	default:
		return TierL7
	}
}

// AutoDetectWithBoost is like AutoDetect but applies a tier boost based on
// project complexity signals (multi-language, multi-module, multi-team).
func AutoDetectWithBoost(root string) ComplexityTier {
	tier := AutoDetect(root)
	stats := scanProject(root)

	boost := 0
	if len(stats.Languages) >= 3 {
		boost++ // Multi-language project
	}
	if stats.ModuleCount >= 5 {
		boost++ // Multi-module project
	}
	if stats.TeamCount >= 10 {
		boost++ // Multi-team project
	}

	tier = ComplexityTier(int(tier) + boost)
	if tier > TierL7 {
		tier = TierL7
	}
	return tier
}

// ScanProject returns detailed statistics about a project directory.
func ScanProject(root string) ProjectStats {
	return scanProject(root)
}

// scanProject is the internal implementation of project scanning.
func scanProject(root string) ProjectStats {
	stats := ProjectStats{
		Languages: make(map[string]int),
	}

	// Known source file extensions
	sourceExts := map[string]bool{
		".go": true, ".c": true, ".cc": true, ".cpp": true, ".cxx": true,
		".h": true, ".hpp": true, ".rs": true, ".cs": true, ".java": true,
		".py": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".proto": true, ".sql": true, ".yaml": true, ".yml": true, ".toml": true,
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			base := info.Name()
			if strings.HasPrefix(base, ".") && base != "." && base != ".." {
				return filepath.SkipDir
			}
			switch base {
			case "node_modules", "vendor", ".git", "target", "build", "dist",
				"__pycache__", ".venv", "venv", "bin", "obj":
				return filepath.SkipDir
			}

			// Track depth
			rel, _ := filepath.Rel(root, path)
			if rel != "." {
				depth := len(strings.Split(rel, string(filepath.Separator)))
				if depth > stats.MaxDepth {
					stats.MaxDepth = depth
				}
			}
			return nil
		}

		// Count module files
		if info.Name() == "go.mod" {
			stats.ModuleCount++
		}

		// Count CODEOWNERS entries
		if info.Name() == "CODEOWNERS" {
			data, err := os.ReadFile(path)
			if err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						stats.TeamCount++
					}
				}
			}
		}

		// Count source files by extension
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if sourceExts[ext] {
			stats.TotalFiles++
			stats.Languages[ext]++
		}

		return nil
	})

	return stats
}