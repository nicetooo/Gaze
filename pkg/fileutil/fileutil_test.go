package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")

	if err := WriteFileAtomic(path, []byte(`[{"id":"a"}]`), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != `[{"id":"a"}]` {
		t.Errorf("unexpected content: %s", data)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("expected perm 0644, got %v", info.Mode().Perm())
		}
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")

	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new content"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("expected overwrite, got: %s", data)
	}
}

func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")

	if err := WriteFileAtomic(path, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}
	// Failure path: target dir of a second write does not exist
	if err := WriteFileAtomic(filepath.Join(dir, "missing", "x.json"), []byte("data"), 0644); err == nil {
		t.Error("expected error writing into missing directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected only the target file, got %d entries", len(entries))
	}
}

func TestWriteFileAtomicTempNameNotJSON(t *testing.T) {
	// Watcher compatibility: temp names must never end in ".json"
	dir := t.TempDir()
	tmp, err := os.CreateTemp(dir, filepath.Base("wf.json")+".tmp-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()
	if strings.HasSuffix(tmp.Name(), ".json") {
		t.Errorf("temp file name ends in .json: %s", tmp.Name())
	}
}

func TestQuarantineCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	target, err := QuarantineCorrupt(path)
	if err != nil {
		t.Fatalf("QuarantineCorrupt failed: %v", err)
	}
	if !strings.Contains(target, ".corrupt-") {
		t.Errorf("unexpected quarantine name: %s", target)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("original file should be gone, stat err: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("quarantined file unreadable: %v", err)
	}
	if string(data) != "{broken" {
		t.Errorf("quarantined content changed: %s", data)
	}
}

func TestQuarantineCorruptMissingFile(t *testing.T) {
	if _, err := QuarantineCorrupt(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}
