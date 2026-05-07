// Package sandbox provides helpers for managing sandbox.
package sandbox

import (
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

func MachineName(name string) string {
	return fmt.Sprintf("opqu-sbx-%v", name)
}

func IsRunning(name string) (bool, error) {
	machine := MachineName(name)
	cmd := exec.Command("machinectl", "show", machine, "--property=State")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)) == "State=running", nil
	}

	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return false, nil
	}
	return false, fmt.Errorf("failed to get sandbox state with machinectl: %v", err)
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

// CreateSnapshot creates a zstd-compressed tarball of the rootfs and changes ownership to SUDO_USER if applicable.
func CreateSnapshot(rootfsPath, snapshotsDir, snapshotName string) error {
	snapshotPath := filepath.Join(snapshotsDir, fmt.Sprintf("%v.%v.tar.zst", snapshotName, time.Now().Format("2006-01-02T15-04-05")))
	if _, err := os.Stat(snapshotPath); err == nil {
		input, err := util.Confirm(fmt.Sprintf("Snapshot %v already exists. Press <Enter> directly to overwrite it, or Ctrl+C to cancel: ", snapshotPath))
		if err != nil {
			return err
		}
		if input != "" {
			return fmt.Errorf("user cancelled overwriting snapshot %v", snapshotPath)
		}
	}

	if err := Compress(rootfsPath, snapshotPath, zstd.SpeedDefault); err != nil {
		if errCleanup := os.RemoveAll(snapshotPath); errCleanup != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up %v: %v\n", snapshotPath, errCleanup)
		}
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if err := changeOwner(snapshotPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to change ownership of %v: %v\n", snapshotPath, err)
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
