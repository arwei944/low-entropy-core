//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

func TestHotReload_Start(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	composer, err := hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}
	defer hotReload.Stop()

	if composer == nil {
		t.Fatal("expected non-nil initial composer")
	}

	result, _, err := composer.Run(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error running pipeline: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestHotReload_Current(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	initial, err := hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}
	defer hotReload.Stop()

	// Current should return the same composer
	current := hotReload.Current()
	if current == nil {
		t.Fatal("expected non-nil composer from Current()")
	}

	// Both should produce the same result
	result1, _, err := initial.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error from initial: %v", err)
	}
	result2, _, err := current.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error from current: %v", err)
	}
	if result1 != result2 {
		t.Errorf("expected same result from initial and current, got %v and %v", result1, result2)
	}
	if result1 != 20 {
		t.Errorf("expected 20, got %v", result1)
	}
}

func TestHotReload_Stop(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file using ioutil
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if err := ioutil.WriteFile(tmpFile.Name(), []byte(configContent), 0644); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	_, err = hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}

	hotReload.Stop()

	// Verify Stop is idempotent / safe to call again
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Stop() panicked: %v", r)
			}
		}()
		hotReload.Stop()
	}()
}
