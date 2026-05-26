package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/klauspost/compress/zstd"
)

func TestTemporaryRestorePathUsesReadableNameAndDoesNotCreatePath(t *testing.T) {
	tmpDir := t.TempDir()

	tmpPath, err := temporaryRestorePath(tmpDir, "test")
	if err != nil {
		t.Fatalf("temporaryRestorePath failed: %v", err)
	}

	pattern := regexp.MustCompile(`^test\.restore\.\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-\d{6}\.\d{6}\.tmp$`)
	if !pattern.MatchString(filepath.Base(tmpPath)) {
		t.Fatalf("temporaryRestorePath base name = %q, want readable restore temp name", filepath.Base(tmpPath))
	}
	if _, err := os.Lstat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("temporaryRestorePath created path or stat failed unexpectedly: %v", err)
	}
}

func TestTemporaryRestorePathDoesNotDeleteExistingTempLikePath(t *testing.T) {
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "test.restore.2026-05-26T14-35-07-482193.734211.tmp")
	if err := os.WriteFile(existingPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("failed to create existing temp-like path: %v", err)
	}

	tmpPath, err := temporaryRestorePath(tmpDir, "test")
	if err != nil {
		t.Fatalf("temporaryRestorePath failed: %v", err)
	}
	if tmpPath == existingPath {
		t.Fatalf("temporaryRestorePath returned existing path %v", tmpPath)
	}

	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("existing temp-like path was removed: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("existing temp-like path content = %q, want %q", string(content), "keep")
	}
}

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
	rootfsPath := filepath.Join(tmpDir, "test")

	if err := os.Symlink(filepath.Join(tmpDir, "target"), rootfsPath); err != nil {
		t.Fatalf("failed to create rootfs symlink: %v", err)
	}
	if err := replaceRootfs(tmpDir, "test", filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a symlink rootfs")
	}
}

func TestReplaceRootfsRejectsNonDirectoryRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "test")

	if err := os.WriteFile(rootfsPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := replaceRootfs(tmpDir, "test", filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a non-directory rootfs")
	}
}

func TestReplaceRootfsRejectsMissingRootfs(t *testing.T) {
	tmpDir := t.TempDir()

	if err := replaceRootfs(tmpDir, "test", filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a missing rootfs")
	}
}

func TestReplaceRootfsRejectsSymlinkBackup(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "test")
	bakPath := rootfsPath + ".bak"

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmpDir, "target"), bakPath); err != nil {
		t.Fatalf("failed to create backup symlink: %v", err)
	}
	if err := replaceRootfs(tmpDir, "test", filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("replaceRootfs accepted a symlink backup")
	}
}

func TestReplaceRootfsPreservesRootfsWhenArchiveIsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "test")
	archivePath := filepath.Join(tmpDir, "archive.tar.zst")

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootfsPath, "file"), []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("not a zstd archive"), 0o644); err != nil {
		t.Fatalf("failed to create invalid archive: %v", err)
	}

	if err := replaceRootfs(tmpDir, "test", archivePath); err == nil {
		t.Fatal("replaceRootfs accepted an invalid archive")
	}

	content, err := os.ReadFile(filepath.Join(rootfsPath, "file"))
	if err != nil {
		t.Fatalf("failed to read preserved rootfs file: %v", err)
	}
	if string(content) != "old" {
		t.Fatalf("rootfs file = %q, want preserved content %q", string(content), "old")
	}
	if _, err := os.Stat(rootfsPath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup exists after failed restore: %v", err)
	}
}

func TestReplaceRootfsWithoutExistingBackup(t *testing.T) {
	tmpDir := t.TempDir()
	oldRootfsPath := filepath.Join(tmpDir, "old-rootfs")
	rootfsPath := filepath.Join(tmpDir, "test")
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

	if err := replaceRootfs(tmpDir, "test", archivePath); err != nil {
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
