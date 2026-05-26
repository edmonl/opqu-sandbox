package main

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

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

		return replaceRootfs(filepath.Join(sbxDir, "rootfs"), name, snapshotPath)
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
// It extracts the archive first, then swaps it with the current rootfs
// so that failed extraction won't touch rootfs.
func replaceRootfs(rootfsDir, name, archivePath string) error {
	rootfsPath := filepath.Join(rootfsDir, name)
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
			return fmt.Errorf("failed to delete existing backup %v: %w", bakPath, err)
		}
	}

	tmpPath, err := temporaryRestorePath(rootfsDir, name)
	if err != nil {
		return err
	}
	if err := sandbox.Extract(archivePath, tmpPath); err != nil {
		if e := os.RemoveAll(tmpPath); e != nil {
			util.Warn("failed to delete unsuccessful extraction result %v: %v", tmpPath, e)
		}
		return fmt.Errorf("failed to extract %v: %w", archivePath, err)
	}

	restoreSuccess := false
	defer func() {
		if !restoreSuccess {
			if err := os.RemoveAll(tmpPath); err != nil {
				util.Warn("failed to delete temporary restore rootfs %v: %v", tmpPath, err)
			}
		}
	}()

	if err := os.Rename(rootfsPath, bakPath); err != nil {
		return fmt.Errorf("failed to backup rootfs %v: %w", rootfsPath, err)
	}

	if err := os.Rename(tmpPath, rootfsPath); err != nil {
		if e := os.Rename(bakPath, rootfsPath); e != nil {
			util.Warn("failed to restore backup %v to %v: %v", bakPath, rootfsPath, e)
		}
		return fmt.Errorf("failed to replace rootfs %v with %v: %w", rootfsPath, tmpPath, err)
	}
	restoreSuccess = true

	if err := os.RemoveAll(bakPath); err != nil {
		util.Warn("failed to delete backup %v: %v", bakPath, err)
	}

	return nil
}

func temporaryRestorePath(rootfsDir, name string) (string, error) {
	for range 16 {
		tmpPath := filepath.Join(rootfsDir, fmt.Sprintf(
			"%v.restore.%v.%v.tmp",
			name,
			time.Now().Format("2006-01-02T15-04-05-000000"),
			rand.IntN(900000)+100000,
		))

		if _, err := os.Lstat(tmpPath); err == nil {
			continue
		} else if errors.Is(err, fs.ErrNotExist) {
			return tmpPath, nil
		} else {
			return "", fmt.Errorf("failed to access temporary restore path %v: %w", tmpPath, err)
		}
	}

	return "", fmt.Errorf("failed to find available temporary restore path for %v", filepath.Join(rootfsDir, name))
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
