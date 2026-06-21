package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ParserBackend is the pluggable interface for language-specific parsers.
type ParserBackend interface {
	Parse(file string) (*UnifiedFile, error)
	ParseDir(dir string) ([]*UnifiedFile, error)
	SupportedExtensions() []string
	Language() string
}

// LanguageDetector detects the programming language of a project directory.
type LanguageDetector struct {
	detectors map[string]func(dir string) bool
}

// NewLanguageDetector creates a LanguageDetector with built-in language detectors.
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{
		detectors: map[string]func(dir string) bool{
			"go":         detectGoProject,
			"python":     detectPythonProject,
			"java":       detectJavaProject,
			"typescript": detectTypeScriptProject,
			"rust":       detectRustProject,
		},
	}
}

// Detect returns the detected language of the project at dir.
func (d *LanguageDetector) Detect(dir string) string {
	for lang, detect := range d.detectors {
		if detect(dir) {
			return lang
		}
	}
	return "unknown"
}

// Global parser registry.
var (
	parserRegistry   map[string]ParserBackend
	parserRegistryMu sync.RWMutex
)

func init() {
	parserRegistry = make(map[string]ParserBackend)
}

// RegisterParser registers a ParserBackend for the given language.
func RegisterParser(lang string, backend ParserBackend) {
	parserRegistryMu.Lock()
	defer parserRegistryMu.Unlock()
	parserRegistry[lang] = backend
}

// GetParser returns the registered ParserBackend for the given language.
func GetParser(lang string) (ParserBackend, error) {
	parserRegistryMu.RLock()
	defer parserRegistryMu.RUnlock()
	p, ok := parserRegistry[lang]
	if !ok {
		return nil, fmt.Errorf("no parser registered for language: %s", lang)
	}
	return p, nil
}

// ListParsers returns the list of all registered parser languages.
func ListParsers() []string {
	parserRegistryMu.RLock()
	defer parserRegistryMu.RUnlock()
	langs := make([]string, 0, len(parserRegistry))
	for lang := range parserRegistry {
		langs = append(langs, lang)
	}
	return langs
}

// FileExtensionToLanguage maps file extensions to language identifiers.
var FileExtensionToLanguage = map[string]string{
	".go":  "go",
	".py":  "python",
	".java": "java",
	".ts":  "typescript",
	".tsx": "typescript",
	".rs":  "rust",
}

// --- language detection helpers ---

func detectGoProject(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod"))
}

func detectPythonProject(dir string) bool {
	return fileExists(filepath.Join(dir, "requirements.txt")) ||
		fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "pyproject.toml"))
}

func detectJavaProject(dir string) bool {
	return fileExists(filepath.Join(dir, "pom.xml")) ||
		fileExists(filepath.Join(dir, "build.gradle"))
}

func detectTypeScriptProject(dir string) bool {
	return fileExists(filepath.Join(dir, "package.json")) &&
		fileExists(filepath.Join(dir, "tsconfig.json"))
}

func detectRustProject(dir string) bool {
	return fileExists(filepath.Join(dir, "Cargo.toml"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
