package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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

func RootfsPath(rootDir, name string) string {
	return filepath.Join(rootDir, "rootfs", name)
}

func BaseTarballPath(rootDir, name string) string {
	return filepath.Join(rootDir, "rootfs", fmt.Sprintf("%v.base.tar.zst", name))
}

func RemoveRootfs(rootDir, name string) error {
	return os.RemoveAll(RootfsPath(rootDir, name))
}

func MachineName(name string) string {
	return fmt.Sprintf("opqu-sbx-%v", name)
}

func BridgeName(zone string) string {
	return fmt.Sprintf("vz-%v", zone)
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

func ReplaceRootfs(rootDir, name, archivePath string) error {
	rootfsPath := RootfsPath(rootDir, name)
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
