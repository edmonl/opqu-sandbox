// Package sandbox provides helpers for managing sandbox.
package sandbox

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/klauspost/compress/zstd"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)
var snapshotNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateName ensures the sandbox name consists only of lowercase alphanumeric characters and hyphens.
func ValidateName(name string) error {
	if nameRegex.MatchString(name) {
		return nil
	}
	return fmt.Errorf("sandbox name %v is invalid, must be lowercase alphanumeric and hyphens only", name)
}

// ValidateSnapshotName ensures the snapshot name consists only of alphanumeric characters, underscores, and hyphens.
func ValidateSnapshotName(name string) error {
	if snapshotNameRegex.MatchString(name) {
		return nil
	}
	return errors.New("snapshot name must be alphanumeric, '_', and '-' only")
}

// RequireInactiveRootfs verifies that path is missing or a real directory without active mounts.
// The returned bool indicates whether the rootfs exists.
func RequireInactiveRootfs(path string) (bool, error) {
	if err := util.RequireRealDirectory(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		} else {
			return false, err
		}
	}

	hasMounts, err := HasMounts(path)
	if err == nil && hasMounts {
		err = fmt.Errorf("%v contains active mounts", path)
	}

	return true, err
}

// MkdirAllAsUser creates each path element under sbxDir and changes ownership of newly created directories to the sandbox user.
func MkdirAllAsUser(conf *config.Config, sbxDir string, elem ...string) (string, error) {
	path := sbxDir
	changeOwner := os.Geteuid() != conf.SandboxUser.UID
	for _, e := range elem {
		path = filepath.Join(path, e)
		if info, err := os.Stat(path); err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%v exists but is not a directory", path)
			}
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("failed to access %v: %w", path, err)
		}

		if err := os.Mkdir(path, 0o755); err != nil {
			return "", fmt.Errorf("failed to create directory %v: %w", path, err)
		}
		if changeOwner {
			if err := util.ChownToUser(path, conf.SandboxUser); err != nil {
				return "", err
			}
		}
	}

	return path, nil
}

// RemoveNspawnFile removes a generated nspawn file and its symlink in the best effort.
func RemoveNspawnFile(sbxDir, name string, conf *config.Config) {
	nspawnFile := filepath.Join(sbxDir, "rootfs", name+".nspawn")
	nspawnSymlinkPath := filepath.Join(conf.NspawnFilesPath, name+".nspawn")

	if ok, err := util.CheckSymlinkTarget(nspawnSymlinkPath, nspawnFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			util.Warn("failed to clean up nspawn file symlink: %v", err)
		}
	} else if !ok {
		util.Warn("keep nspawn file symlink %v because it does not point to %v", nspawnSymlinkPath, nspawnFile)
	} else if err := os.Remove(nspawnSymlinkPath); err != nil {
		util.Warn("failed to delete nspawn file symlink %v: %v", nspawnSymlinkPath, err)
	}

	info, err := os.Lstat(nspawnFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			util.Warn("failed to delete nspawn file %v: %v", nspawnFile, err)
		}
		return
	}

	if !info.Mode().IsRegular() {
		util.Warn("failed to delete nspawn file %v: not a regular file", nspawnFile)
		return
	}

	if err := os.Remove(nspawnFile); err != nil {
		util.Warn("failed to delete nspawn file %v: %v", nspawnFile, err)
	}
}

type runningMachineInfo struct {
	Machine string `json:"machine"`
}

// IsRunning checks whether the sandbox with the specified name is currently running via machinectl.
func IsRunning(name string) (bool, error) {
	// `machinectl show NAME` reports a missing machine through undocumented stderr
	// text, so avoid parsing that diagnostic. `machinectl list` documents that it
	// lists running machines, making absence from the list a reliable not-running
	// result while still surfacing list/DBus/polkit failures.
	cmd := exec.Command("machinectl", "list", "--output=json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputText := strings.TrimSpace(string(output))
		if outputText == "" {
			return false, fmt.Errorf("failed to list running sandboxes with machinectl: %w", err)
		}
		return false, fmt.Errorf("failed to list running sandboxes with machinectl: %w: %s", err, outputText)
	}

	var machines []runningMachineInfo
	if err := json.Unmarshal(output, &machines); err != nil {
		return false, fmt.Errorf("failed to parse machinectl output: %w", err)
	}

	for _, machine := range machines {
		if machine.Machine == name {
			return true, nil
		}
	}

	return false, nil
}

// EnsureStopped verifies that the sandbox is not running, returning an error if it is active or invalid.
func EnsureStopped(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	running, err := IsRunning(name)
	if err != nil {
		return err
	}

	if running {
		return errors.New("cannot operate a running sandbox")
	}

	return nil
}

// CreateSnapshot creates a zstd-compressed tarball of the rootfs and changes ownership to owner if applicable.
func CreateSnapshot(rootfsPath, snapshotsDir, snapshotName string, owner *util.User) error {
	if rootfsExists, err := RequireInactiveRootfs(rootfsPath); err != nil {
		return err
	} else if !rootfsExists {
		return fmt.Errorf("%v is missing", rootfsPath)
	}

	pattern := filepath.Join(snapshotsDir, snapshotName+".*.tar.zst")
	oldSnapshots, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list old snapshots in %v: %w", snapshotsDir, err)
	}

	if len(oldSnapshots) > 0 {
		confirmed, err := util.Confirm(fmt.Sprintf("Snapshot %v already exists. Press <Enter> directly to overwrite, or Ctrl+C to cancel: ", snapshotName))
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("user cancelled overwriting snapshot %v", snapshotName)
		}
	}

	snapshotPath := filepath.Join(snapshotsDir, fmt.Sprintf("%v.%v.tar.zst", snapshotName, time.Now().Format("2006-01-02T15-04-05")))

	if err := Compress(rootfsPath, snapshotPath, zstd.SpeedDefault); err != nil {
		if e := os.RemoveAll(snapshotPath); e != nil {
			util.Warn("failed to delete unsuccessful snapshot %v: %v", snapshotPath, e)
		}

		return fmt.Errorf("failed to create snapshot from %v: %w", rootfsPath, err)
	}

	if err := util.ChownToUser(snapshotPath, owner); err != nil {
		util.Warn("failed to change ownership of %v: %v", snapshotPath, err)
	}

	for _, old := range oldSnapshots {
		if old != snapshotPath {
			if err := os.RemoveAll(old); err != nil {
				util.Warn("failed to remove old snapshot %v: %v", old, err)
			}
		}
	}

	return nil
}

var mountPathReplacer = strings.NewReplacer(
	`\040`, " ",
	`\011`, "\t",
	`\012`, "\n",
	`\134`, `\`,
)

// HasMounts checks /proc/self/mountinfo to determine if the given directory or any of its subdirectories are mount points.
func HasMounts(dir string) (bool, error) {
	// Resolve symlinks since mountinfo reports canonical paths.
	// Fallback to Abs if it fails (e.g., the directory does not exist).
	absDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		absDir, err = filepath.Abs(dir)
		if err != nil {
			return false, fmt.Errorf("failed to resolve path %v: %w", dir, err)
		}
	}

	// Ensure the directory has a trailing slash for prefix matching,
	// except when matching the exact directory itself.
	prefix := absDir
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, fmt.Errorf("failed to access mount info: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 {
			mountPoint := mountPathReplacer.Replace(fields[4])
			if mountPoint == absDir || strings.HasPrefix(mountPoint, prefix) {
				return true, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading mount info: %w", err)
	}

	return false, nil
}

// CreateSymlink safely creates or updates a symbolic link to point to the target path.
// It returns an error if the symlinkPath exists but is not a symlink.
func CreateSymlink(targetPath, symlinkPath string) error {
	info, err := os.Lstat(symlinkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%v exists but is not a symlink", symlinkPath)
		}

		existingTarget, errReadLink := os.Readlink(symlinkPath)
		if errReadLink != nil {
			return fmt.Errorf("failed to read symlink %v: %w", symlinkPath, errReadLink)
		}

		if existingTarget == targetPath {
			return nil
		}

		if errRemove := os.Remove(symlinkPath); errRemove != nil {
			return fmt.Errorf("failed to remove existing symlink %v: %w", symlinkPath, errRemove)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cannot access existing %v: %w", symlinkPath, err)
	}

	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink %v -> %v: %w", symlinkPath, targetPath, err)
	}

	return nil
}
