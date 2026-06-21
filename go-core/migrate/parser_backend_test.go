package migrate

import (
	"testing"
)

// mockParser is a test-only ParserBackend implementation.
type mockParser struct {
	lang       string
	extensions []string
}

func (m *mockParser) Parse(file string) (*UnifiedFile, error) {
	return &UnifiedFile{Path: file, Language: m.lang}, nil
}

func (m *mockParser) ParseDir(dir string) ([]*UnifiedFile, error) {
	return []*UnifiedFile{{Path: dir, Language: m.lang}}, nil
}

func (m *mockParser) SupportedExtensions() []string { return m.extensions }
func (m *mockParser) Language() string              { return m.lang }

func TestRegisterParserAndGet(t *testing.T) {
	parserRegistryMu.Lock()
	parserRegistry = make(map[string]ParserBackend)
	parserRegistryMu.Unlock()

	RegisterParser("testlang", &mockParser{lang: "testlang", extensions: []string{".tl"}})

	p, err := GetParser("testlang")
	if err != nil {
		t.Fatalf("GetParser failed: %v", err)
	}
	if p.Language() != "testlang" {
		t.Errorf("Language: got %q, want %q", p.Language(), "testlang")
	}
}

func TestGetParser_Unregistered(t *testing.T) {
	parserRegistryMu.Lock()
	parserRegistry = make(map[string]ParserBackend)
	parserRegistryMu.Unlock()

	_, err := GetParser("nonexistent")
	if err == nil {
		t.Error("expected error for unregistered parser, got nil")
	}
}

func TestListParsers(t *testing.T) {
	parserRegistryMu.Lock()
	parserRegistry = make(map[string]ParserBackend)
	parserRegistryMu.Unlock()

	RegisterParser("a", &mockParser{lang: "a"})
	RegisterParser("b", &mockParser{lang: "b"})

	langs := ListParsers()
	if len(langs) != 2 {
		t.Errorf("ListParsers count: got %d, want 2", len(langs))
	}
}

func TestLanguageDetector_GoProject(t *testing.T) {
	detector := NewLanguageDetector()
	// go-core has go.mod
	lang := detector.Detect("../..")
	if lang != "go" {
		t.Errorf("Detect: got %q, want %q", lang, "go")
	}
}

func TestLanguageDetector_Unknown(t *testing.T) {
	detector := NewLanguageDetector()
	lang := detector.Detect("c:\\nonexistent_dir_xyz")
	if lang != "unknown" {
		t.Errorf("Detect for nonexistent dir: got %q, want %q", lang, "unknown")
	}
}

func TestFileExtensionToLanguage(t *testing.T) {
	tests := []struct {
		ext  string
		lang string
	}{
		{".go", "go"},
		{".py", "python"},
		{".java", "java"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".rs", "rust"},
	}
	for _, tt := range tests {
		got := FileExtensionToLanguage[tt.ext]
		if got != tt.lang {
			t.Errorf("FileExtensionToLanguage[%q] = %q, want %q", tt.ext, got, tt.lang)
		}
	}
}
