// Package sandbox provides helpers for managing sandbox.
package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/klauspost/compress/zstd"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

func ValidateName(name string) error {
	if nameRegex.MatchString(name) {
		return nil
	}
	return fmt.Errorf("sandbox name %v is invalid, must be lowercase alphanumeric and hyphens only", name)
}

func MachineName(name string) string {
	return fmt.Sprintf("opqu-sbx-%v", name)
}

func RunCmd(cmd string, args ...string) error {
	execCmd := exec.Command(cmd, args...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func ReplaceRootfs(rootfsPath, archivePath string) error {
	bakPath := rootfsPath + ".bak"

	// Remove any existing backup
	os.RemoveAll(bakPath)

	// Move existing rootfs to backup if it exists
	if err := os.Rename(rootfsPath, bakPath); err != nil {
		return fmt.Errorf("failed to backup rootfs %v: %v", rootfsPath, err)
	}

	if err := Extract(archivePath, rootfsPath); err != nil {
		// Restore backup on failure
		os.RemoveAll(rootfsPath)
		if renameErr := os.Rename(bakPath, rootfsPath); renameErr != nil {
			return fmt.Errorf("failed to extract %v: %v; also failed to restore backup %v to %v: %v", archivePath, err, bakPath, rootfsPath, renameErr)
		}
		return fmt.Errorf("failed to extract %v: %v", archivePath, err)
	}

	// Cleanup backup on success
	if err := os.RemoveAll(bakPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete rootfs backup %v: %v\n", bakPath, err)
	}

	return nil
}

func IsRunning(name string) (bool, error) {
	cmd := exec.Command("machinectl", "show", name, "--property=State", "--value")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)) == "running", nil
	}

	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return false, nil
	}
	return false, fmt.Errorf("failed to get sandbox state with machinectl: %v", err)
}

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

// CreateSnapshot creates a zstd-compressed tarball of the rootfs and changes ownership to SUDO_USER if applicable.
func CreateSnapshot(rootfsPath, snapshotsDir, snapshotName string) error {
	pattern := filepath.Join(snapshotsDir, snapshotName+".*.tar.zst")
	oldSnapshots, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list old snapshots: %w", err)
	}

	if len(oldSnapshots) > 0 {
		input, err := util.Confirm(fmt.Sprintf("Snapshot %v already exists. Press <Enter> directly to overwrite, or Ctrl+C to cancel: ", snapshotName))
		if err != nil {
			return err
		}
		if input != "" {
			return fmt.Errorf("user cancelled overwriting snapshot %v", snapshotName)
		}
	}

	snapshotPath := filepath.Join(snapshotsDir, fmt.Sprintf("%v.%v.tar.zst", snapshotName, time.Now().Format("2006-01-02T15-04-05")))

	if err := Compress(rootfsPath, snapshotPath, zstd.SpeedDefault); err != nil {
		if errCleanup := os.RemoveAll(snapshotPath); errCleanup != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up %v: %v\n", snapshotPath, errCleanup)
		}
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if err := changeOwner(snapshotPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to change ownership of %v: %v\n", snapshotPath, err)
	}

	for _, old := range oldSnapshots {
		if old != snapshotPath {
			if err := os.RemoveAll(old); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove old snapshot %v: %v\n", old, err)
			}
		}
	}

	return nil
}

func changeOwner(path string) error {
	uid := -1
	gid := -1
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		var u *user.User
		var err error
		if u, err = user.Lookup(sudoUser); err != nil {
			return fmt.Errorf("failed to look up user %v: %w", sudoUser, err)
		}
		if uid, err = strconv.Atoi(u.Uid); err != nil {
			return fmt.Errorf("invalid user ID %v for %v: %w", u.Uid, sudoUser, err)
		}
		if gid, err = strconv.Atoi(u.Gid); err != nil {
			return fmt.Errorf("invalid group ID %v for %v: %w", u.Gid, sudoUser, err)
		}
	}

	if uid >= 0 || gid >= 0 {
		if err := os.Chown(path, uid, gid); err != nil {
			return err
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
