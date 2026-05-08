package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var snapshotNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [name] [snapshot name]",
	Short: "Save a sandbox rootfs to a tar.zst archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		snapshotName := args[1]
		if !snapshotNameRegex.MatchString(snapshotName) {
			return fmt.Errorf("snapshot name %q is invalid, must be alphanumeric, '_', and '-' only", snapshotName)
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
		if err := os.MkdirAll(snapshotsDir, 0o755); err != nil {
			return fmt.Errorf("failed to create snapshots directory: %w", err)
		}

		rootfsPath := filepath.Join(conf.ImagePath, name)
		if err := sandbox.CreateSnapshot(rootfsPath, snapshotsDir, snapshotName); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
