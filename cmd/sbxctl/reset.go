package main

import (
	"fmt"
	"os"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset [name]",
	Short: "Restore a sandbox from its clean base image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		if err := sudo(); err != nil {
			return err
		}

		rootfsPath := sandbox.RootfsPath(rootDir, name)
		bakPath := rootfsPath + ".bak"

		// Remove any existing backup
		os.RemoveAll(bakPath)

		// Move existing rootfs to backup if it exists
		if err := os.Rename(rootfsPath, bakPath); err != nil {
			return fmt.Errorf("failed to backup rootfs: %v", err)
		}

		tarball := sandbox.BaseTarballPath(rootDir, name)
		if err := sandbox.Extract(tarball, rootfsPath); err != nil {
			// Restore backup on failure
			os.RemoveAll(rootfsPath)
			if renameErr := os.Rename(bakPath, rootfsPath); renameErr != nil {
				return fmt.Errorf("failed to extract base image: %v; also failed to restore backup %v to %v: %v", err, bakPath, rootfsPath, renameErr)
			}
			return fmt.Errorf("failed to extract base image: %v", err)
		}

		// Cleanup backup on success
		if err := os.RemoveAll(bakPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete rootfs backup %v: %v", bakPath, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
