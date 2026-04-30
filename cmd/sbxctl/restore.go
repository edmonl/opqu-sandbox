package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [name] [snapshot_path]",
	Short: "Replace a sandbox rootfs from a snapshot archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sudo(); err != nil {
			return err
		}
		name := args[0]
		snapshotPath := args[1]

		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if running {
			return fmt.Errorf("sandbox %v is running; stop it first", name)
		}

		info, err := os.Stat(snapshotPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("snapshot %v does not exist", snapshotPath)
			}
			return fmt.Errorf("failed to stat snapshot %v: %v", snapshotPath, err)
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot %v is not a regular file", snapshotPath)
		}

		// Check snapshot contents
		expectedRootfs := fmt.Sprintf("%v/", name)
		paths, err := sandbox.ListPaths(snapshotPath)
		if err != nil {
			return fmt.Errorf("failed to list snapshot contents: %v", err)
		}

		foundRootfs := false
		for _, p := range paths {
			if strings.TrimSpace(p) == expectedRootfs {
				foundRootfs = true
				break
			}
		}

		if !foundRootfs {
			return fmt.Errorf("snapshot %v does not contain top-level %v", snapshotPath, expectedRootfs)
		}

		rootfs := filepath.Join(rootDir, "rootfs", name)
		if err := os.RemoveAll(rootfs); err != nil {
			return fmt.Errorf("failed to remove rootfs for sandbox %v: %v", name, err)
		}

		if err := sandbox.Extract(snapshotPath, filepath.Join(rootDir, "rootfs")); err != nil {
			return fmt.Errorf("failed to extract snapshot: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
