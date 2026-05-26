package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/klauspost/compress/zstd"
)

func TestResolveSnapshotPathUsesNamedSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshots", "test", "base.2026-05-25T12-00-00.tar.zst")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}
	if err := os.WriteFile(snapshotPath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("failed to create snapshot archive: %v", err)
	}

	got, err := resolveSnapshotPath(tmpDir, "test", "base")
	if err != nil {
		t.Fatalf("resolveSnapshotPath failed: %v", err)
	}
	if got != snapshotPath {
		t.Fatalf("resolveSnapshotPath = %q, want %q", got, snapshotPath)
	}
}

func TestResolveSnapshotPathRejectsMissingSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := resolveSnapshotPath(tmpDir, "test", "base")
	if err == nil {
		t.Fatal("resolveSnapshotPath accepted a missing snapshot")
	}
	if !strings.Contains(err.Error(), "snapshot base not found") {
		t.Fatalf("resolveSnapshotPath error = %q, want not-found error", err)
	}
}

func TestResolveSnapshotPathRejectsNonRegularSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotDir := filepath.Join(tmpDir, "snapshots", "test", "base.2026-05-25T12-00-00.tar.zst")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("failed to create non-file snapshot archive match: %v", err)
	}

	_, err := resolveSnapshotPath(tmpDir, "test", "base")
	if err == nil {
		t.Fatal("resolveSnapshotPath accepted a non-regular snapshot")
	}
	if !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("resolveSnapshotPath error = %q, want non-regular-file error", err)
	}
}

func TestResolveSnapshotPathRejectsMultipleSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotsDir := filepath.Join(tmpDir, "snapshots", "test")
	if err := os.MkdirAll(snapshotsDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}
	for _, name := range []string{"base.2026-05-25T12-00-00.tar.zst", "base.2026-05-25T12-01-00.tar.zst"} {
		if err := os.WriteFile(filepath.Join(snapshotsDir, name), []byte("archive"), 0o644); err != nil {
			t.Fatalf("failed to create snapshot archive: %v", err)
		}
	}

	_, err := resolveSnapshotPath(tmpDir, "test", "base")
	if err == nil {
		t.Fatal("resolveSnapshotPath accepted multiple snapshots")
	}
	if !strings.Contains(err.Error(), "multiple snapshots named base") {
		t.Fatalf("resolveSnapshotPath error = %q, want multiple-snapshots error", err)
	}
}

func TestReplaceRootfsRejectsSymlinkRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.Symlink(filepath.Join(tmpDir, "target"), rootfsPath); err != nil {
		t.Fatalf("failed to create rootfs symlink: %v", err)
	}
	if err := replaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a symlink rootfs")
	}
}

func TestReplaceRootfsRejectsNonDirectoryRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.WriteFile(rootfsPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := replaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a non-directory rootfs")
	}
}

func TestReplaceRootfsRejectsMissingRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := replaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a missing rootfs")
	}
}

func TestReplaceRootfsRejectsSymlinkBackup(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	bakPath := rootfsPath + ".bak"

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmpDir, "target"), bakPath); err != nil {
		t.Fatalf("failed to create backup symlink: %v", err)
	}
	if err := replaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a symlink backup")
	}
}

func TestReplaceRootfsWithoutExistingBackup(t *testing.T) {
	tmpDir := t.TempDir()
	oldRootfsPath := filepath.Join(tmpDir, "old-rootfs")
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	archivePath := filepath.Join(tmpDir, "archive.tar.zst")

	if err := os.Mkdir(oldRootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create old rootfs directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldRootfsPath, "file"), []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to create archive source file: %v", err)
	}
	if err := sandbox.Compress(oldRootfsPath, archivePath, zstd.SpeedDefault); err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootfsPath, "file"), []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create old rootfs file: %v", err)
	}

	if err := replaceRootfs(rootfsPath, archivePath); err != nil {
		t.Fatalf("replaceRootfs failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(rootfsPath, "file"))
	if err != nil {
		t.Fatalf("failed to read restored rootfs file: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("restored rootfs file = %q, want %q", string(content), "new")
	}
	if _, err := os.Stat(rootfsPath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup still exists after successful restore: %v", err)
	}
}
