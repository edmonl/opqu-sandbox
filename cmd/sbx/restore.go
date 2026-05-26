package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [name] [snapshot name]",
	Short: "Replace a sandbox rootfs from a named snapshot archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		snapshotName := args[1]
		if err := sandbox.ValidateSnapshotName(snapshotName); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		snapshotPath, err := resolveSnapshotPath(sbxDir, name, snapshotName)
		if err != nil {
			return err
		}

		return replaceRootfs(filepath.Join(sbxDir, "rootfs", name), snapshotPath)
	},
}

func resolveSnapshotPath(sbxDir, name, snapshotName string) (string, error) {
	snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
	matches, err := filepath.Glob(filepath.Join(snapshotsDir, snapshotName+".*.tar.zst"))
	if err != nil {
		return "", fmt.Errorf("failed to list snapshots: %w", err)
	}

	var snapshotPaths []string
	for _, match := range matches {
		info, err := os.Lstat(match)
		if err != nil {
			return "", fmt.Errorf("failed to access snapshot archive %v: %w", match, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("snapshot archive %v is not a regular file", match)
		}
		snapshotPaths = append(snapshotPaths, match)
	}

	switch len(snapshotPaths) {
	case 0:
		return "", fmt.Errorf("snapshot %v not found in %v", snapshotName, snapshotsDir)
	case 1:
		return snapshotPaths[0], nil
	default:
		return "", fmt.Errorf("multiple snapshots named %v found in %v; please clean up archives manually", snapshotName, snapshotsDir)
	}
}

// replaceRootfs replaces an existing root filesystem by extracting a new archive.
// It creates a backup of the current rootfs, extracts the archive, and restores the backup if extraction fails.
func replaceRootfs(rootfsPath, archivePath string) error {
	if rootfsExists, err := sandbox.RequireInactiveRootfs(rootfsPath); err != nil {
		return err
	} else if !rootfsExists {
		return fmt.Errorf("%v is missing", rootfsPath)
	}

	bakPath := rootfsPath + ".bak"
	if bakExists, err := sandbox.RequireInactiveRootfs(bakPath); err != nil {
		return err
	} else if bakExists {
		if err := os.RemoveAll(bakPath); err != nil {
			return fmt.Errorf("failed to delete exiting backup %v: %w", bakPath, err)
		}
	}

	if err := os.Rename(rootfsPath, bakPath); err != nil {
		return fmt.Errorf("failed to backup rootfs %v: %w", rootfsPath, err)
	}

	if err := sandbox.Extract(archivePath, rootfsPath); err != nil {
		if e := os.RemoveAll(rootfsPath); e != nil {
			util.Warn("failed to restore backup %v: failed to delete unsuccessful extraction result %v: %v", bakPath, rootfsPath, e)
		} else if e := os.Rename(bakPath, rootfsPath); e != nil {
			util.Warn("failed to restore backup %v to %v: %v", bakPath, rootfsPath, e)
		}

		return fmt.Errorf("failed to extract %v: %w", archivePath, err)
	}

	if err := os.RemoveAll(bakPath); err != nil {
		util.Warn("failed to delete backup %v: %v", bakPath, err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
