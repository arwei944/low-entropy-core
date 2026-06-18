//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

func TestFileStorageBackend_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStorageBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	ctx := context.Background()
	if err := fs.Save(ctx, "test/key1", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	data, err := fs.Load(ctx, "test/key1")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", data)
	}
}

func TestFileStorageBackend_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	defer fs.Close()

	_, err := fs.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestFileStorageBackend_Delete(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	defer fs.Close()

	ctx := context.Background()
	fs.Save(ctx, "k", []byte("v"))
	fs.Delete(ctx, "k")
	_, err := fs.Load(ctx, "k")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFileStorageBackend_DeleteMissing(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	defer fs.Close()

	// 删除不存在的 key 不应报错
	err := fs.Delete(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("expected nil error for missing key, got %v", err)
	}
}

func TestFileStorageBackend_List(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	defer fs.Close()

	ctx := context.Background()
	fs.Save(ctx, "events/1", []byte("a"))
	fs.Save(ctx, "events/2", []byte("b"))
	fs.Save(ctx, "other/x", []byte("c"))

	keys, err := fs.List(ctx, "events/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d: %v", len(keys), keys)
	}
}

func TestFileStorageBackend_Overwrite(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	defer fs.Close()

	ctx := context.Background()
	fs.Save(ctx, "k", []byte("v1"))
	fs.Save(ctx, "k", []byte("v2"))
	data, _ := fs.Load(ctx, "k")
	if string(data) != "v2" {
		t.Errorf("expected 'v2' after overwrite, got %q", data)
	}
}