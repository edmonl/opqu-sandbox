package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [name] [snapshot path]",
	Short: "Replace a sandbox rootfs from a snapshot archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		if err := sudo(); err != nil {
			return err
		}

		snapshotPath := args[1]
		if info, err := os.Stat(snapshotPath); err == nil {
			if !info.Mode().IsRegular() {
				return errors.New("snapshot is not a regular file")
			}
		} else if errors.Is(err, fs.ErrNotExist) {
			return errors.New("snapshot does not exist")
		} else {
			return fmt.Errorf("failed to access snapshot: %v", err)
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
