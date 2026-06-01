package sandbox

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/util"
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

func TestIsRunningReportsListedMachine(t *testing.T) {
	installFakeMachinectl(t)
	t.Setenv("MACHINECTL_FAKE_MODE", "running")

	running, err := IsRunning("test")
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Fatal("IsRunning returned false for listed machine")
	}
}

func TestIsRunningReportsUnlistedMachineNotRunning(t *testing.T) {
	installFakeMachinectl(t)
	t.Setenv("MACHINECTL_FAKE_MODE", "other")

	running, err := IsRunning("test")
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Fatal("IsRunning returned true for unlisted machine")
	}
}

func TestIsRunningReturnsMachinectlListError(t *testing.T) {
	installFakeMachinectl(t)
	t.Setenv("MACHINECTL_FAKE_MODE", "error")

	running, err := IsRunning("test")
	if err == nil {
		t.Fatal("IsRunning succeeded when machinectl list failed")
	}
	if running {
		t.Fatal("IsRunning returned true with an error")
	}
	if !strings.Contains(err.Error(), "DBus unavailable") {
		t.Fatalf("IsRunning error = %q, want machinectl stderr", err)
	}
}

func TestIsRunningReturnsJSONError(t *testing.T) {
	installFakeMachinectl(t)
	t.Setenv("MACHINECTL_FAKE_MODE", "bad-json")

	running, err := IsRunning("test")
	if err == nil {
		t.Fatal("IsRunning succeeded with invalid machinectl JSON")
	}
	if running {
		t.Fatal("IsRunning returned true with an error")
	}
	if !strings.Contains(err.Error(), "failed to parse machinectl output") {
		t.Fatalf("IsRunning error = %q, want JSON parse context", err)
	}
}

func installFakeMachinectl(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	machinectlPath := filepath.Join(binDir, "machinectl")
	script := `#!/bin/sh
if [ "$1" != "list" ] || [ "$2" != "--output=json" ]; then
	echo "unexpected arguments: $*" >&2
	exit 64
fi

case "$MACHINECTL_FAKE_MODE" in
	running)
		printf '[{"machine":"test"}]\n'
		;;
	other)
		printf '[{"machine":"other"}]\n'
		;;
	bad-json)
		printf 'not json\n'
		;;
	error)
		echo "DBus unavailable" >&2
		exit 1
		;;
	*)
		echo "missing fake mode" >&2
		exit 2
		;;
esac
`
	if err := os.WriteFile(machinectlPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake machinectl: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
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

func TestRemoveNspawnFileRemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	nspawnPath := filepath.Join(tmpDir, "rootfs", "test.nspawn")
	nspawnSymlinkPath := filepath.Join(tmpDir, "nspawn", "test.nspawn")
	conf := &config.Config{
		NspawnFilesPath: filepath.Join(tmpDir, "nspawn"),
		SandboxUser:     &util.User{User: &user.User{Username: "test"}},
	}

	if err := os.MkdirAll(filepath.Dir(nspawnPath), 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(nspawnSymlinkPath), 0o755); err != nil {
		t.Fatalf("failed to create nspawn directory: %v", err)
	}
	if err := os.WriteFile(nspawnPath, []byte("config"), 0o644); err != nil {
		t.Fatalf("failed to create nspawn file: %v", err)
	}
	if err := os.Symlink(nspawnPath, nspawnSymlinkPath); err != nil {
		t.Fatalf("failed to create nspawn symlink: %v", err)
	}

	RemoveNspawnFile(tmpDir, "test", conf)
	if _, err := os.Lstat(nspawnSymlinkPath); !os.IsNotExist(err) {
		t.Fatalf("nspawn symlink still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Lstat(nspawnPath); !os.IsNotExist(err) {
		t.Fatalf("nspawn file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestRemoveNspawnFileKeepsRepointedSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	nspawnPath := filepath.Join(tmpDir, "rootfs", "test.nspawn")
	nspawnSymlinkPath := filepath.Join(tmpDir, "nspawn", "test.nspawn")
	otherTarget := filepath.Join(tmpDir, "other.nspawn")
	conf := &config.Config{
		NspawnFilesPath: filepath.Join(tmpDir, "nspawn"),
		SandboxUser:     &util.User{User: &user.User{Username: "test"}},
	}

	if err := os.MkdirAll(filepath.Dir(nspawnPath), 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(nspawnSymlinkPath), 0o755); err != nil {
		t.Fatalf("failed to create nspawn directory: %v", err)
	}
	if err := os.WriteFile(nspawnPath, []byte("config"), 0o644); err != nil {
		t.Fatalf("failed to create nspawn file: %v", err)
	}
	if err := os.WriteFile(otherTarget, []byte("other"), 0o644); err != nil {
		t.Fatalf("failed to create other target: %v", err)
	}
	if err := os.Symlink(otherTarget, nspawnSymlinkPath); err != nil {
		t.Fatalf("failed to create nspawn symlink: %v", err)
	}

	RemoveNspawnFile(tmpDir, "test", conf)

	target, err := os.Readlink(nspawnSymlinkPath)
	if err != nil {
		t.Fatalf("failed to read nspawn symlink: %v", err)
	}
	if target != otherTarget {
		t.Fatalf("nspawn symlink target = %q, want %q", target, otherTarget)
	}
	if _, err := os.Lstat(nspawnPath); !os.IsNotExist(err) {
		t.Fatalf("nspawn file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestRemoveNspawnFileRejectsNonRegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	nspawnPath := filepath.Join(tmpDir, "rootfs", "test.nspawn")
	childPath := filepath.Join(nspawnPath, "child")
	conf := &config.Config{
		NspawnFilesPath: filepath.Join(tmpDir, "nspawn"),
		SandboxUser:     &util.User{User: &user.User{Username: "test"}},
	}

	if err := os.MkdirAll(nspawnPath, 0o755); err != nil {
		t.Fatalf("failed to create nspawn directory: %v", err)
	}
	if err := os.WriteFile(childPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("failed to create child file: %v", err)
	}

	RemoveNspawnFile(tmpDir, "test", conf)
	if _, err := os.Stat(childPath); err != nil {
		t.Fatalf("directory contents were removed: %v", err)
	}
}

func TestCreateSnapshotRejectsNonDirectoryRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	snapshotsDir := filepath.Join(tmpDir, "snapshots")
	owner := currentTestUser(t)

	if err := os.WriteFile(rootfsPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := os.Mkdir(snapshotsDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}

	err := CreateSnapshot(rootfsPath, snapshotsDir, "test", owner)
	if err == nil {
		t.Fatal("CreateSnapshot accepted a non-directory rootfs")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("CreateSnapshot error = %q, want non-directory error", err)
	}

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		t.Fatalf("failed to read snapshots directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("CreateSnapshot wrote archive output after source rejection: %v", entries)
	}
}

func TestCreateSnapshotRejectsSymlinkRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	snapshotsDir := filepath.Join(tmpDir, "snapshots")
	owner := currentTestUser(t)

	if err := os.Mkdir(targetPath, 0o755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}
	if err := os.Symlink(targetPath, rootfsPath); err != nil {
		t.Fatalf("failed to create rootfs symlink: %v", err)
	}
	if err := os.Mkdir(snapshotsDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}

	err := CreateSnapshot(rootfsPath, snapshotsDir, "test", owner)
	if err == nil {
		t.Fatal("CreateSnapshot accepted a symlink rootfs")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("CreateSnapshot error = %q, want non-directory error", err)
	}
}

func TestCreateSnapshotArchivesInactiveRootfs(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	snapshotsDir := filepath.Join(tmpDir, "snapshots")
	owner := currentTestUser(t)

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootfsPath, "file"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create rootfs file: %v", err)
	}
	if err := os.Mkdir(snapshotsDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}

	if err := CreateSnapshot(rootfsPath, snapshotsDir, "test", owner); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(snapshotsDir, "test.*.tar.zst"))
	if err != nil {
		t.Fatalf("failed to list snapshots: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("snapshot count = %d, want 1: %v", len(matches), matches)
	}
}

func currentTestUser(t *testing.T) *util.User {
	t.Helper()
	u, err := util.InvokingUser()
	if err != nil {
		t.Fatalf("failed to get invoking user: %v", err)
	}
	return u
}
