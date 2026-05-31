package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSbxDirCreatesLeafOnly(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "sandboxes")

	if err := ensureSbxDir(path); err != nil {
		t.Fatalf("ensureSbxDir failed: %v", err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("failed to stat sandbox directory: %v", err)
	} else if !info.IsDir() {
		t.Fatal("sandbox path is not a directory")
	}
}

func TestEnsureSbxDirRejectsMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "sandboxes")

	err := ensureSbxDir(path)
	if err == nil {
		t.Fatal("ensureSbxDir accepted missing parent")
	}
	if !strings.Contains(err.Error(), "parent directory") {
		t.Fatalf("ensureSbxDir error = %q, want parent directory error", err)
	}
}
