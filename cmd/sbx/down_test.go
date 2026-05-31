package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
)

func TestRemoveNspawnFileRemovesManagedSymlinkAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	nspawnFile := filepath.Join(tmpDir, "rootfs", "test.nspawn")
	nspawnSymlinkPath := filepath.Join(tmpDir, "nspawn", "test.nspawn")
	conf := &config.Config{
		NspawnFilesPath: filepath.Join(tmpDir, "nspawn"),
		SandboxUser:     &util.User{},
	}

	if err := os.MkdirAll(filepath.Dir(nspawnFile), 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(nspawnSymlinkPath), 0o755); err != nil {
		t.Fatalf("failed to create nspawn directory: %v", err)
	}
	if err := os.WriteFile(nspawnFile, []byte("config"), 0o644); err != nil {
		t.Fatalf("failed to create nspawn file: %v", err)
	}
	if err := os.Symlink(nspawnFile, nspawnSymlinkPath); err != nil {
		t.Fatalf("failed to create nspawn symlink: %v", err)
	}

	sandbox.RemoveNspawnFile(tmpDir, "test", conf)

	if _, err := os.Lstat(nspawnSymlinkPath); !os.IsNotExist(err) {
		t.Fatalf("nspawn symlink still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Lstat(nspawnFile); !os.IsNotExist(err) {
		t.Fatalf("nspawn file still exists or stat failed unexpectedly: %v", err)
	}
}
