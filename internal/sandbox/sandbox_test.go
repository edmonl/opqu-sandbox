package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestValidateName(t *testing.T) {
	validNames := []string{"test", "a", "123", "a-b", "test-sandbox", "this-is-longer-than-twelve-characters"}
	for _, name := range validNames {
		if err := ValidateName(name); err != nil {
			t.Errorf("Expected name %v to be valid, got error: %v", name, err)
		}
	}

	invalidNames := []string{
		"",          // too short
		"Test",      // uppercase
		"test_1",    // underscore
		"test name", // space
	}
	for _, name := range invalidNames {
		if err := ValidateName(name); err == nil {
			t.Errorf("Expected name %v to be invalid, but it was accepted", name)
		}
	}
}

func TestCreateSymlinkCreatesMissingLink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	symlinkPath := filepath.Join(tmpDir, "link")

	if err := CreateSymlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != targetPath {
		t.Fatalf("symlink target = %q, want %q", target, targetPath)
	}
}

func TestCreateSymlinkKeepsExistingCorrectLink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	symlinkPath := filepath.Join(tmpDir, "link")

	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("failed to create initial symlink: %v", err)
	}
	if err := CreateSymlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != targetPath {
		t.Fatalf("symlink target = %q, want %q", target, targetPath)
	}
}

func TestCreateSymlinkReplacesExistingWrongLink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	oldTargetPath := filepath.Join(tmpDir, "old-target")
	symlinkPath := filepath.Join(tmpDir, "link")

	if err := os.Symlink(oldTargetPath, symlinkPath); err != nil {
		t.Fatalf("failed to create initial symlink: %v", err)
	}
	if err := CreateSymlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != targetPath {
		t.Fatalf("symlink target = %q, want %q", target, targetPath)
	}
}

func TestCreateSymlinkRejectsNonSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	symlinkPath := filepath.Join(tmpDir, "link")

	if err := os.WriteFile(symlinkPath, []byte("not a symlink"), 0o644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}
	if err := CreateSymlink(targetPath, symlinkPath); err == nil {
		t.Fatal("CreateSymlink accepted a non-symlink path")
	}
}

func TestReplaceRootfsRejectsSymlinkRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.Symlink(filepath.Join(tmpDir, "target"), rootfsPath); err != nil {
		t.Fatalf("failed to create rootfs symlink: %v", err)
	}
	if err := ReplaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("ReplaceRootfs accepted a symlink rootfs")
	}
}

func TestReplaceRootfsRejectsNonDirectoryRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.WriteFile(rootfsPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := ReplaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("ReplaceRootfs accepted a non-directory rootfs")
	}
}

func TestReplaceRootfsRejectsMissingRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := ReplaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("ReplaceRootfs accepted a missing rootfs")
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
	if err := ReplaceRootfs(rootfsPath, filepath.Join(tmpDir, "archive.tar.zst")); err == nil {
		t.Fatal("ReplaceRootfs accepted a symlink backup")
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
	if err := Compress(oldRootfsPath, archivePath, zstd.SpeedDefault); err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootfsPath, "file"), []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create old rootfs file: %v", err)
	}

	if err := ReplaceRootfs(rootfsPath, archivePath); err != nil {
		t.Fatalf("ReplaceRootfs failed: %v", err)
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
