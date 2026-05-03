package main

import (
	"fmt"
	"path/filepath"

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

		if err := sandbox.RemoveRootfs(rootDir, name); err != nil {
			return fmt.Errorf("failed to remove sandbox rootfs: %v", err)
		}

		tarball := sandbox.BaseTarballPath(rootDir, name)
		if err := sandbox.Extract(tarball, filepath.Join(rootDir, "rootfs")); err != nil {
			return fmt.Errorf("failed to extract base tarball: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
