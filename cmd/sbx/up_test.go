package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
)

func TestCreateNspawnFileCreatesFile(t *testing.T) {
	rootfsDir := t.TempDir()
	conf := &config.Config{
		NetworkZone: "opqu-sbx",
		ResolvConf:  "auto",
	}

	nspawnPath, err := createNspawnFile(rootfsDir, "test", conf, nil)
	if err != nil {
		t.Fatalf("createNspawnFile failed: %v", err)
	}

	content, err := os.ReadFile(nspawnPath)
	if err != nil {
		t.Fatalf("failed to read nspawn file: %v", err)
	}
	if !strings.Contains(string(content), "Zone=opqu-sbx") {
		t.Fatalf("nspawn file missing network zone: %q", string(content))
	}
}

func TestCreateNspawnFileRejectsSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsDir := filepath.Join(tmpDir, "rootfs")
	targetPath := filepath.Join(tmpDir, "target")
	nspawnPath := filepath.Join(rootfsDir, "test.nspawn")
	conf := &config.Config{
		NetworkZone: "opqu-sbx",
		ResolvConf:  "auto",
	}

	if err := os.Mkdir(rootfsDir, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("failed to create symlink target: %v", err)
	}
	if err := os.Symlink(targetPath, nspawnPath); err != nil {
		t.Fatalf("failed to create nspawn symlink: %v", err)
	}

	_, err := createNspawnFile(rootfsDir, "test", conf, nil)
	if err == nil {
		t.Fatal("createNspawnFile accepted a symlink nspawn path")
	}
	if !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("createNspawnFile error = %q, want non-regular file error", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read symlink target: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("symlink target content = %q, want %q", string(content), "keep")
	}
}
